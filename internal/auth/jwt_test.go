package auth

import (
	"encoding/hex"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func TestGenerateRefreshToken_IsHexAndLength(t *testing.T) {
	token, err := GenerateRefreshToken()
	if err != nil {
		t.Fatalf("GenerateRefreshToken error: %v", err)
	}
	// 32 random bytes encoded as hex => 64 chars
	if len(token) != 64 {
		t.Fatalf("expected refresh token length 64, got %d", len(token))
	}
	if strings.ToLower(token) != token {
		t.Fatalf("expected lowercase hex token, got %q", token)
	}
	// ensure it's valid hex
	_, err = hex.DecodeString(token)
	if err != nil {
		t.Fatalf("expected valid hex token, got error: %v", err)
	}
}

func TestNewToken_ContainsClaims(t *testing.T) {
	secret := "test-secret"
	issuer := "smartheart-test"
	subject := uuid.New().String()
	roles := []string{"user", "tester"}

	ttl := 2 * time.Minute
	tokenStr, err := NewToken(secret, issuer, subject, roles, ttl)
	if err != nil {
		t.Fatalf("NewToken error: %v", err)
	}

	parser := jwt.NewParser(jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	claims := &Claims{}
	_, err = parser.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (any, error) {
		return []byte(secret), nil
	})
	if err != nil {
		t.Fatalf("ParseWithClaims error: %v", err)
	}

	if claims.Issuer != issuer {
		t.Fatalf("expected issuer %q, got %q", issuer, claims.Issuer)
	}
	if claims.UserID != subject {
		t.Fatalf("expected user_id %q, got %q", subject, claims.UserID)
	}
	// Note: Claims defines its own `Sub` field (json:"sub") in addition to RegisteredClaims.Subject.
	// In the current implementation, `sub` is effectively stored in Claims.Sub.
	if claims.Sub != subject {
		t.Fatalf("expected sub %q, got %q", subject, claims.Sub)
	}
	if len(claims.Roles) != len(roles) {
		t.Fatalf("expected %d roles, got %d", len(roles), len(claims.Roles))
	}
	for i := range roles {
		if claims.Roles[i] != roles[i] {
			t.Fatalf("expected roles[%d]=%q, got %q", i, roles[i], claims.Roles[i])
		}
	}
	if claims.ExpiresAt == nil || claims.IssuedAt == nil {
		t.Fatalf("expected iat/exp to be set")
	}
	if claims.ExpiresAt.Time.Before(claims.IssuedAt.Time) {
		t.Fatalf("expected exp after iat")
	}
}

func TestNewTokenPair_ReturnsAccessAndRefresh(t *testing.T) {
	secret := "test-secret"
	issuer := "smartheart-test"
	userID := uuid.New()
	roles := []string{"user"}

	pair, err := NewTokenPair(secret, issuer, userID, roles, 1*time.Minute, 7*24*time.Hour)
	if err != nil {
		t.Fatalf("NewTokenPair error: %v", err)
	}
	if pair.AccessToken == "" || pair.RefreshToken == "" {
		t.Fatalf("expected non-empty tokens, got access=%q refresh=%q", pair.AccessToken, pair.RefreshToken)
	}
	if len(pair.RefreshToken) != 64 {
		t.Fatalf("expected refresh token length 64, got %d", len(pair.RefreshToken))
	}
}
