package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/savvyinsight/posthub/internal/contracts"
	"github.com/savvyinsight/posthub/internal/logger"
)

// fakeTaskStateStore implements TaskStateStore for testing.
type fakeTaskStateStore struct {
	updateErr       error
	recordResultErr error
	updatedStatuses []statusCall
	recordedResults []resultCall
}

type statusCall struct {
	TaskID string
	Status contracts.PublishTaskStatus
	ErrMsg string
}

type resultCall struct {
	TaskID string
	Result *contracts.PublishResult
}

func (f *fakeTaskStateStore) UpdateTaskStatus(_ context.Context, taskID string, status contracts.PublishTaskStatus, errMsg string) error {
	f.updatedStatuses = append(f.updatedStatuses, statusCall{TaskID: taskID, Status: status, ErrMsg: errMsg})
	return f.updateErr
}

func (f *fakeTaskStateStore) RecordPublishResult(_ context.Context, taskID string, result *contracts.PublishResult) error {
	f.recordedResults = append(f.recordedResults, resultCall{TaskID: taskID, Result: result})
	return f.recordResultErr
}

// testMetrics implements Metrics for capturing calls in tests.
type testMetrics struct {
	enqueued   []string
	started    []string
	completed  []string
	failed     []string
	retried    []string
	dead       []string
	idemHits   []string
}

func (m *testMetrics) TaskEnqueued(q string)                          { m.enqueued = append(m.enqueued, q) }
func (m *testMetrics) TaskStarted(taskID, _ string)                   { m.started = append(m.started, taskID) }
func (m *testMetrics) TaskCompleted(taskID, _ string, _ time.Duration) { m.completed = append(m.completed, taskID) }
func (m *testMetrics) TaskFailed(taskID, _ string, _ error)           { m.failed = append(m.failed, taskID) }
func (m *testMetrics) TaskRetried(taskID, _ string, _ int)            { m.retried = append(m.retried, taskID) }
func (m *testMetrics) TaskDead(taskID, _ string)                      { m.dead = append(m.dead, taskID) }
func (m *testMetrics) IdempotencyHit(key string)                      { m.idemHits = append(m.idemHits, key) }

func newTestLogger() *logger.Logger {
	return logger.New("error", "development")
}

func TestTaskHandler_HandlePublish_Success(t *testing.T) {
	store := &fakeTaskStateStore{}
	publisher := &MockPublisher{
		Result: &contracts.PublishResult{
			PlatformPostID: "post-123",
			PlatformURL:    "https://twitter.com/user/status/123",
			PublishedAt:    time.Now(),
		},
	}
	metrics := &testMetrics{}
	handler := NewTaskHandler(publisher, store, nil, newTestLogger(), metrics)

	payload := PublishPayload{
		TaskID:    "task-1",
		ContentID: "content-1",
		Platform:  "twitter",
	}

	err := handler.HandlePublish(context.Background(), payload)
	if err != nil {
		t.Fatalf("HandlePublish() error = %v, want nil", err)
	}

	// Verify state transitions: processing → succeeded.
	if len(store.updatedStatuses) != 2 {
		t.Fatalf("expected 2 status updates, got %d", len(store.updatedStatuses))
	}
	if store.updatedStatuses[0].Status != contracts.PublishTaskStatusProcessing {
		t.Errorf("first update = %s, want processing", store.updatedStatuses[0].Status)
	}
	if store.updatedStatuses[1].Status != contracts.PublishTaskStatusSucceeded {
		t.Errorf("second update = %s, want succeeded", store.updatedStatuses[1].Status)
	}

	// Verify result was recorded.
	if len(store.recordedResults) != 1 {
		t.Fatalf("expected 1 recorded result, got %d", len(store.recordedResults))
	}
	if store.recordedResults[0].Result.PlatformPostID != "post-123" {
		t.Errorf("result PlatformPostID = %s, want post-123", store.recordedResults[0].Result.PlatformPostID)
	}

	// Verify publisher was called.
	if len(publisher.Calls) != 1 {
		t.Fatalf("expected 1 publish call, got %d", len(publisher.Calls))
	}
	if publisher.Calls[0].ContentID != "content-1" || publisher.Calls[0].Platform != "twitter" {
		t.Errorf("publish call = %+v, want content-1/twitter", publisher.Calls[0])
	}

	// Verify metrics.
	if len(metrics.started) != 1 || metrics.started[0] != "task-1" {
		t.Errorf("metrics.started = %v, want [task-1]", metrics.started)
	}
	if len(metrics.completed) != 1 || metrics.completed[0] != "task-1" {
		t.Errorf("metrics.completed = %v, want [task-1]", metrics.completed)
	}
	if len(metrics.failed) != 0 {
		t.Errorf("metrics.failed should be empty, got %v", metrics.failed)
	}
}

