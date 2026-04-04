package models

import (
	"time"

	"github.com/google/uuid"
)

// KBCacheEntry represents a cached knowledge-base answer.
type KBCacheEntry struct {
	ID                 uuid.UUID `json:"id"`
	QuestionNormalized string    `json:"question_normalized"`
	Answer             string    `json:"answer"`
	SourceMeta         string    `json:"source_meta,omitempty"` // raw JSON
	HitCount           int       `json:"hit_count"`
	Similarity         float64   `json:"similarity,omitempty"` // filled by lookup query
	CreatedAt          time.Time `json:"created_at"`
	ExpiresAt          time.Time `json:"expires_at"`
}
