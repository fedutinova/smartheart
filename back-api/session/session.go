package session

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type Service struct {
	client *redis.Client
}

func New(redisURL string) (*Service, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Redis URL: %w", err)
	}

	client := redis.NewClient(opts)

	if err := client.Ping(context.Background()).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &Service{client: client}, nil
}

func (s *Service) Close() error {
	return s.client.Close()
}

// Client returns the underlying Redis client.
func (s *Service) Client() *redis.Client {
	return s.client
}

// Ping checks Redis connectivity.
func (s *Service) Ping(ctx context.Context) error {
	return s.client.Ping(ctx).Err()
}

func (s *Service) StoreRefreshToken(ctx context.Context, userID, tokenHash string, ttl time.Duration) error {
	key := fmt.Sprintf("refresh_token:%s", tokenHash)
	indexKey := fmt.Sprintf("user_tokens:%s", userID)

	pipe := s.client.Pipeline()
	pipe.Set(ctx, key, userID, ttl)
	pipe.SAdd(ctx, indexKey, tokenHash)
	// Keep the index alive at least as long as the longest token.
	pipe.Expire(ctx, indexKey, ttl)
	_, err := pipe.Exec(ctx)
	return err
}

func (s *Service) GetRefreshTokenUserID(ctx context.Context, tokenHash string) (string, error) {
	key := fmt.Sprintf("refresh_token:%s", tokenHash)
	userID, err := s.client.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return "", fmt.Errorf("refresh token not found")
	}
	if err != nil {
		return "", fmt.Errorf("failed to get refresh token: %w", err)
	}
	return userID, nil
}

func (s *Service) RevokeRefreshToken(ctx context.Context, tokenHash string) error {
	key := fmt.Sprintf("refresh_token:%s", tokenHash)

	// Look up the owning user so we can remove from the index.
	userID, err := s.client.Get(ctx, key).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return fmt.Errorf("failed to get token owner: %w", err)
	}

	pipe := s.client.Pipeline()
	pipe.Del(ctx, key)
	if userID != "" {
		pipe.SRem(ctx, fmt.Sprintf("user_tokens:%s", userID), tokenHash)
	}
	_, err = pipe.Exec(ctx)
	return err
}

func (s *Service) RevokeAllUserTokens(ctx context.Context, userID string) error {
	indexKey := fmt.Sprintf("user_tokens:%s", userID)

	hashes, err := s.client.SMembers(ctx, indexKey).Result()
	if err != nil {
		return fmt.Errorf("failed to get user token index: %w", err)
	}

	if len(hashes) == 0 {
		return nil
	}

	// Build keys to delete in one pipeline round-trip.
	pipe := s.client.Pipeline()
	for _, h := range hashes {
		pipe.Del(ctx, fmt.Sprintf("refresh_token:%s", h))
	}
	pipe.Del(ctx, indexKey)

	_, err = pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to revoke user tokens: %w", err)
	}
	return nil
}

// IncrLoginAttempts increments the failed login counter for a given email
// and returns the current count. The counter expires after the given window.
func (s *Service) IncrLoginAttempts(ctx context.Context, email string, window time.Duration) (int64, error) {
	key := fmt.Sprintf("login_attempts:%s", email)

	// Use a pipeline: INCR + ExpireNX in one round-trip.
	// ExpireNX sets TTL only on the first attempt so the window is anchored
	// to the first failure and does not reset on subsequent attempts.
	pipe := s.client.Pipeline()
	incrCmd := pipe.Incr(ctx, key)
	pipe.ExpireNX(ctx, key, window)
	if _, err := pipe.Exec(ctx); err != nil {
		return 0, fmt.Errorf("failed to increment login attempts: %w", err)
	}

	return incrCmd.Val(), nil
}

// ResetLoginAttempts clears the failed login counter after successful login.
func (s *Service) ResetLoginAttempts(ctx context.Context, email string) error {
	key := fmt.Sprintf("login_attempts:%s", email)
	return s.client.Del(ctx, key).Err()
}

// GetLoginAttempts returns the current failed login count for an email.
func (s *Service) GetLoginAttempts(ctx context.Context, email string) (int64, error) {
	key := fmt.Sprintf("login_attempts:%s", email)
	count, err := s.client.Get(ctx, key).Int64()
	if errors.Is(err, redis.Nil) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("failed to get login attempts: %w", err)
	}
	return count, nil
}

func (s *Service) StoreBlacklistedToken(ctx context.Context, tokenHash string, ttl time.Duration) error {
	key := fmt.Sprintf("blacklist:%s", tokenHash)
	return s.client.Set(ctx, key, "revoked", ttl).Err()
}

func (s *Service) IsTokenBlacklisted(ctx context.Context, tokenHash string) (bool, error) {
	key := fmt.Sprintf("blacklist:%s", tokenHash)
	exists, err := s.client.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check blacklist: %w", err)
	}
	return exists > 0, nil
}
