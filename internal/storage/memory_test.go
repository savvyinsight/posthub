package storage

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/savvyinsight/posthub/internal/contracts"
)

// --- Content Store Tests ---

func TestMemoryContentStore_CreateAndGet(t *testing.T) {
	s := NewMemoryStorage()
	ctx := context.Background()

	c, err := s.Content.CreateContent(ctx, "title", "body", []string{"go", "api"})
	if err != nil {
		t.Fatalf("CreateContent: %v", err)
	}
	if c.ID == "" {
		t.Error("expected non-empty ID")
	}
	if c.Status != contracts.ContentStatusDraft {
		t.Errorf("got status %q, want %q", c.Status, contracts.ContentStatusDraft)
	}
	if c.Title != "title" {
		t.Errorf("got title %q, want %q", c.Title, "title")
	}

	got, err := s.Content.GetContent(ctx, c.ID)
	if err != nil {
		t.Fatalf("GetContent: %v", err)
	}
	if got.ID != c.ID {
		t.Errorf("got ID %q, want %q", got.ID, c.ID)
	}
}

func TestMemoryContentStore_Get_NotFound(t *testing.T) {
	s := NewMemoryStorage()
	ctx := context.Background()

	_, err := s.Content.GetContent(ctx, "nonexistent")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("got error %v, want ErrNotFound", err)
	}
}

func TestMemoryContentStore_ListContent_Pagination(t *testing.T) {
	s := NewMemoryStorage()
	ctx := context.Background()

	// Create 5 content items.
	for i := range 5 {
		c, err := s.Content.CreateContent(ctx, fmt.Sprintf("title-%d", i), "body", nil)
		if err != nil {
			t.Fatalf("CreateContent %d: %v", i, err)
		}
		// Move to ready status for listing.
		err = s.Content.UpdateContent(ctx, c.ID, contracts.ContentStatusReady, 1)
		if err != nil {
			t.Fatalf("UpdateContent %d: %v", i, err)
		}
	}

	tests := []struct {
		name      string
		page      Pagination
		wantLen   int
		wantTotal int
	}{
		{name: "first_page", page: Pagination{Limit: 2, Offset: 0}, wantLen: 2, wantTotal: 5},
		{name: "second_page", page: Pagination{Limit: 2, Offset: 2}, wantLen: 2, wantTotal: 5},
		{name: "last_page", page: Pagination{Limit: 2, Offset: 4}, wantLen: 1, wantTotal: 5},
		{name: "beyond_total", page: Pagination{Limit: 10, Offset: 10}, wantLen: 0, wantTotal: 5},
		{name: "defaults", page: Pagination{}, wantLen: 5, wantTotal: 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, page, err := s.Content.ListContent(ctx, contracts.ContentStatusReady, tt.page)
			if err != nil {
				t.Fatalf("ListContent: %v", err)
			}
			if len(result) != tt.wantLen {
				t.Errorf("got %d items, want %d", len(result), tt.wantLen)
			}
			if page.Total != tt.wantTotal {
				t.Errorf("got total %d, want %d", page.Total, tt.wantTotal)
			}
		})
	}
}

func TestMemoryContentStore_UpdateContent_OptimisticLocking(t *testing.T) {
	s := NewMemoryStorage()
	ctx := context.Background()

	c, err := s.Content.CreateContent(ctx, "title", "body", nil)
	if err != nil {
		t.Fatalf("CreateContent: %v", err)
	}

	// Get versioned content.
	_, version, err := s.Content.GetContentVersioned(ctx, c.ID)
	if err != nil {
		t.Fatalf("GetContentVersioned: %v", err)
	}

	// Successful update with correct version.
	err = s.Content.UpdateContent(ctx, c.ID, contracts.ContentStatusReady, version)
	if err != nil {
		t.Fatalf("UpdateContent: %v", err)
	}

	// Stale version should fail.
	err = s.Content.UpdateContent(ctx, c.ID, contracts.ContentStatusPublishing, version)
	if !errors.Is(err, ErrVersionConflict) {
		t.Errorf("got error %v, want ErrVersionConflict", err)
	}

	// Correct version should succeed.
	err = s.Content.UpdateContent(ctx, c.ID, contracts.ContentStatusPublishing, version+1)
	if err != nil {
		t.Fatalf("UpdateContent with correct version: %v", err)
	}
}

func TestMemoryContentStore_UpdateContent_NotFound(t *testing.T) {
	s := NewMemoryStorage()
	ctx := context.Background()

	err := s.Content.UpdateContent(ctx, "nonexistent", contracts.ContentStatusReady, 1)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("got error %v, want ErrNotFound", err)
	}
}