func TestTaskHandler_HandlePublish_RetryableFailure(t *testing.T) {
	store := &fakeTaskStateStore{}
	retryableErr := &contracts.PlatformError{
		Platform:   "twitter",
		StatusCode: 429,
		Message:    "rate limited",
		Retryable:  true,
	}
	publisher := &MockPublisher{Err: retryableErr}
	metrics := &testMetrics{}
	handler := NewTaskHandler(publisher, store, nil, newTestLogger(), metrics)

	payload := PublishPayload{
		TaskID:    "task-2",
		ContentID: "content-1",
		Platform:  "twitter",
	}

	err := handler.HandlePublish(context.Background(), payload)
	if err == nil {
		t.Fatal("HandlePublish() error = nil, want retryable error")
	}
	if !errors.Is(err, retryableErr) {
		t.Errorf("error should wrap the platform error, got %v", err)
	}

	// Task should only be set to processing (no succeeded/dead transition).
	if len(store.updatedStatuses) != 1 {
		t.Fatalf("expected 1 status update, got %d", len(store.updatedStatuses))
	}
	if store.updatedStatuses[0].Status != contracts.PublishTaskStatusProcessing {
		t.Errorf("status = %s, want processing", store.updatedStatuses[0].Status)
	}

	// No result should be recorded.
	if len(store.recordedResults) != 0 {
		t.Errorf("expected 0 recorded results, got %d", len(store.recordedResults))
	}

	// Metrics: started + failed, no completed/dead.
	if len(metrics.failed) != 1 {
		t.Errorf("metrics.failed = %v, want [task-2]", metrics.failed)
	}
	if len(metrics.completed) != 0 {
		t.Errorf("metrics.completed should be empty, got %v", metrics.completed)
	}
	if len(metrics.dead) != 0 {
		t.Errorf("metrics.dead should be empty, got %v", metrics.dead)
	}
}

func TestTaskHandler_HandlePublish_PermanentFailure_PlatformError(t *testing.T) {
	store := &fakeTaskStateStore{}
	permanentErr := &contracts.PlatformError{
		Platform:   "twitter",
		StatusCode: 403,
		Message:    "forbidden",
		Retryable:  false,
	}
	publisher := &MockPublisher{Err: permanentErr}
	metrics := &testMetrics{}
	handler := NewTaskHandler(publisher, store, nil, newTestLogger(), metrics)

	payload := PublishPayload{
		TaskID:    "task-3",
		ContentID: "content-1",
		Platform:  "twitter",
	}

	err := handler.HandlePublish(context.Background(), payload)
	if err != nil {
		t.Fatalf("HandlePublish() error = %v, want nil (permanent errors return nil)", err)
	}

	// Task should be set to dead.
	if len(store.updatedStatuses) != 2 {
		t.Fatalf("expected 2 status updates, got %d", len(store.updatedStatuses))
	}
	if store.updatedStatuses[0].Status != contracts.PublishTaskStatusProcessing {
		t.Errorf("first update = %s, want processing", store.updatedStatuses[0].Status)
	}
	if store.updatedStatuses[1].Status != contracts.PublishTaskStatusDead {
		t.Errorf("second update = %s, want dead", store.updatedStatuses[1].Status)
	}

	// Metrics: dead recorded.
	if len(metrics.dead) != 1 || metrics.dead[0] != "task-3" {
		t.Errorf("metrics.dead = %v, want [task-3]", metrics.dead)
	}
}

