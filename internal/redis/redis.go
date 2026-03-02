package redis

import (
	"context"
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

// Client returns the underlying Redis client
func (s *Service) Client() *redis.Client {
	return s.client
}

func (s *Service) StoreRefreshToken(ctx context.Context, userID, tokenHash string, ttl time.Duration) error {
	key := fmt.Sprintf("refresh_token:%s", tokenHash)
	return s.client.Set(ctx, key, userID, ttl).Err()
}

func (s *Service) GetRefreshTokenUserID(ctx context.Context, tokenHash string) (string, error) {
	key := fmt.Sprintf("refresh_token:%s", tokenHash)
	userID, err := s.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", fmt.Errorf("refresh token not found")
	}
	if err != nil {
		return "", fmt.Errorf("failed to get refresh token: %w", err)
	}
	return userID, nil
}

func (s *Service) RevokeRefreshToken(ctx context.Context, tokenHash string) error {
	key := fmt.Sprintf("refresh_token:%s", tokenHash)
	return s.client.Del(ctx, key).Err()
}

func (s *Service) RevokeAllUserTokens(ctx context.Context, userID string) error {
	var cursor uint64
	for {
		keys, nextCursor, err := s.client.Scan(ctx, cursor, "refresh_token:*", 100).Result()
		if err != nil {
			return fmt.Errorf("failed to scan refresh token keys: %w", err)
		}

		for _, key := range keys {
			storedUserID, err := s.client.Get(ctx, key).Result()
			if err != nil {
				continue
			}
			if storedUserID == userID {
				s.client.Del(ctx, key)
			}
		}

		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}

	return nil
}

// IncrLoginAttempts increments the failed login counter for a given email
// and returns the current count. The counter expires after the given window.
func (s *Service) IncrLoginAttempts(ctx context.Context, email string, window time.Duration) (int64, error) {
	key := fmt.Sprintf("login_attempts:%s", email)
	count, err := s.client.Incr(ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to increment login attempts: %w", err)
	}
	// Set expiry only on first attempt (count == 1)
	if count == 1 {
		s.client.Expire(ctx, key, window)
	}
	return count, nil
}

// ResetLoginAttempts clears the failed login counter after successful login.
func (s *Service) ResetLoginAttempts(ctx context.Context, email string) {
	key := fmt.Sprintf("login_attempts:%s", email)
	s.client.Del(ctx, key)
}

// GetLoginAttempts returns the current failed login count for an email.
func (s *Service) GetLoginAttempts(ctx context.Context, email string) (int64, error) {
	key := fmt.Sprintf("login_attempts:%s", email)
	count, err := s.client.Get(ctx, key).Int64()
	if err == redis.Nil {
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
