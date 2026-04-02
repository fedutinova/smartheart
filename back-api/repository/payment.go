package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/fedutinova/smartheart/back-api/apperr"
	"github.com/fedutinova/smartheart/back-api/models"
)

// CreatePayment inserts a new payment record. If a record with the same
// yookassa_id already exists, the call is a no-op (idempotent).
// Returns ErrConflict if a duplicate pending subscription is detected
// (protected by the uq_payments_pending_subscription partial unique index).
func (r *Repository) CreatePayment(ctx context.Context, p *models.Payment) error {
	_, err := r.querier.Exec(ctx, `
		INSERT INTO payments (id, user_id, yookassa_id, status, amount_kopecks, description, analyses_count, payment_type)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (yookassa_id) DO NOTHING
	`, p.ID, p.UserID, p.YooKassaID, p.Status, p.AmountKopecks, p.Description, p.AnalysesCount, p.PaymentType)
	if err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("duplicate pending payment: %w", apperr.ErrConflict)
		}
		return fmt.Errorf("create payment: %w", err)
	}
	return nil
}

// HasPendingPayment checks whether the user already has a pending payment of
// the given type. Used to prevent duplicate concurrent payment creation.
func (r *Repository) HasPendingPayment(ctx context.Context, userID uuid.UUID, paymentType string) (bool, error) {
	var exists bool
	err := r.querier.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM payments
			WHERE user_id = $1 AND payment_type = $2 AND status = 'pending'
		)
	`, userID, paymentType).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check pending payment: %w", err)
	}
	return exists, nil
}

// ConfirmPayment marks a payment as succeeded and credits the user atomically.
// For "analyses" type — increments paid_analyses_remaining.
// For "subscription" type — sets subscription_expires_at to 30 days from now.
func (r *Repository) ConfirmPayment(ctx context.Context, yookassaID string) error {
	return r.db.WithTx(ctx, func(tx pgx.Tx) error {
		// Confirm the payment and get its type.
		var userID uuid.UUID
		var paymentType string
		var analysesCount int
		err := tx.QueryRow(ctx, `
			UPDATE payments
			SET status = 'succeeded', confirmed_at = now()
			WHERE yookassa_id = $1 AND status = 'pending'
			RETURNING user_id, payment_type, analyses_count
		`, yookassaID).Scan(&userID, &paymentType, &analysesCount)
		if err != nil {
			return fmt.Errorf("confirm payment: %w", err)
		}

		// Credit the user based on payment type.
		switch paymentType {
		case models.PaymentTypeSubscription:
			_, err = tx.Exec(ctx, `
				UPDATE users
				SET subscription_expires_at = GREATEST(COALESCE(subscription_expires_at, now()), now()) + INTERVAL '30 days'
				WHERE id = $1
			`, userID)
		default:
			_, err = tx.Exec(ctx, `
				UPDATE users
				SET paid_analyses_remaining = paid_analyses_remaining + $2
				WHERE id = $1
			`, userID, analysesCount)
		}
		if err != nil {
			return fmt.Errorf("credit user after payment: %w", err)
		}
		return nil
	})
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

// CancelStalePayments cancels all pending payments older than the given duration.
// Returns the number of canceled payments.
func (r *Repository) CancelStalePayments(ctx context.Context, olderThan time.Duration) (int, error) {
	cutoff := time.Now().Add(-olderThan)
	tag, err := r.querier.Exec(ctx, `
		UPDATE payments SET status = 'canceled'
		WHERE status = 'pending' AND created_at < $1
	`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("cancel stale payments: %w", err)
	}
	return int(tag.RowsAffected()), nil
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

// GetSubscriptionExpiresAt returns the user's subscription expiration time, or nil if none.
func (r *Repository) GetSubscriptionExpiresAt(ctx context.Context, userID uuid.UUID) (*time.Time, error) {
	var expiresAt *time.Time
	err := r.querier.QueryRow(ctx, `
		SELECT subscription_expires_at FROM users WHERE id = $1
	`, userID).Scan(&expiresAt)
	if err != nil {
		return nil, fmt.Errorf("get subscription: %w", err)
	}
	return expiresAt, nil
}

// GetPaymentsByUserID returns payment history for a user.
func (r *Repository) GetPaymentsByUserID(ctx context.Context, userID uuid.UUID) ([]models.Payment, error) {
	rows, err := r.querier.Query(ctx, `
		SELECT id, user_id, yookassa_id, status, amount_kopecks, description, analyses_count, payment_type, created_at, confirmed_at
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
			&p.Description, &p.AnalysesCount, &p.PaymentType, &p.CreatedAt, &p.ConfirmedAt); err != nil {
			return nil, fmt.Errorf("scan payment: %w", err)
		}
		payments = append(payments, p)
	}
	return payments, nil
}
