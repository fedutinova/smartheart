package queue

import (
	"context"
	"log/slog"
	"time"

	"github.com/fedutinova/smartheart/internal/job"
	"github.com/google/uuid"
)

type memQueue struct {
	buf     chan *job.Job
	maxWait time.Duration
	cache   *job.Cache
}

func NewMemoryQueue(buffer int, maxJobDuration time.Duration) job.Queue {
	return &memQueue{
		buf:     make(chan *job.Job, buffer),
		maxWait: maxJobDuration,
		cache:   job.NewCache(buffer).WithMaxSize(buffer * 10),
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
		q.cache.Put(j)
		return j.ID, nil
	case <-ctx.Done():
		return uuid.Nil, ctx.Err()
	}
}

func (q *memQueue) Status(ctx context.Context, id uuid.UUID) (*job.Job, bool) {
	return q.cache.Get(id)
}

func (q *memQueue) StartConsumers(ctx context.Context, n int, handler job.Handler) {
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

	// Periodically clean up finished jobs older than cleanupMaxAge
	go func() {
		ticker := time.NewTicker(cleanupInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				q.cache.CleanupOlderThan(cleanupMaxAge)
			}
		}
	}()
}

func (q *memQueue) Len() int {
	return len(q.buf)
}

func (q *memQueue) Close() error {
	// In-memory queue doesn't need cleanup
	return nil
}
