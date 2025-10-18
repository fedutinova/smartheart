package auth

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type Claims struct {
	UserID string   `json:"user_id"`
	Sub    string   `json:"sub"`
	Roles  []string `json:"roles"`
	jwt.RegisteredClaims
}

type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

func NewToken(secret, issuer, subject string, roles []string, ttl time.Duration) (string, error) {
	now := time.Now()
	cl := Claims{
		UserID: subject,
		Sub:    subject,
		Roles:  roles,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    issuer,
			Subject:   subject,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
			Audience:  []string{"smartheart"}, // TODO change this later
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

func NewTokenPair(secret, issuer string, userID uuid.UUID, roles []string, accessTTL, refreshTTL time.Duration) (*TokenPair, error) {
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
