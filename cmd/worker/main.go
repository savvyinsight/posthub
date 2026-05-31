// Package main is the entry point for the posthub worker process.
//
// The worker consumes tasks from the Asynq queue and processes them
// by loading content, transforming it, and publishing to platforms.
//
// This is a bootstrap stub. The actual Asynq worker will be implemented
// when the queue and storage layers are built.
//
// Usage:
//
//	go run cmd/worker/main.go
//	WORKER_CONCURRENCY=5 go run cmd/worker/main.go
package main

import (
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

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

	log.Info("starting posthub worker",
		"concurrency", cfg.WorkerConcurrency,
		"environment", cfg.Environment,
	)

	// TODO: Initialize Asynq server when queue layer is implemented
	// srv := asynq.NewServer(
	//     asynq.RedisClientOpt{Addr: cfg.RedisURL},
	//     asynq.Config{Concurrency: cfg.WorkerConcurrency},
	// )
	//
	// mux := asynq.NewServeMux()
	// mux.HandleFunc(queue.TypePublishContent, handler.HandlePublish)
	//
	// if err := srv.Run(mux); err != nil {
	//     log.Error("worker error", "error", err)
	//     os.Exit(1)
	// }

	log.Info("worker stub: waiting for shutdown signal (queue not yet implemented)")

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit

	log.Info("shutdown signal received", "signal", sig)
	log.Info("posthub worker stopped")
}
