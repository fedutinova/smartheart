package handler

import (
	"github.com/fedutinova/smartheart/internal/auth"
	"github.com/fedutinova/smartheart/internal/config"
	"github.com/fedutinova/smartheart/internal/job"
	"github.com/fedutinova/smartheart/internal/repository"
	"github.com/fedutinova/smartheart/internal/storage"
	"github.com/go-chi/chi/v5"
)

// AuthHandler handles authentication endpoints (register, login, refresh, logout).
type AuthHandler struct {
	Repo     repository.Store
	Sessions auth.SessionService
	Config   config.Config
}

// EKGHandler handles EKG submission endpoints.
type EKGHandler struct {
	Queue job.Queue
	Repo  repository.Store
}

// GPTHandler handles GPT processing submission endpoints.
type GPTHandler struct {
	Queue   job.Queue
	Repo    repository.Store
	Storage storage.Storage
}

// RequestHandler handles request/job query endpoints and file serving.
type RequestHandler struct {
	Queue   job.Queue
	Repo    repository.Store
	Storage storage.Storage
	Config  config.Config
}

// HealthHandler handles health and readiness endpoints.
type HealthHandler struct {
	Queue    job.Queue
	Repo     repository.Store
	Sessions auth.SessionService
	Storage  storage.Storage
}

// Handler composes all focused handlers and registers routes.
type Handler struct {
	Auth    *AuthHandler
	EKG     *EKGHandler
	GPT     *GPTHandler
	Request *RequestHandler
	Healthz *HealthHandler
}

// NewHandler creates a Handler with all sub-handlers wired to shared dependencies.
func NewHandler(queue job.Queue, repo repository.Store, sessions auth.SessionService, storageService storage.Storage, cfg config.Config) *Handler {
	return &Handler{
		Auth:    &AuthHandler{Repo: repo, Sessions: sessions, Config: cfg},
		EKG:     &EKGHandler{Queue: queue, Repo: repo},
		GPT:     &GPTHandler{Queue: queue, Repo: repo, Storage: storageService},
		Request: &RequestHandler{Queue: queue, Repo: repo, Storage: storageService, Config: cfg},
		Healthz: &HealthHandler{Queue: queue, Repo: repo, Sessions: sessions, Storage: storageService},
	}
}

// RegisterRoutes registers all HTTP routes.
func (h *Handler) RegisterRoutes(r chi.Router) {
	// Health check (no auth required, for load balancer)
	r.Get("/health", h.Healthz.Health)

	// Public auth endpoints
	r.Group(func(r chi.Router) {
		r.Post("/v1/auth/register", h.Auth.Register)
		r.Post("/v1/auth/login", h.Auth.Login)
		r.Post("/v1/auth/refresh", h.Auth.Refresh)
	})

	// Protected endpoints
	r.Group(func(r chi.Router) {
		r.Use(auth.JWTMiddleware(h.Auth.Config.JWT.Secret, h.Auth.Config.JWT.Issuer, auth.WithBlacklist(h.Auth.Sessions)))

		// Static file serving for local storage (requires auth)
		if h.Request.Config.Storage.Mode == config.StorageModeLocal || h.Request.Config.Storage.Mode == config.StorageModeFilesystem {
			r.Get("/files/*", h.Request.ServeFiles)
		}

		r.Post("/v1/auth/logout", h.Auth.Logout)

		r.With(auth.RequirePerm(auth.PermEKGSubmit)).Post("/v1/ekg/analyze", h.EKG.SubmitEKGAnalyze)
		r.With(auth.RequirePerm(auth.PermEKGSubmit)).Post("/v1/gpt/process", h.GPT.SubmitGPTRequest)

		r.With(auth.RequirePerm(auth.PermJobReadOwn)).Get("/v1/jobs/{id}", h.Request.GetJob)
		r.With(auth.RequirePerm(auth.PermJobReadOwn)).Get("/v1/requests/{id}", h.Request.GetRequest)
		r.With(auth.RequirePerm(auth.PermJobReadOwn)).Get("/v1/requests", h.Request.GetUserRequests)

		// Admin-only endpoints
		r.With(auth.RequirePerm(auth.PermAdminAll)).Get("/ready", h.Healthz.Ready)
	})
}