// --- Publish Task Store Tests ---

func TestMemoryPublishTaskStore_CreateAndGet(t *testing.T) {
	s := NewMemoryStorage()
	ctx := context.Background()

	task, err := s.Tasks.CreateTask(ctx, "content-1", "zhihu")
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	if task.ID == "" {
		t.Error("expected non-empty ID")
	}
	if task.Status != contracts.PublishTaskStatusPending {
		t.Errorf("got status %q, want %q", task.Status, contracts.PublishTaskStatusPending)
	}

	got, err := s.Tasks.GetTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if got.ContentID != "content-1" {
		t.Errorf("got content_id %q, want %q", got.ContentID, "content-1")
	}
}

func TestMemoryPublishTaskStore_CreateTask_DuplicateConflict(t *testing.T) {
	s := NewMemoryStorage()
	ctx := context.Background()

	_, err := s.Tasks.CreateTask(ctx, "content-1", "zhihu")
	if err != nil {
		t.Fatalf("first CreateTask: %v", err)
	}

	_, err = s.Tasks.CreateTask(ctx, "content-1", "zhihu")
	if !errors.Is(err, ErrConflict) {
		t.Errorf("got error %v, want ErrConflict", err)
	}

	// Same content, different platform should succeed.
	_, err = s.Tasks.CreateTask(ctx, "content-1", "bilibili")
	if err != nil {
		t.Fatalf("different platform: %v", err)
	}
}

func TestMemoryPublishTaskStore_GetTask_NotFound(t *testing.T) {
	s := NewMemoryStorage()
	ctx := context.Background()

	_, err := s.Tasks.GetTask(ctx, "nonexistent")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("got error %v, want ErrNotFound", err)
	}
}

func TestMemoryPublishTaskStore_GetTasksByContent(t *testing.T) {
	s := NewMemoryStorage()
	ctx := context.Background()

	_, _ = s.Tasks.CreateTask(ctx, "content-1", "zhihu")
	_, _ = s.Tasks.CreateTask(ctx, "content-1", "bilibili")
	_, _ = s.Tasks.CreateTask(ctx, "content-2", "zhihu")

	tasks, err := s.Tasks.GetTasksByContent(ctx, "content-1")
	if err != nil {
		t.Fatalf("GetTasksByContent: %v", err)
	}
	if len(tasks) != 2 {
		t.Errorf("got %d tasks, want 2", len(tasks))
	}
}

func TestMemoryPublishTaskStore_GetTaskByContentPlatform(t *testing.T) {
	s := NewMemoryStorage()
	ctx := context.Background()

	created, _ := s.Tasks.CreateTask(ctx, "content-1", "zhihu")

	got, err := s.Tasks.GetTaskByContentPlatform(ctx, "content-1", "zhihu")
	if err != nil {
		t.Fatalf("GetTaskByContentPlatform: %v", err)
	}
	if got.ID != created.ID {
		t.Errorf("got ID %q, want %q", got.ID, created.ID)
	}

	_, err = s.Tasks.GetTaskByContentPlatform(ctx, "content-1", "weibo")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("got error %v, want ErrNotFound", err)
	}
}

func TestMemoryPublishTaskStore_UpdateTaskStatus(t *testing.T) {
	s := NewMemoryStorage()
	ctx := context.Background()

	task, _ := s.Tasks.CreateTask(ctx, "content-1", "zhihu")

	err := s.Tasks.UpdateTaskStatus(ctx, task.ID, contracts.PublishTaskStatusProcessing, "")
	if err != nil {
		t.Fatalf("UpdateTaskStatus: %v", err)
	}

	got, _ := s.Tasks.GetTask(ctx, task.ID)
	if got.Status != contracts.PublishTaskStatusProcessing {
		t.Errorf("got status %q, want %q", got.Status, contracts.PublishTaskStatusProcessing)
	}
}

func TestMemoryPublishTaskStore_UpdateTaskStatus_NotFound(t *testing.T) {
	s := NewMemoryStorage()
	ctx := context.Background()

	err := s.Tasks.UpdateTaskStatus(ctx, "nonexistent", contracts.PublishTaskStatusProcessing, "")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("got error %v, want ErrNotFound", err)
	}
}

