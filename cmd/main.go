package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/fedutinova/smartheart/internal/auth"
	appconfig "github.com/fedutinova/smartheart/internal/config"
	"github.com/fedutinova/smartheart/internal/database"
	"github.com/fedutinova/smartheart/internal/gpt"
	"github.com/fedutinova/smartheart/internal/job"
	"github.com/fedutinova/smartheart/internal/memq"
	"github.com/fedutinova/smartheart/internal/queue"
	"github.com/fedutinova/smartheart/internal/redis"
	"github.com/fedutinova/smartheart/internal/repository"
	"github.com/fedutinova/smartheart/internal/handler"
	"github.com/fedutinova/smartheart/internal/server"
	"github.com/fedutinova/smartheart/internal/storage"
	"github.com/fedutinova/smartheart/internal/workers"
)

func main() {
	cfg := appconfig.Load()
	slog.Info("starting smartheart", "addr", cfg.HTTPAddr, "workers", cfg.QueueWorkers)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db, err := database.NewDB(ctx, cfg.DatabaseURL)
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

	redisService, err := redis.New(cfg.RedisURL)
	if err != nil {
		slog.Error("failed to connect to Redis", "err", err)
		os.Exit(1)
	}
	defer redisService.Close()

	gptClient := gpt.NewClient(cfg.OpenAIAPIKey, storageService)
	repo := repository.New(db)

	// Load role→permissions mapping from DB so auth middleware uses DB as source of truth
	if rolePerms, err := repo.LoadRolePermissions(ctx); err != nil {
		slog.Warn("failed to load role permissions from DB, using defaults", "err", err)
	} else if len(rolePerms) > 0 {
		auth.InitPermsFromDB(rolePerms)
		slog.Info("loaded role permissions from DB", "roles", len(rolePerms))
	}

	// Initialize job queue based on configuration
	var q memq.JobQueue
	switch cfg.QueueMode {
	case "redis":
		redisQueue, err := queue.NewRedisQueue(redisService.Client(), queue.RedisQueueConfig{
			Stream:        cfg.QueueStream,
			Group:         cfg.QueueGroup,
			MaxJobTime:    cfg.JobMaxDuration,
			ClaimInterval: 10 * time.Second,
			ClaimTimeout:  cfg.JobClaimTimeout,
		})
		if err != nil {
			slog.Error("failed to create Redis queue", "err", err)
			os.Exit(1)
		}
		q = redisQueue
		slog.Info("using Redis Streams queue", "stream", cfg.QueueStream, "group", cfg.QueueGroup)
	default:
		q = memq.NewMemoryQueue(cfg.QueueBuf, cfg.JobMaxDuration)
		slog.Warn("using in-memory queue (not recommended for production)")
	}
	defer q.Close()

	gptHandler := workers.NewGPTHandler(db, gptClient)
	ekgHandler := workers.NewEKGHandler(db, q, storageService, repo)

	handlers := &handler.Handlers{
		Q:       q,
		Repo:    repo,
		Storage: storageService,
		Redis:   redisService,
		Config:  cfg,
	}
	r := server.NewRouter(handlers, cfg)

	// Job handler function
	jobHandler := func(ctx context.Context, j *job.Job) error {
		switch j.Type {
		case job.TypeEKGAnalyze:
			return ekgHandler.HandleEKGJob(ctx, j)
		case job.TypeGPTProcess:
			return gptHandler.HandleGPTJob(ctx, j)
		default:
			return fmt.Errorf("unknown job type: %s", j.Type)
		}
	}

	// Start queue consumers
	q.StartConsumers(ctx, cfg.QueueWorkers, jobHandler)

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
	_ = srv.Shutdown(shCtx)
	cancel()
}
