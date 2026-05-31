package publish

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/savvyinsight/posthub/internal/contracts"
	"github.com/savvyinsight/posthub/internal/storage"
)

// --- MemoryContentStore tests ---

func TestMemoryContentStore_CreateAndGet(t *testing.T) {
	s := NewMemoryContentStore()
	ctx := context.Background()

	c, err := s.CreateContent(ctx, "title", "body", []string{"go"})
	if err != nil {
		t.Fatalf("CreateContent() error = %v", err)
	}
	if c.ID == "" {
		t.Error("CreateContent() ID is empty")
	}
	if c.Status != contracts.ContentStatusDraft {
		t.Errorf("CreateContent() status = %s, want draft", c.Status)
	}

	got, err := s.GetContent(ctx, c.ID)
	if err != nil {
		t.Fatalf("GetContent() error = %v", err)
	}
	if got.Title != "title" {
		t.Errorf("GetContent().Title = %s, want 'title'", got.Title)
	}
}

func TestMemoryContentStore_GetNotFound(t *testing.T) {
	s := NewMemoryContentStore()
	_, err := s.GetContent(context.Background(), "nonexistent")
	if !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("GetContent() error = %v, want ErrNotFound", err)
	}
}

func TestMemoryContentStore_UpdateContent(t *testing.T) {
	s := NewMemoryContentStore()
	ctx := context.Background()

	c, _ := s.CreateContent(ctx, "t", "b", nil)
	// Version 0 skips check in our MVP store.
	if err := s.UpdateContent(ctx, c.ID, contracts.ContentStatusReady, 0); err != nil {
		t.Fatalf("UpdateContent() error = %v", err)
	}

	got, _ := s.GetContent(ctx, c.ID)
	if got.Status != contracts.ContentStatusReady {
		t.Errorf("status after update = %s, want ready", got.Status)
	}
}

func TestMemoryContentStore_UpdateContent_VersionConflict(t *testing.T) {
	s := NewMemoryContentStore()
	ctx := context.Background()

	c, _ := s.CreateContent(ctx, "t", "b", nil)
	// Pass wrong version.
	err := s.UpdateContent(ctx, c.ID, contracts.ContentStatusReady, 99)
	if !errors.Is(err, storage.ErrVersionConflict) {
		t.Errorf("UpdateContent() error = %v, want ErrVersionConflict", err)
	}
}

func TestMemoryContentStore_UpdateContent_InvalidTransition(t *testing.T) {
	s := NewMemoryContentStore()
	ctx := context.Background()

	c, _ := s.CreateContent(ctx, "t", "b", nil)
	// draft -> published is invalid.
	err := s.UpdateContent(ctx, c.ID, contracts.ContentStatusPublished, 0)
	if err == nil {
		t.Error("UpdateContent() error = nil, want invalid transition error")
	}
}

func TestMemoryContentStore_UpdateContent_NotFound(t *testing.T) {
	s := NewMemoryContentStore()
	err := s.UpdateContent(context.Background(), "nope", contracts.ContentStatusReady, 0)
	if !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("UpdateContent() error = %v, want ErrNotFound", err)
	}
}

func TestMemoryContentStore_ListContent(t *testing.T) {
	s := NewMemoryContentStore()
	ctx := context.Background()

	s.CreateContent(ctx, "a", "body", nil)
	s.CreateContent(ctx, "b", "body", nil)

	all, pr, err := s.ListContent(ctx, "", storage.Pagination{Limit: 10})
	if err != nil {
		t.Fatalf("ListContent() error = %v", err)
	}
	if len(all) != 2 {
		t.Errorf("ListContent() returned %d items, want 2", len(all))
	}
	if pr.Total != 2 {
		t.Errorf("PageResult.Total = %d, want 2", pr.Total)
	}
}

func TestMemoryContentStore_ListContent_FilterByStatus(t *testing.T) {
	s := NewMemoryContentStore()
	ctx := context.Background()

	c, _ := s.CreateContent(ctx, "a", "body", nil)
	s.CreateContent(ctx, "b", "body", nil)
	s.UpdateContent(ctx, c.ID, contracts.ContentStatusReady, 0)

	ready, _, _ := s.ListContent(ctx, contracts.ContentStatusReady, storage.Pagination{Limit: 10})
	if len(ready) != 1 {
		t.Errorf("ListContent(ready) returned %d items, want 1", len(ready))
	}
}

// --- MemoryPublishTaskStore tests ---

