package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/fedutinova/smartheart/back-api/models"
)

// AdminUserRow is a user record enriched with admin-relevant fields.
type AdminUserRow struct {
	ID                    uuid.UUID  `json:"id"`
	Username              string     `json:"username"`
	Email                 string     `json:"email"`
	Roles                 []string   `json:"roles"`
	PaidAnalysesRemaining int        `json:"paid_analyses_remaining"`
	SubscriptionExpiresAt *time.Time `json:"subscription_expires_at,omitempty"`
	RequestsCount         int        `json:"requests_count"`
	CreatedAt             time.Time  `json:"created_at"`
}

// AdminPaymentRow is a payment record with the user's email.
type AdminPaymentRow struct {
	models.Payment
	UserEmail string `json:"user_email"`
}

// AdminFeedbackRow is a RAG feedback record with the user's email.
type AdminFeedbackRow struct {
	models.RAGFeedback
	UserEmail string `json:"user_email"`
}

// DailyCount is a date→count pair for time-series charts.
type DailyCount struct {
	Date  string `json:"date"`
	Count int    `json:"count"`
}

// AdminStats holds aggregate dashboard numbers.
type AdminStats struct {
	UsersCount           int            `json:"users_count"`
	RequestsByStatus     map[string]int `json:"requests_by_status"`
	RequestsDaily        []DailyCount   `json:"requests_daily"`
	PaymentsSucceeded    int            `json:"payments_succeeded"`
	PaymentsTotalRub     float64        `json:"payments_total_rub"`
	FeedbackPositive     int            `json:"feedback_positive"`
	FeedbackNegative     int            `json:"feedback_negative"`
	FeedbackSatisfaction float64        `json:"feedback_satisfaction_pct"`
}

