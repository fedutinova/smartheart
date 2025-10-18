package gpt

import (
	"github.com/google/uuid"
)

type GPTJobPayload struct {
	RequestID uuid.UUID `json:"request_id"`
	TextQuery string    `json:"text_query,omitempty"`
	FileKeys  []string  `json:"file_keys"`
	UserID    string    `json:"user_id,omitempty"`
}