func TestTaskHandler_HandlePublish_PermanentFailure_NotFound(t *testing.T) {
	store := &fakeTaskStateStore{}
	notFoundErr := contracts.NewNotFoundError("content")
	publisher := &MockPublisher{Err: notFoundErr}
	handler := NewTaskHandler(publisher, store, nil, newTestLogger(), nil)

	payload := PublishPayload{
		TaskID:    "task-4",
		ContentID: "missing-content",
		Platform:  "twitter",
	}

	err := handler.HandlePublish(context.Background(), payload)
	if err != nil {
		t.Fatalf("HandlePublish() error = %v, want nil", err)
	}

	// Should transition to dead.
	if len(store.updatedStatuses) != 2 {
		t.Fatalf("expected 2 status updates, got %d", len(store.updatedStatuses))
	}
	if store.updatedStatuses[1].Status != contracts.PublishTaskStatusDead {
		t.Errorf("second update = %s, want dead", store.updatedStatuses[1].Status)
	}
}

func TestTaskHandler_HandlePublish_PermanentFailure_Validation(t *testing.T) {
	store := &fakeTaskStateStore{}
	validationErr := contracts.NewValidationError("invalid content")
	publisher := &MockPublisher{Err: validationErr}
	handler := NewTaskHandler(publisher, store, nil, newTestLogger(), nil)

	payload := PublishPayload{
		TaskID:    "task-5",
		ContentID: "bad-content",
		Platform:  "twitter",
	}

	err := handler.HandlePublish(context.Background(), payload)
	if err != nil {
		t.Fatalf("HandlePublish() error = %v, want nil", err)
	}

	if len(store.updatedStatuses) != 2 {
		t.Fatalf("expected 2 status updates, got %d", len(store.updatedStatuses))
	}
	if store.updatedStatuses[1].Status != contracts.PublishTaskStatusDead {
		t.Errorf("second update = %s, want dead", store.updatedStatuses[1].Status)
	}
}

func TestTaskHandler_HandlePublish_IdempotencyHit(t *testing.T) {
	store := &fakeTaskStateStore{}
	publisher := &MockPublisher{
		Result: &contracts.PublishResult{PlatformPostID: "post-1"},
	}
	idem := NewInMemoryIdempotencyGuard()
	metrics := &testMetrics{}
	handler := NewTaskHandler(publisher, store, idem, newTestLogger(), metrics)

	payload := PublishPayload{
		TaskID:    "task-6",
		ContentID: "content-1",
		Platform:  "twitter",
	}

	// First call should succeed.
	if err := handler.HandlePublish(context.Background(), payload); err != nil {
		t.Fatalf("first HandlePublish() error = %v", err)
	}

	// Second call with same content:platform should be deduplicated.
	if err := handler.HandlePublish(context.Background(), payload); err != nil {
		t.Fatalf("second HandlePublish() error = %v", err)
	}

	// Publisher should only be called once.
	if len(publisher.Calls) != 1 {
		t.Errorf("publisher.Calls = %d, want 1", len(publisher.Calls))
	}

	// Idempotency hit metric should be recorded.
	if len(metrics.idemHits) != 1 {
		t.Errorf("metrics.idemHits = %v, want 1 entry", metrics.idemHits)
	}
}

