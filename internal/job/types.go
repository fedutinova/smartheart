package job

import (
	"sync"
	"time"

	uuid "github.com/google/uuid"
)

type Type string

const (
	TypeEKGAnalyze Type = "ekg_analyze"
	TypeGPTProcess Type = "gpt_process"
)

// EKGJobPayload represents the payload for EKG analysis jobs.
type EKGJobPayload struct {
	ImageTempURL string `json:"image_temp_url"`
	Notes        string `json:"notes,omitempty"`
	UserID       string `json:"user_id,omitempty"`
	RequestID    string `json:"request_id,omitempty"`
}

type Status string

const (
	StatusQueued    Status = "queued"
	StatusRunning   Status = "running"
	StatusSucceeded Status = "succeeded"
	StatusFailed    Status = "failed"
)

type Job struct {
	mu       sync.Mutex `json:"-"`
	ID       uuid.UUID  `json:"id"`
	Type     Type       `json:"type"`
	Payload  []byte     `json:"payload"`
	Status   Status     `json:"status"`
	Error    string     `json:"error,omitempty"`
	Enqueued time.Time  `json:"enqueued_at"`
	Started  *time.Time `json:"started_at,omitempty"`
	Finished *time.Time `json:"finished_at,omitempty"`
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
