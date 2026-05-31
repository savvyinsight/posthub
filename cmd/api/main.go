// Package main is the entry point for the posthub API server.
//
// It starts an HTTP server that serves the REST API for content management
// and publish orchestration. The server uses chi for routing and slog
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
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/savvyinsight/posthub/internal/config"
	"github.com/savvyinsight/posthub/internal/logger"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	log := logger.New(cfg.LogLevel, cfg.Environment)
	slog.SetDefault(log)

	log.Info("starting posthub api",
		"port", cfg.APIPort,
		"environment", cfg.Environment,
	)

	// Build router
	r := buildRouter(log)

	// Create server
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.APIPort),
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	errCh := make(chan error, 1)
	go func() {
		log.Info("api server listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-quit:
		log.Info("shutdown signal received", "signal", sig)
	case err := <-errCh:
		log.Error("server error", "error", err)
	}

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Error("shutdown error", "error", err)
		os.Exit(1)
	}

	log.Info("api server stopped")
}

// buildRouter creates the HTTP router with all routes and middleware.
func buildRouter(log *slog.Logger) *chi.Mux {
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(LoggerMiddleware(log))
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))

	// Health check
	r.Get("/health", handleHealth)

	// API v1 routes (stubs — handlers will be added when business logic is implemented)
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(middleware.SetHeader("Content-Type", "application/json"))

		// Content endpoints
		// r.Post("/content", contentHandler.Create)
		// r.Get("/content", contentHandler.List)
		// r.Get("/content/{id}", contentHandler.Get)
		// r.Put("/content/{id}", contentHandler.Update)
		// r.Delete("/content/{id}", contentHandler.Delete)

		// Publishing endpoints
		// r.Post("/content/{id}/publish", publishHandler.Publish)
		// r.Get("/content/{id}/jobs", publishHandler.ListJobs)
		// r.Get("/jobs/{id}", publishHandler.GetJob)
	})

	return r
}

// handleHealth returns a health check response.
func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"status":"ok","service":"posthub"}`)
}

// LoggerMiddleware returns middleware that logs each request.
func LoggerMiddleware(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			defer func() {
				log.Info("request completed",
					"method", r.Method,
					"path", r.URL.Path,
					"status", ww.Status(),
					"bytes", ww.BytesWritten(),
					"duration_ms", time.Since(start).Milliseconds(),
					"request_id", middleware.GetReqID(r.Context()),
				)
			}()

			next.ServeHTTP(ww, r)
		})
	}
}
