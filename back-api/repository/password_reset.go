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

// CreatePasswordResetToken persists a new password reset token.
func (r *Repository) CreatePasswordResetToken(ctx context.Context, token *models.PasswordResetToken) error {
	if token.ID == uuid.Nil {
		token.ID = uuid.New()
	}

	query := `
		INSERT INTO password_reset_tokens (id, user_id, token_hash, expires_at, created_at)
		VALUES ($1, $2, $3, $4, NOW())
	`

	_, err := r.querier.Exec(ctx, query, token.ID, token.UserID, token.TokenHash, token.ExpiresAt)
	if err != nil {
		return fmt.Errorf("create password reset token: %w", err)
	}
	return nil
}

// GetValidPasswordResetToken returns an unused, non-expired token by hash.
func (r *Repository) GetValidPasswordResetToken(ctx context.Context, tokenHash string) (*models.PasswordResetToken, error) {
	query := `
		SELECT id, user_id, token_hash, expires_at, created_at, used_at
		FROM password_reset_tokens
		WHERE token_hash = $1 AND used_at IS NULL AND expires_at > NOW()
	`

	var t models.PasswordResetToken
	err := r.querier.QueryRow(ctx, query, tokenHash).Scan(
		&t.ID, &t.UserID, &t.TokenHash, &t.ExpiresAt, &t.CreatedAt, &t.UsedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperr.ErrInvalidToken
		}
		return nil, fmt.Errorf("get valid password reset token: %w", err)
	}
	return &t, nil
}

// MarkPasswordResetTokenUsed marks a token as used.
func (r *Repository) MarkPasswordResetTokenUsed(ctx context.Context, tokenID uuid.UUID) error {
	query := `UPDATE password_reset_tokens SET used_at = NOW() WHERE id = $1`

	_, err := r.querier.Exec(ctx, query, tokenID)
	if err != nil {
		return fmt.Errorf("mark password reset token used: %w", err)
	}
	return nil
}

// InvalidateUserPasswordResetTokens marks all unused tokens for a user as used.
func (r *Repository) InvalidateUserPasswordResetTokens(ctx context.Context, userID uuid.UUID) error {
	query := `UPDATE password_reset_tokens SET used_at = NOW() WHERE user_id = $1 AND used_at IS NULL`

	_, err := r.querier.Exec(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("invalidate user password reset tokens: %w", err)
	}
	return nil
}

// UpdateUserPassword updates a user's password hash.
func (r *Repository) UpdateUserPassword(ctx context.Context, userID uuid.UUID, passwordHash string) error {
	query := `UPDATE users SET password_hash = $1, updated_at = NOW() WHERE id = $2`

	tag, err := r.querier.Exec(ctx, query, passwordHash, userID)
	if err != nil {
		return fmt.Errorf("update user password: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperr.ErrUserNotFound
	}
	return nil
}
