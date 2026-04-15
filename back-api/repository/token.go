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

// CreateRefreshToken creates a new refresh token record.
func (r *Repository) CreateRefreshToken(ctx context.Context, token *models.RefreshToken) error {
	if token.ID == uuid.Nil {
		token.ID = uuid.New()
	}

	query := `
		INSERT INTO refresh_tokens (id, user_id, token_hash, expires_at, created_at)
		VALUES ($1, $2, $3, $4, NOW())
	`

	_, err := r.querier.Exec(ctx, query, token.ID, token.UserID, token.TokenHash, token.ExpiresAt)
	if err != nil {
		return fmt.Errorf("failed to create refresh token: %w", err)
	}
	return nil
}

// GetRefreshToken retrieves a valid refresh token by hash.
func (r *Repository) GetRefreshToken(ctx context.Context, tokenHash string) (*models.RefreshToken, error) {
	query := `
		SELECT id, user_id, token_hash, expires_at, created_at, revoked_at
		FROM refresh_tokens
		WHERE token_hash = $1 AND expires_at > NOW() AND revoked_at IS NULL
	`

	var token models.RefreshToken
	err := r.querier.QueryRow(ctx, query, tokenHash).Scan(
		&token.ID,
		&token.UserID,
		&token.TokenHash,
		&token.ExpiresAt,
		&token.CreatedAt,
		&token.RevokedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperr.ErrInvalidToken
		}
		return nil, fmt.Errorf("failed to get refresh token: %w", err)
	}

	return &token, nil
}

// RevokeRefreshToken revokes a refresh token by hash.
func (r *Repository) RevokeRefreshToken(ctx context.Context, tokenHash string) error {
	query := `
		UPDATE refresh_tokens
		SET revoked_at = NOW()
		WHERE token_hash = $1
	`

	_, err := r.querier.Exec(ctx, query, tokenHash)
	if err != nil {
		return fmt.Errorf("failed to revoke refresh token: %w", err)
	}
	return nil
}

// GetRevokedRefreshTokenOwner returns the owning user ID of a refresh token
// that has already been revoked. Returns apperr.ErrNotFound if no such
// revoked token exists (i.e. the token was never issued or is still active).
func (r *Repository) GetRevokedRefreshTokenOwner(ctx context.Context, tokenHash string) (uuid.UUID, error) {
	query := `
		SELECT user_id FROM refresh_tokens
		WHERE token_hash = $1 AND revoked_at IS NOT NULL
	`

	var userID uuid.UUID
	err := r.querier.QueryRow(ctx, query, tokenHash).Scan(&userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, apperr.ErrNotFound
		}
		return uuid.Nil, fmt.Errorf("failed to get revoked token owner: %w", err)
	}
	return userID, nil
}

// RevokeAllUserRefreshTokens revokes all refresh tokens for the given user.
func (r *Repository) RevokeAllUserRefreshTokens(ctx context.Context, userID uuid.UUID) error {
	query := `
		UPDATE refresh_tokens
		SET revoked_at = NOW()
		WHERE user_id = $1 AND revoked_at IS NULL
	`

	_, err := r.querier.Exec(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("failed to revoke all user refresh tokens: %w", err)
	}
	return nil
}
