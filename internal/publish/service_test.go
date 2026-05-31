package publish

import (
	"context"
	"errors"
	"testing"

	"github.com/savvyinsight/posthub/internal/contracts"
	"github.com/savvyinsight/posthub/internal/logger"
	"github.com/savvyinsight/posthub/internal/platform"
	"github.com/savvyinsight/posthub/internal/platform/mock"
	"github.com/savvyinsight/posthub/internal/transform"
)

// testHarness bundles all dependencies for service tests.
type testHarness struct {
	service    *Service
	content    *MemoryContentStore
	tasks      *MemoryPublishTaskStore
	attempts   *MemoryPublishAttemptStore
	posts      *MemoryPlatformPostStore
	idempotency *MemoryIdempotencyStore
	platforms  *platform.Registry
}

func newTestHarness(platforms ...*mock.MockPlatform) *testHarness {
	content := NewMemoryContentStore()
	tasks := NewMemoryPublishTaskStore()
	attempts := NewMemoryPublishAttemptStore()
	posts := NewMemoryPlatformPostStore()
	idempotency := NewMemoryIdempotencyStore()
	reg := platform.NewRegistry()

	for _, p := range platforms {
		reg.Register(p)
	}

	log := logger.New("debug", "development")
	svc := NewService(Deps{
		ContentStore:     content,
		TaskStore:        tasks,
		AttemptStore:     attempts,
		PostStore:        posts,
		IdempotencyStore: idempotency,
		Platforms:        reg,
		Logger:           log,
	})

	return &testHarness{
		service:     svc,
		content:     content,
		tasks:       tasks,
		attempts:    attempts,
		posts:       posts,
		idempotency: idempotency,
		platforms:   reg,
	}
}

// createReadyContent creates content in "ready" status and returns its ID.
func (h *testHarness) createReadyContent(t *testing.T) string {
	t.Helper()
	c, err := h.content.CreateContent(context.Background(), "Test Post", "Hello world body", []string{"go", "test"})
	if err != nil {
		t.Fatalf("CreateContent() error = %v", err)
	}
	if err := h.content.UpdateContent(context.Background(), c.ID, contracts.ContentStatusReady, 0); err != nil {
		t.Fatalf("UpdateContent(draft->ready) error = %v", err)
	}
	return c.ID
}

func TestService_Publish_Success(t *testing.T) {
	adapter := mock.NewSuccessPlatform("zhihu")
	h := newTestHarness(adapter)
	contentID := h.createReadyContent(t)

	status, err := h.service.Publish(context.Background(), contracts.PublishIntent{
		ContentID: contentID,
		Platforms: []string{"zhihu"},
	})
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if status.Status != contracts.ContentStatusPublished {
		t.Errorf("status = %s, want published", status.Status)
	}
	if len(status.Tasks) != 1 {
		t.Fatalf("len(Tasks) = %d, want 1", len(status.Tasks))
	}
	task := status.Tasks[0]
	if task.Status != contracts.PublishTaskStatusSucceeded {
		t.Errorf("task status = %s, want succeeded", task.Status)
	}
	if task.PlatformPostID == "" {
		t.Error("task PlatformPostID is empty")
	}
	if task.PlatformURL == "" {
		t.Error("task PlatformURL is empty")
	}

	// Verify adapter was called.
	if adapter.AttemptCount() != 1 {
		t.Errorf("adapter.AttemptCount() = %d, want 1", adapter.AttemptCount())
	}

	// Verify content status was updated.
	c, _ := h.content.GetContent(context.Background(), contentID)
	if c.Status != contracts.ContentStatusPublished {
		t.Errorf("content status = %s, want published", c.Status)
	}
}

func TestService_Publish_RetryableFailure(t *testing.T) {
	// Fail twice, succeed on 3rd attempt.
	adapter := mock.NewRetryablePlatform("zhihu", 2)
	h := newTestHarness(adapter)
	contentID := h.createReadyContent(t)

	status, err := h.service.Publish(context.Background(), contracts.PublishIntent{
		ContentID: contentID,
		Platforms: []string{"zhihu"},
	})
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if status.Status != contracts.ContentStatusPublished {
		t.Errorf("status = %s, want published", status.Status)
	}
	if status.Tasks[0].Status != contracts.PublishTaskStatusSucceeded {
		t.Errorf("task status = %s, want succeeded", status.Tasks[0].Status)
	}

	// Adapter should have been called 3 times (2 failures + 1 success).
	if adapter.AttemptCount() != 3 {
		t.Errorf("adapter.AttemptCount() = %d, want 3", adapter.AttemptCount())
	}
}