// GetAdminStats returns aggregate dashboard statistics.
func (r *Repository) GetAdminStats(ctx context.Context) (*AdminStats, error) {
	stats := &AdminStats{RequestsByStatus: make(map[string]int)}

	// Users count
	if err := r.querier.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&stats.UsersCount); err != nil {
		return nil, fmt.Errorf("count users: %w", err)
	}

	// Requests by status
	rows, err := r.querier.Query(ctx, `SELECT status, COUNT(*) FROM requests GROUP BY status`)
	if err != nil {
		return nil, fmt.Errorf("count requests: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, fmt.Errorf("scan request status: %w", err)
		}
		stats.RequestsByStatus[status] = count
	}

	// Payments
	if err := r.querier.QueryRow(ctx, `
		SELECT COUNT(*), COALESCE(SUM(amount_kopecks), 0)
		FROM payments WHERE status = 'succeeded'
	`).Scan(&stats.PaymentsSucceeded, &stats.PaymentsTotalRub); err != nil {
		return nil, fmt.Errorf("payment stats: %w", err)
	}
	stats.PaymentsTotalRub /= 100 // kopecks → rubles

	// Requests daily (last 30 days)
	dailyRows, err := r.querier.Query(ctx, `
		SELECT created_at::date AS day, COUNT(*)
		FROM requests
		WHERE created_at >= now() - INTERVAL '30 days'
		GROUP BY day ORDER BY day
	`)
	if err != nil {
		return nil, fmt.Errorf("daily requests: %w", err)
	}
	defer dailyRows.Close()
	for dailyRows.Next() {
		var dc DailyCount
		var day time.Time
		if err := dailyRows.Scan(&day, &dc.Count); err != nil {
			return nil, fmt.Errorf("scan daily: %w", err)
		}
		dc.Date = day.Format("2006-01-02")
		stats.RequestsDaily = append(stats.RequestsDaily, dc)
	}

	// Feedback
	if err := r.querier.QueryRow(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE rating = 1),
			COUNT(*) FILTER (WHERE rating = -1)
		FROM rag_feedback
	`).Scan(&stats.FeedbackPositive, &stats.FeedbackNegative); err != nil {
		return nil, fmt.Errorf("feedback stats: %w", err)
	}

	total := stats.FeedbackPositive + stats.FeedbackNegative
	if total > 0 {
		stats.FeedbackSatisfaction = float64(stats.FeedbackPositive) / float64(total) * 100
	}

	return stats, nil
}

// ListUsers returns a paginated list of users with admin-relevant fields.
func (r *Repository) ListUsers(ctx context.Context, limit, offset int, search string) ([]AdminUserRow, int, error) {
	var total int
	if search != "" {
		pattern := "%" + search + "%"
		if err := r.querier.QueryRow(ctx, `
			SELECT COUNT(*) FROM users WHERE username ILIKE $1 OR email ILIKE $1
		`, pattern).Scan(&total); err != nil {
			return nil, 0, fmt.Errorf("count users: %w", err)
		}
	} else {
		if err := r.querier.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&total); err != nil {
			return nil, 0, fmt.Errorf("count users: %w", err)
		}
	}

	query := `
		SELECT u.id, u.username, u.email,
		       COALESCE(ARRAY(SELECT r.name FROM user_roles ur JOIN roles r ON r.id = ur.role_id WHERE ur.user_id = u.id ORDER BY r.name), ARRAY[]::text[]) AS roles,
		       u.paid_analyses_remaining,
		       u.subscription_expires_at,
		       (SELECT COUNT(*) FROM requests req WHERE req.user_id = u.id) AS requests_count,
		       u.created_at
		FROM users u
	`
	var args []any
	argIdx := 1

	if search != "" {
		pattern := "%" + search + "%"
		query += fmt.Sprintf(" WHERE u.username ILIKE $%d OR u.email ILIKE $%d", argIdx, argIdx)
		args = append(args, pattern)
		argIdx++
	}

	query += " ORDER BY u.created_at DESC"
	query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
	args = append(args, limit, offset)

	rows, err := r.querier.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()

	var users []AdminUserRow
	for rows.Next() {
		var u AdminUserRow
		if err := rows.Scan(&u.ID, &u.Username, &u.Email, &u.Roles,
			&u.PaidAnalysesRemaining, &u.SubscriptionExpiresAt,
			&u.RequestsCount, &u.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan user: %w", err)
		}
		users = append(users, u)
	}
	return users, total, nil
}

// ListPayments returns a paginated list of all payments with user email.
func (r *Repository) ListPayments(ctx context.Context, limit, offset int) ([]AdminPaymentRow, int, error) {
	var total int
	if err := r.querier.QueryRow(ctx, `SELECT COUNT(*) FROM payments`).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count payments: %w", err)
	}

	rows, err := r.querier.Query(ctx, `
		SELECT p.id, p.user_id, p.yookassa_id, p.status, p.amount_kopecks,
		       p.description, p.analyses_count, p.payment_type, p.created_at, p.confirmed_at,
		       u.email
		FROM payments p
		JOIN users u ON u.id = p.user_id
		ORDER BY p.created_at DESC
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list payments: %w", err)
	}
	defer rows.Close()

	var payments []AdminPaymentRow
	for rows.Next() {
		var p AdminPaymentRow
		if err := rows.Scan(&p.ID, &p.UserID, &p.YooKassaID, &p.Status, &p.AmountKopecks,
			&p.Description, &p.AnalysesCount, &p.PaymentType, &p.CreatedAt, &p.ConfirmedAt,
			&p.UserEmail); err != nil {
			return nil, 0, fmt.Errorf("scan payment: %w", err)
		}
		payments = append(payments, p)
	}
	return payments, total, nil
}

// ListRAGFeedback returns a paginated list of RAG feedback with user email.
func (r *Repository) ListRAGFeedback(ctx context.Context, limit, offset int) ([]AdminFeedbackRow, int, error) {
	var total int
	if err := r.querier.QueryRow(ctx, `SELECT COUNT(*) FROM rag_feedback`).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count feedback: %w", err)
	}

	rows, err := r.querier.Query(ctx, `
		SELECT f.id, f.user_id, f.question, f.answer, f.rating, f.created_at,
		       u.email
		FROM rag_feedback f
		JOIN users u ON u.id = f.user_id
		ORDER BY f.created_at DESC
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list feedback: %w", err)
	}
	defer rows.Close()

	var feedback []AdminFeedbackRow
	for rows.Next() {
		var f AdminFeedbackRow
		if err := rows.Scan(&f.ID, &f.UserID, &f.Question, &f.Answer, &f.Rating, &f.CreatedAt,
			&f.UserEmail); err != nil {
			return nil, 0, fmt.Errorf("scan feedback: %w", err)
		}
		feedback = append(feedback, f)
	}
	return feedback, total, nil
}