func TestMemoryPublishTaskStore_IncrementAttemptCount(t *testing.T) {
	s := NewMemoryStorage()
	ctx := context.Background()

	task, _ := s.Tasks.CreateTask(ctx, "content-1", "zhihu")
	if task.AttemptCount != 0 {
		t.Errorf("got attempt_count %d, want 0", task.AttemptCount)
	}

	err := s.Tasks.IncrementAttemptCount(ctx, task.ID)
	if err != nil {
		t.Fatalf("IncrementAttemptCount: %v", err)
	}

	err = s.Tasks.IncrementAttemptCount(ctx, task.ID)
	if err != nil {
		t.Fatalf("IncrementAttemptCount: %v", err)
	}

	got, _ := s.Tasks.GetTask(ctx, task.ID)
	if got.AttemptCount != 2 {
		t.Errorf("got attempt_count %d, want 2", got.AttemptCount)
	}
}

// --- Publish Attempt Store Tests ---

func TestMemoryPublishAttemptStore_CreateAndGet(t *testing.T) {
	s := NewMemoryStorage()
	ctx := context.Background()

	attempt, err := s.Attempts.CreateAttempt(ctx, "task-1", 1)
	if err != nil {
		t.Fatalf("CreateAttempt: %v", err)
	}
	if attempt.ID == "" {
		t.Error("expected non-empty ID")
	}
	if attempt.AttemptNumber != 1 {
		t.Errorf("got attempt_number %d, want 1", attempt.AttemptNumber)
	}
	if attempt.Status != contracts.PublishTaskStatusProcessing {
		t.Errorf("got status %q, want %q", attempt.Status, contracts.PublishTaskStatusProcessing)
	}
}

func TestMemoryPublishAttemptStore_CompleteAttempt(t *testing.T) {
	s := NewMemoryStorage()
	ctx := context.Background()

	attempt, _ := s.Attempts.CreateAttempt(ctx, "task-1", 1)

	err := s.Attempts.CompleteAttempt(ctx, attempt.ID, contracts.PublishTaskStatusSucceeded, "")
	if err != nil {
		t.Fatalf("CompleteAttempt: %v", err)
	}

	attempts, _ := s.Attempts.GetAttemptsByTask(ctx, "task-1")
	if len(attempts) != 1 {
		t.Fatalf("got %d attempts, want 1", len(attempts))
	}
	if attempts[0].CompletedAt == nil {
		t.Error("expected completed_at to be set")
	}
}

func TestMemoryPublishAttemptStore_CompleteAttempt_NotFound(t *testing.T) {
	s := NewMemoryStorage()
	ctx := context.Background()

	err := s.Attempts.CompleteAttempt(ctx, "nonexistent", contracts.PublishTaskStatusFailed, "err")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("got error %v, want ErrNotFound", err)
	}
}

func TestMemoryPublishAttemptStore_GetAttemptsByTask_Ordered(t *testing.T) {
	s := NewMemoryStorage()
	ctx := context.Background()

	_, _ = s.Attempts.CreateAttempt(ctx, "task-1", 3)
	_, _ = s.Attempts.CreateAttempt(ctx, "task-1", 1)
	_, _ = s.Attempts.CreateAttempt(ctx, "task-1", 2)
	_, _ = s.Attempts.CreateAttempt(ctx, "task-2", 1)

	attempts, err := s.Attempts.GetAttemptsByTask(ctx, "task-1")
	if err != nil {
		t.Fatalf("GetAttemptsByTask: %v", err)
	}
	if len(attempts) != 3 {
		t.Fatalf("got %d attempts, want 3", len(attempts))
	}
	for i, a := range attempts {
		if a.AttemptNumber != i+1 {
			t.Errorf("attempt[%d].AttemptNumber = %d, want %d", i, a.AttemptNumber, i+1)
		}
	}
}

// --- Platform Post Store Tests ---

func TestMemoryPlatformPostStore_CreateAndGet(t *testing.T) {
	s := NewMemoryStorage()
	ctx := context.Background()

	err := s.Posts.CreatePlatformPost(ctx, "task-1", "zhihu", "post-123", "https://zhihu.com/p/123", []byte(`{"ok":true}`))
	if err != nil {
		t.Fatalf("CreatePlatformPost: %v", err)
	}

	rec, err := s.Posts.GetByTask(ctx, "task-1")
	if err != nil {
		t.Fatalf("GetByTask: %v", err)
	}
	if rec.PlatformPostID != "post-123" {
		t.Errorf("got platform_post_id %q, want %q", rec.PlatformPostID, "post-123")
	}
	if rec.PlatformURL != "https://zhihu.com/p/123" {
		t.Errorf("got platform_url %q, want %q", rec.PlatformURL, "https://zhihu.com/p/123")
	}
}

