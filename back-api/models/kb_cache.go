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
	TrigramSimilarity  *float64  `json:"trigram_similarity,omitempty"`
	VectorSimilarity   *float64  `json:"vector_similarity,omitempty"`
	CombinedSimilarity float64   `json:"combined_similarity,omitempty"`
	MatchMethod        string    `json:"match_method,omitempty"`
	CreatedAt          time.Time `json:"created_at"`
	ExpiresAt          time.Time `json:"expires_at"`
}
