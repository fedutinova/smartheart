package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// IncrementFreeAnalysesUsed atomically increments the user's lifetime free analyses
// counter and returns the new count.
func (r *Repository) IncrementFreeAnalysesUsed(ctx context.Context, userID uuid.UUID) (int, error) {
	var count int
	err := r.querier.QueryRow(ctx, `
		UPDATE users SET free_analyses_used = free_analyses_used + 1
		WHERE id = $1
		RETURNING free_analyses_used
	`, userID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("increment free analyses used: %w", err)
	}
	return count, nil
}

// DecrementFreeAnalysesUsed decreases the user's lifetime counter by 1 (floor at 0).
// Used to "refund" a count when an analysis fails.
func (r *Repository) DecrementFreeAnalysesUsed(ctx context.Context, userID uuid.UUID) error {
	_, err := r.querier.Exec(ctx, `
		UPDATE users SET free_analyses_used = GREATEST(free_analyses_used - 1, 0)
		WHERE id = $1
	`, userID)
	if err != nil {
		return fmt.Errorf("decrement free analyses used: %w", err)
	}
	return nil
}

// GetFreeAnalysesUsed returns the user's lifetime free analyses count.
func (r *Repository) GetFreeAnalysesUsed(ctx context.Context, userID uuid.UUID) (int, error) {
	var count int
	err := r.querier.QueryRow(ctx, `
		SELECT free_analyses_used FROM users WHERE id = $1
	`, userID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("get free analyses used: %w", err)
	}
	return count, nil
}
