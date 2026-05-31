// HTTP handlers for the posthub API.
//
// Handlers are thin: they decode requests, validate, delegate to stores/queue,
// and encode responses. No business logic lives here.
package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/savvyinsight/posthub/internal/contracts"
	"github.com/savvyinsight/posthub/internal/logger"
	"github.com/savvyinsight/posthub/internal/queue"
	"github.com/savvyinsight/posthub/internal/storage"
)

// HealthHandler handles health check requests.
type HealthHandler struct {
	Version string
}

// HealthResponse is the response from the health endpoint.
type HealthResponse struct {
	Status  string `json:"status"`
	Version string `json:"version"`
}

// HandleHealth returns a 200 OK with service status.
func (h *HealthHandler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, HealthResponse{
		Status:  "ok",
		Version: h.Version,
	})
}

// PublishHandler handles publish creation and status queries.
type PublishHandler struct {
	ContentStore storage.ContentStore
	TaskStore    storage.PublishTaskStore
	Enqueuer     queue.Enqueuer
}

// publishRequest is the API request body for creating a publish job.
// It extends the content creation with the target platforms.
type publishRequest struct {
	Title     string   `json:"title"`
	Body      string   `json:"body"`
	Tags      []string `json:"tags"`
	Platforms []string `json:"platforms"`
}

// HandlePublish creates content, publish tasks, and enqueues them.
//
//	POST /publish → 202 with PublishBatchResponse
func (h *PublishHandler) HandlePublish(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logger.FromContext(ctx)

	var req publishRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, contracts.ErrCodeValidation, "invalid request body")
		return
	}

	if err := validatePublishRequest(&req); err != nil {
		writeAppError(w, err)
		return
	}

	// Create content in draft status.
	content, err := h.ContentStore.CreateContent(ctx, req.Title, req.Body, req.Tags)
	if err != nil {
		log.Error("failed to create content", zap.Error(err))
		writeError(w, http.StatusInternalServerError, contracts.ErrCodeInternal, "failed to create content")
		return
	}

	// Transition to ready, then publishing.
	if err := h.ContentStore.UpdateContent(ctx, content.ID, contracts.ContentStatusReady, 1); err != nil {
		log.Error("failed to update content status", zap.Error(err))
		writeError(w, http.StatusInternalServerError, contracts.ErrCodeInternal, "failed to prepare content")
		return
	}
	if err := h.ContentStore.UpdateContent(ctx, content.ID, contracts.ContentStatusPublishing, 2); err != nil {
		log.Error("failed to update content status", zap.Error(err))
		writeError(w, http.StatusInternalServerError, contracts.ErrCodeInternal, "failed to prepare content")
		return
	}

	// Create a task per platform and enqueue.
	tasks := make([]contracts.TaskStatus, 0, len(req.Platforms))
	for _, platform := range req.Platforms {
		task, err := h.TaskStore.CreateTask(ctx, content.ID, platform)
		if err != nil {
			log.Error("failed to create task",
				zap.String("platform", platform),
				zap.Error(err),
			)
			continue
		}

		payload := queue.PublishPayload{
			TaskID:    task.ID,
			ContentID: content.ID,
			Platform:  platform,
		}
		if err := h.Enqueuer.EnqueuePublish(ctx, payload, queue.EnqueueOptions{
			MaxRetry: task.RetryPolicy.MaxRetries,
		}); err != nil {
			log.Error("failed to enqueue task",
				zap.String("task_id", task.ID),
				zap.String("platform", platform),
				zap.Error(err),
			)
			// Task is created but not enqueued — worker will not pick it up.
			// Status remains pending. In a full implementation, we'd mark it as failed.
		}

		tasks = append(tasks, contracts.TaskStatus{
			TaskID:    task.ID,
			Platform:  platform,
			Status:    task.Status,
			CreatedAt: task.CreatedAt,
			UpdatedAt: task.UpdatedAt,
		})
	}

	if len(tasks) == 0 {
		writeError(w, http.StatusInternalServerError, contracts.ErrCodeInternal, "failed to create any publish tasks")
		return
	}

	writeJSON(w, http.StatusAccepted, contracts.PublishBatchResponse{
		ContentID: content.ID,
		Tasks:     tasks,
	})
}

