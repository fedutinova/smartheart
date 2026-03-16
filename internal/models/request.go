package models

import (
	"time"

	"github.com/google/uuid"
)

// RequestStatus is a typed string for request lifecycle states,
// distinct from job.Status which tracks queue lifecycle.
type RequestStatus = string

// Request status constants.
const (
	StatusPending    RequestStatus = "pending"
	StatusProcessing RequestStatus = "processing"
	StatusCompleted  RequestStatus = "completed"
	StatusFailed     RequestStatus = "failed"
)

// Request represents an EKG or GPT analysis request
type Request struct {
	ID        uuid.UUID     `json:"id"`
	UserID    uuid.UUID     `json:"user_id,omitempty"`
	TextQuery *string       `json:"text_query,omitempty"`
	Status    RequestStatus `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Files     []File    `json:"files,omitempty"`
	Response  *Response `json:"response,omitempty"`
}

