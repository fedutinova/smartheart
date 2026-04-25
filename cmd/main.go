package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/fedutinova/smartheart/back-api/auth"
	appconfig "github.com/fedutinova/smartheart/back-api/config"
	"github.com/fedutinova/smartheart/back-api/database"
	"github.com/fedutinova/smartheart/back-api/gpt"
	"github.com/fedutinova/smartheart/back-api/handler"
	"github.com/fedutinova/smartheart/back-api/job"
	"github.com/fedutinova/smartheart/back-api/mail"
	"github.com/fedutinova/smartheart/back-api/notify"
	"github.com/fedutinova/smartheart/back-api/queue"
	"github.com/fedutinova/smartheart/back-api/repository"
	"github.com/fedutinova/smartheart/back-api/server"
	"github.com/fedutinova/smartheart/back-api/service"
	"github.com/fedutinova/smartheart/back-api/session"
	"github.com/fedutinova/smartheart/back-api/storage"
	"github.com/fedutinova/smartheart/back-api/workers"
)

// Set at build time via -ldflags.
var (
	Version = "dev"
	Commit  = "unknown"
)

func main() {
	cfg := appconfig.Load()
	initLogger()
	validateConfig(cfg)

	slog.Info("starting smartheart", "addr", cfg.HTTPAddr, "workers", cfg.Queue.Workers, "version", Version, "commit", Commit)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db, sessions, storageService := initInfra(ctx, cfg)
	defer db.Close()
	defer func() { _ = sessions.Close() }()

	runMigrations(ctx, db)

	repo := repository.New(db, repository.WithQueryTimeout(cfg.DB.QueryTimeout))
	loadPermissions(ctx, repo)

	q := initQueue(cfg, sessions)
	defer func() { _ = q.Close() }()

	hub := notify.NewHub()
	var gptClient gpt.Processor
	if os.Getenv("GPT_MOCK") == "true" {
		mockDelay, _ := time.ParseDuration(os.Getenv("GPT_MOCK_DELAY"))
		if mockDelay == 0 {
			mockDelay = 5 * time.Second
		}
		slog.Warn("GPT_MOCK enabled — using simulated responses", "delay", mockDelay)
		gptClient = &gpt.MockProcessor{Delay: mockDelay}
	} else {
		gptClient = gpt.NewClient(cfg.GPT.APIKey, storageService, gpt.WithModel(cfg.GPT.Model))
	}
	startWorkers(ctx, cfg, db, q, storageService, repo, hub, gptClient)
	srv := startHTTPServer(cfg, repo, sessions, storageService, q, hub)

	// Cancel pending payments older than 1 hour, check every 10 minutes.
	service.StartStalePaymentCleaner(ctx, repo, 10*time.Minute, 1*time.Hour)

	waitForShutdown(srv, cancel)
}

func initLogger() {
	logLevel := slog.LevelInfo
	if v := os.Getenv("LOG_LEVEL"); v != "" {
		switch v {
		case "debug":
			logLevel = slog.LevelDebug
		case "warn":
			logLevel = slog.LevelWarn
		case "error":
			logLevel = slog.LevelError
		}
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level:     logLevel,
		AddSource: logLevel == slog.LevelDebug,
	})))
}

func runMigrations(ctx context.Context, db *database.DB) {
	migrationsDir := envMigrationsDir()
	if err := db.Migrate(ctx, migrationsDir); err != nil {
		slog.Error("failed to run migrations", "dir", migrationsDir, "err", err)
		os.Exit(1)
	}
}

func envMigrationsDir() string {
	if v := os.Getenv("MIGRATIONS_DIR"); v != "" {
		return v
	}

	return "./migrations"
}

func validateConfig(cfg appconfig.Config) {
	if err := cfg.Validate(); err != nil {
		slog.Error("invalid configuration", "err", err)
		os.Exit(1)
	}
	if err := auth.ValidateSecret(cfg.JWT.Secret); err != nil {
		slog.Error("invalid configuration", "err", err)
		os.Exit(1)
	}
}

func initInfra(ctx context.Context, cfg appconfig.Config) (*database.DB, *session.Service, storage.Storage) {
	db, err := database.NewDB(ctx, cfg.DB.URL, func(pc *database.PoolConfig) {
		pc.MaxConns = int32(cfg.DB.MaxConns)
		pc.MinConns = int32(cfg.DB.MinConns)
	})
	if err != nil {
		slog.Error("failed to connect to database", "err", err)
		os.Exit(1)
	}

	storageService, err := storage.NewStorage(ctx, cfg)
	if err != nil {
		slog.Error("failed to initialize storage", "err", err)
		os.Exit(1)
	}
	slog.Info("storage initialized", "type", storage.GetStorageType(cfg))

	sessions, err := session.New(cfg.RedisURL)
	if err != nil {
		slog.Error("failed to connect to Redis", "err", err)
		os.Exit(1)
	}

	return db, sessions, storageService
}

