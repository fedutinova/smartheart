package auth

import (
	"context"
	"time"
)

// SessionService abstracts session-related operations backed by Redis
// (token storage, login rate limiting, token blacklisting).
type SessionService interface {
	TokenBlacklistChecker

	// Ping checks connectivity to the underlying store.
	Ping(ctx context.Context) error

	// Login rate limiting
	GetLoginAttempts(ctx context.Context, email string) (int64, error)
	IncrLoginAttempts(ctx context.Context, email string, window time.Duration) (int64, error)
	ResetLoginAttempts(ctx context.Context, email string) error

	// Refresh token management
	StoreRefreshToken(ctx context.Context, userID, tokenHash string, ttl time.Duration) error
	GetRefreshTokenUserID(ctx context.Context, tokenHash string) (string, error)
	RevokeRefreshToken(ctx context.Context, tokenHash string) error
	RevokeAllUserTokens(ctx context.Context, userID string) error

	// Access token blacklisting
	StoreBlacklistedToken(ctx context.Context, tokenHash string, ttl time.Duration) error
}