// HandlePublishStatus returns the publish status for a content item.
//
//	GET /publish/{id} → 200 with PublishStatus
func (h *PublishHandler) HandlePublishStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	contentID := chi.URLParam(r, "id")

	if contentID == "" {
		writeError(w, http.StatusBadRequest, contracts.ErrCodeValidation, "content id is required")
		return
	}

	// Verify content exists.
	content, err := h.ContentStore.GetContent(ctx, contentID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			writeError(w, http.StatusNotFound, contracts.ErrCodeNotFound, "content not found")
			return
		}
		logger.FromContext(ctx).Error("failed to get content", zap.Error(err))
		writeError(w, http.StatusInternalServerError, contracts.ErrCodeInternal, "failed to get content")
		return
	}

	// Get all tasks for this content.
	taskList, err := h.TaskStore.GetTasksByContent(ctx, contentID)
	if err != nil {
		logger.FromContext(ctx).Error("failed to get tasks", zap.Error(err))
		writeError(w, http.StatusInternalServerError, contracts.ErrCodeInternal, "failed to get publish status")
		return
	}

	tasks := make([]contracts.TaskStatus, 0, len(taskList))
	for _, t := range taskList {
		tasks = append(tasks, contracts.TaskStatus{
			TaskID:    t.ID,
			Platform:  t.Platform,
			Status:    t.Status,
			Error:     t.Error,
			CreatedAt: t.CreatedAt,
			UpdatedAt: t.UpdatedAt,
		})
	}

	writeJSON(w, http.StatusOK, contracts.PublishStatus{
		ContentID: content.ID,
		Status:    content.Status,
		Tasks:     tasks,
	})
}

// validatePublishRequest checks required fields.
func validatePublishRequest(req *publishRequest) *contracts.AppError {
	var details []contracts.ErrorDetail

	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" {
		details = append(details, contracts.ErrorDetail{Field: "title", Message: "title is required"})
	} else if len(req.Title) > 200 {
		details = append(details, contracts.ErrorDetail{Field: "title", Message: "title must be 200 characters or less"})
	}

	req.Body = strings.TrimSpace(req.Body)
	if req.Body == "" {
		details = append(details, contracts.ErrorDetail{Field: "body", Message: "body is required"})
	}

	if len(req.Platforms) == 0 {
		details = append(details, contracts.ErrorDetail{Field: "platforms", Message: "at least one platform is required"})
	}

	if len(details) > 0 {
		return &contracts.AppError{
			Code:    contracts.ErrCodeValidation,
			Message: fmt.Sprintf("validation failed with %d error(s)", len(details)),
			Details: details,
		}
	}
	return nil
}

// --- JSON response helpers ---

// writeJSON encodes v as JSON with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError writes a JSON error response.
func writeError(w http.ResponseWriter, status int, code contracts.ErrorCode, message string) {
	writeJSON(w, status, contracts.AppError{
		Code:    code,
		Message: message,
	})
}

// writeAppError writes an AppError as JSON, mapping the error code to an HTTP status.
func writeAppError(w http.ResponseWriter, err *contracts.AppError) {
	status := http.StatusInternalServerError
	switch err.Code {
	case contracts.ErrCodeValidation:
		status = http.StatusBadRequest
	case contracts.ErrCodeNotFound:
		status = http.StatusNotFound
	case contracts.ErrCodeForbidden:
		status = http.StatusForbidden
	case contracts.ErrCodeConflict:
		status = http.StatusConflict
	case contracts.ErrCodeRateLimited:
		status = http.StatusTooManyRequests
	}
	writeJSON(w, status, err)
}