func TestMemoryPlatformPostStore_GetByTask_NotFound(t *testing.T) {
	s := NewMemoryStorage()
	ctx := context.Background()

	_, err := s.Posts.GetByTask(ctx, "nonexistent")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("got error %v, want ErrNotFound", err)
	}
}

func TestMemoryPlatformPostStore_ResponseCopied(t *testing.T) {
	s := NewMemoryStorage()
	ctx := context.Background()

	original := []byte(`{"key":"value"}`)
	_ = s.Posts.CreatePlatformPost(ctx, "task-1", "zhihu", "p1", "", original)

	rec, _ := s.Posts.GetByTask(ctx, "task-1")
	rec.Response[0] = 'X' // mutate the copy

	rec2, _ := s.Posts.GetByTask(ctx, "task-1")
	if string(rec2.Response) != string(original) {
		t.Errorf("response was mutated: got %q, want %q", rec2.Response, original)
	}
}

// --- Asset Store Tests ---

func TestMemoryAssetStore_CreateAndGet(t *testing.T) {
	s := NewMemoryStorage()
	ctx := context.Background()

	asset := &Asset{
		ContentID: "content-1",
		Type:      "image",
		URL:       "https://example.com/img.jpg",
	}
	err := s.Assets.CreateAsset(ctx, asset)
	if err != nil {
		t.Fatalf("CreateAsset: %v", err)
	}
	if asset.ID == "" {
		t.Error("expected ID to be generated")
	}

	got, err := s.Assets.GetAsset(ctx, asset.ID)
	if err != nil {
		t.Fatalf("GetAsset: %v", err)
	}
	if got.URL != asset.URL {
		t.Errorf("got url %q, want %q", got.URL, asset.URL)
	}
}

func TestMemoryAssetStore_GetAsset_NotFound(t *testing.T) {
	s := NewMemoryStorage()
	ctx := context.Background()

	_, err := s.Assets.GetAsset(ctx, "nonexistent")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("got error %v, want ErrNotFound", err)
	}
}

func TestMemoryAssetStore_GetAssetsByContent(t *testing.T) {
	s := NewMemoryStorage()
	ctx := context.Background()

	_ = s.Assets.CreateAsset(ctx, &Asset{ContentID: "c1", Type: "image", URL: "a.jpg"})
	_ = s.Assets.CreateAsset(ctx, &Asset{ContentID: "c1", Type: "image", URL: "b.jpg"})
	_ = s.Assets.CreateAsset(ctx, &Asset{ContentID: "c2", Type: "image", URL: "c.jpg"})

	assets, err := s.Assets.GetAssetsByContent(ctx, "c1")
	if err != nil {
		t.Fatalf("GetAssetsByContent: %v", err)
	}
	if len(assets) != 2 {
		t.Errorf("got %d assets, want 2", len(assets))
	}
}

// --- Idempotency Store Tests ---

func TestMemoryIdempotencyStore_ClaimAndCheck(t *testing.T) {
	s := NewMemoryStorage()
	ctx := context.Background()

	key := contracts.IdempotencyKey{
		Key:      "content-1:zhihu",
		Scope:    contracts.IdempotencyScopeTask,
		EntityID: "task-1",
		TTL:      1 * time.Hour,
	}

	ok, err := s.Idempotency.Claim(ctx, key)
	if err != nil {
		t.Fatalf("Claim: %v", err)
	}
	if !ok {
		t.Fatal("expected first claim to succeed")
	}

	// Duplicate claim should return false.
	ok, err = s.Idempotency.Claim(ctx, key)
	if err != nil {
		t.Fatalf("Claim duplicate: %v", err)
	}
	if ok {
		t.Fatal("expected duplicate claim to return false")
	}

	// Get should return the entity ID.
	entityID, err := s.Idempotency.Get(ctx, key.Key, key.Scope)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if entityID != "task-1" {
		t.Errorf("got entity_id %q, want %q", entityID, "task-1")
	}
}

func TestMemoryIdempotencyStore_Get_NotFound(t *testing.T) {
	s := NewMemoryStorage()
	ctx := context.Background()

	_, err := s.Idempotency.Get(ctx, "nonexistent", contracts.IdempotencyScopeTask)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("got error %v, want ErrNotFound", err)
	}
}

