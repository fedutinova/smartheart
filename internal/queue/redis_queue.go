package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/fedutinova/smartheart/internal/job"
	"github.com/fedutinova/smartheart/internal/memq"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// RedisQueue implements JobQueue using Redis Streams
type RedisQueue struct {
	client        *redis.Client
	stream        string
	group         string
	maxWait       time.Duration
	claimInterval time.Duration // how often to check for stuck jobs
	claimTimeout  time.Duration // consider job stuck after this duration

	mu      sync.RWMutex
	jobs    map[uuid.UUID]*job.Job // local cache for status lookups
	wg      sync.WaitGroup
	closing chan struct{}
}

// RedisQueueConfig holds configuration for RedisQueue
type RedisQueueConfig struct {
	Stream        string
	Group         string
	MaxJobTime    time.Duration
	ClaimInterval time.Duration
	ClaimTimeout  time.Duration
}

// DefaultConfig returns default queue configuration
func DefaultConfig() RedisQueueConfig {
	return RedisQueueConfig{
		Stream:        "smartheart:jobs",
		Group:         "workers",
		MaxJobTime:    30 * time.Second,
		ClaimInterval: 10 * time.Second,
		ClaimTimeout:  60 * time.Second,
	}
}

// NewRedisQueue creates a new Redis Streams based queue
func NewRedisQueue(client *redis.Client, cfg RedisQueueConfig) (*RedisQueue, error) {
	q := &RedisQueue{
		client:        client,
		stream:        cfg.Stream,
		group:         cfg.Group,
		maxWait:       cfg.MaxJobTime,
		claimInterval: cfg.ClaimInterval,
		claimTimeout:  cfg.ClaimTimeout,
		jobs:          make(map[uuid.UUID]*job.Job),
		closing:       make(chan struct{}),
	}

	// Create consumer group if it doesn't exist
	ctx := context.Background()
	err := q.client.XGroupCreateMkStream(ctx, q.stream, q.group, "0").Err()
	if err != nil && !isGroupExistsError(err) {
		return nil, fmt.Errorf("failed to create consumer group: %w", err)
	}

	slog.Info("Redis queue initialized",
		"stream", q.stream,
		"group", q.group,
		"max_job_time", q.maxWait,
		"claim_timeout", q.claimTimeout)

	return q, nil
}

// Enqueue adds a job to the queue
func (q *RedisQueue) Enqueue(ctx context.Context, j *job.Job) (uuid.UUID, error) {
	if j.ID == uuid.Nil {
		j.ID = uuid.New()
	}
	j.Status = job.StatusQueued
	j.Enqueued = time.Now()

	// Serialize job
	data, err := json.Marshal(j)
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to marshal job: %w", err)
	}

	// Add to stream
	_, err = q.client.XAdd(ctx, &redis.XAddArgs{
		Stream: q.stream,
		Values: map[string]any{
			"id":   j.ID.String(),
			"data": string(data),
		},
	}).Result()
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to add job to stream: %w", err)
	}

	// Cache locally for status lookups
	q.mu.Lock()
	q.jobs[j.ID] = j
	q.mu.Unlock()

	slog.Debug("Job enqueued", "job_id", j.ID, "type", j.Type)
	return j.ID, nil
}

// Status returns the current status of a job
func (q *RedisQueue) Status(ctx context.Context, id uuid.UUID) (*job.Job, bool) {
	q.mu.RLock()
	defer q.mu.RUnlock()
	j, ok := q.jobs[id]
	return j, ok
}

// Len returns approximate number of pending jobs
func (q *RedisQueue) Len() int {
	ctx := context.Background()
	info, err := q.client.XInfoGroups(ctx, q.stream).Result()
	if err != nil {
		return 0
	}
	for _, g := range info {
		if g.Name == q.group {
			return int(g.Pending)
		}
	}
	return 0
}

// StartConsumers starts n consumer goroutines
func (q *RedisQueue) StartConsumers(ctx context.Context, n int, handler memq.JobHandler) {
	// Start consumers
	for i := 0; i < n; i++ {
		q.wg.Add(1)
		go q.consumer(ctx, i+1, handler)
	}

	// Start claimer for stuck jobs
	q.wg.Add(1)
	go q.claimer(ctx, handler)

	slog.Info("Started queue consumers", "count", n)
}

