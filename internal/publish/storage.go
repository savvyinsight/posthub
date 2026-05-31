package publish

import (
	"context"
	"crypto/rand"
	"fmt"
	"sync"
	"time"

	"github.com/savvyinsight/posthub/internal/contracts"
	"github.com/savvyinsight/posthub/internal/storage"
)

// generateID produces a UUID v4 string without external dependencies.
func generateID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// --- MemoryContentStore ---

type contentEntry struct {
	content contracts.Content
	version int
}

// MemoryContentStore is an in-memory implementation of storage.ContentStore.
type MemoryContentStore struct {
	mu      sync.RWMutex
	entries map[string]*contentEntry
}

var _ storage.ContentStore = (*MemoryContentStore)(nil)

// NewMemoryContentStore creates an empty in-memory content store.
func NewMemoryContentStore() *MemoryContentStore {
	return &MemoryContentStore{entries: make(map[string]*contentEntry)}
}

func (s *MemoryContentStore) CreateContent(_ context.Context, title, body string, tags []string) (*contracts.Content, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	c := contracts.Content{
		ID:        generateID(),
		Title:     title,
		Body:      body,
		Tags:      tags,
		Status:    contracts.ContentStatusDraft,
		CreatedAt: now,
		UpdatedAt: now,
	}
	s.entries[c.ID] = &contentEntry{content: c, version: 1}
	cp := c
	return &cp, nil
}

func (s *MemoryContentStore) GetContent(_ context.Context, id string) (*contracts.Content, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entry, ok := s.entries[id]
	if !ok {
		return nil, storage.ErrNotFound
	}
	cp := entry.content
	return &cp, nil
}

func (s *MemoryContentStore) ListContent(_ context.Context, status contracts.ContentStatus, page storage.Pagination) ([]*contracts.Content, storage.PageResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	page = page.Normalize()
	var filtered []*contracts.Content
	for _, entry := range s.entries {
		if status == "" || entry.content.Status == status {
			cp := entry.content
			filtered = append(filtered, &cp)
		}
	}

	total := len(filtered)
	start := page.Offset
	if start > total {
		start = total
	}
	end := start + page.Limit
	if end > total {
		end = total
	}

	return filtered[start:end], storage.PageResult{Total: total, Limit: page.Limit, Offset: page.Offset}, nil
}

func (s *MemoryContentStore) UpdateContent(_ context.Context, id string, status contracts.ContentStatus, version int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.entries[id]
	if !ok {
		return storage.ErrNotFound
	}
	if version > 0 && entry.version != version {
		return storage.ErrVersionConflict
	}
	if !entry.content.Status.CanTransitionTo(status) {
		return fmt.Errorf("invalid status transition: %s -> %s", entry.content.Status, status)
	}
	entry.content.Status = status
	entry.content.UpdatedAt = time.Now().UTC()
	entry.version++
	return nil
}

// --- MemoryPublishTaskStore ---

// MemoryPublishTaskStore is an in-memory implementation of storage.PublishTaskStore.
type MemoryPublishTaskStore struct {
	mu      sync.RWMutex
	tasks   map[string]*contracts.PublishTask
	byCombo map[string]string // "contentID:platform" -> taskID
}

var _ storage.PublishTaskStore = (*MemoryPublishTaskStore)(nil)

// NewMemoryPublishTaskStore creates an empty in-memory task store.
func NewMemoryPublishTaskStore() *MemoryPublishTaskStore {
	return &MemoryPublishTaskStore{
		tasks:   make(map[string]*contracts.PublishTask),
		byCombo: make(map[string]string),
	}
}

