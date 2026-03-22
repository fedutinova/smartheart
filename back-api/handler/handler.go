package handler

import (
	"github.com/fedutinova/smartheart/back-api/auth"
	"github.com/fedutinova/smartheart/back-api/config"
	"github.com/fedutinova/smartheart/back-api/job"
	"github.com/fedutinova/smartheart/back-api/notify"
	"github.com/fedutinova/smartheart/back-api/repository"
	"github.com/fedutinova/smartheart/back-api/service"
	"github.com/fedutinova/smartheart/back-api/storage"
	"github.com/go-chi/chi/v5"
)

// AuthHandler handles authentication endpoints (register, login, refresh, logout).
type AuthHandler struct {
	Service service.AuthService
}

// EKGHandler handles EKG submission endpoints.
type EKGHandler struct {
	Service service.SubmissionService
}

// GPTHandler handles GPT processing submission endpoints.
type GPTHandler struct {
	Service service.SubmissionService
}

// RequestHandler handles request/job query endpoints and file serving.
type RequestHandler struct {
	Service service.RequestService
	Config  config.Config
	Storage storage.Storage
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
	Events  *EventsHandler
	RAG     *RAGHandler
	Payment *PaymentHandler
	Profile *ProfileHandler
	Config  config.Config
}

// NewHandler creates a Handler with all sub-handlers wired to shared dependencies.
func NewHandler(
	authSvc service.AuthService,
	submissionSvc service.SubmissionService,
	requestSvc service.RequestService,
	paymentSvc service.PaymentService,
	queue job.Queue,
	repo repository.Store,
	sessions auth.SessionService,
	storageService storage.Storage,
	hub *notify.Hub,
	cfg config.Config,
) *Handler {
	return &Handler{
		Auth:    &AuthHandler{Service: authSvc},
		EKG:     &EKGHandler{Service: submissionSvc},
		GPT:     &GPTHandler{Service: submissionSvc},
		Request: &RequestHandler{Service: requestSvc, Config: cfg, Storage: storageService},
		Healthz: &HealthHandler{Queue: queue, Repo: repo, Sessions: sessions, Storage: storageService},
		Events:  &EventsHandler{Hub: hub},
		RAG:     NewRAGHandler(cfg.RAG.URL, repo),
		Payment: &PaymentHandler{Service: paymentSvc},
		Profile: &ProfileHandler{Repo: repo},
		Config:  cfg,
	}
}

// RegisterRoutes registers all HTTP routes.
func (h *Handler) RegisterRoutes(r chi.Router) {
	// Health check (no auth required, for load balancer)
	r.Get("/health", h.Healthz.Health)

	// OpenAPI spec (public)
	r.Get("/openapi.yaml", OpenAPISpec)

	// Public auth endpoints
	r.Group(func(r chi.Router) {
		r.Post("/v1/auth/register", h.Auth.Register)
		r.Post("/v1/auth/login", h.Auth.Login)
		r.Post("/v1/auth/refresh", h.Auth.Refresh)
	})

	// YooKassa webhook (public — called by YooKassa servers, no JWT)
	r.Post("/v1/payments/webhook", h.Payment.Webhook)

	// Protected endpoints
	r.Group(func(r chi.Router) {
		r.Use(auth.JWTMiddleware(h.Config.JWT.Secret, h.Config.JWT.Issuer, auth.WithBlacklist(h.Healthz.Sessions)))

		// Static file serving for local storage (requires auth)
		if h.Config.Storage.Mode == config.StorageModeLocal || h.Config.Storage.Mode == config.StorageModeFilesystem {
			r.Get("/files/*", h.Request.ServeFiles)
		}

		r.Post("/v1/auth/logout", h.Auth.Logout)

		r.With(auth.RequirePerm(auth.PermEKGSubmit)).Post("/v1/ekg/analyze", h.EKG.SubmitEKGAnalyze)
		r.With(auth.RequirePerm(auth.PermEKGSubmit)).Post("/v1/gpt/process", h.GPT.SubmitGPTRequest)

		r.With(auth.RequirePerm(auth.PermJobReadOwn)).Get("/v1/jobs/{id}", h.Request.GetJob)
		r.With(auth.RequirePerm(auth.PermJobReadOwn)).Get("/v1/requests/{id}", h.Request.GetRequest)
		r.With(auth.RequirePerm(auth.PermJobReadOwn)).Get("/v1/requests/{id}/files/{fileId}", h.Request.GetRequestFile)
		r.With(auth.RequirePerm(auth.PermJobReadOwn)).Get("/v1/requests", h.Request.GetUserRequests)

		// SSE event stream
		r.Get("/v1/events", h.Events.StreamEvents)

		// RAG knowledge base
		r.Post("/v1/rag/query", h.RAG.Query)
		r.Post("/v1/rag/feedback", h.RAG.Feedback)

		// Profile
		r.Get("/v1/me", h.Profile.GetMe)

		// Payments & quota
		r.Get("/v1/quota", h.Payment.GetQuota)
		r.Post("/v1/payments", h.Payment.CreatePayment)

		// Admin-only endpoints
		r.With(auth.RequirePerm(auth.PermAdminAll)).Get("/ready", h.Healthz.Ready)
	})
}