// consumer processes jobs from the stream
func (q *RedisQueue) consumer(ctx context.Context, workerID int, handler memq.JobHandler) {
	defer q.wg.Done()
	consumerName := fmt.Sprintf("worker-%d", workerID)

	for {
		select {
		case <-ctx.Done():
			slog.Info("Consumer shutting down", "worker", workerID)
			return
		case <-q.closing:
			slog.Info("Consumer received close signal", "worker", workerID)
			return
		default:
		}

		// Read new messages (blocking with timeout)
		streams, err := q.client.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    q.group,
			Consumer: consumerName,
			Streams:  []string{q.stream, ">"},
			Count:    1,
			Block:    5 * time.Second,
		}).Result()

		if err != nil {
			if errors.Is(err, redis.Nil) || errors.Is(err, context.Canceled) {
				continue
			}
			slog.Error("Failed to read from stream", "error", err, "worker", workerID)
			time.Sleep(time.Second) // backoff on error
			continue
		}

		for _, stream := range streams {
			for _, msg := range stream.Messages {
				q.processMessage(ctx, msg, handler, workerID)
			}
		}
	}
}

// claimer reclaims stuck jobs from dead consumers
func (q *RedisQueue) claimer(ctx context.Context, handler memq.JobHandler) {
	defer q.wg.Done()
	ticker := time.NewTicker(q.claimInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-q.closing:
			return
		case <-ticker.C:
			q.claimStuckJobs(ctx, handler)
		}
	}
}

// claimStuckJobs finds and reclaims jobs that have been pending too long
func (q *RedisQueue) claimStuckJobs(ctx context.Context, handler memq.JobHandler) {
	// Get pending entries that are older than claimTimeout
	pending, err := q.client.XPendingExt(ctx, &redis.XPendingExtArgs{
		Stream: q.stream,
		Group:  q.group,
		Start:  "-",
		End:    "+",
		Count:  100,
	}).Result()

	if err != nil {
		if !errors.Is(err, redis.Nil) {
			slog.Error("Failed to get pending entries", "error", err)
		}
		return
	}

	for _, p := range pending {
		if p.Idle < q.claimTimeout {
			continue
		}

		// Claim the message
		msgs, err := q.client.XClaim(ctx, &redis.XClaimArgs{
			Stream:   q.stream,
			Group:    q.group,
			Consumer: "claimer",
			MinIdle:  q.claimTimeout,
			Messages: []string{p.ID},
		}).Result()

		if err != nil {
			slog.Error("Failed to claim stuck job", "message_id", p.ID, "error", err)
			continue
		}

		for _, msg := range msgs {
			slog.Warn("Reclaimed stuck job",
				"message_id", msg.ID,
				"idle_time", p.Idle,
				"retry_count", p.RetryCount)

			// Check retry count - if too many retries, move to dead letter
			if p.RetryCount > 3 {
				q.moveToDeadLetter(ctx, msg, fmt.Sprintf("exceeded max retries: %d", p.RetryCount))
				continue
			}

			// Reprocess
			go q.processMessage(ctx, msg, handler, 0)
		}
	}
}