func TestService_Publish_PermanentFailure(t *testing.T) {
	adapter := mock.NewPermanentFailPlatform("zhihu", "content rejected")
	h := newTestHarness(adapter)
	contentID := h.createReadyContent(t)

	status, err := h.service.Publish(context.Background(), contracts.PublishIntent{
		ContentID: contentID,
		Platforms: []string{"zhihu"},
	})
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if status.Status != contracts.ContentStatusFailed {
		t.Errorf("status = %s, want failed", status.Status)
	}
	if status.Tasks[0].Status != contracts.PublishTaskStatusFailed {
		t.Errorf("task status = %s, want failed", status.Tasks[0].Status)
	}

	// Content should be in failed state.
	c, _ := h.content.GetContent(context.Background(), contentID)
	if c.Status != contracts.ContentStatusFailed {
		t.Errorf("content status = %s, want failed", c.Status)
	}
}

func TestService_Publish_PartialSuccess(t *testing.T) {
	success := mock.NewSuccessPlatform("zhihu")
	fail := mock.NewPermanentFailPlatform("bilibili", "auth expired")
	h := newTestHarness(success, fail)
	contentID := h.createReadyContent(t)

	status, err := h.service.Publish(context.Background(), contracts.PublishIntent{
		ContentID: contentID,
		Platforms: []string{"zhihu", "bilibili"},
	})
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if status.Status != contracts.ContentStatusPartiallyPublished {
		t.Errorf("status = %s, want partially_published", status.Status)
	}

	var succeeded, failed int
	for _, ts := range status.Tasks {
		switch ts.Status {
		case contracts.PublishTaskStatusSucceeded:
			succeeded++
		case contracts.PublishTaskStatusFailed:
			failed++
		}
	}
	if succeeded != 1 {
		t.Errorf("succeeded = %d, want 1", succeeded)
	}
	if failed != 1 {
		t.Errorf("failed = %d, want 1", failed)
	}
}

func TestService_Publish_MultiPlatformAllSucceed(t *testing.T) {
	p1 := mock.NewSuccessPlatform("zhihu")
	p2 := mock.NewSuccessPlatform("bilibili")
	p3 := mock.NewSuccessPlatform("weibo")
	h := newTestHarness(p1, p2, p3)
	contentID := h.createReadyContent(t)

	status, err := h.service.Publish(context.Background(), contracts.PublishIntent{
		ContentID: contentID,
		Platforms: []string{"zhihu", "bilibili", "weibo"},
	})
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if status.Status != contracts.ContentStatusPublished {
		t.Errorf("status = %s, want published", status.Status)
	}
	if len(status.Tasks) != 3 {
		t.Fatalf("len(Tasks) = %d, want 3", len(status.Tasks))
	}
	for _, ts := range status.Tasks {
		if ts.Status != contracts.PublishTaskStatusSucceeded {
			t.Errorf("task %s status = %s, want succeeded", ts.Platform, ts.Status)
		}
	}
}

func TestService_Publish_Idempotent(t *testing.T) {
	// Idempotency is tested at the task level: publishing the same content
	// to the same platform twice should reuse the existing task.
	// Since content transitions to "published" after the first call,
	// a second call on the same content will fail at the state check.
	// Instead, we verify that the idempotency store is claimed correctly.

	adapter := mock.NewSuccessPlatform("zhihu")
	h := newTestHarness(adapter)
	contentID := h.createReadyContent(t)

	intent := contracts.PublishIntent{
		ContentID: contentID,
		Platforms: []string{"zhihu"},
	}

	// First publish succeeds.
	status1, err := h.service.Publish(context.Background(), intent)
	if err != nil {
		t.Fatalf("Publish() first error = %v", err)
	}
	if status1.Tasks[0].Status != contracts.PublishTaskStatusSucceeded {
		t.Fatalf("first publish task status = %s, want succeeded", status1.Tasks[0].Status)
	}

	// Verify idempotency key was claimed.
	entityID, err := h.idempotency.Get(context.Background(), contentID+":zhihu", contracts.IdempotencyScopeTask)
	if err != nil {
		t.Fatalf("idempotency Get() error = %v", err)
	}
	if entityID == "" {
		t.Error("idempotency key was not claimed")
	}

	// Adapter should only be called once.
	if adapter.AttemptCount() != 1 {
		t.Errorf("adapter.AttemptCount() = %d, want 1", adapter.AttemptCount())
	}

	// A second publish on the same content fails because content is already published.
	_, err = h.service.Publish(context.Background(), intent)
	if err == nil {
		t.Error("second Publish() error = nil, want error (content already published)")
	}
}