func TestMemoryIdempotencyStore_ExpiredKey(t *testing.T) {
	s := NewMemoryStorage()
	ctx := context.Background()

	// Use a fake clock to test expiration.
	originalNowUTC := nowUTC
	baseTime := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	nowUTC = func() time.Time { return baseTime }
	defer func() { nowUTC = originalNowUTC }()

	key := contracts.IdempotencyKey{
		Key:      "content-1:zhihu",
		Scope:    contracts.IdempotencyScopeTask,
		EntityID: "task-1",
		TTL:      1 * time.Minute,
	}

	ok, err := s.Idempotency.Claim(ctx, key)
	if err != nil || !ok {
		t.Fatalf("Claim: ok=%v, err=%v", ok, err)
	}

	// Advance time past TTL.
	nowUTC = func() time.Time { return baseTime.Add(2 * time.Minute) }

	// Expired key should not be found.
	_, err = s.Idempotency.Get(ctx, key.Key, key.Scope)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("got error %v, want ErrNotFound for expired key", err)
	}

	// Should be able to re-claim expired key.
	ok, err = s.Idempotency.Claim(ctx, key)
	if err != nil || !ok {
		t.Fatalf("re-claim: ok=%v, err=%v", ok, err)
	}
}

func TestMemoryIdempotencyStore_Cleanup(t *testing.T) {
	s := NewMemoryStorage()
	ctx := context.Background()

	originalNowUTC := nowUTC
	baseTime := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	nowUTC = func() time.Time { return baseTime }
	defer func() { nowUTC = originalNowUTC }()

	// Create two keys with different TTLs.
	_, _ = s.Idempotency.Claim(ctx, contracts.IdempotencyKey{
		Key: "k1", Scope: contracts.IdempotencyScopeTask, EntityID: "e1", TTL: 1 * time.Minute,
	})
	_, _ = s.Idempotency.Claim(ctx, contracts.IdempotencyKey{
		Key: "k2", Scope: contracts.IdempotencyScopeTask, EntityID: "e2", TTL: 10 * time.Minute,
	})

	// Advance past first TTL but not second.
	nowUTC = func() time.Time { return baseTime.Add(5 * time.Minute) }

	removed, err := s.Idempotency.Cleanup(ctx)
	if err != nil {
		t.Fatalf("Cleanup: %v", err)
	}
	if removed != 1 {
		t.Errorf("removed %d, want 1", removed)
	}

	// k1 should be gone, k2 should remain.
	_, err = s.Idempotency.Get(ctx, "k1", contracts.IdempotencyScopeTask)
	if !errors.Is(err, ErrNotFound) {
		t.Error("expected k1 to be cleaned up")
	}
	_, err = s.Idempotency.Get(ctx, "k2", contracts.IdempotencyScopeTask)
	if err != nil {
		t.Errorf("expected k2 to remain: %v", err)
	}
}

// --- Transaction Tests ---

