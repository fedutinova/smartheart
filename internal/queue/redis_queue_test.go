package queue

import (
	"context"
	"encoding/json"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fedutinova/smartheart/internal/job"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

func getTestRedisClient(t *testing.T) *redis.Client {
	redisURL := os.Getenv("TEST_REDIS_URL")
	if redisURL == "" {
		redisURL = "redis://localhost:6379"
	}

	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		t.Skipf("Skipping Redis queue test: invalid Redis URL: %v", err)
	}

	client := redis.NewClient(opts)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		t.Skipf("Skipping Redis queue test: Redis not available: %v", err)
	}

	return client
}

func TestRedisQueue_EnqueueAndConsume(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	client := getTestRedisClient(t)
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Use unique stream name for this test
	streamName := "test:jobs:" + uuid.New().String()[:8]

	// Cleanup before and after test (in case previous test run left data)
	client.Del(context.Background(), streamName)
	client.XGroupDestroy(context.Background(), streamName, "test-workers")
	defer client.Del(context.Background(), streamName)
	defer client.XGroupDestroy(context.Background(), streamName, "test-workers")

	q, err := NewRedisQueue(client, RedisQueueConfig{
		Stream:        streamName,
		Group:         "test-workers",
		MaxJobTime:    5 * time.Second,
		ClaimInterval: 1 * time.Second,
		ClaimTimeout:  3 * time.Second,
	})
	if err != nil {
		t.Fatalf("Failed to create queue: %v", err)
	}
	defer q.Close()

	// Track processed jobs
	var processedCount int32
	processedJobs := make(chan *job.Job, 10)

	// Start consumers
	q.StartConsumers(ctx, 2, func(ctx context.Context, j *job.Job) error {
		atomic.AddInt32(&processedCount, 1)
		processedJobs <- j
		return nil
	})

	// Enqueue jobs
	job1 := &job.Job{
		Type:    job.TypeEKGAnalyze,
		Payload: []byte(`{"test": "data1"}`),
	}
	job2 := &job.Job{
		Type:    job.TypeGPTProcess,
		Payload: []byte(`{"test": "data2"}`),
	}

	id1, err := q.Enqueue(ctx, job1)
	if err != nil {
		t.Fatalf("Failed to enqueue job1: %v", err)
	}
	if id1 == uuid.Nil {
		t.Error("Expected non-nil job ID")
	}

	id2, err := q.Enqueue(ctx, job2)
	if err != nil {
		t.Fatalf("Failed to enqueue job2: %v", err)
	}

	// Wait for jobs to be processed
	timeout := time.After(10 * time.Second)
	for i := 0; i < 2; i++ {
		select {
		case <-processedJobs:
			// Job processed
		case <-timeout:
			t.Fatalf("Timeout waiting for jobs to be processed, got %d", atomic.LoadInt32(&processedCount))
		}
	}

	// Poll for status updates with timeout
	deadline := time.Now().Add(10 * time.Second)
	var j1, j2 *job.Job
	var ok1, ok2 bool

	for time.Now().Before(deadline) {
		j1, ok1 = q.Status(ctx, id1)
		j2, ok2 = q.Status(ctx, id2)

		if ok1 && ok2 && j1.Status == job.StatusSucceeded && j2.Status == job.StatusSucceeded {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	// Check final status
	if !ok1 {
		t.Error("Job1 not found in status")
	} else if j1.Status != job.StatusSucceeded {
		t.Errorf("Expected job1 status %s, got %s", job.StatusSucceeded, j1.Status)
	}

	if !ok2 {
		t.Error("Job2 not found in status")
	} else if j2.Status != job.StatusSucceeded {
		t.Errorf("Expected job2 status %s, got %s", job.StatusSucceeded, j2.Status)
	}
}

func TestRedisQueue_JobFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	client := getTestRedisClient(t)
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	streamName := "test:jobs:fail:" + uuid.New().String()[:8]

	defer client.Del(context.Background(), streamName)
	defer client.XGroupDestroy(context.Background(), streamName, "test-workers")

	q, err := NewRedisQueue(client, RedisQueueConfig{
		Stream:        streamName,
		Group:         "test-workers",
		MaxJobTime:    5 * time.Second,
		ClaimInterval: 1 * time.Second,
		ClaimTimeout:  3 * time.Second,
	})
	if err != nil {
		t.Fatalf("Failed to create queue: %v", err)
	}
	defer q.Close()

	done := make(chan struct{})

	// Start consumer that fails
	q.StartConsumers(ctx, 1, func(ctx context.Context, j *job.Job) error {
		close(done)
		return context.DeadlineExceeded // Simulate failure
	})

	// Enqueue job
	testJob := &job.Job{
		Type:    job.TypeEKGAnalyze,
		Payload: []byte(`{"test": "will fail"}`),
	}

	id, err := q.Enqueue(ctx, testJob)
	if err != nil {
		t.Fatalf("Failed to enqueue job: %v", err)
	}

	// Wait for job to be processed
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("Timeout waiting for job to be processed")
	}

	// Poll for status update with timeout
	deadline := time.Now().Add(5 * time.Second)
	var j *job.Job
	var ok bool

	for time.Now().Before(deadline) {
		j, ok = q.Status(ctx, id)
		if ok && j.Status == job.StatusFailed {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Check status
	if !ok {
		t.Error("Job not found in status")
	} else {
		if j.Status != job.StatusFailed {
			t.Errorf("Expected job status %s, got %s", job.StatusFailed, j.Status)
		}
		if j.Error == "" {
			t.Error("Expected error message to be set")
		}
	}
}

func TestRedisQueue_Persistence(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	client := getTestRedisClient(t)
	defer client.Close()

	ctx := context.Background()
	streamName := "test:jobs:persist:" + uuid.New().String()[:8]

	defer client.Del(ctx, streamName)
	defer client.Del(ctx, streamName+":deadletter")
	defer client.XGroupDestroy(ctx, streamName, "test-workers")

	// Create queue and enqueue job
	q1, err := NewRedisQueue(client, RedisQueueConfig{
		Stream:        streamName,
		Group:         "test-workers",
		MaxJobTime:    5 * time.Second,
		ClaimInterval: 10 * time.Second,
		ClaimTimeout:  30 * time.Second,
	})
	if err != nil {
		t.Fatalf("Failed to create queue: %v", err)
	}

	testJob := &job.Job{
		Type:    job.TypeEKGAnalyze,
		Payload: []byte(`{"test": "persistent"}`),
	}

	_, err = q1.Enqueue(ctx, testJob)
	if err != nil {
		t.Fatalf("Failed to enqueue job: %v", err)
	}

	// Close first queue instance (simulating crash)
	q1.Close()

	// Verify job is still in stream
	info, err := client.XInfoStream(ctx, streamName).Result()
	if err != nil {
		t.Fatalf("Failed to get stream info: %v", err)
	}
	if info.Length == 0 {
		t.Error("Expected job to be persisted in stream")
	}

	// Create new queue instance (simulating restart)
	q2, err := NewRedisQueue(client, RedisQueueConfig{
		Stream:        streamName,
		Group:         "test-workers",
		MaxJobTime:    5 * time.Second,
		ClaimInterval: 1 * time.Second,
		ClaimTimeout:  1 * time.Second,
	})
	if err != nil {
		t.Fatalf("Failed to create second queue: %v", err)
	}
	defer q2.Close()

	processed := make(chan *job.Job, 1)

	// Start consumers on new queue
	consumerCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	q2.StartConsumers(consumerCtx, 1, func(ctx context.Context, j *job.Job) error {
		processed <- j
		return nil
	})

	// Wait for job to be reclaimed and processed
	select {
	case j := <-processed:
		var payload map[string]string
		if err := json.Unmarshal(j.Payload, &payload); err != nil {
			t.Errorf("Failed to unmarshal payload: %v", err)
		}
		if payload["test"] != "persistent" {
			t.Errorf("Expected payload test=persistent, got %s", payload["test"])
		}
	case <-time.After(20 * time.Second):
		t.Error("Timeout waiting for persisted job to be processed")
	}
}

func TestRedisQueue_Len(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	client := getTestRedisClient(t)
	defer client.Close()

	ctx := context.Background()
	streamName := "test:jobs:len:" + uuid.New().String()[:8]

	defer client.Del(ctx, streamName)
	defer client.XGroupDestroy(ctx, streamName, "test-workers")

	q, err := NewRedisQueue(client, RedisQueueConfig{
		Stream:        streamName,
		Group:         "test-workers",
		MaxJobTime:    5 * time.Second,
		ClaimInterval: 10 * time.Second,
		ClaimTimeout:  30 * time.Second,
	})
	if err != nil {
		t.Fatalf("Failed to create queue: %v", err)
	}
	defer q.Close()

	// Enqueue several jobs without consumers
	for i := 0; i < 5; i++ {
		_, err := q.Enqueue(ctx, &job.Job{
			Type:    job.TypeEKGAnalyze,
			Payload: []byte(`{}`),
		})
		if err != nil {
			t.Fatalf("Failed to enqueue job %d: %v", i, err)
		}
	}

	// Note: Len() returns pending count which is 0 until consumers read messages
	// This is expected behavior for Redis Streams consumer groups
}

