package repository

import (
	"context"
	"fmt"

	"github.com/fedutinova/smartheart/back-api/models"
	"github.com/google/uuid"
)

// CreatePayment inserts a new payment record.
func (r *Repository) CreatePayment(ctx context.Context, p *models.Payment) error {
	_, err := r.querier.Exec(ctx, `
		INSERT INTO payments (id, user_id, yookassa_id, status, amount_kopecks, description, analyses_count)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, p.ID, p.UserID, p.YooKassaID, p.Status, p.AmountKopecks, p.Description, p.AnalysesCount)
	if err != nil {
		return fmt.Errorf("create payment: %w", err)
	}
	return nil
}

// ConfirmPayment marks a payment as succeeded and credits paid analyses to the user.
func (r *Repository) ConfirmPayment(ctx context.Context, yookassaID string) error {
	tag, err := r.querier.Exec(ctx, `
		WITH confirmed AS (
			UPDATE payments
			SET status = 'succeeded', confirmed_at = now()
			WHERE yookassa_id = $1 AND status = 'pending'
			RETURNING user_id, analyses_count
		)
		UPDATE users
		SET paid_analyses_remaining = paid_analyses_remaining + confirmed.analyses_count
		FROM confirmed
		WHERE users.id = confirmed.user_id
	`, yookassaID)
	if err != nil {
		return fmt.Errorf("confirm payment: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("payment not found or already confirmed")
	}
	return nil
}

// CancelPayment marks a payment as canceled.
func (r *Repository) CancelPayment(ctx context.Context, yookassaID string) error {
	_, err := r.querier.Exec(ctx, `
		UPDATE payments SET status = 'canceled' WHERE yookassa_id = $1 AND status = 'pending'
	`, yookassaID)
	if err != nil {
		return fmt.Errorf("cancel payment: %w", err)
	}
	return nil
}

// GetPaidAnalysesRemaining returns how many paid analyses the user has left.
func (r *Repository) GetPaidAnalysesRemaining(ctx context.Context, userID uuid.UUID) (int, error) {
	var count int
	err := r.querier.QueryRow(ctx, `
		SELECT paid_analyses_remaining FROM users WHERE id = $1
	`, userID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("get paid analyses: %w", err)
	}
	return count, nil
}

// DecrementPaidAnalyses decrements the user's paid analysis counter by 1.
// Returns the new remaining count. Returns error if count is already 0.
func (r *Repository) DecrementPaidAnalyses(ctx context.Context, userID uuid.UUID) (int, error) {
	var remaining int
	err := r.querier.QueryRow(ctx, `
		UPDATE users
		SET paid_analyses_remaining = paid_analyses_remaining - 1
		WHERE id = $1 AND paid_analyses_remaining > 0
		RETURNING paid_analyses_remaining
	`, userID).Scan(&remaining)
	if err != nil {
		return 0, fmt.Errorf("decrement paid analyses: %w", err)
	}
	return remaining, nil
}

// GetPaymentsByUserID returns payment history for a user.
func (r *Repository) GetPaymentsByUserID(ctx context.Context, userID uuid.UUID) ([]models.Payment, error) {
	rows, err := r.querier.Query(ctx, `
		SELECT id, user_id, yookassa_id, status, amount_kopecks, description, analyses_count, created_at, confirmed_at
		FROM payments WHERE user_id = $1 ORDER BY created_at DESC LIMIT 50
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("get payments: %w", err)
	}
	defer rows.Close()

	var payments []models.Payment
	for rows.Next() {
		var p models.Payment
		if err := rows.Scan(&p.ID, &p.UserID, &p.YooKassaID, &p.Status, &p.AmountKopecks,
			&p.Description, &p.AnalysesCount, &p.CreatedAt, &p.ConfirmedAt); err != nil {
			return nil, fmt.Errorf("scan payment: %w", err)
		}
		payments = append(payments, p)
	}
	return payments, nil
}
