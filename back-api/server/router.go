package server

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-chi/httprate"

	"github.com/fedutinova/smartheart/back-api/auth"
	"github.com/fedutinova/smartheart/back-api/config"
	"github.com/fedutinova/smartheart/back-api/handler"
)

func NewRouter(h *handler.Handler, cfg config.Config) http.Handler {
	r := chi.NewRouter()

	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   cfg.CORS.Origins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "X-Request-ID"},
		ExposedHeaders:   []string{"X-Request-ID"},
		AllowCredentials: cfg.CORS.Credentials,
		MaxAge:           86400,
	}))
	r.Use(apiSecurityHeaders)
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(timeoutExcept(60*time.Second, "/v1/events"))

	// Global rate limiting by IP address
	if cfg.RateLimit.RPM > 0 {
		r.Use(httprate.Limit(
			cfg.RateLimit.RPM,
			time.Minute,
			httprate.WithKeyFuncs(httprate.KeyByIP),
			httprate.WithLimitHandler(rateLimitHandler),
		))
	}

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

// apiSecurityHeaders adds standard security headers to API responses.
func apiSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "0")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		next.ServeHTTP(w, r)
	})
}

// timeoutExcept wraps chi's Timeout middleware but skips it for the given paths
// (e.g. long-lived SSE connections that must stay open indefinitely).
func timeoutExcept(timeout time.Duration, skipPaths ...string) func(http.Handler) http.Handler {
	skip := make(map[string]bool, len(skipPaths))
	for _, p := range skipPaths {
		skip[p] = true
	}
	timeoutMW := middleware.Timeout(timeout)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if skip[r.URL.Path] {
				next.ServeHTTP(w, r)
			} else {
				timeoutMW(next).ServeHTTP(w, r)
			}
		})
	}
}
