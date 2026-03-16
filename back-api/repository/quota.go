package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// IncrementDailyUsage atomically increments the user's daily submission count
// and returns the new count. Uses an upsert to handle the first submission of the day.
func (r *Repository) IncrementDailyUsage(ctx context.Context, userID uuid.UUID) (int, error) {
	var count int
	err := r.querier.QueryRow(ctx, `
		INSERT INTO user_daily_usage (user_id, usage_date, count)
		VALUES ($1, CURRENT_DATE, 1)
		ON CONFLICT (user_id, usage_date) DO UPDATE SET count = user_daily_usage.count + 1
		RETURNING count
	`, userID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("increment daily usage: %w", err)
	}
	return count, nil
}

// GetDailyUsage returns the current daily submission count for a user.
func (r *Repository) GetDailyUsage(ctx context.Context, userID uuid.UUID) (int, error) {
	var count int
	err := r.querier.QueryRow(ctx, `
		SELECT COALESCE(count, 0) FROM user_daily_usage
		WHERE user_id = $1 AND usage_date = CURRENT_DATE
	`, userID).Scan(&count)
	if err != nil {
		// No row means 0 usage today
		return 0, nil
	}
	return count, nil
}