func TestTaskHandler_HandlePublish_IdempotencyDisabled(t *testing.T) {
	store := &fakeTaskStateStore{}
	publisher := &MockPublisher{
		Result: &contracts.PublishResult{PlatformPostID: "post-1"},
	}
	// nil idempotency guard = disabled.
	handler := NewTaskHandler(publisher, store, nil, newTestLogger(), nil)

	payload := PublishPayload{
		TaskID:    "task-7",
		ContentID: "content-1",
		Platform:  "twitter",
	}

	// Both calls should process normally.
	if err := handler.HandlePublish(context.Background(), payload); err != nil {
		t.Fatalf("first HandlePublish() error = %v", err)
	}
	if err := handler.HandlePublish(context.Background(), payload); err != nil {
		t.Fatalf("second HandlePublish() error = %v", err)
	}

	if len(publisher.Calls) != 2 {
		t.Errorf("publisher.Calls = %d, want 2", len(publisher.Calls))
	}
}

func TestTaskHandler_HandlePublish_StoreTransitionError(t *testing.T) {
	store := &fakeTaskStateStore{
		updateErr: errors.New("db connection lost"),
	}
	publisher := &MockPublisher{
		Result: &contracts.PublishResult{PlatformPostID: "post-1"},
	}
	handler := NewTaskHandler(publisher, store, nil, newTestLogger(), nil)

	payload := PublishPayload{
		TaskID:    "task-8",
		ContentID: "content-1",
		Platform:  "twitter",
	}

	err := handler.HandlePublish(context.Background(), payload)
	if err == nil {
		t.Fatal("HandlePublish() error = nil, want error from store")
	}
	if !strings.Contains(err.Error(), "set processing") {
		t.Errorf("error = %v, want 'set processing' error", err)
	}

	// Publisher should not be called if transition fails.
	if len(publisher.Calls) != 0 {
		t.Errorf("publisher.Calls = %d, want 0", len(publisher.Calls))
	}
}

func TestTaskHandler_HandlePublish_StoreResultError(t *testing.T) {
	store := &fakeTaskStateStore{
		recordResultErr: errors.New("disk full"),
	}
	publisher := &MockPublisher{
		Result: &contracts.PublishResult{PlatformPostID: "post-1"},
	}
	handler := NewTaskHandler(publisher, store, nil, newTestLogger(), nil)

	payload := PublishPayload{
		TaskID:    "task-9",
		ContentID: "content-1",
		Platform:  "twitter",
	}

	err := handler.HandlePublish(context.Background(), payload)
	if err == nil {
		t.Fatal("HandlePublish() error = nil, want error from result store")
	}
	if !strings.Contains(err.Error(), "record result") {
		t.Errorf("error = %v, want 'record result' error", err)
	}
}

func TestTaskHandler_HandlePublish_ContextEnrichment(t *testing.T) {
	store := &fakeTaskStateStore{}
	publisher := &MockPublisher{
		Result: &contracts.PublishResult{PlatformPostID: "post-1"},
	}
	handler := NewTaskHandler(publisher, store, nil, newTestLogger(), nil)

	payload := PublishPayload{
		TaskID:    "task-10",
		ContentID: "content-1",
		Platform:  "mastodon",
	}

	err := handler.HandlePublish(context.Background(), payload)
	if err != nil {
		t.Fatalf("HandlePublish() error = %v", err)
	}

	// Verify the payload was passed correctly to the publisher.
	if len(publisher.Calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(publisher.Calls))
	}
	if publisher.Calls[0].Platform != "mastodon" {
		t.Errorf("platform = %s, want mastodon", publisher.Calls[0].Platform)
	}
}

