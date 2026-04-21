package auth

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

// writeJSONError writes a JSON error response from middleware.
// Duplicates the {"error":"..."} shape from handler.APIError because
// the auth package cannot import handler (circular dependency).
func writeJSONError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	type errBody struct {
		Error string `json:"error"`
	}
	json.NewEncoder(w).Encode(errBody{Error: msg}) //nolint:errcheck // response write error is unrecoverable
}

type ctxKey string

const (
	ctxKeyClaims ctxKey = "claims"
)

func FromContext(ctx context.Context) (*Claims, bool) {
	cl, ok := ctx.Value(ctxKeyClaims).(*Claims)
	return cl, ok
}

// NewContext returns a context carrying the given claims.
func NewContext(ctx context.Context, claims *Claims) context.Context {
	return context.WithValue(ctx, ctxKeyClaims, claims)
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
				writeJSONError(w, http.StatusUnauthorized, "missing bearer token")
				return
			}
			tokenStr := strings.TrimPrefix(raw, "Bearer ")

			parser := jwt.NewParser(jwt.WithValidMethods([]string{"HS256"}))
			cl := &Claims{}
			_, err := parser.ParseWithClaims(tokenStr, cl, func(_ *jwt.Token) (any, error) {
				return []byte(secret), nil
			})
			if err != nil {
				slog.Warn("Jwt parse failed", "error", err)
				writeJSONError(w, http.StatusUnauthorized, "invalid token")
				return
			}
			if cl.Issuer != issuer {
				writeJSONError(w, http.StatusUnauthorized, "invalid issuer")
				return
			}
			if cl.UserID == "" {
				writeJSONError(w, http.StatusUnauthorized, "invalid token: missing user_id")
				return
			}

			// Check token blacklist (for logged-out tokens).
			// Fail-open: if the blacklist store is unreachable we log the
			// error but allow the request so that a Redis outage does not
			// cause a full authentication outage.
			if cfg.blacklist != nil {
				tokenHash := HashToken(tokenStr)
				blacklisted, err := cfg.blacklist.IsTokenBlacklisted(r.Context(), tokenHash)
				if err != nil {
					slog.Error("Failed to check token blacklist, allowing request", "error", err)
				} else if blacklisted {
					writeJSONError(w, http.StatusUnauthorized, "token has been revoked")
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

func RequirePerm(required string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cl, ok := FromContext(r.Context())
			if !ok {
				writeJSONError(w, http.StatusUnauthorized, "no auth context")
				return
			}
			perms := PermsForRoles(cl.Roles)
			if _, hasAdmin := perms[PermAdminAll]; hasAdmin {
				next.ServeHTTP(w, r)
				return
			}
			if _, hasPerm := perms[required]; !hasPerm {
				writeJSONError(w, http.StatusForbidden, "forbidden")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

var ErrNoClaims = errors.New("no claims in context")
