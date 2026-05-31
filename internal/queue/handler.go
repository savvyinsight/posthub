// TaskHandler processes publish tasks from the queue.
//
// It orchestrates the full lifecycle: idempotency check → state transition →
// publish → result recording. Retryable errors are returned to Asynq for
// automatic retry; permanent failures mark the task as dead.
package queue

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/savvyinsight/posthub/internal/contracts"
	"github.com/savvyinsight/posthub/internal/logger"
	"go.uber.org/zap"
)

// TaskStateStore persists task lifecycle state.
//
// The concrete implementation bridges to storage.PublishTaskStore
// and is wired in cmd/worker.
type TaskStateStore interface {
	// UpdateTaskStatus transitions the task to a new status.
	UpdateTaskStatus(ctx context.Context, taskID string, status contracts.PublishTaskStatus, errMsg string) error
	// RecordPublishResult stores the outcome of a successful publish.
	RecordPublishResult(ctx context.Context, taskID string, result *contracts.PublishResult) error
}

// TaskHandler implements Handler for publish tasks.
//
// It uses a Publisher for the actual platform call, a TaskStateStore
// for persistence, and optional IdempotencyGuard for deduplication.
type TaskHandler struct {
	publisher   Publisher
	stateStore  TaskStateStore
	idempotency IdempotencyGuard // nil = disabled
	log         *logger.Logger
	metrics     Metrics
}

// Compile-time interface check.
var _ Handler = (*TaskHandler)(nil)

// NewTaskHandler creates a handler with the given dependencies.
// If metrics is nil, NopMetrics is used. If idempotency is nil,
// duplicate detection is disabled.
func NewTaskHandler(
	publisher Publisher,
	store TaskStateStore,
	idem IdempotencyGuard,
	log *logger.Logger,
	m Metrics,
) *TaskHandler {
	if m == nil {
		m = NopMetrics{}
	}
	return &TaskHandler{
		publisher:   publisher,
		stateStore:  store,
		idempotency: idem,
		log:         log,
		metrics:     m,
	}
}

// idempotencyTTL is how long a processing lock is held.
// Should exceed the maximum task processing time.
const idempotencyTTL = 10 * time.Minute

// HandlePublish processes a single publish task.
//
// Flow:
//  1. Check idempotency guard (skip if duplicate)
//  2. Transition task to processing
//  3. Call publisher
//  4. On success: record result, transition to succeeded
//  5. On permanent failure: transition to dead, return nil (no retry)
//  6. On retryable failure: return error for Asynq retry
func (h *TaskHandler) HandlePublish(ctx context.Context, payload PublishPayload) error {
	start := time.Now()

	// Enrich context with structured fields for downstream logging.
	ctx = logger.WithJobID(ctx, payload.TaskID)
	ctx = logger.WithPlatform(ctx, payload.Platform)

	h.log.Info("processing publish task",
		zap.String("task_id", payload.TaskID),
		zap.String("content_id", payload.ContentID),
		zap.String("platform", payload.Platform),
	)
	h.metrics.TaskStarted(payload.TaskID, payload.Platform)

	// Idempotency check.
	if h.idempotency != nil {
		key := payload.ContentID + ":" + payload.Platform
		acquired, err := h.idempotency.Acquire(ctx, key, idempotencyTTL)
		if err != nil {
			// Log but don't fail — idempotency is best-effort.
			h.log.Warn("idempotency check failed, proceeding anyway", zap.Error(err))
		} else if !acquired {
			h.metrics.IdempotencyHit(key)
			h.log.Warn("duplicate task detected, skipping",
				zap.String("idempotency_key", key),
			)
			return nil
		}
	}

	// Transition to processing.
	if err := h.stateStore.UpdateTaskStatus(ctx, payload.TaskID, contracts.PublishTaskStatusProcessing, ""); err != nil {
		h.log.Error("failed to transition task to processing", zap.Error(err))
		return fmt.Errorf("set processing: %w", err)
	}

	// Execute the publish.
	result, err := h.publisher.Publish(ctx, payload.ContentID, payload.Platform)
	duration := time.Since(start)

	if err != nil {
		return h.handleFailure(ctx, payload, err, duration)
	}

	// Success path.
	if err := h.stateStore.RecordPublishResult(ctx, payload.TaskID, result); err != nil {
		h.log.Error("failed to record publish result", zap.Error(err))
		return fmt.Errorf("record result: %w", err)
	}

	if err := h.stateStore.UpdateTaskStatus(ctx, payload.TaskID, contracts.PublishTaskStatusSucceeded, ""); err != nil {
		h.log.Error("failed to transition task to succeeded", zap.Error(err))
		return fmt.Errorf("set succeeded: %w", err)
	}

	h.metrics.TaskCompleted(payload.TaskID, payload.Platform, duration)
	h.log.Info("publish task succeeded",
		zap.String("platform_post_id", result.PlatformPostID),
		zap.String("platform_url", result.PlatformURL),
		zap.Duration("duration", duration),
	)

	return nil
}

// handleFailure classifies the error and either marks the task dead
// (permanent) or returns the error for Asynq retry (retryable).
func (h *TaskHandler) handleFailure(ctx context.Context, payload PublishPayload, err error, duration time.Duration) error {
	h.metrics.TaskFailed(payload.TaskID, payload.Platform, err)

	if isPermanentError(err) {
		if dbErr := h.stateStore.UpdateTaskStatus(ctx, payload.TaskID, contracts.PublishTaskStatusDead, err.Error()); dbErr != nil {
			h.log.Error("failed to mark task dead", zap.Error(dbErr))
		}
		h.metrics.TaskDead(payload.TaskID, payload.Platform)
		h.log.Error("publish task permanently failed",
			zap.Error(err),
			zap.Duration("duration", duration),
		)
		// Return nil — Asynq should NOT retry permanent failures.
		return nil
	}

	// Retryable: log and return error for Asynq to re-enqueue.
	h.log.Warn("publish task failed, will retry",
		zap.Error(err),
		zap.Duration("duration", duration),
	)
	return fmt.Errorf("publish failed (retryable): %w", err)
}

// isPermanentError determines if an error should NOT be retried.
//
// Permanent errors:
//   - non-retryable PlatformError
//   - validation errors (bad input)
//   - not-found errors (content/platform doesn't exist)
func isPermanentError(err error) bool {
	// Non-retryable platform error.
	var pErr *contracts.PlatformError
	if errors.As(err, &pErr) && !pErr.IsRetryable() {
		return true
	}

	// Application errors by code.
	var appErr *contracts.AppError
	if errors.As(err, &appErr) {
		switch appErr.Code {
		case contracts.ErrCodeValidation, contracts.ErrCodeNotFound, contracts.ErrCodeForbidden:
			return true
		}
	}

	return false
}
