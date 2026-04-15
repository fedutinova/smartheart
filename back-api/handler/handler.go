package handler

import (
	"context"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/fedutinova/smartheart/back-api/gpt"

	"github.com/fedutinova/smartheart/back-api/auth"
	"github.com/fedutinova/smartheart/back-api/config"
	"github.com/fedutinova/smartheart/back-api/job"
	"github.com/fedutinova/smartheart/back-api/notify"
	"github.com/fedutinova/smartheart/back-api/repository"
	"github.com/fedutinova/smartheart/back-api/service"
	"github.com/fedutinova/smartheart/back-api/storage"
)

// AuthHandler handles authentication endpoints (register, login, refresh, logout).
type AuthHandler struct {
	Service service.AuthService
	Config  config.Config
}

// ECGHandler handles EKG submission endpoints.
type ECGHandler struct {
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

// Middleware is an HTTP middleware function.
type Middleware = func(http.Handler) http.Handler

// Middlewares holds per-endpoint middleware injected at construction time.
type Middlewares struct {
	// WebhookIP restricts webhook access to trusted IPs.
	WebhookIP Middleware
	// AnalyzeRateLimit is a stricter per-endpoint rate limit for analysis submission.
	AnalyzeRateLimit Middleware
	// SubscriptionRateLimit is a stricter per-endpoint rate limit for subscription creation.
	SubscriptionRateLimit Middleware
}

// Handler composes all focused handlers and registers routes.
type Handler struct {
	Auth    *AuthHandler
	EKG     *ECGHandler
	EKGSync *ECGSyncHandler // non-nil when ECG_SYNC_MODE=true
	GPT     *GPTHandler
	Request *RequestHandler
	Healthz *HealthHandler
	Events  *EventsHandler
	RAG     *RAGHandler
	Payment *PaymentHandler
	Profile *ProfileHandler
	Admin   *AdminHandler
	Config  config.Config
	MW      Middlewares
	MockGPT *gpt.MockProcessor // non-nil when GPT_MOCK=true; exposes /debug/h2
}

// ECGSyncProcessor is implemented by workers.ECGWorker to avoid a direct import.
type ECGSyncProcessor interface {
	UploadForSync(ctx context.Context, filename string, reader io.Reader, contentType string) (string, error)
	ProcessECGSync(ctx context.Context, payload *job.ECGJobPayload) error
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
	mw Middlewares,
) *Handler {
	return &Handler{
		Auth:    &AuthHandler{Service: authSvc, Config: cfg},
		EKG:     &ECGHandler{Service: submissionSvc},
		GPT:     &GPTHandler{Service: submissionSvc},
		Request: &RequestHandler{Service: requestSvc, Config: cfg, Storage: storageService},
		Healthz: &HealthHandler{Queue: queue, Repo: repo, Sessions: sessions, Storage: storageService},
		Events:  &EventsHandler{Hub: hub},
		RAG:     NewRAGHandler(cfg.RAG.URL, repo),
		Payment: &PaymentHandler{Service: paymentSvc},
		Profile: &ProfileHandler{Repo: repo},
		Admin:   &AdminHandler{Repo: repo},
		Config:  cfg,
		MW:      mw,
	}
}

// RegisterRoutes registers all HTTP routes.
func (h *Handler) RegisterRoutes(r chi.Router) {
	// Health check (no auth required, for load balancer)
	r.Get("/health", h.Healthz.Health)

	// Debug endpoint for H2 hypothesis testing (mock mode only).
	if h.MockGPT != nil {
		r.Get("/debug/h2", func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(w, http.StatusOK, map[string]int64{
				"max_concurrent_gpt": h.MockGPT.ConcurrentMax(),
			})
		})
		r.Post("/debug/h2/reset", func(w http.ResponseWriter, _ *http.Request) {
			h.MockGPT.ResetConcurrentMax()
			writeJSON(w, http.StatusOK, map[string]string{"status": "reset"})
		})
	}

	// OpenAPI spec (public)
	r.Get("/openapi.yaml", OpenAPISpec)

	// Public auth endpoints
	r.Group(func(r chi.Router) {
		r.Post("/v1/auth/register", h.Auth.Register)
		r.Post("/v1/auth/login", h.Auth.Login)
		r.Post("/v1/auth/refresh", h.Auth.Refresh)
	})

	// YooKassa webhook (public — called by YooKassa servers, no JWT)
	if h.MW.WebhookIP != nil {
		r.With(h.MW.WebhookIP).Post("/v1/payments/webhook", h.Payment.Webhook)
	} else {
		r.Post("/v1/payments/webhook", h.Payment.Webhook)
	}

	// Protected endpoints
	r.Group(func(r chi.Router) {
		r.Use(auth.JWTMiddleware(h.Config.JWT.Secret, h.Config.JWT.Issuer, auth.WithBlacklist(h.Healthz.Sessions)))

		// Static file serving for local storage (requires auth)
		if h.Config.Storage.Mode == config.StorageModeLocal || h.Config.Storage.Mode == config.StorageModeFilesystem {
			r.Get("/files/*", h.Request.ServeFiles)
		}

		r.Post("/v1/auth/logout", h.Auth.Logout)

		ekgMiddleware := []func(http.Handler) http.Handler{auth.RequirePerm(auth.PermECGSubmit)}
		if h.MW.AnalyzeRateLimit != nil {
			ekgMiddleware = append(ekgMiddleware, h.MW.AnalyzeRateLimit)
		}
		if h.EKGSync != nil {
			r.With(ekgMiddleware...).Post("/v1/ecg/analyze", h.EKGSync.SubmitECGAnalyzeSync)
		} else {
			r.With(ekgMiddleware...).Post("/v1/ecg/analyze", h.EKG.SubmitECGAnalyze)
		}
		r.With(ekgMiddleware...).Post("/v1/gpt/process", h.GPT.SubmitGPTRequest)

		r.With(auth.RequirePerm(auth.PermJobReadOwn)).Get("/v1/jobs/{id}", h.Request.GetJob)
		r.With(auth.RequirePerm(auth.PermJobReadOwn)).Get("/v1/requests/{id}", h.Request.GetRequest)
		r.With(auth.RequirePerm(auth.PermJobReadOwn)).Get("/v1/requests/{id}/files/{fileId}/url", h.Request.GetRequestFileURL)
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
		if h.MW.SubscriptionRateLimit != nil {
			r.With(h.MW.SubscriptionRateLimit).Post("/v1/subscriptions", h.Payment.CreateSubscription)
		} else {
			r.Post("/v1/subscriptions", h.Payment.CreateSubscription)
		}

		// Admin-only endpoints
		r.With(auth.RequirePerm(auth.PermAdminAll)).Get("/ready", h.Healthz.Ready)
		r.Route("/v1/admin", func(r chi.Router) {
			r.Use(auth.RequirePerm(auth.PermAdminAll))
			r.Get("/stats", h.Admin.GetStats)
			r.Get("/users", h.Admin.ListUsers)
			r.Get("/payments", h.Admin.ListPayments)
			r.Get("/feedback", h.Admin.ListFeedback)
		})
	})
}
