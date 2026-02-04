package server

import (
	"net/http"
	"strings"
	"time"

	"github.com/fedutinova/smartheart/internal/config"
	httpapi "github.com/fedutinova/smartheart/internal/transport/http"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httprate"
)

func NewRouter(h *httpapi.Handlers, cfg config.Config) http.Handler {
	r := chi.NewRouter()

	// CORS middleware - must be first
	r.Use(corsMiddleware(cfg.CORSOrigins, cfg.CORSCredentials))

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	// Rate limiting by IP address
	if cfg.RateLimitRPS > 0 {
		r.Use(httprate.Limit(
			cfg.RateLimitRPS,
			time.Minute,
			httprate.WithKeyFuncs(httprate.KeyByIP),
			httprate.WithLimitHandler(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				w.Write([]byte(`{"error":"rate limit exceeded","retry_after":"60s"}`))
			}),
		))
	}

	h.Routers(r)
	return r
}

// corsMiddleware creates a CORS middleware with configurable origins
func corsMiddleware(allowedOrigins []string, allowCredentials bool) func(http.Handler) http.Handler {
	// Build allowed origins map for O(1) lookup
	originsMap := make(map[string]bool, len(allowedOrigins))
	allowAll := false
	for _, origin := range allowedOrigins {
		if origin == "*" {
			allowAll = true
		}
		originsMap[strings.ToLower(origin)] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			// Check if origin is allowed
			allowed := false
			if allowAll {
				allowed = true
			} else if origin != "" {
				allowed = originsMap[strings.ToLower(origin)]
			}

			if allowed && origin != "" {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				if allowCredentials {
					w.Header().Set("Access-Control-Allow-Credentials", "true")
				}
			}

			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Accept, Authorization, Content-Type, X-CSRF-Token, X-Request-ID")
			w.Header().Set("Access-Control-Expose-Headers", "X-Request-ID")
			w.Header().Set("Access-Control-Max-Age", "86400") // 24 hours

			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
