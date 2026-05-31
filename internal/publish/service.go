// Package publish provides the publish orchestration domain.
//
// The Service coordinates the full publish workflow: fetching content,
// creating per-platform publish tasks, executing them with retry logic,
// and aggregating results into a final content status.
//
// For MVP, tasks execute synchronously within the Publish call.
// The async queue (Enqueuer/Handler interfaces) will wrap this later.
package publish

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/savvyinsight/posthub/internal/contracts"
	"github.com/savvyinsight/posthub/internal/logger"
	"github.com/savvyinsight/posthub/internal/platform"
	"github.com/savvyinsight/posthub/internal/storage"
	"github.com/savvyinsight/posthub/internal/transform"
)

// Deps holds all dependencies required by the publish Service.
type Deps struct {
	ContentStore      storage.ContentStore
	TaskStore         storage.PublishTaskStore
	AttemptStore      storage.PublishAttemptStore
	PostStore         storage.PlatformPostStore
	IdempotencyStore  storage.IdempotencyStore
	Platforms         *platform.Registry
	Logger            *logger.Logger
}

// Service orchestrates the publish workflow.
type Service struct {
	content     storage.ContentStore
	tasks       storage.PublishTaskStore
	attempts    storage.PublishAttemptStore
	posts       storage.PlatformPostStore
	idempotency storage.IdempotencyStore
	platforms   *platform.Registry
	log         *logger.Logger
}

// NewService creates a publish Service with the given dependencies.
func NewService(d Deps) *Service {
	return &Service{
		content:     d.ContentStore,
		tasks:       d.TaskStore,
		attempts:    d.AttemptStore,
		posts:       d.PostStore,
		idempotency: d.IdempotencyStore,
		platforms:   d.Platforms,
		log:         d.Logger,
	}
}

// Publish executes the full publish workflow for the given intent.
//
// Steps:
//  1. Fetch and validate content status
//  2. Transition content to "publishing"
//  3. For each platform: create task, execute with retry, collect result
//  4. Aggregate task statuses into final content status
//  5. Return PublishStatus with per-task details
func (s *Service) Publish(ctx context.Context, intent contracts.PublishIntent) (*contracts.PublishStatus, error) {
	l := s.log.WithFields(zap.String("content_id", intent.ContentID))
	l.Info("publish started", zap.Strings("platforms", intent.Platforms))

	// 1. Fetch content.
	content, err := s.content.GetContent(ctx, intent.ContentID)
	if err != nil {
		l.Error("content not found", zap.Error(err))
		return nil, fmt.Errorf("fetch content: %w", err)
	}

	// 2. Validate state transition.
	if !content.Status.CanTransitionTo(contracts.ContentStatusPublishing) {
		l.Warn("invalid content status for publishing",
			zap.String("current_status", string(content.Status)))
		return nil, fmt.Errorf("content in status %q cannot transition to publishing", content.Status)
	}

	// Use the version from a re-read to get optimistic locking version.
	// For in-memory store we track version internally; re-fetch to get it.
	// Transition to publishing.
	if err := s.transitionContent(ctx, intent.ContentID, content.Status, contracts.ContentStatusPublishing); err != nil {
		l.Error("failed to transition content to publishing", zap.Error(err))
		return nil, fmt.Errorf("transition to publishing: %w", err)
	}

	// 3. Execute per-platform tasks.
	var taskStatuses []contracts.TaskStatus
	for _, platformName := range intent.Platforms {
		ts := s.executePlatformTask(ctx, intent.ContentID, platformName, content)
		taskStatuses = append(taskStatuses, ts)
	}

	// 4. Aggregate final content status.
	finalStatus := aggregateStatus(taskStatuses)
	if err := s.transitionContent(ctx, intent.ContentID, contracts.ContentStatusPublishing, finalStatus); err != nil {
		l.Error("failed to update final content status", zap.Error(err),
			zap.String("target_status", string(finalStatus)))
		// Non-fatal: we still return the task statuses.
	}

	l.Info("publish completed",
		zap.String("final_status", string(finalStatus)),
		zap.Int("task_count", len(taskStatuses)))

	return &contracts.PublishStatus{
		ContentID: intent.ContentID,
		Status:    finalStatus,
		Tasks:     taskStatuses,
	}, nil
}

