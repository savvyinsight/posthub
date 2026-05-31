// Package main is the entry point for the posthub API server.
//
// It starts an HTTP server that serves the REST API for content management
// and publish orchestration. The server uses chi for routing and zap
// for structured logging.
//
// Usage:
//
//	go run cmd/api/main.go
//	API_PORT=9090 go run cmd/api/main.go
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"

	"github.com/savvyinsight/posthub/internal/api"
	"github.com/savvyinsight/posthub/internal/config"
	"github.com/savvyinsight/posthub/internal/logger"
	"github.com/savvyinsight/posthub/internal/queue"
	"github.com/savvyinsight/posthub/internal/storage"
	"github.com/savvyinsight/posthub/internal/web"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	log := logger.New(cfg.Logging.Level, cfg.Environment)

	log.Info("starting posthub api",
		zap.Int("port", cfg.API.Port),
		zap.String("environment", cfg.Environment),
	)

	// Initialize storage (in-memory for MVP).
	ms := storage.NewMemoryStorage()

	// Initialize queue enqueuer.
	// Use a no-op enqueuer when Redis is not configured.
	var enqueuer queue.Enqueuer = &noOpEnqueuer{log: log}

	// Build router with handlers.
	hh := &api.HealthHandler{Version: "0.1.0"}
	ph := &api.PublishHandler{
		ContentStore: ms.Content,
		TaskStore:    ms.Tasks,
		Enqueuer:     enqueuer,
	}
	r := buildRouter(log, hh, ph)

	// Create server
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.API.Port),
		Handler:      r,
		ReadTimeout:  cfg.API.ReadTimeout,
		WriteTimeout: cfg.API.WriteTimeout,
		IdleTimeout:  cfg.API.IdleTimeout,
	}

	// Start server in goroutine
	errCh := make(chan error, 1)
	go func() {
		log.Info("api server listening", zap.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-quit:
		log.Info("shutdown signal received", zap.String("signal", sig.String()))
	case err := <-errCh:
		log.Error("server error", zap.Error(err))
	}

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Error("shutdown error", zap.Error(err))
		os.Exit(1)
	}

	_ = log.Sync()
	log.Info("api server stopped")
}

// buildRouter creates the HTTP router with all routes and middleware.
func buildRouter(log *logger.Logger, hh *api.HealthHandler, ph *api.PublishHandler) *chi.Mux {
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(api.RequestLoggerMiddleware(log))
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))

	// Health check (JSON)
	r.Get("/api/health", hh.HandleHealth)

	// Web UI
	wh := web.NewHandler(ph.ContentStore, ph.TaskStore)
	wh.Routes(r)

	// API v1 routes
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(middleware.SetHeader("Content-Type", "application/json"))

		// Publishing endpoints
		r.Post("/publish", ph.HandlePublish)
		r.Get("/publish/{id}", ph.HandlePublishStatus)
	})

	return r
}

// noOpEnqueuer is a queue.Enqueueuer that logs but does not enqueue.
// Used when Redis is not available (MVP / local dev without Redis).
type noOpEnqueuer struct {
	log *logger.Logger
}

func (n *noOpEnqueuer) EnqueuePublish(_ context.Context, payload queue.PublishPayload, _ queue.EnqueueOptions) error {
	n.log.Warn("no-op enqueuer: task not enqueued (Redis not configured)",
		zap.String("task_id", payload.TaskID),
		zap.String("content_id", payload.ContentID),
		zap.String("platform", payload.Platform),
	)
	return nil
}
