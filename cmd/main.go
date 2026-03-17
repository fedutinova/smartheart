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

	// Structured JSON logging for production readiness.
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

	if err := cfg.Validate(); err != nil {
		slog.Error("invalid configuration", "err", err)
		os.Exit(1)
	}

	if err := auth.ValidateSecret(cfg.JWT.Secret); err != nil {
		slog.Error("invalid configuration", "err", err)
		os.Exit(1)
	}

	slog.Info("starting smartheart", "addr", cfg.HTTPAddr, "workers", cfg.Queue.Workers, "version", Version, "commit", Commit)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db, err := database.NewDB(ctx, cfg.DB.URL, func(pc *database.PoolConfig) {
		pc.MaxConns = int32(cfg.DB.MaxConns)
		pc.MinConns = int32(cfg.DB.MinConns)
	})
	if err != nil {
		slog.Error("failed to connect to database", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	storageService, err := storage.NewStorage(ctx, cfg)
	if err != nil {
		slog.Error("failed to initialize storage", "err", err)
		os.Exit(1)
	}

	storageType := storage.GetStorageType(cfg)
	slog.Info("storage initialized", "type", storageType)

	sessions, err := session.New(cfg.RedisURL)
	if err != nil {
		slog.Error("failed to connect to Redis", "err", err)
		os.Exit(1)
	}
	defer sessions.Close()

	gptClient := gpt.NewClient(cfg.GPT.APIKey, storageService, gpt.WithModel(cfg.GPT.Model))
	repo := repository.New(db, repository.WithQueryTimeout(cfg.DB.QueryTimeout))

	// Load role→permissions mapping from DB so auth middleware uses DB as source of truth
	if rolePerms, err := repo.LoadRolePermissions(ctx); err != nil {
		slog.Warn("failed to load role permissions from DB, using defaults", "err", err)
	} else if len(rolePerms) > 0 {
		auth.InitPermsFromDB(rolePerms)
		slog.Info("loaded role permissions from DB", "roles", len(rolePerms))
	}

	// Initialize job queue based on configuration
	var q job.Queue
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
		q = redisQueue
		slog.Info("using Redis Streams queue", "stream", cfg.Queue.Stream, "group", cfg.Queue.Group)
	default:
		q = queue.NewMemoryQueue(cfg.Queue.Buffer, cfg.Queue.MaxDuration)
		slog.Warn("using in-memory queue (not recommended for production)")
	}
	defer q.Close()

	// Notification hub for SSE
	hub := notify.NewHub()

	gptWorker := workers.NewGPTWorker(db, gptClient, repo, hub)
	ekgWorker := workers.NewEKGWorker(db, q, storageService, repo)

	authSvc := service.NewAuthService(repo, sessions, cfg.JWT)
	submissionSvc := service.NewSubmissionService(repo, q, storageService, cfg.Quota)
	requestSvc := service.NewRequestService(repo, q)

	handlers := handler.NewHandler(authSvc, submissionSvc, requestSvc, q, repo, sessions, storageService, hub, cfg)
	r := server.NewRouter(handlers, cfg)

	// Register job handlers
	registry := job.NewRegistry()
	registry.Register(job.TypeEKGAnalyze, ekgWorker.HandleEKGJob)
	registry.Register(job.TypeGPTProcess, gptWorker.HandleGPTJob)

	// Start queue consumers
	q.StartConsumers(ctx, cfg.Queue.Workers, registry.Dispatch)

	srv := &http.Server{
		Addr:         cfg.HTTPAddr,
		Handler:      r,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  90 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		slog.Info("received signal", "signal", sig)
	case err := <-errCh:
		slog.Error("server error", "err", err)
	}
	slog.Info("shutting down")

	shCtx, shCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shCancel()
	if err := srv.Shutdown(shCtx); err != nil {
		slog.Error("HTTP server shutdown error", "err", err)
	}
	cancel()
}
