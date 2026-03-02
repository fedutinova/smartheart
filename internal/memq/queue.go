package memq

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/fedutinova/smartheart/internal/job"
	"github.com/google/uuid"
)

type JobHandler func(ctx context.Context, j *job.Job) error

type JobQueue interface {
	Enqueue(ctx context.Context, j *job.Job) (uuid.UUID, error)
	Status(ctx context.Context, id uuid.UUID) (*job.Job, bool)
	StartConsumers(ctx context.Context, n int, handler JobHandler)
	Len() int
	Close() error
}

type memQueue struct {
	buf     chan *job.Job
	maxWait time.Duration

	mu   sync.RWMutex
	jobs map[uuid.UUID]*job.Job
}

func NewMemoryQueue(buffer int, maxJobDuration time.Duration) JobQueue {
	return &memQueue{
		buf:     make(chan *job.Job, buffer),
		maxWait: maxJobDuration,
		jobs:    make(map[uuid.UUID]*job.Job, buffer),
	}
}

func (q *memQueue) Enqueue(ctx context.Context, j *job.Job) (uuid.UUID, error) {
	if j.ID == uuid.Nil {
		j.ID = uuid.New()
	}
	j.Status = job.StatusQueued
	j.Enqueued = time.Now()

	select {
	case q.buf <- j:
		q.mu.Lock()
		q.jobs[j.ID] = j
		q.mu.Unlock()
		return j.ID, nil
	case <-ctx.Done():
		return uuid.Nil, ctx.Err()
	}
}

func (q *memQueue) Status(ctx context.Context, id uuid.UUID) (*job.Job, bool) {
	q.mu.RLock()
	defer q.mu.RUnlock()
	j, ok := q.jobs[id]
	return j, ok
}

func (q *memQueue) StartConsumers(ctx context.Context, n int, handler JobHandler) {
	for i := 0; i < n; i++ {
		go func(workerID int) {
			for {
				select {
				case <-ctx.Done():
					return
				case j := <-q.buf:
					j.SetRunning()

					runCtx, cancel := context.WithTimeout(ctx, q.maxWait)
					err := handler(runCtx, j)
					cancel()

					j.SetFinished(err)

					if err != nil {
						slog.Error("job failed", "id", j.ID, "type", j.Type, "err", err, "worker", workerID)
					} else {
						slog.Info("job done", "id", j.ID, "type", j.Type, "worker", workerID)
					}
				}
			}
		}(i + 1)
	}

	// Periodically clean up finished jobs older than 30 minutes
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				q.cleanupFinishedJobs(30 * time.Minute)
			}
		}
	}()
}

// cleanupFinishedJobs removes completed/failed jobs older than maxAge from the cache
func (q *memQueue) cleanupFinishedJobs(maxAge time.Duration) {
	q.mu.Lock()
	defer q.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	removed := 0
	for id, j := range q.jobs {
		if j.Finished != nil && j.Finished.Before(cutoff) {
			delete(q.jobs, id)
			removed++
		}
	}
	if removed > 0 {
		slog.Debug("cleaned up finished jobs from cache", "removed", removed, "remaining", len(q.jobs))
	}
}

func (q *memQueue) Len() int {
	return len(q.buf)
}

func (q *memQueue) Close() error {
	// In-memory queue doesn't need cleanup
	return nil
}

func SimulateEKGHandler(delay time.Duration) JobHandler {
	return func(ctx context.Context, j *job.Job) error {
		select {
		case <-time.After(delay):
			return nil
		case <-ctx.Done():
			return errors.New("job timeout/canceled")
		}
	}
}
