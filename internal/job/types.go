package job

import (
	"time"

	uuid "github.com/google/uuid"
)

type Type string

const (
	TypeEKGAnalyze Type = "ekg_analyze"
	TypeGPTProcess Type = "gpt_process"
)

type Status string

const (
	StatusQueued    Status = "queued"
	StatusRunning   Status = "running"
	StatusSucceeded Status = "succeeded"
	StatusFailed    Status = "failed"
)

type Job struct {
	ID       uuid.UUID  `json:"id"`
	Type     Type       `json:"type"`
	Payload  []byte     `json:"payload"` // TODO change raw slice of bytes to type
	Status   Status     `json:"status"`
	Error    string     `json:"error,omitempty"`
	Enqueued time.Time  `json:"enqueued_at"`
	Started  *time.Time `json:"started_at,omitempty"`
	Finished *time.Time `json:"finished_at,omitempty"`
}