func TestMemoryTx_Run_Commit(t *testing.T) {
	s := NewMemoryStorage()
	ctx := context.Background()

	tx := s.Begin()
	err := tx.Run(ctx, func(scope *TxScope) error {
		c, err := scope.ContentStore.CreateContent(ctx, "title", "body", nil)
		if err != nil {
			return err
		}
		_, err = scope.PublishTaskStore.CreateTask(ctx, c.ID, "zhihu")
		return err
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Verify both were created.
	c, _, _ := s.Content.ListContent(ctx, contracts.ContentStatusDraft, Pagination{Limit: 10})
	if len(c) != 1 {
		t.Errorf("got %d content, want 1", len(c))
	}
}

func TestMemoryTx_Run_PropagatesError(t *testing.T) {
	s := NewMemoryStorage()
	ctx := context.Background()

	// Create two tasks for the same content/platform inside the transaction.
	// The second should fail with ErrConflict.
	tx := s.Begin()
	err := tx.Run(ctx, func(scope *TxScope) error {
		_, err := scope.PublishTaskStore.CreateTask(ctx, "content-1", "zhihu")
		if err != nil {
			return err
		}
		// Duplicate — should conflict.
		_, err = scope.PublishTaskStore.CreateTask(ctx, "content-1", "zhihu")
		return err
	})
	if !errors.Is(err, ErrConflict) {
		t.Errorf("got error %v, want ErrConflict", err)
	}
}

func TestMemoryTx_Run_ErrorDoesNotAffectOtherOperations(t *testing.T) {
	s := NewMemoryStorage()
	ctx := context.Background()

	// Run a failing transaction.
	_ = s.Begin().Run(ctx, func(scope *TxScope) error {
		_, _ = scope.ContentStore.CreateContent(ctx, "title1", "body", nil)
		return fmt.Errorf("simulated error")
	})

	// Content from the failed tx should still exist (MVP: no true rollback).
	result, _, _ := s.Content.ListContent(ctx, contracts.ContentStatusDraft, Pagination{Limit: 10})
	if len(result) != 1 {
		t.Errorf("got %d content, want 1 (MVP: no rollback)", len(result))
	}
}

// --- Concurrency Tests ---

func TestMemoryContentStore_Concurrent(t *testing.T) {
	s := NewMemoryStorage()
	ctx := context.Background()
	const goroutines = 50

	var wg sync.WaitGroup
	errs := make(chan error, goroutines)

	for i := range goroutines {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			c, err := s.Content.CreateContent(ctx, fmt.Sprintf("title-%d", id), "body", nil)
			if err != nil {
				errs <- err
				return
			}
			_, err = s.Content.GetContent(ctx, c.ID)
			if err != nil {
				errs <- err
			}
		}(i)
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Errorf("concurrent error: %v", err)
	}

	result, _, _ := s.Content.ListContent(ctx, contracts.ContentStatusDraft, Pagination{Limit: 100})
	if len(result) != goroutines {
		t.Errorf("got %d content, want %d", len(result), goroutines)
	}
}

func TestMemoryPublishTaskStore_Concurrent(t *testing.T) {
	s := NewMemoryStorage()
	ctx := context.Background()
	const goroutines = 50

	var wg sync.WaitGroup
	errs := make(chan error, goroutines)

	for i := range goroutines {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			_, err := s.Tasks.CreateTask(ctx, fmt.Sprintf("content-%d", id), "zhihu")
			if err != nil {
				errs <- err
			}
		}(i)
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Errorf("concurrent error: %v", err)
	}
}

func TestMemoryIdempotencyStore_Concurrent(t *testing.T) {
	s := NewMemoryStorage()
	ctx := context.Background()
	const goroutines = 50

	var wg sync.WaitGroup
	results := make(chan bool, goroutines)

	for range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ok, _ := s.Idempotency.Claim(ctx, contracts.IdempotencyKey{
				Key:      "same-key",
				Scope:    contracts.IdempotencyScopeTask,
				EntityID: "entity",
				TTL:      1 * time.Hour,
			})
			results <- ok
		}()
	}

	wg.Wait()
	close(results)

	claimed := 0
	for ok := range results {
		if ok {
			claimed++
		}
	}
	if claimed != 1 {
		t.Errorf("got %d successful claims, want 1", claimed)
	}
}

// --- Pagination Tests ---

func TestPagination_Normalize(t *testing.T) {
	tests := []struct {
		name    string
		input   Pagination
		wantLim int
		wantOff int
	}{
		{name: "zero_defaults", input: Pagination{}, wantLim: 20, wantOff: 0},
		{name: "negative_limit", input: Pagination{Limit: -1}, wantLim: 20, wantOff: 0},
		{name: "over_max", input: Pagination{Limit: 500}, wantLim: 100, wantOff: 0},
		{name: "negative_offset", input: Pagination{Limit: 10, Offset: -5}, wantLim: 10, wantOff: 0},
		{name: "valid", input: Pagination{Limit: 50, Offset: 10}, wantLim: 50, wantOff: 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.input.Normalize()
			if got.Limit != tt.wantLim {
				t.Errorf("Limit: got %d, want %d", got.Limit, tt.wantLim)
			}
			if got.Offset != tt.wantOff {
				t.Errorf("Offset: got %d, want %d", got.Offset, tt.wantOff)
			}
		})
	}
}

// --- ID Generation Tests ---

func TestGenerateID_Unique(t *testing.T) {
	seen := make(map[string]bool)
	const count = 1000

	for range count {
		id := generateID()
		if id == "" {
			t.Fatal("expected non-empty ID")
		}
		if seen[id] {
			t.Fatalf("duplicate ID: %s", id)
		}
		seen[id] = true
	}
}

func TestGenerateID_Format(t *testing.T) {
	id := generateID()
	// UUID v4 format: 8-4-4-4-12
	if len(id) != 36 {
		t.Errorf("got length %d, want 36", len(id))
	}
	if id[8] != '-' || id[13] != '-' || id[18] != '-' || id[23] != '-' {
		t.Errorf("invalid UUID format: %s", id)
	}
	// Version nibble should be 4.
	if id[14] != '4' {
		t.Errorf("got version %c, want 4", id[14])
	}
}