func TestMemoryPublishTaskStore_CreateAndGet(t *testing.T) {
	s := NewMemoryPublishTaskStore()
	ctx := context.Background()

	task, err := s.CreateTask(ctx, "content-1", "zhihu")
	if err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	if task.ID == "" {
		t.Error("CreateTask() ID is empty")
	}
	if task.Status != contracts.PublishTaskStatusPending {
		t.Errorf("CreateTask() status = %s, want pending", task.Status)
	}

	got, err := s.GetTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("GetTask() error = %v", err)
	}
	if got.Platform != "zhihu" {
		t.Errorf("GetTask().Platform = %s, want zhihu", got.Platform)
	}
}

func TestMemoryPublishTaskStore_CreateTask_Conflict(t *testing.T) {
	s := NewMemoryPublishTaskStore()
	ctx := context.Background()

	s.CreateTask(ctx, "content-1", "zhihu")
	_, err := s.CreateTask(ctx, "content-1", "zhihu")
	if !errors.Is(err, storage.ErrConflict) {
		t.Errorf("CreateTask() error = %v, want ErrConflict", err)
	}
}

func TestMemoryPublishTaskStore_GetTaskByContentPlatform(t *testing.T) {
	s := NewMemoryPublishTaskStore()
	ctx := context.Background()

	s.CreateTask(ctx, "c1", "zhihu")
	s.CreateTask(ctx, "c1", "bilibili")

	got, err := s.GetTaskByContentPlatform(ctx, "c1", "zhihu")
	if err != nil {
		t.Fatalf("GetTaskByContentPlatform() error = %v", err)
	}
	if got.Platform != "zhihu" {
		t.Errorf("Platform = %s, want zhihu", got.Platform)
	}
}

func TestMemoryPublishTaskStore_GetTasksByContent(t *testing.T) {
	s := NewMemoryPublishTaskStore()
	ctx := context.Background()

	s.CreateTask(ctx, "c1", "zhihu")
	s.CreateTask(ctx, "c1", "bilibili")
	s.CreateTask(ctx, "c2", "zhihu")

	tasks, err := s.GetTasksByContent(ctx, "c1")
	if err != nil {
		t.Fatalf("GetTasksByContent() error = %v", err)
	}
	if len(tasks) != 2 {
		t.Errorf("GetTasksByContent() returned %d tasks, want 2", len(tasks))
	}
}

func TestMemoryPublishTaskStore_UpdateTaskStatus(t *testing.T) {
	s := NewMemoryPublishTaskStore()
	ctx := context.Background()

	task, _ := s.CreateTask(ctx, "c1", "zhihu")

	if err := s.UpdateTaskStatus(ctx, task.ID, contracts.PublishTaskStatusProcessing, ""); err != nil {
		t.Fatalf("UpdateTaskStatus(pending->processing) error = %v", err)
	}

	got, _ := s.GetTask(ctx, task.ID)
	if got.Status != contracts.PublishTaskStatusProcessing {
		t.Errorf("status = %s, want processing", got.Status)
	}
}

func TestMemoryPublishTaskStore_UpdateTaskStatus_InvalidTransition(t *testing.T) {
	s := NewMemoryPublishTaskStore()
	ctx := context.Background()

	task, _ := s.CreateTask(ctx, "c1", "zhihu")

	err := s.UpdateTaskStatus(ctx, task.ID, contracts.PublishTaskStatusSucceeded, "")
	if err == nil {
		t.Error("UpdateTaskStatus(pending->succeeded) error = nil, want error")
	}
}

func TestMemoryPublishTaskStore_IncrementAttemptCount(t *testing.T) {
	s := NewMemoryPublishTaskStore()
	ctx := context.Background()

	task, _ := s.CreateTask(ctx, "c1", "zhihu")
	s.IncrementAttemptCount(ctx, task.ID)
	s.IncrementAttemptCount(ctx, task.ID)

	got, _ := s.GetTask(ctx, task.ID)
	if got.AttemptCount != 2 {
		t.Errorf("AttemptCount = %d, want 2", got.AttemptCount)
	}
}

// --- MemoryPublishAttemptStore tests ---

func TestMemoryPublishAttemptStore_CreateAndComplete(t *testing.T) {
	s := NewMemoryPublishAttemptStore()
	ctx := context.Background()

	attempt, err := s.CreateAttempt(ctx, "task-1", 1)
	if err != nil {
		t.Fatalf("CreateAttempt() error = %v", err)
	}
	if attempt.Status != contracts.PublishTaskStatusProcessing {
		t.Errorf("CreateAttempt() status = %s, want processing", attempt.Status)
	}

	err = s.CompleteAttempt(ctx, attempt.ID, contracts.PublishTaskStatusSucceeded, "")
	if err != nil {
		t.Fatalf("CompleteAttempt() error = %v", err)
	}

	attempts, _ := s.GetAttemptsByTask(ctx, "task-1")
	if len(attempts) != 1 {
		t.Fatalf("GetAttemptsByTask() returned %d, want 1", len(attempts))
	}
	if attempts[0].CompletedAt == nil {
		t.Error("CompletedAt is nil after CompleteAttempt")
	}
}

