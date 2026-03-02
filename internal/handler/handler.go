package handler

import (
	"github.com/fedutinova/smartheart/internal/auth"
	"github.com/fedutinova/smartheart/internal/config"
	"github.com/fedutinova/smartheart/internal/memq"
	"github.com/fedutinova/smartheart/internal/redis"
	"github.com/fedutinova/smartheart/internal/repository"
	"github.com/fedutinova/smartheart/internal/storage"
	"github.com/go-chi/chi/v5"
)

// Handlers contains all HTTP handler dependencies
type Handlers struct {
	Q       memq.JobQueue
	Repo    *repository.Repository
	Storage storage.Storage
	Redis   *redis.Service
	Config  config.Config
}

// RegisterRoutes registers all HTTP routes
func (h *Handlers) RegisterRoutes(r chi.Router) {
	// Health check (no auth required, for load balancer)
	r.Get("/health", h.Health)

	// Public auth endpoints
	r.Group(func(r chi.Router) {
		r.Post("/v1/auth/register", h.Register)
		r.Post("/v1/auth/login", h.Login)
		r.Post("/v1/auth/refresh", h.Refresh)
	})

	// Protected endpoints
	r.Group(func(r chi.Router) {
		r.Use(auth.JWTMiddleware(h.Config.JWTSecret, h.Config.JWTIssuer, auth.WithBlacklist(h.Redis)))

		// Static file serving for local storage (requires auth)
		if h.Config.StorageMode == "local" || h.Config.StorageMode == "filesystem" {
			r.Get("/files/*", h.ServeFiles)
		}

		r.Post("/v1/auth/logout", h.Logout)

		r.With(auth.RequirePerm(auth.PermEKGSubmit)).Post("/v1/ekg/analyze", h.SubmitEKGAnalyze)
		r.With(auth.RequirePerm(auth.PermEKGSubmit)).Post("/v1/gpt/process", h.SubmitGPTRequest)

		r.With(auth.RequirePerm(auth.PermJobReadOwn)).Get("/v1/jobs/{id}", h.GetJob)
		r.With(auth.RequirePerm(auth.PermJobReadOwn)).Get("/v1/requests/{id}", h.GetRequest)
		r.With(auth.RequirePerm(auth.PermJobReadOwn)).Get("/v1/requests", h.GetUserRequests)

		// Admin-only endpoints
		r.With(auth.RequirePerm(auth.PermAdminAll)).Get("/ready", h.Ready)
	})
}