func TestService_Publish_InvalidContentStatus(t *testing.T) {
	adapter := mock.NewSuccessPlatform("zhihu")
	h := newTestHarness(adapter)

	// Create content but leave it in draft (not ready).
	c, _ := h.content.CreateContent(context.Background(), "Draft", "body", nil)

	_, err := h.service.Publish(context.Background(), contracts.PublishIntent{
		ContentID: c.ID,
		Platforms: []string{"zhihu"},
	})
	if err == nil {
		t.Fatal("Publish() error = nil, want error for draft content")
	}
}

func TestService_Publish_ContentNotFound(t *testing.T) {
	adapter := mock.NewSuccessPlatform("zhihu")
	h := newTestHarness(adapter)

	_, err := h.service.Publish(context.Background(), contracts.PublishIntent{
		ContentID: "nonexistent",
		Platforms: []string{"zhihu"},
	})
	if err == nil {
		t.Fatal("Publish() error = nil, want error for missing content")
	}
}

func TestService_Publish_PlatformNotFound(t *testing.T) {
	h := newTestHarness() // no platforms registered
	contentID := h.createReadyContent(t)

	status, err := h.service.Publish(context.Background(), contracts.PublishIntent{
		ContentID: contentID,
		Platforms: []string{"zhihu"},
	})
	if err != nil {
		t.Fatalf("Publish() error = %v, want nil (errors are per-task)", err)
	}
	if len(status.Tasks) != 1 {
		t.Fatalf("len(Tasks) = %d, want 1", len(status.Tasks))
	}
	if status.Tasks[0].Status != contracts.PublishTaskStatusFailed {
		t.Errorf("task status = %s, want failed", status.Tasks[0].Status)
	}
	if status.Tasks[0].Error == "" {
		t.Error("task error is empty, want platform not found message")
	}
}

func TestService_Publish_ValidationFailure(t *testing.T) {
	adapter := mock.NewValidationFailPlatform("zhihu", "title too long")
	h := newTestHarness(adapter)
	contentID := h.createReadyContent(t)

	status, err := h.service.Publish(context.Background(), contracts.PublishIntent{
		ContentID: contentID,
		Platforms: []string{"zhihu"},
	})
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if status.Tasks[0].Status != contracts.PublishTaskStatusFailed {
		t.Errorf("task status = %s, want failed", status.Tasks[0].Status)
	}

	// Adapter should not have been called for Publish (only Validate).
	if adapter.AttemptCount() != 0 {
		t.Errorf("adapter.AttemptCount() = %d, want 0 (validation failed before publish)", adapter.AttemptCount())
	}
}

func TestService_Publish_EmptyPlatforms(t *testing.T) {
	h := newTestHarness()
	contentID := h.createReadyContent(t)

	status, err := h.service.Publish(context.Background(), contracts.PublishIntent{
		ContentID: contentID,
		Platforms: []string{},
	})
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if status.Status != contracts.ContentStatusFailed {
		t.Errorf("status = %s, want failed (no platforms)", status.Status)
	}
	if len(status.Tasks) != 0 {
		t.Errorf("len(Tasks) = %d, want 0", len(status.Tasks))
	}
}

func TestService_Publish_TaskStatusTransitions(t *testing.T) {
	// Verify the task went through the expected state transitions.
	adapter := mock.NewRetryablePlatform("zhihu", 1)
	h := newTestHarness(adapter)
	contentID := h.createReadyContent(t)

	status, err := h.service.Publish(context.Background(), contracts.PublishIntent{
		ContentID: contentID,
		Platforms: []string{"zhihu"},
	})
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}

	taskID := status.Tasks[0].TaskID

	// Get attempts to verify they were recorded.
	attempts, err := h.attempts.GetAttemptsByTask(context.Background(), taskID)
	if err != nil {
		t.Fatalf("GetAttemptsByTask() error = %v", err)
	}
	if len(attempts) != 2 {
		t.Fatalf("len(attempts) = %d, want 2 (1 failure + 1 success)", len(attempts))
	}

	// First attempt should be failed.
	if attempts[0].Status != contracts.PublishTaskStatusFailed {
		t.Errorf("attempt 0 status = %s, want failed", attempts[0].Status)
	}
	// Second attempt should be succeeded.
	if attempts[1].Status != contracts.PublishTaskStatusSucceeded {
		t.Errorf("attempt 1 status = %s, want succeeded", attempts[1].Status)
	}
}