// processMessage handles a single message from the stream
func (q *RedisQueue) processMessage(ctx context.Context, msg redis.XMessage, handler memq.JobHandler, workerID int) {
	// Parse job data
	data, ok := msg.Values["data"].(string)
	if !ok {
		slog.Error("Invalid message format", "message_id", msg.ID)
		q.ackMessage(ctx, msg.ID)
		return
	}

	var j job.Job
	if err := json.Unmarshal([]byte(data), &j); err != nil {
		slog.Error("Failed to unmarshal job", "message_id", msg.ID, "error", err)
		q.ackMessage(ctx, msg.ID)
		return
	}

	// Update job status
	now := time.Now()
	j.Status = job.StatusRunning
	j.Started = &now

	// Update cache with running status
	q.mu.Lock()
	q.jobs[j.ID] = &j
	q.mu.Unlock()

	slog.Info("Processing job", "job_id", j.ID, "type", j.Type, "worker", workerID)

	// Execute with timeout
	runCtx, cancel := context.WithTimeout(ctx, q.maxWait)
	err := handler(runCtx, &j)
	cancel()

	// Update final status
	fin := time.Now()
	j.Finished = &fin

	if err != nil {
		j.Status = job.StatusFailed
		j.Error = err.Error()
		slog.Error("Job failed", "job_id", j.ID, "type", j.Type, "error", err, "worker", workerID)
	} else {
		j.Status = job.StatusSucceeded
		slog.Info("Job completed", "job_id", j.ID, "type", j.Type, "worker", workerID,
			"duration", fin.Sub(*j.Started))
	}

	// Update cache with final status
	q.mu.Lock()
	q.jobs[j.ID] = &j
	q.mu.Unlock()

	// Acknowledge the message
	q.ackMessage(ctx, msg.ID)
}

// moveToDeadLetter moves a failed job to the dead letter stream
func (q *RedisQueue) moveToDeadLetter(ctx context.Context, msg redis.XMessage, reason string) {
	dlStream := q.stream + ":deadletter"

	_, err := q.client.XAdd(ctx, &redis.XAddArgs{
		Stream: dlStream,
		Values: map[string]any{
			"original_id": msg.ID,
			"data":        msg.Values["data"],
			"reason":      reason,
			"moved_at":    time.Now().Format(time.RFC3339),
		},
	}).Result()

	if err != nil {
		slog.Error("Failed to move to dead letter", "message_id", msg.ID, "error", err)
	} else {
		slog.Warn("Moved job to dead letter queue", "message_id", msg.ID, "reason", reason)
	}

	// Ack the original message
	q.ackMessage(ctx, msg.ID)
}

// ackMessage acknowledges a message
func (q *RedisQueue) ackMessage(ctx context.Context, messageID string) {
	err := q.client.XAck(ctx, q.stream, q.group, messageID).Err()
	if err != nil {
		slog.Error("Failed to ack message", "message_id", messageID, "error", err)
	}
}

// Close gracefully shuts down the queue
func (q *RedisQueue) Close() error {
	close(q.closing)
	q.wg.Wait()
	slog.Info("Queue closed gracefully")
	return nil
}

// isGroupExistsError checks if error is "BUSYGROUP Consumer Group name already exists"
func isGroupExistsError(err error) bool {
	return err != nil && err.Error() == "BUSYGROUP Consumer Group name already exists"
}

// GetDeadLetterCount returns count of jobs in dead letter queue
func (q *RedisQueue) GetDeadLetterCount(ctx context.Context) (int64, error) {
	dlStream := q.stream + ":deadletter"
	return q.client.XLen(ctx, dlStream).Result()
}

// RetryDeadLetterJob moves a job from dead letter back to main queue
func (q *RedisQueue) RetryDeadLetterJob(ctx context.Context, messageID string) error {
	dlStream := q.stream + ":deadletter"

	// Read the message
	msgs, err := q.client.XRange(ctx, dlStream, messageID, messageID).Result()
	if err != nil {
		return fmt.Errorf("failed to read dead letter message: %w", err)
	}
	if len(msgs) == 0 {
		return fmt.Errorf("message not found: %s", messageID)
	}

	msg := msgs[0]
	data, ok := msg.Values["data"].(string)
	if !ok {
		return fmt.Errorf("invalid message format")
	}

	// Re-add to main stream
	_, err = q.client.XAdd(ctx, &redis.XAddArgs{
		Stream: q.stream,
		Values: map[string]any{
			"id":   msg.Values["original_id"],
			"data": data,
		},
	}).Result()
	if err != nil {
		return fmt.Errorf("failed to re-add job: %w", err)
	}

	// Delete from dead letter
	_, err = q.client.XDel(ctx, dlStream, messageID).Result()
	if err != nil {
		slog.Warn("Failed to delete from dead letter", "message_id", messageID, "error", err)
	}

	return nil
}
