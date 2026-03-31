package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const minSecretLen = 32 // HS256 requires at least 256 bits

// ValidateSecret checks that the JWT signing secret meets the minimum length
// requirement for HS256. Call this at application startup.
func ValidateSecret(secret string) error {
	if len(secret) < minSecretLen {
		return fmt.Errorf("JWT secret too short: got %d bytes, need at least %d", len(secret), minSecretLen)
	}
	return nil
}

type Claims struct {
	UserID string   `json:"user_id"`
	Roles  []string `json:"roles"`
	jwt.RegisteredClaims
}

type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

func NewToken(secret, issuer, subject string, roles []string, ttl time.Duration, audiences ...string) (string, error) {
	now := time.Now()
	aud := audiences
	if len(aud) == 0 {
		aud = []string{"smartheart"}
	}
	cl := Claims{
		UserID: subject,
		Roles:  roles,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    issuer,
			Subject:   subject,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
			Audience:  aud,
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, cl)
	return token.SignedString([]byte(secret))
}

func GenerateRefreshToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func NewTokenPair(secret, issuer string, userID uuid.UUID, roles []string, accessTTL, _ time.Duration) (*TokenPair, error) {
	accessToken, err := NewToken(secret, issuer, userID.String(), roles, accessTTL)
	if err != nil {
		return nil, err
	}

	refreshToken, err := GenerateRefreshToken()
	if err != nil {
		return nil, err
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}