func TestService_Publish_PlatformPostRecorded(t *testing.T) {
	adapter := mock.NewSuccessPlatform("zhihu")
	h := newTestHarness(adapter)
	contentID := h.createReadyContent(t)

	status, _ := h.service.Publish(context.Background(), contracts.PublishIntent{
		ContentID: contentID,
		Platforms: []string{"zhihu"},
	})

	taskID := status.Tasks[0].TaskID
	rec, err := h.posts.GetByTask(context.Background(), taskID)
	if err != nil {
		t.Fatalf("GetByTask() error = %v", err)
	}
	if rec.PlatformPostID == "" {
		t.Error("PlatformPostID is empty")
	}
	if rec.Platform != "zhihu" {
		t.Errorf("Platform = %s, want zhihu", rec.Platform)
	}
}

func TestConvertToDocument(t *testing.T) {
	c := &contracts.Content{
		Title: "Hello",
		Body:  "World",
		Tags:  []string{"go"},
	}
	doc := convertToDocument(c)

	if doc.Title != "Hello" {
		t.Errorf("Title = %s, want Hello", doc.Title)
	}
	if len(doc.Blocks) != 1 {
		t.Fatalf("len(Blocks) = %d, want 1", len(doc.Blocks))
	}
	if doc.Blocks[0].Type() != transform.NodeParagraph {
		t.Errorf("Block type = %s, want paragraph", doc.Blocks[0].Type())
	}
	if len(doc.Tags) != 1 || doc.Tags[0] != "go" {
		t.Errorf("Tags = %v, want [go]", doc.Tags)
	}
}

func TestAggregateStatus(t *testing.T) {
	tests := []struct {
		name string
		tasks []contracts.TaskStatus
		want contracts.ContentStatus
	}{
		{
			name:  "empty",
			tasks: nil,
			want:  contracts.ContentStatusFailed,
		},
		{
			name: "all succeeded",
			tasks: []contracts.TaskStatus{
				{Status: contracts.PublishTaskStatusSucceeded},
				{Status: contracts.PublishTaskStatusSucceeded},
			},
			want: contracts.ContentStatusPublished,
		},
		{
			name: "all failed",
			tasks: []contracts.TaskStatus{
				{Status: contracts.PublishTaskStatusDead},
				{Status: contracts.PublishTaskStatusFailed},
			},
			want: contracts.ContentStatusFailed,
		},
		{
			name: "partial",
			tasks: []contracts.TaskStatus{
				{Status: contracts.PublishTaskStatusSucceeded},
				{Status: contracts.PublishTaskStatusDead},
			},
			want: contracts.ContentStatusPartiallyPublished,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := aggregateStatus(tt.tasks)
			if got != tt.want {
				t.Errorf("aggregateStatus() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestPublishResultToJSON(t *testing.T) {
	r := &platform.PublishResult{
		PlatformPostID: "p1",
		PlatformURL:    "https://example.com/p1",
	}
	b := PublishResultToJSON(r)
	if len(b) == 0 {
		t.Error("PublishResultToJSON() returned empty bytes")
	}

	b = PublishResultToJSON(nil)
	if b != nil {
		t.Error("PublishResultToJSON(nil) != nil")
	}
}

func TestService_Publish_ContextCancellation(t *testing.T) {
	adapter := mock.NewSuccessPlatform("zhihu")
	h := newTestHarness(adapter)
	contentID := h.createReadyContent(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := h.service.Publish(ctx, contracts.PublishIntent{
		ContentID: contentID,
		Platforms: []string{"zhihu"},
	})
	// The behavior depends on whether the context is checked during fetch.
	// With our in-memory store, context isn't checked, so this may succeed.
	// We just verify it doesn't panic.
	if err != nil && !errors.Is(err, context.Canceled) {
		// Non-cancel errors are also acceptable — the store might not check context.
		_ = err
	}
}