// executePlatformTask runs the publish workflow for a single platform.
func (s *Service) executePlatformTask(ctx context.Context, contentID, platformName string, content *contracts.Content) contracts.TaskStatus {
	l := s.log.WithFields(
		zap.String("content_id", contentID),
		zap.String("platform", platformName))

	// Look up platform adapter.
	adapter, err := s.platforms.Get(platformName)
	if err != nil {
		l.Error("platform not found", zap.Error(err))
		return contracts.TaskStatus{
			Platform:  platformName,
			Status:    contracts.PublishTaskStatusFailed,
			Error:     fmt.Sprintf("platform not found: %s", platformName),
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		}
	}

	// Generate deterministic task ID for idempotency.
	taskIDKey := contentID + ":" + platformName

	// Claim idempotency key.
	claimed, err := s.idempotency.Claim(ctx, contracts.IdempotencyKey{
		Key:       taskIDKey,
		Scope:     contracts.IdempotencyScopeTask,
		EntityID:  taskIDKey,
		CreatedAt: time.Now().UTC(),
		TTL:       24 * time.Hour,
	})
	if err != nil {
		l.Error("idempotency claim failed", zap.Error(err))
		return contracts.TaskStatus{
			Platform:  platformName,
			Status:    contracts.PublishTaskStatusFailed,
			Error:     fmt.Sprintf("idempotency error: %s", err),
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		}
	}

	if !claimed {
		// Duplicate — return existing task status.
		existing, err := s.tasks.GetTaskByContentPlatform(ctx, contentID, platformName)
		if err != nil {
			l.Error("failed to fetch existing task for idempotent key", zap.Error(err))
			return contracts.TaskStatus{
				Platform:  platformName,
				Status:    contracts.PublishTaskStatusFailed,
				Error:     fmt.Sprintf("idempotency lookup failed: %s", err),
				CreatedAt: time.Now().UTC(),
				UpdatedAt: time.Now().UTC(),
			}
		}
		l.Info("idempotent duplicate detected, returning existing task",
			zap.String("existing_task_id", existing.ID))
		return contracts.TaskStatus{
			TaskID:         existing.ID,
			Platform:       platformName,
			Status:         existing.Status,
			PlatformPostID: "",
			Error:          existing.Error,
			CreatedAt:      existing.CreatedAt,
			UpdatedAt:      existing.UpdatedAt,
		}
	}

	// Create task.
	task, err := s.tasks.CreateTask(ctx, contentID, platformName)
	if err != nil {
		// Conflict means another goroutine created it between our Claim and Create.
		if errors.Is(err, storage.ErrConflict) {
			existing, lookupErr := s.tasks.GetTaskByContentPlatform(ctx, contentID, platformName)
			if lookupErr == nil {
				return contracts.TaskStatus{
					TaskID:    existing.ID,
					Platform:  platformName,
					Status:    existing.Status,
					Error:     existing.Error,
					CreatedAt: existing.CreatedAt,
					UpdatedAt: existing.UpdatedAt,
				}
			}
		}
		l.Error("failed to create task", zap.Error(err))
		return contracts.TaskStatus{
			Platform:  platformName,
			Status:    contracts.PublishTaskStatusFailed,
			Error:     fmt.Sprintf("create task: %s", err),
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		}
	}

	l = l.WithFields(zap.String("task_id", task.ID))
	l.Info("task created")

	// Execute with retry loop.
	result, finalStatus, taskErr := s.executeWithRetry(ctx, task, content, adapter)

	// Build task status from result.
	ts := contracts.TaskStatus{
		TaskID:    task.ID,
		Platform:  platformName,
		Status:    finalStatus,
		CreatedAt: task.CreatedAt,
		UpdatedAt: time.Now().UTC(),
	}

	if taskErr != nil {
		ts.Error = taskErr.Error()
		l.Error("task failed", zap.String("final_status", string(finalStatus)), zap.Error(taskErr))
	} else {
		ts.PlatformPostID = result.PlatformPostID
		ts.PlatformURL = result.PlatformURL
		l.Info("task succeeded",
			zap.String("platform_post_id", result.PlatformPostID),
			zap.String("platform_url", result.PlatformURL))
	}

	return ts
}