func loadPermissions(ctx context.Context, repo repository.Store) {
	rolePerms, err := repo.LoadRolePermissions(ctx)
	if err != nil {
		slog.Warn("failed to load role permissions from DB, using defaults", "err", err)
		return
	}
	if len(rolePerms) > 0 {
		auth.InitPermsFromDB(rolePerms)
		slog.Info("loaded role permissions from DB", "roles", len(rolePerms))
	}
}

func initQueue(cfg appconfig.Config, sessions *session.Service) job.Queue {
	switch cfg.Queue.Mode {
	case appconfig.QueueModeRedis:
		redisQueue, err := queue.NewRedisQueue(sessions.Client(), queue.RedisQueueConfig{
			Stream:        cfg.Queue.Stream,
			Group:         cfg.Queue.Group,
			MaxJobTime:    cfg.Queue.MaxDuration,
			ClaimInterval: 10 * time.Second,
			ClaimTimeout:  cfg.Queue.ClaimTimeout,
		})
		if err != nil {
			slog.Error("failed to create Redis queue", "err", err)
			os.Exit(1)
		}
		slog.Info("using Redis Streams queue", "stream", cfg.Queue.Stream, "group", cfg.Queue.Group)
		return redisQueue
	default:
		slog.Warn("using in-memory queue (not recommended for production)")
		return queue.NewMemoryQueue(cfg.Queue.Buffer, cfg.Queue.MaxDuration)
	}
}

func startWorkers(ctx context.Context, cfg appconfig.Config, db *database.DB, q job.Queue, storageService storage.Storage, repo repository.Store, hub *notify.Hub, gptClient gpt.Processor) {
	gptWorker := workers.NewGPTWorker(db, gptClient, repo, hub)
	ecgWorker := workers.NewECGWorker(db, q, storageService, repo, gptClient, hub)

	registry := job.NewRegistry()
	registry.Register(job.TypeECGAnalyze, ecgWorker.HandleECGJob)
	registry.Register(job.TypeGPTProcess, gptWorker.HandleGPTJob)

	q.StartConsumers(ctx, cfg.Queue.Workers, registry.Dispatch)
}

func startHTTPServer(
	cfg appconfig.Config,
	repo repository.Store,
	sessions *session.Service,
	storageService storage.Storage,
	q job.Queue,
	hub *notify.Hub,
) *http.Server {
	authSvc := service.NewAuthService(repo, sessions, cfg.JWT)
	mailer := mail.NewSender(cfg.SMTP)
	passwordSvc := service.NewPasswordService(repo, sessions, mailer, cfg)
	submissionSvc := service.NewSubmissionService(repo, q, storageService, cfg.Quota)
	requestSvc := service.NewRequestService(repo, q)
	paymentSvc := service.NewPaymentService(repo, cfg.YooKassa, cfg.Quota.DailyLimit)

	mw := handler.Middlewares{
		WebhookIP: server.WebhookIPWhitelist(cfg.YooKassa.ShopID),
	}
	if cfg.RateLimit.RPM > 0 {
		if cfg.RateLimit.AnalyzeRPM > 0 {
			mw.AnalyzeRateLimit = server.EndpointRateLimit(cfg.RateLimit.AnalyzeRPM)
		}
		if cfg.RateLimit.SubscriptionRPM > 0 {
			mw.SubscriptionRateLimit = server.EndpointRateLimit(cfg.RateLimit.SubscriptionRPM)
		}
		if cfg.RateLimit.PasswordResetRPM > 0 {
			mw.PasswordResetRateLimit = server.EndpointRateLimit(cfg.RateLimit.PasswordResetRPM)
		}
	}
	handlers := handler.NewHandler(authSvc, passwordSvc, submissionSvc, requestSvc, paymentSvc, q, repo, sessions, storageService, hub, cfg, mw)
	r := server.NewRouter(handlers, cfg)

	srv := &http.Server{
		Addr:        cfg.HTTPAddr,
		Handler:     r,
		ReadTimeout: 30 * time.Second,
		IdleTimeout: 90 * time.Second,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	return srv
}

func waitForShutdown(srv *http.Server, cancel context.CancelFunc) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	sig := <-sigCh
	slog.Info("received signal, shutting down", "signal", sig)

	shCtx, shCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shCancel()
	if err := srv.Shutdown(shCtx); err != nil {
		slog.Error("HTTP server shutdown error", "err", err)
	}
	cancel()
}