func TestMemoryPublishAttemptStore_GetAttemptsByTask_Multiple(t *testing.T) {
	s := NewMemoryPublishAttemptStore()
	ctx := context.Background()

	s.CreateAttempt(ctx, "task-1", 1)
	s.CreateAttempt(ctx, "task-1", 2)
	s.CreateAttempt(ctx, "task-2", 1)

	attempts, _ := s.GetAttemptsByTask(ctx, "task-1")
	if len(attempts) != 2 {
		t.Errorf("GetAttemptsByTask() returned %d, want 2", len(attempts))
	}
}

// --- MemoryPlatformPostStore tests ---

func TestMemoryPlatformPostStore_CreateAndGet(t *testing.T) {
	s := NewMemoryPlatformPostStore()
	ctx := context.Background()

	err := s.CreatePlatformPost(ctx, "task-1", "zhihu", "post-123", "https://zhihu.com/p/123", []byte(`{}`))
	if err != nil {
		t.Fatalf("CreatePlatformPost() error = %v", err)
	}

	got, err := s.GetByTask(ctx, "task-1")
	if err != nil {
		t.Fatalf("GetByTask() error = %v", err)
	}
	if got.PlatformPostID != "post-123" {
		t.Errorf("PlatformPostID = %s, want post-123", got.PlatformPostID)
	}
}

func TestMemoryPlatformPostStore_Conflict(t *testing.T) {
	s := NewMemoryPlatformPostStore()
	ctx := context.Background()

	s.CreatePlatformPost(ctx, "task-1", "zhihu", "p1", "url", nil)
	err := s.CreatePlatformPost(ctx, "task-1", "zhihu", "p2", "url", nil)
	if !errors.Is(err, storage.ErrConflict) {
		t.Errorf("CreatePlatformPost() error = %v, want ErrConflict", err)
	}
}

func TestMemoryPlatformPostStore_GetByTask_NotFound(t *testing.T) {
	s := NewMemoryPlatformPostStore()
	_, err := s.GetByTask(context.Background(), "nonexistent")
	if !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("GetByTask() error = %v, want ErrNotFound", err)
	}
}

// --- MemoryIdempotencyStore tests ---

func TestMemoryIdempotencyStore_Claim(t *testing.T) {
	s := NewMemoryIdempotencyStore()
	ctx := context.Background()

	key := contracts.IdempotencyKey{
		Key:       "c1:zhihu",
		Scope:     contracts.IdempotencyScopeTask,
		EntityID:  "task-1",
		CreatedAt: time.Now().UTC(),
		TTL:       time.Hour,
	}

	ok, err := s.Claim(ctx, key)
	if err != nil {
		t.Fatalf("Claim() error = %v", err)
	}
	if !ok {
		t.Error("Claim() = false, want true (first claim)")
	}

	// Second claim should fail.
	ok, err = s.Claim(ctx, key)
	if err != nil {
		t.Fatalf("Claim() second error = %v", err)
	}
	if ok {
		t.Error("Claim() second = true, want false (duplicate)")
	}
}

func TestMemoryIdempotencyStore_Get(t *testing.T) {
	s := NewMemoryIdempotencyStore()
	ctx := context.Background()

	key := contracts.IdempotencyKey{
		Key:      "c1:zhihu",
		Scope:    contracts.IdempotencyScopeTask,
		EntityID: "task-1",
		TTL:      time.Hour,
	}
	s.Claim(ctx, key)

	entityID, err := s.Get(ctx, "c1:zhihu", contracts.IdempotencyScopeTask)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if entityID != "task-1" {
		t.Errorf("Get() = %s, want task-1", entityID)
	}
}

func TestMemoryIdempotencyStore_Get_NotFound(t *testing.T) {
	s := NewMemoryIdempotencyStore()
	_, err := s.Get(context.Background(), "nonexistent", contracts.IdempotencyScopeTask)
	if !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("Get() error = %v, want ErrNotFound", err)
	}
}

func TestMemoryIdempotencyStore_Cleanup(t *testing.T) {
	s := NewMemoryIdempotencyStore()
	ctx := context.Background()

	// Claim with zero TTL (already expired).
	s.Claim(ctx, contracts.IdempotencyKey{
		Key:      "expired",
		Scope:    contracts.IdempotencyScopeTask,
		EntityID: "x",
		TTL:      0,
	})

	removed, err := s.Cleanup(ctx)
	if err != nil {
		t.Fatalf("Cleanup() error = %v", err)
	}
	if removed != 1 {
		t.Errorf("Cleanup() removed = %d, want 1", removed)
	}
}
