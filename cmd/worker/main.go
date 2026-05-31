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
	"os"
	"os/signal"
	"syscall"

	"go.uber.org/zap"

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
	log := logger.New(cfg.Logging.Level, cfg.Environment)

	log.Info("starting posthub worker",
		zap.Int("concurrency", cfg.Queue.Concurrency),
		zap.String("environment", cfg.Environment),
	)

	// TODO: Initialize Asynq server when queue layer is implemented
	// srv := asynq.NewServer(
	//     asynq.RedisClientOpt{Addr: cfg.Redis.URL},
	//     asynq.Config{Concurrency: cfg.Queue.Concurrency},
	// )
	//
	// mux := asynq.NewServeMux()
	// mux.HandleFunc(queue.TypePublishContent, handler.HandlePublish)
	//
	// if err := srv.Run(mux); err != nil {
	//     log.Error("worker error", zap.Error(err))
	//     os.Exit(1)
	// }

	log.Info("worker stub: waiting for shutdown signal (queue not yet implemented)")

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit

	log.Info("shutdown signal received", zap.String("signal", sig.String()))
	_ = log.Sync()
	log.Info("posthub worker stopped")
}
