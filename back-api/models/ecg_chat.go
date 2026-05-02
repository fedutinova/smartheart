package models

import (
	"time"

	"github.com/google/uuid"
)

// ECGChatRole is the message author within an ECG-contextual chat.
type ECGChatRole string

const (
	ECGChatRoleUser      ECGChatRole = "user"
	ECGChatRoleAssistant ECGChatRole = "assistant"
)

// ECGChatCitation is a single source reference attached to an assistant reply.
type ECGChatCitation struct {
	Title   string `json:"title"`
	Source  string `json:"source"`
	Excerpt string `json:"excerpt"`
}

// ECGChatMessage is one message in the contextual chat for a specific ECG.
type ECGChatMessage struct {
	ID        uuid.UUID         `json:"id"`
	RequestID uuid.UUID         `json:"request_id"`
	UserID    uuid.UUID         `json:"user_id"`
	Role      ECGChatRole       `json:"role"`
	Content   string            `json:"content"`
	Citations []ECGChatCitation `json:"citations,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
}
