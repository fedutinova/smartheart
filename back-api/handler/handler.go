package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/fedutinova/smartheart/back-api/auth"
	"github.com/fedutinova/smartheart/back-api/config"
	"github.com/fedutinova/smartheart/back-api/job"
	"github.com/fedutinova/smartheart/back-api/notify"
	"github.com/fedutinova/smartheart/back-api/repository"
	"github.com/fedutinova/smartheart/back-api/service"
	"github.com/fedutinova/smartheart/back-api/storage"
)

type AuthHandler struct {
	Service service.AuthService
	Config  config.Config
}

type ECGHandler struct {
	Service service.SubmissionService
}

type GPTHandler struct {
	Service service.SubmissionService
}

type RequestHandler struct {
	Service service.RequestService
	Config  config.Config
	Storage storage.Storage
}

type HealthHandler struct {
	Queue    job.Queue
	Repo     repository.Store
	Sessions auth.SessionService
	Storage  storage.Storage
}

type Middleware = func(http.Handler) http.Handler

type Middlewares struct {
	WebhookIP              Middleware
	AnalyzeRateLimit       Middleware
	SubscriptionRateLimit  Middleware
	PasswordResetRateLimit Middleware
}

type Handler struct {
	Auth     *AuthHandler
	Password *PasswordHandler
	EKG      *ECGHandler
	GPT      *GPTHandler
	Request  *RequestHandler
	Healthz  *HealthHandler
	Events   *EventsHandler
	RAG      *RAGHandler
	Payment  *PaymentHandler
	Profile  *ProfileHandler
	Admin    *AdminHandler
	Config   config.Config
	MW       Middlewares
}

func NewHandler(
	authSvc service.AuthService,
	passwordSvc service.PasswordService,
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
		Auth:     &AuthHandler{Service: authSvc, Config: cfg},
		Password: &PasswordHandler{Service: passwordSvc},
		EKG:      &ECGHandler{Service: submissionSvc},
		GPT:      &GPTHandler{Service: submissionSvc},
		Request:  &RequestHandler{Service: requestSvc, Config: cfg, Storage: storageService},
		Healthz:  &HealthHandler{Queue: queue, Repo: repo, Sessions: sessions, Storage: storageService},
		Events:   &EventsHandler{Hub: hub},
		RAG:      NewRAGHandler(cfg.RAG.URL, repo),
		Payment:  &PaymentHandler{Service: paymentSvc},
		Profile:  &ProfileHandler{Repo: repo},
		Admin:    &AdminHandler{Repo: repo},
		Config:   cfg,
		MW:       mw,
	}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/health", h.Healthz.Health)

	r.Get("/openapi.yaml", OpenAPISpec)

	r.Group(func(r chi.Router) {
		r.Post("/v1/auth/register", h.Auth.Register)
		r.Post("/v1/auth/login", h.Auth.Login)
		r.Post("/v1/auth/refresh", h.Auth.Refresh)
		if h.MW.PasswordResetRateLimit != nil {
			r.With(h.MW.PasswordResetRateLimit).Post("/v1/auth/password-reset", h.Password.RequestReset)
		} else {
			r.Post("/v1/auth/password-reset", h.Password.RequestReset)
		}
		r.Post("/v1/auth/password-reset/confirm", h.Password.ConfirmReset)
	})

	if h.MW.WebhookIP != nil {
		r.With(h.MW.WebhookIP).Post("/v1/payments/webhook", h.Payment.Webhook)
	} else {
		r.Post("/v1/payments/webhook", h.Payment.Webhook)
	}

	r.Group(func(r chi.Router) {
		r.Use(auth.JWTMiddleware(h.Config.JWT.Secret, h.Config.JWT.Issuer, auth.WithBlacklist(h.Healthz.Sessions)))

		if h.Config.Storage.Mode == config.StorageModeLocal || h.Config.Storage.Mode == config.StorageModeFilesystem {
			r.Get("/files/*", h.Request.ServeFiles)
		}

		r.Post("/v1/auth/logout", h.Auth.Logout)
		r.Post("/v1/auth/password-change", h.Password.ChangePassword)

		ekgMiddleware := []func(http.Handler) http.Handler{auth.RequirePerm(auth.PermECGSubmit)}
		if h.MW.AnalyzeRateLimit != nil {
			ekgMiddleware = append(ekgMiddleware, h.MW.AnalyzeRateLimit)
		}
		r.With(ekgMiddleware...).Post("/v1/ecg/analyze", h.EKG.SubmitECGAnalyze)
		r.With(ekgMiddleware...).Post("/v1/ecg/analyze-h2-compare", h.EKG.CompareH2Redaction)
		r.With(ekgMiddleware...).Post("/v1/gpt/process", h.GPT.SubmitGPTRequest)

		r.With(auth.RequirePerm(auth.PermJobReadOwn)).Get("/v1/jobs/{id}", h.Request.GetJob)
		r.With(auth.RequirePerm(auth.PermJobReadOwn)).Get("/v1/requests/{id}", h.Request.GetRequest)
		r.With(auth.RequirePerm(auth.PermJobReadOwn)).Get("/v1/requests/{id}/files/{fileId}/url", h.Request.GetRequestFileURL)
		r.With(auth.RequirePerm(auth.PermJobReadOwn)).Get("/v1/requests/{id}/files/{fileId}", h.Request.GetRequestFile)
		r.With(auth.RequirePerm(auth.PermJobReadOwn)).Get("/v1/requests", h.Request.GetUserRequests)

		r.Get("/v1/events", h.Events.StreamEvents)

		r.Post("/v1/rag/query", h.RAG.Query)
		r.Post("/v1/rag/feedback", h.RAG.Feedback)

		r.Get("/v1/me", h.Profile.GetMe)

		r.Get("/v1/quota", h.Payment.GetQuota)
		r.Post("/v1/payments", h.Payment.CreatePayment)
		if h.MW.SubscriptionRateLimit != nil {
			r.With(h.MW.SubscriptionRateLimit).Post("/v1/subscriptions", h.Payment.CreateSubscription)
		} else {
			r.Post("/v1/subscriptions", h.Payment.CreateSubscription)
		}

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
