package job

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Handler processes a single job.
type Handler func(ctx context.Context, j *Job) error

// Registry maps job types to their handlers, enabling Open/Closed extension
// without modifying the dispatch logic. Safe for concurrent use.
type Registry struct {
	mu       sync.RWMutex
	handlers map[Type]Handler
}

// NewRegistry creates an empty job registry.
func NewRegistry() *Registry {
	return &Registry{handlers: make(map[Type]Handler)}
}

// Register adds a handler for the given job type.
func (r *Registry) Register(t Type, h Handler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers[t] = h
}

// Dispatch routes a job to the registered handler.
func (r *Registry) Dispatch(ctx context.Context, j *Job) error {
	r.mu.RLock()
	h, ok := r.handlers[j.Type]
	r.mu.RUnlock()
	if !ok {
		return fmt.Errorf("unknown job type: %s", j.Type)
	}
	return h(ctx, j)
}

// Queue is the interface for job queue implementations.
type Queue interface {
	Enqueue(ctx context.Context, j *Job) (uuid.UUID, error)
	Status(ctx context.Context, id uuid.UUID) (*Job, bool)
	StartConsumers(ctx context.Context, n int, handler Handler)
	Len() int
	Close() error
}

type Type string

const (
	TypeECGAnalyze Type = "ekg_analyze"
	TypeGPTProcess Type = "gpt_process"
)

// ECGJobPayload represents the payload for EKG analysis jobs.
// Either ImageTempURL (URL mode) or ImageFileKey (file upload mode) is set.
type ECGJobPayload struct {
	ImageTempURL  string    `json:"image_temp_url,omitempty"`
	ImageFileKey  string    `json:"image_file_key,omitempty"`
	Notes         string    `json:"notes,omitempty"`
	UserID        uuid.UUID `json:"user_id"`
	RequestID     uuid.UUID `json:"request_id"`
	Age           *int      `json:"age,omitempty"`
	Sex           string    `json:"sex,omitempty"`
	PaperSpeedMMS float64   `json:"paper_speed_mms,omitempty"`
	MmPerMvLimb   float64   `json:"mm_per_mv_limb,omitempty"`
	MmPerMvChest  float64   `json:"mm_per_mv_chest,omitempty"`
}

type Status string

const (
	StatusQueued    Status = "queued"
	StatusRunning   Status = "running"
	StatusSucceeded Status = "succeeded"
	StatusFailed    Status = "failed"
)

type Job struct {
	mu       sync.Mutex
	ID       uuid.UUID  `json:"id"`
	Type     Type       `json:"type"`
	Payload  []byte     `json:"payload"`
	Status   Status     `json:"status"`
	Error    string     `json:"error,omitempty"`
	Enqueued time.Time  `json:"enqueued_at"`
	Started  *time.Time `json:"started_at,omitempty"`
	Finished *time.Time `json:"finished_at,omitempty"`
}

// snapshot returns a copy of the job without the mutex, safe to return to callers.
func (j *Job) snapshot() *Job {
	j.mu.Lock()
	defer j.mu.Unlock()
	cp := &Job{
		ID:       j.ID,
		Type:     j.Type,
		Payload:  j.Payload,
		Status:   j.Status,
		Error:    j.Error,
		Enqueued: j.Enqueued,
		Started:  j.Started,
		Finished: j.Finished,
	}
	return cp
}

// SetRunning marks the job as running (goroutine-safe).
func (j *Job) SetRunning() {
	j.mu.Lock()
	defer j.mu.Unlock()
	now := time.Now()
	j.Status = StatusRunning
	j.Started = &now
}

// SetFinished marks the job as succeeded or failed (goroutine-safe).
func (j *Job) SetFinished(err error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	now := time.Now()
	j.Finished = &now
	if err != nil {
		j.Status = StatusFailed
		j.Error = err.Error()
	} else {
		j.Status = StatusSucceeded
	}
}