func (s *MemoryPublishTaskStore) CreateTask(_ context.Context, contentID, platform string) (*contracts.PublishTask, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	combo := contentID + ":" + platform
	if _, exists := s.byCombo[combo]; exists {
		return nil, storage.ErrConflict
	}

	now := time.Now().UTC()
	task := &contracts.PublishTask{
		ID:          generateID(),
		ContentID:   contentID,
		Platform:    platform,
		Status:      contracts.PublishTaskStatusPending,
		RetryPolicy: contracts.DefaultRetryPolicy(),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	s.tasks[task.ID] = task
	s.byCombo[combo] = task.ID
	return task, nil
}

func (s *MemoryPublishTaskStore) GetTask(_ context.Context, id string) (*contracts.PublishTask, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	task, ok := s.tasks[id]
	if !ok {
		return nil, storage.ErrNotFound
	}
	cp := *task
	return &cp, nil
}

func (s *MemoryPublishTaskStore) GetTasksByContent(_ context.Context, contentID string) ([]*contracts.PublishTask, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*contracts.PublishTask
	for _, task := range s.tasks {
		if task.ContentID == contentID {
			cp := *task
			result = append(result, &cp)
		}
	}
	return result, nil
}

func (s *MemoryPublishTaskStore) GetTaskByContentPlatform(_ context.Context, contentID, platform string) (*contracts.PublishTask, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	combo := contentID + ":" + platform
	taskID, ok := s.byCombo[combo]
	if !ok {
		return nil, storage.ErrNotFound
	}
	task := s.tasks[taskID]
	cp := *task
	return &cp, nil
}

func (s *MemoryPublishTaskStore) UpdateTaskStatus(_ context.Context, id string, status contracts.PublishTaskStatus, errMsg string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, ok := s.tasks[id]
	if !ok {
		return storage.ErrNotFound
	}
	if !task.Status.CanTransitionTo(status) {
		return fmt.Errorf("invalid task status transition: %s -> %s", task.Status, status)
	}
	task.Status = status
	task.Error = errMsg
	task.UpdatedAt = time.Now().UTC()
	return nil
}

func (s *MemoryPublishTaskStore) IncrementAttemptCount(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, ok := s.tasks[id]
	if !ok {
		return storage.ErrNotFound
	}
	task.AttemptCount++
	task.UpdatedAt = time.Now().UTC()
	return nil
}

// --- MemoryPublishAttemptStore ---

// MemoryPublishAttemptStore is an in-memory implementation of storage.PublishAttemptStore.
type MemoryPublishAttemptStore struct {
	mu      sync.RWMutex
	attempts map[string]*contracts.PublishAttempt
}

var _ storage.PublishAttemptStore = (*MemoryPublishAttemptStore)(nil)

// NewMemoryPublishAttemptStore creates an empty in-memory attempt store.
func NewMemoryPublishAttemptStore() *MemoryPublishAttemptStore {
	return &MemoryPublishAttemptStore{attempts: make(map[string]*contracts.PublishAttempt)}
}

func (s *MemoryPublishAttemptStore) CreateAttempt(_ context.Context, taskID string, attemptNumber int) (*contracts.PublishAttempt, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	attempt := &contracts.PublishAttempt{
		ID:            generateID(),
		TaskID:        taskID,
		AttemptNumber: attemptNumber,
		Status:        contracts.PublishTaskStatusProcessing,
		StartedAt:     time.Now().UTC(),
	}
	s.attempts[attempt.ID] = attempt
	cp := *attempt
	return &cp, nil
}

func (s *MemoryPublishAttemptStore) CompleteAttempt(_ context.Context, id string, status contracts.PublishTaskStatus, errMsg string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	attempt, ok := s.attempts[id]
	if !ok {
		return storage.ErrNotFound
	}
	now := time.Now().UTC()
	attempt.Status = status
	attempt.Error = errMsg
	attempt.CompletedAt = &now
	return nil
}

func (s *MemoryPublishAttemptStore) GetAttemptsByTask(_ context.Context, taskID string) ([]*contracts.PublishAttempt, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*contracts.PublishAttempt
	for _, a := range s.attempts {
		if a.TaskID == taskID {
			cp := *a
			result = append(result, &cp)
		}
	}
	return result, nil
}

// --- MemoryPlatformPostStore ---

// MemoryPlatformPostStore is an in-memory implementation of storage.PlatformPostStore.
type MemoryPlatformPostStore struct {
	mu      sync.RWMutex
	records map[string]*storage.PlatformPostRecord // keyed by taskID
}

var _ storage.PlatformPostStore = (*MemoryPlatformPostStore)(nil)

// NewMemoryPlatformPostStore creates an empty in-memory platform post store.
func NewMemoryPlatformPostStore() *MemoryPlatformPostStore {
	return &MemoryPlatformPostStore{records: make(map[string]*storage.PlatformPostRecord)}
}

func (s *MemoryPlatformPostStore) CreatePlatformPost(_ context.Context, taskID, platform, platformPostID, platformURL string, response []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.records[taskID]; exists {
		return storage.ErrConflict
	}
	s.records[taskID] = &storage.PlatformPostRecord{
		ID:             generateID(),
		TaskID:         taskID,
		Platform:       platform,
		PlatformPostID: platformPostID,
		PlatformURL:    platformURL,
		Response:       response,
	}
	return nil
}

func (s *MemoryPlatformPostStore) GetByTask(_ context.Context, taskID string) (*storage.PlatformPostRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rec, ok := s.records[taskID]
	if !ok {
		return nil, storage.ErrNotFound
	}
	cp := *rec
	return &cp, nil
}

// --- MemoryIdempotencyStore ---

type idempotencyEntry struct {
	entityID  string
	expiresAt time.Time
}

// MemoryIdempotencyStore is an in-memory implementation of storage.IdempotencyStore.
type MemoryIdempotencyStore struct {
	mu      sync.RWMutex
	entries map[string]*idempotencyEntry
}

var _ storage.IdempotencyStore = (*MemoryIdempotencyStore)(nil)

// NewMemoryIdempotencyStore creates an empty in-memory idempotency store.
func NewMemoryIdempotencyStore() *MemoryIdempotencyStore {
	return &MemoryIdempotencyStore{entries: make(map[string]*idempotencyEntry)}
}

func (s *MemoryIdempotencyStore) Claim(_ context.Context, key contracts.IdempotencyKey) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	fullKey := string(key.Scope) + ":" + key.Key
	entry, exists := s.entries[fullKey]
	if exists && time.Now().UTC().Before(entry.expiresAt) {
		return false, nil // already claimed
	}
	s.entries[fullKey] = &idempotencyEntry{
		entityID:  key.EntityID,
		expiresAt: time.Now().UTC().Add(key.TTL),
	}
	return true, nil
}

func (s *MemoryIdempotencyStore) Get(_ context.Context, key string, scope contracts.IdempotencyKeyScope) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	fullKey := string(scope) + ":" + key
	entry, exists := s.entries[fullKey]
	if !exists || time.Now().UTC().After(entry.expiresAt) {
		return "", storage.ErrNotFound
	}
	return entry.entityID, nil
}

func (s *MemoryIdempotencyStore) Cleanup(_ context.Context) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	removed := 0
	for k, entry := range s.entries {
		if now.After(entry.expiresAt) {
			delete(s.entries, k)
			removed++
		}
	}
	return removed, nil
}