func TestTaskHandler_HandlePublish_PublishFn(t *testing.T) {
	store := &fakeTaskStateStore{}
	publisher := &MockPublisher{
		PublishFn: func(_ context.Context, contentID, platform string) (*contracts.PublishResult, error) {
			if contentID == "" {
				return nil, contracts.NewValidationError("content_id required")
			}
			return &contracts.PublishResult{
				PlatformPostID: "dynamic-post",
				PlatformURL:    "https://example.com/post/dynamic-post",
			}, nil
		},
	}
	handler := NewTaskHandler(publisher, store, nil, newTestLogger(), nil)

	// Valid payload.
	payload := PublishPayload{
		TaskID:    "task-11",
		ContentID: "content-1",
		Platform:  "twitter",
	}
	if err := handler.HandlePublish(context.Background(), payload); err != nil {
		t.Fatalf("HandlePublish() error = %v", err)
	}
	if len(store.recordedResults) != 1 {
		t.Fatalf("expected 1 result, got %d", len(store.recordedResults))
	}
	if store.recordedResults[0].Result.PlatformPostID != "dynamic-post" {
		t.Errorf("PlatformPostID = %s, want dynamic-post", store.recordedResults[0].Result.PlatformPostID)
	}

	// Reset store.
	store = &fakeTaskStateStore{}
	publisher.Calls = nil
	handler = NewTaskHandler(publisher, store, nil, newTestLogger(), nil)

	// Invalid payload (empty content ID).
	payload.ContentID = ""
	if err := handler.HandlePublish(context.Background(), payload); err != nil {
		t.Fatalf("HandlePublish() error = %v, want nil (permanent failure)", err)
	}
	if len(store.updatedStatuses) != 2 {
		t.Fatalf("expected 2 status updates, got %d", len(store.updatedStatuses))
	}
	if store.updatedStatuses[1].Status != contracts.PublishTaskStatusDead {
		t.Errorf("status = %s, want dead", store.updatedStatuses[1].Status)
	}
}

func TestIsPermanentError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "nil_error",
			err:  nil,
			want: false,
		},
		{
			name: "non_retryable_platform_error",
			err:  &contracts.PlatformError{Retryable: false},
			want: true,
		},
		{
			name: "retryable_platform_error",
			err:  &contracts.PlatformError{Retryable: true},
			want: false,
		},
		{
			name: "validation_error",
			err:  contracts.NewValidationError("bad input"),
			want: true,
		},
		{
			name: "not_found_error",
			err:  contracts.NewNotFoundError("content"),
			want: true,
		},
		{
			name: "forbidden_error",
			err:  contracts.NewForbiddenError("not allowed"),
			want: true,
		},
		{
			name: "internal_error",
			err:  contracts.NewInternalError("oops"),
			want: false,
		},
		{
			name: "unavailable_error",
			err:  contracts.NewUnavailableError("down"),
			want: false,
		},
		{
			name: "conflict_error",
			err:  contracts.NewConflictError("duplicate"),
			want: false,
		},
		{
			name: "rate_limit_error",
			err:  &contracts.RateLimitError{Platform: "twitter", RetryAfter: 5 * time.Second},
			want: false,
		},
		{
			name: "generic_error",
			err:  errors.New("something broke"),
			want: false,
		},
		{
			name: "wrapped_retryable_platform_error",
			err:  fmt.Errorf("outer: %w", &contracts.PlatformError{Retryable: true}),
			want: false,
		},
		{
			name: "wrapped_permanent_platform_error",
			err:  fmt.Errorf("outer: %w", &contracts.PlatformError{Retryable: false}),
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isPermanentError(tt.err)
			if got != tt.want {
				t.Errorf("isPermanentError() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Verify PublishPayload serializes/deserializes correctly.
func TestPublishPayload_JSON(t *testing.T) {
	original := PublishPayload{
		TaskID:    "task-abc",
		ContentID: "content-xyz",
		Platform:  "mastodon",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var decoded PublishPayload
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if decoded.TaskID != original.TaskID {
		t.Errorf("TaskID = %s, want %s", decoded.TaskID, original.TaskID)
	}
	if decoded.ContentID != original.ContentID {
		t.Errorf("ContentID = %s, want %s", decoded.ContentID, original.ContentID)
	}
	if decoded.Platform != original.Platform {
		t.Errorf("Platform = %s, want %s", decoded.Platform, original.Platform)
	}
}

// Compile-time interface checks.
var (
	_ Handler      = (*TaskHandler)(nil)
	_ Publisher    = (*MockPublisher)(nil)
	_ Metrics      = (*NopMetrics)(nil)
	_ Metrics      = (*testMetrics)(nil)
)