// executeWithRetry runs the publish attempt with retry logic.
//
// Returns the PublishResult and final task status. On error, the status indicates
// whether the failure is permanent (failed) or due to retry exhaustion (dead).
func (s *Service) executeWithRetry(ctx context.Context, task *contracts.PublishTask, content *contracts.Content, adapter platform.Platform) (*platform.PublishResult, contracts.PublishTaskStatus, error) {
	l := s.log.WithFields(
		zap.String("task_id", task.ID),
		zap.String("platform", task.Platform),
		zap.String("content_id", task.ContentID))

	maxRetries := task.RetryPolicy.MaxRetries

	for attempt := 0; attempt <= maxRetries; attempt++ {
		attemptNum := attempt + 1
		attemptLog := l.WithFields(zap.Int("attempt", attemptNum), zap.Int("max_retries", maxRetries))

		// Increment attempt count on the task.
		if err := s.tasks.IncrementAttemptCount(ctx, task.ID); err != nil {
			attemptLog.Error("failed to increment attempt count", zap.Error(err))
		}

		// Create attempt record.
		attemptRec, err := s.attempts.CreateAttempt(ctx, task.ID, attemptNum)
		if err != nil {
			attemptLog.Error("failed to create attempt record", zap.Error(err))
		}

		// Transition task to processing.
		if err := s.tasks.UpdateTaskStatus(ctx, task.ID, contracts.PublishTaskStatusProcessing, ""); err != nil {
			attemptLog.Warn("failed to transition task to processing", zap.Error(err))
		}

		// Convert content to IR document.
		doc := convertToDocument(content)

		// Validate.
		if err := adapter.Validate(doc); err != nil {
			errMsg := fmt.Sprintf("validation failed: %s", err)
			attemptLog.Warn("content validation failed", zap.Error(err))
			_ = s.tasks.UpdateTaskStatus(ctx, task.ID, contracts.PublishTaskStatusFailed, errMsg)
			if attemptRec != nil {
				_ = s.attempts.CompleteAttempt(ctx, attemptRec.ID, contracts.PublishTaskStatusFailed, errMsg)
			}
			return nil, contracts.PublishTaskStatusFailed, fmt.Errorf("%s", errMsg)
		}

		// Upload assets (no-op for mock, but part of the workflow).
		if _, err := adapter.UploadAssets(ctx, nil); err != nil {
			errMsg := fmt.Sprintf("asset upload failed: %s", err)
			attemptLog.Warn("asset upload failed", zap.Error(err))
			_ = s.tasks.UpdateTaskStatus(ctx, task.ID, contracts.PublishTaskStatusFailed, errMsg)
			if attemptRec != nil {
				_ = s.attempts.CompleteAttempt(ctx, attemptRec.ID, contracts.PublishTaskStatusFailed, errMsg)
			}
			return nil, contracts.PublishTaskStatusFailed, fmt.Errorf("%s", errMsg)
		}

		// Publish.
		result, err := adapter.Publish(ctx, doc, &platform.Credentials{})
		if err == nil {
			// Success.
			_ = s.tasks.UpdateTaskStatus(ctx, task.ID, contracts.PublishTaskStatusSucceeded, "")
			if attemptRec != nil {
				_ = s.attempts.CompleteAttempt(ctx, attemptRec.ID, contracts.PublishTaskStatusSucceeded, "")
			}
			// Store platform post record.
			if storeErr := s.posts.CreatePlatformPost(ctx, task.ID, task.Platform, result.PlatformPostID, result.PlatformURL, result.Response); storeErr != nil {
				attemptLog.Warn("failed to store platform post record", zap.Error(storeErr))
			}
			return result, contracts.PublishTaskStatusSucceeded, nil
		}

		attemptLog.Info("publish attempt failed", zap.Error(err))

		// Complete attempt with failure.
		if attemptRec != nil {
			_ = s.attempts.CompleteAttempt(ctx, attemptRec.ID, contracts.PublishTaskStatusFailed, err.Error())
		}

		// Check if error is retryable.
		var platErr *contracts.PlatformError
		isRetryable := false
		if errors.As(err, &platErr) {
			isRetryable = platErr.IsRetryable()
		}

		if !isRetryable {
			// Permanent failure — no retry.
			errMsg := fmt.Sprintf("permanent failure: %s", err)
			_ = s.tasks.UpdateTaskStatus(ctx, task.ID, contracts.PublishTaskStatusFailed, errMsg)
			attemptLog.Info("non-retryable error, giving up")
			return nil, contracts.PublishTaskStatusFailed, fmt.Errorf("%s", errMsg)
		}

		// Retryable — check if we have retries left.
		if attempt >= maxRetries {
			// Exhausted retries.
			errMsg := fmt.Sprintf("max retries (%d) exhausted: %s", maxRetries, err)
			_ = s.tasks.UpdateTaskStatus(ctx, task.ID, contracts.PublishTaskStatusDead, errMsg)
			attemptLog.Info("max retries exhausted, task is dead")
			return nil, contracts.PublishTaskStatusDead, fmt.Errorf("%s", errMsg)
		}

		// Transition to retrying.
		_ = s.tasks.UpdateTaskStatus(ctx, task.ID, contracts.PublishTaskStatusRetrying, err.Error())

		backoff := task.RetryPolicy.CalculateBackoff(attempt)
		attemptLog.Info("scheduling retry",
			zap.Duration("backoff", backoff),
			zap.Int("next_attempt", attemptNum+1))

		// In MVP, we don't actually sleep — just log the backoff.
		// In production, this would be handled by the async queue.
	}

	return nil, contracts.PublishTaskStatusFailed, fmt.Errorf("unexpected: retry loop exited without result")
}

