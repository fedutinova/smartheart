package models

import (
	"time"

	"github.com/google/uuid"
)

// ECGFeedbackRating is the user's evaluation of the ML rhythm conclusion.
// Persisted as TEXT in rhythm_feedback.rating with a CHECK constraint —
// keep the constant set and the migration in sync.
type ECGFeedbackRating string

const (
	ECGFeedbackHelpful    ECGFeedbackRating = "helpful"
	ECGFeedbackInaccurate ECGFeedbackRating = "inaccurate"
	ECGFeedbackUnclear    ECGFeedbackRating = "unclear"
)

// IsValid reports whether r is one of the three supported ratings.
func (r ECGFeedbackRating) IsValid() bool {
	switch r {
	case ECGFeedbackHelpful, ECGFeedbackInaccurate, ECGFeedbackUnclear:
		return true
	}
	return false
}

// ECGFeedback is the persisted feedback row for a single rhythm conclusion.
// Comment is optional — typically filled when Rating is "inaccurate" so users
// can describe what was wrong; that text is a retraining signal.
type ECGFeedback struct {
	RequestID uuid.UUID         `json:"request_id"`
	UserID    uuid.UUID         `json:"user_id"`
	Rating    ECGFeedbackRating `json:"rating"`
	Comment   string            `json:"comment,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
}
