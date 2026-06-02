package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/fedutinova/smartheart/back-api/apperr"
	"github.com/fedutinova/smartheart/back-api/models"
)

// UpsertECGFeedback inserts a new rhythm-conclusion feedback row, or updates
// the rating + optional comment if the user already voted on this request.
// An empty comment is stored as SQL NULL so absence is unambiguous.
func (r *Repository) UpsertECGFeedback(ctx context.Context, f *models.ECGFeedback) error {
	if !f.Rating.IsValid() {
		return fmt.Errorf("upsert ecg feedback: invalid rating %q: %w", f.Rating, apperr.ErrValidation)
	}
	var commentArg any
	if f.Comment != "" {
		commentArg = f.Comment
	}
	_, err := r.querier.Exec(ctx, `
		INSERT INTO rhythm_feedback (request_id, user_id, rating, comment)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (request_id)
		DO UPDATE SET rating = EXCLUDED.rating,
		              comment = EXCLUDED.comment,
		              updated_at = now()
	`, f.RequestID, f.UserID, string(f.Rating), commentArg)
	if err != nil {
		return fmt.Errorf("upsert ecg feedback: %w", err)
	}
	return nil
}

// GetECGFeedback returns the feedback row for a request. Returns (nil, nil)
// when the user has not voted yet — handlers translate that to HTTP 204.
func (r *Repository) GetECGFeedback(ctx context.Context, requestID uuid.UUID) (*models.ECGFeedback, error) {
	var f models.ECGFeedback
	var ratingStr string
	var comment *string
	err := r.querier.QueryRow(ctx, `
		SELECT request_id, user_id, rating, comment, created_at, updated_at
		FROM rhythm_feedback
		WHERE request_id = $1
	`, requestID).Scan(&f.RequestID, &f.UserID, &ratingStr, &comment, &f.CreatedAt, &f.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil //nolint:nilnil // absence is meaningful for callers
		}
		return nil, fmt.Errorf("get ecg feedback: %w", err)
	}
	f.Rating = models.ECGFeedbackRating(ratingStr)
	if comment != nil {
		f.Comment = *comment
	}
	return &f, nil
}
