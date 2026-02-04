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
	pattern := "refresh_token:*"
	keys, err := s.client.Keys(ctx, pattern).Result()
	if err != nil {
		return fmt.Errorf("failed to get refresh token keys: %w", err)
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

	return nil
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
