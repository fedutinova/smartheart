package memq

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/fedutinova/smartheart/internal/job"
)

func TestEnqueue_SetsDefaults(t *testing.T) {
	q := NewMemoryQueue(10, 50*time.Millisecond)
	j := &job.Job{Type: job.TypeEKGAnalyze, Payload: []byte(`{}`)}

	id, err := q.Enqueue(context.Background(), j)
	if err != nil {
		t.Fatalf("Enqueue error: %v", err)
	}
	if id.String() == "" {
		t.Fatalf("expected non-empty id")
	}
	if j.Status != job.StatusQueued {
		t.Fatalf("expected status queued, got %s", j.Status)
	}
	if j.Enqueued.IsZero() {
		t.Fatalf("expected enqueued timestamp to be set")
	}

	st, ok := q.Status(context.Background(), id)
	if !ok || st == nil {
		t.Fatalf("expected to find job by id")
	}
	if st.ID != j.ID {
		t.Fatalf("expected stored job id to match")
	}
}

func TestStartConsumers_SucceedsAndUpdatesStatus(t *testing.T) {
	q := NewMemoryQueue(10, 200*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{}, 1)
	q.StartConsumers(ctx, 1, func(ctx context.Context, j *job.Job) error {
		done <- struct{}{}
		return nil
	})

	j := &job.Job{Type: job.TypeEKGAnalyze, Payload: []byte(`{}`)}
	id, err := q.Enqueue(context.Background(), j)
	if err != nil {
		t.Fatalf("Enqueue error: %v", err)
	}

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatalf("timeout waiting for job handler")
	}

	st, ok := q.Status(context.Background(), id)
	if !ok {
		t.Fatalf("job not found")
	}
	if st.Status != job.StatusSucceeded {
		t.Fatalf("expected succeeded, got %s (err=%s)", st.Status, st.Error)
	}
	if st.Started == nil || st.Finished == nil {
		t.Fatalf("expected started/finished timestamps to be set")
	}
}

func TestStartConsumers_TimeoutMarksFailed(t *testing.T) {
	q := NewMemoryQueue(10, 20*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{}, 1)
	q.StartConsumers(ctx, 1, func(ctx context.Context, j *job.Job) error {
		<-ctx.Done()
		done <- struct{}{}
		return errors.New("handler timed out")
	})

	j := &job.Job{Type: job.TypeEKGAnalyze, Payload: []byte(`{}`)}
	id, err := q.Enqueue(context.Background(), j)
	if err != nil {
		t.Fatalf("Enqueue error: %v", err)
	}

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatalf("timeout waiting for job handler")
	}

	st, ok := q.Status(context.Background(), id)
	if !ok {
		t.Fatalf("job not found")
	}
	if st.Status != job.StatusFailed {
		t.Fatalf("expected failed, got %s", st.Status)
	}
	if st.Error == "" {
		t.Fatalf("expected error message to be set")
	}
}