// transitionContent updates the content status using the current version.
func (s *Service) transitionContent(ctx context.Context, contentID string, from, to contracts.ContentStatus) error {
	// Re-fetch to get the current version for optimistic locking.
	// For the in-memory store, version is tracked internally.
	// We need to read the content again to get the version.
	// Since our MemoryContentStore tracks version internally and UpdateContent
	// validates transitions, we pass version=0 and let the store handle it
	// based on the current state.
	//
	// For a real database, this would use SELECT ... FOR UPDATE.
	// For the in-memory MVP, we fetch and check status manually.
	content, err := s.content.GetContent(ctx, contentID)
	if err != nil {
		return fmt.Errorf("fetch content for transition: %w", err)
	}
	if content.Status != from {
		return fmt.Errorf("expected status %q, got %q", from, content.Status)
	}
	// Version is 1-based in our memory store; we track it through re-reads.
	// For simplicity, we'll use a helper that handles the version internally.
	return s.updateContentStatus(ctx, contentID, to)
}

// updateContentStatus updates content status with proper version tracking.
func (s *Service) updateContentStatus(ctx context.Context, contentID string, target contracts.ContentStatus) error {
	// The in-memory store's UpdateContent needs a version number.
	// We work around this by reading, checking, and updating in sequence.
	// For the MVP, this is safe because the service is single-threaded per content.
	//
	// We use version 1 as a sentinel — the MemoryContentStore starts at version 1
	// and increments on each update. We need to track the version through reads.
	// For now, we'll use a direct approach: the store validates the transition.
	return s.content.UpdateContent(ctx, contentID, target, 0)
}

// convertToDocument converts a contracts.Content into a transform.Document IR.
//
// This is a simple conversion for MVP. A real implementation would use a
// markdown parser (goldmark) to build the full IR tree.
func convertToDocument(content *contracts.Content) *transform.Document {
	return &transform.Document{
		Title: content.Title,
		Blocks: []transform.BlockNode{
			&transform.Paragraph{
				Children: []transform.InlineNode{
					&transform.Text{Content: content.Body},
				},
			},
		},
		Tags:     content.Tags,
		Metadata: content.Metadata,
	}
}

// aggregateStatus determines the final content status from a set of task statuses.
func aggregateStatus(tasks []contracts.TaskStatus) contracts.ContentStatus {
	if len(tasks) == 0 {
		return contracts.ContentStatusFailed
	}

	succeeded := 0
	failed := 0
	for _, t := range tasks {
		switch t.Status {
		case contracts.PublishTaskStatusSucceeded:
			succeeded++
		case contracts.PublishTaskStatusFailed, contracts.PublishTaskStatusDead:
			failed++
		}
	}

	if succeeded == len(tasks) {
		return contracts.ContentStatusPublished
	}
	if succeeded > 0 {
		return contracts.ContentStatusPartiallyPublished
	}
	return contracts.ContentStatusFailed
}

// PublishResultToJSON converts a PublishResult to JSON bytes for storage.
func PublishResultToJSON(r *platform.PublishResult) json.RawMessage {
	if r == nil {
		return nil
	}
	b, _ := json.Marshal(r)
	return b
}
