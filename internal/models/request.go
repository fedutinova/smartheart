package models

import (
	"time"

	"github.com/google/uuid"
)

// Request status constants
const (
	StatusPending    = "pending"
	StatusProcessing = "processing"
	StatusCompleted  = "completed"
	StatusFailed     = "failed"
)

// Request represents an EKG or GPT analysis request
type Request struct {
	ID        uuid.UUID `json:"id"`
	UserID    uuid.UUID `json:"user_id,omitempty"`
	TextQuery *string   `json:"text_query,omitempty"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Files     []File    `json:"files,omitempty"`
	Response  *Response `json:"response,omitempty"`
}

