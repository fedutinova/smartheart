package auth

import (
	"context"
	"errors"
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

func JWTMiddleware(secret, issuer string) func(http.Handler) http.Handler {
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
			ctx := context.WithValue(r.Context(), ctxKeyClaims, cl)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
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
