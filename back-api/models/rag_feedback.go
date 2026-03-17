package models

import (
	"time"

	"github.com/google/uuid"
)

// RAGFeedback stores user feedback on RAG knowledge base answers.
type RAGFeedback struct {
	ID        uuid.UUID `json:"id"`
	UserID    uuid.UUID `json:"user_id"`
	Question  string    `json:"question"`
	Answer    string    `json:"answer"`
	Rating    int       `json:"rating"` // -1 = bad, 1 = good
	CreatedAt time.Time `json:"created_at"`
}
