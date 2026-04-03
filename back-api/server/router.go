package server

import (
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httprate"

	"github.com/fedutinova/smartheart/back-api/auth"
	"github.com/fedutinova/smartheart/back-api/config"
	"github.com/fedutinova/smartheart/back-api/handler"
)

func NewRouter(h *handler.Handler, cfg config.Config) http.Handler {
	r := chi.NewRouter()

	// CORS middleware - must be first, before route matching.
	r.Use(corsMiddleware(cfg.CORS.Origins, cfg.CORS.Credentials))
	r.Use(securityHeaders)
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	// Global rate limiting by IP address
	if cfg.RateLimit.RPM > 0 {
		r.Use(httprate.Limit(
			cfg.RateLimit.RPM,
			time.Minute,
			httprate.WithKeyFuncs(httprate.KeyByIP),
			httprate.WithLimitHandler(rateLimitHandler),
		))
	}

	// Catch-all OPTIONS handler for CORS preflight.
	// Must be after r.Use() (chi requirement) but before other routes.
	r.Options("/*", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	h.RegisterRoutes(r)
	return r
}

// rateLimitHandler is the shared response for rate-limited requests.
func rateLimitHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusTooManyRequests)
	_, _ = w.Write([]byte(`{"error":"rate limit exceeded","retry_after":"60s"}`))
}

// EndpointRateLimit returns a rate-limiting middleware for a specific endpoint.
// For authenticated requests the key is the user ID; otherwise it falls back to IP.
func EndpointRateLimit(rpm int) func(http.Handler) http.Handler {
	return httprate.Limit(
		rpm,
		time.Minute,
		httprate.WithKeyFuncs(keyByUserOrIP),
		httprate.WithLimitHandler(rateLimitHandler),
	)
}

// keyByUserOrIP returns the authenticated user's ID as rate-limit key,
// falling back to the client IP for anonymous requests.
func keyByUserOrIP(r *http.Request) (string, error) {
	if claims, ok := auth.FromContext(r.Context()); ok && claims.UserID != "" {
		return "user:" + claims.UserID, nil
	}
	return httprate.KeyByIP(r)
}

// securityHeaders adds standard security headers to every response.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "0")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		next.ServeHTTP(w, r)
	})
}

// corsMiddleware creates a CORS middleware with configurable origins
func corsMiddleware(allowedOrigins []string, allowCredentials bool) func(http.Handler) http.Handler {
	originsMap := make(map[string]bool, len(allowedOrigins))
	allowAll := false
	for _, origin := range allowedOrigins {
		if origin == "*" {
			allowAll = true
		}
		originsMap[strings.ToLower(origin)] = true
	}

	reflectOrigin := allowAll && allowCredentials
	if reflectOrigin {
		allowAll = false
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			allowed := false
			if allowAll || reflectOrigin {
				allowed = true
			} else if origin != "" {
				allowed = originsMap[strings.ToLower(origin)]
			}

			if allowed && origin != "" {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Accept, Authorization, Content-Type, X-CSRF-Token, X-Request-ID")
				w.Header().Set("Access-Control-Expose-Headers", "X-Request-ID")
				w.Header().Set("Access-Control-Max-Age", "86400")
				if allowCredentials {
					w.Header().Set("Access-Control-Allow-Credentials", "true")
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}
