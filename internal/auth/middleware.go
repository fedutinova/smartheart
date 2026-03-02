package auth

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

type ctxKey string

const (
	ctxKeyClaims ctxKey = "claims"
)

func FromContext(ctx context.Context) (*Claims, bool) {
	cl, ok := ctx.Value(ctxKeyClaims).(*Claims)
	return cl, ok
}

// TokenBlacklistChecker checks whether a token hash has been blacklisted.
type TokenBlacklistChecker interface {
	IsTokenBlacklisted(ctx context.Context, tokenHash string) (bool, error)
}

func JWTMiddleware(secret, issuer string, opts ...func(*jwtMWConfig)) func(http.Handler) http.Handler {
	cfg := jwtMWConfig{}
	for _, o := range opts {
		o(&cfg)
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw := r.Header.Get("Authorization")
			if raw == "" || !strings.HasPrefix(raw, "Bearer ") {
				http.Error(w, "missing bearer token", http.StatusUnauthorized)
				return
			}
			tokenStr := strings.TrimPrefix(raw, "Bearer ")

			parser := jwt.NewParser(jwt.WithValidMethods([]string{"HS256"}))
			cl := &Claims{}
			_, err := parser.ParseWithClaims(tokenStr, cl, func(t *jwt.Token) (any, error) {
				return []byte(secret), nil
			})
			if err != nil {
				slog.Warn("jwt parse failed", "error", err)
				http.Error(w, "invalid token", http.StatusUnauthorized)
				return
			}
			if cl.Issuer != issuer {
				http.Error(w, "invalid issuer", http.StatusUnauthorized)
				return
			}

			// Check token blacklist (for logged-out tokens)
			if cfg.blacklist != nil {
				tokenHash := hashToken(tokenStr)
				if blacklisted, err := cfg.blacklist.IsTokenBlacklisted(r.Context(), tokenHash); err == nil && blacklisted {
					http.Error(w, "token has been revoked", http.StatusUnauthorized)
					return
				}
			}

			ctx := context.WithValue(r.Context(), ctxKeyClaims, cl)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

type jwtMWConfig struct {
	blacklist TokenBlacklistChecker
}

// WithBlacklist configures the JWT middleware to check a token blacklist.
func WithBlacklist(bl TokenBlacklistChecker) func(*jwtMWConfig) {
	return func(c *jwtMWConfig) { c.blacklist = bl }
}

func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return fmt.Sprintf("%x", h)
}

func RequirePerm(required string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cl, ok := FromContext(r.Context())
			if !ok {
				http.Error(w, "no auth context", http.StatusUnauthorized)
				return
			}
			perms := PermsForRoles(cl.Roles)
			if _, ok := perms[PermAdminAll]; ok {
				next.ServeHTTP(w, r)
				return
			}
			if _, ok := perms[required]; !ok {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

var ErrNoClaims = errors.New("no claims in context")
