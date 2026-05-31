// In-memory implementations of all storage interfaces.
//
// These implementations are intended for MVP, testing, and development.
// They are concurrency-safe but not durable — data lives only in process memory.
//
// All stores share a single sync.RWMutex to support transactional semantics
// via the MemoryTx implementation.
package storage

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/savvyinsight/posthub/internal/contracts"
)

// MemoryStorage holds all in-memory store implementations.
//
// Construct via NewMemoryStorage. All stores share the same mutex
// to support transactional semantics.
type MemoryStorage struct {
	Content     *MemoryContentStore
	Tasks       *MemoryPublishTaskStore
	Attempts    *MemoryPublishAttemptStore
	Posts       *MemoryPlatformPostStore
	Assets      *MemoryAssetStore
	Idempotency *MemoryIdempotencyStore
	mu          sync.RWMutex

	content     map[string]*contentRecord
	tasks       map[string]*contracts.PublishTask
	attempts    map[string]*contracts.PublishAttempt
	posts       map[string]*PlatformPostRecord
	assets      map[string]*Asset
	idempotency map[string]*idempotencyRecord
}

// NewMemoryStorage creates a fully wired in-memory storage layer.
func NewMemoryStorage() *MemoryStorage {
	m := &MemoryStorage{
		content:     make(map[string]*contentRecord),
		tasks:       make(map[string]*contracts.PublishTask),
		attempts:    make(map[string]*contracts.PublishAttempt),
		posts:       make(map[string]*PlatformPostRecord),
		assets:      make(map[string]*Asset),
		idempotency: make(map[string]*idempotencyRecord),
	}
	m.Content = &MemoryContentStore{storage: m}
	m.Tasks = &MemoryPublishTaskStore{storage: m}
	m.Attempts = &MemoryPublishAttemptStore{storage: m}
	m.Posts = &MemoryPlatformPostStore{storage: m}
	m.Assets = &MemoryAssetStore{storage: m}
	m.Idempotency = &MemoryIdempotencyStore{storage: m}
	return m
}

// Begin starts a new in-memory transaction.
func (m *MemoryStorage) Begin() *MemoryTx {
	return &MemoryTx{storage: m}
}

// --- Content Store ---

// MemoryContentStore is a concurrency-safe in-memory ContentStore.
type MemoryContentStore struct {
	storage *MemoryStorage
}

func (s *MemoryContentStore) CreateContent(_ context.Context, title, body string, tags []string) (*contracts.Content, error) {
	s.storage.mu.Lock()
	defer s.storage.mu.Unlock()

	now := nowUTC()
	c := &contracts.Content{
		ID:        generateID(),
		Title:     title,
		Body:      body,
		Tags:      tags,
		Status:    contracts.ContentStatusDraft,
		CreatedAt: now,
		UpdatedAt: now,
	}
	s.storage.content[c.ID] = &contentRecord{content: c, version: 1}
	return copyContent(c), nil
}

func (s *MemoryContentStore) GetContent(_ context.Context, id string) (*contracts.Content, error) {
	s.storage.mu.RLock()
	defer s.storage.mu.RUnlock()

	rec, ok := s.storage.content[id]
	if !ok {
		return nil, fmt.Errorf("content %s: %w", id, ErrNotFound)
	}
	return copyContent(rec.content), nil
}

func (s *MemoryContentStore) ListContent(_ context.Context, status contracts.ContentStatus, page Pagination) ([]*contracts.Content, PageResult, error) {
	s.storage.mu.RLock()
	defer s.storage.mu.RUnlock()

	page = page.Normalize()

	var filtered []*contracts.Content
	for _, rec := range s.storage.content {
		if rec.content.Status == status {
			filtered = append(filtered, rec.content)
		}
	}

	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].CreatedAt.After(filtered[j].CreatedAt)
	})

	total := len(filtered)
	start := page.Offset
	if start > total {
		start = total
	}
	end := start + page.Limit
	if end > total {
		end = total
	}

	result := make([]*contracts.Content, 0, end-start)
	for _, c := range filtered[start:end] {
		result = append(result, copyContent(c))
	}

	return result, PageResult{Total: total, Limit: page.Limit, Offset: page.Offset}, nil
}

func (s *MemoryContentStore) UpdateContent(_ context.Context, id string, status contracts.ContentStatus, version int) error {
	s.storage.mu.Lock()
	defer s.storage.mu.Unlock()

	rec, ok := s.storage.content[id]
	if !ok {
		return fmt.Errorf("content %s: %w", id, ErrNotFound)
	}
	if rec.version != version {
		return fmt.Errorf("content %s: expected version %d, got %d: %w", id, version, rec.version, ErrVersionConflict)
	}
	rec.content.Status = status
	rec.content.UpdatedAt = nowUTC()
	rec.version++
	return nil
}

// GetContentVersioned returns the content with its current version.
// This is specific to the in-memory implementation for optimistic locking.
func (s *MemoryContentStore) GetContentVersioned(_ context.Context, id string) (*contracts.Content, int, error) {
	s.storage.mu.RLock()
	defer s.storage.mu.RUnlock()

	rec, ok := s.storage.content[id]
	if !ok {
		return nil, 0, fmt.Errorf("content %s: %w", id, ErrNotFound)
	}
	return copyContent(rec.content), rec.version, nil
}

// --- Publish Task Store ---

// MemoryPublishTaskStore is a concurrency-safe in-memory PublishTaskStore.
type MemoryPublishTaskStore struct {
	storage *MemoryStorage
}

func (s *MemoryPublishTaskStore) CreateTask(_ context.Context, contentID, platform string) (*contracts.PublishTask, error) {
	s.storage.mu.Lock()
	defer s.storage.mu.Unlock()

	for _, t := range s.storage.tasks {
		if t.ContentID == contentID && t.Platform == platform {
			return nil, fmt.Errorf("task for %s/%s already exists: %w", contentID, platform, ErrConflict)
		}
	}

	now := nowUTC()
	task := &contracts.PublishTask{
		ID:          generateID(),
		ContentID:   contentID,
		Platform:    platform,
		Status:      contracts.PublishTaskStatusPending,
		RetryPolicy: contracts.DefaultRetryPolicy(),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	s.storage.tasks[task.ID] = task
	return copyTask(task), nil
}

func (s *MemoryPublishTaskStore) GetTask(_ context.Context, id string) (*contracts.PublishTask, error) {
	s.storage.mu.RLock()
	defer s.storage.mu.RUnlock()

	task, ok := s.storage.tasks[id]
	if !ok {
		return nil, fmt.Errorf("task %s: %w", id, ErrNotFound)
	}
	return copyTask(task), nil
}

func (s *MemoryPublishTaskStore) GetTasksByContent(_ context.Context, contentID string) ([]*contracts.PublishTask, error) {
	s.storage.mu.RLock()
	defer s.storage.mu.RUnlock()

	var result []*contracts.PublishTask
	for _, t := range s.storage.tasks {
		if t.ContentID == contentID {
			result = append(result, copyTask(t))
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.Before(result[j].CreatedAt)
	})
	return result, nil
}

func (s *MemoryPublishTaskStore) GetTaskByContentPlatform(_ context.Context, contentID, platform string) (*contracts.PublishTask, error) {
	s.storage.mu.RLock()
	defer s.storage.mu.RUnlock()

	for _, t := range s.storage.tasks {
		if t.ContentID == contentID && t.Platform == platform {
			return copyTask(t), nil
		}
	}
	return nil, fmt.Errorf("task for %s/%s: %w", contentID, platform, ErrNotFound)
}

func (s *MemoryPublishTaskStore) UpdateTaskStatus(_ context.Context, id string, status contracts.PublishTaskStatus, errMsg string) error {
	s.storage.mu.Lock()
	defer s.storage.mu.Unlock()

	task, ok := s.storage.tasks[id]
	if !ok {
		return fmt.Errorf("task %s: %w", id, ErrNotFound)
	}
	task.Status = status
	task.Error = errMsg
	task.UpdatedAt = nowUTC()
	return nil
}

func (s *MemoryPublishTaskStore) IncrementAttemptCount(_ context.Context, id string) error {
	s.storage.mu.Lock()
	defer s.storage.mu.Unlock()

	task, ok := s.storage.tasks[id]
	if !ok {
		return fmt.Errorf("task %s: %w", id, ErrNotFound)
	}
	task.AttemptCount++
	task.UpdatedAt = nowUTC()
	return nil
}

// --- Publish Attempt Store ---

// MemoryPublishAttemptStore is a concurrency-safe in-memory PublishAttemptStore.
type MemoryPublishAttemptStore struct {
	storage *MemoryStorage
}

func (s *MemoryPublishAttemptStore) CreateAttempt(_ context.Context, taskID string, attemptNumber int) (*contracts.PublishAttempt, error) {
	s.storage.mu.Lock()
	defer s.storage.mu.Unlock()

	attempt := &contracts.PublishAttempt{
		ID:            generateID(),
		TaskID:        taskID,
		AttemptNumber: attemptNumber,
		Status:        contracts.PublishTaskStatusProcessing,
		StartedAt:     nowUTC(),
	}
	s.storage.attempts[attempt.ID] = attempt
	return copyAttempt(attempt), nil
}

func (s *MemoryPublishAttemptStore) CompleteAttempt(_ context.Context, id string, status contracts.PublishTaskStatus, errMsg string) error {
	s.storage.mu.Lock()
	defer s.storage.mu.Unlock()

	attempt, ok := s.storage.attempts[id]
	if !ok {
		return fmt.Errorf("attempt %s: %w", id, ErrNotFound)
	}
	attempt.Status = status
	attempt.Error = errMsg
	now := nowUTC()
	attempt.CompletedAt = &now
	return nil
}

func (s *MemoryPublishAttemptStore) GetAttemptsByTask(_ context.Context, taskID string) ([]*contracts.PublishAttempt, error) {
	s.storage.mu.RLock()
	defer s.storage.mu.RUnlock()

	var result []*contracts.PublishAttempt
	for _, a := range s.storage.attempts {
		if a.TaskID == taskID {
			result = append(result, copyAttempt(a))
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].AttemptNumber < result[j].AttemptNumber
	})
	return result, nil
}

// --- Platform Post Store ---

// MemoryPlatformPostStore is a concurrency-safe in-memory PlatformPostStore.
type MemoryPlatformPostStore struct {
	storage *MemoryStorage
}

func (s *MemoryPlatformPostStore) CreatePlatformPost(_ context.Context, taskID, platform, platformPostID, platformURL string, response []byte) error {
	s.storage.mu.Lock()
	defer s.storage.mu.Unlock()

	rec := &PlatformPostRecord{
		ID:             generateID(),
		TaskID:         taskID,
		Platform:       platform,
		PlatformPostID: platformPostID,
		PlatformURL:    platformURL,
		Response:       response,
	}
	s.storage.posts[taskID] = rec
	return nil
}

func (s *MemoryPlatformPostStore) GetByTask(_ context.Context, taskID string) (*PlatformPostRecord, error) {
	s.storage.mu.RLock()
	defer s.storage.mu.RUnlock()

	rec, ok := s.storage.posts[taskID]
	if !ok {
		return nil, fmt.Errorf("platform post for task %s: %w", taskID, ErrNotFound)
	}
	out := *rec
	if rec.Response != nil {
		out.Response = make([]byte, len(rec.Response))
		copy(out.Response, rec.Response)
	}
	return &out, nil
}

// --- Asset Store ---

// MemoryAssetStore is a concurrency-safe in-memory AssetStore.
type MemoryAssetStore struct {
	storage *MemoryStorage
}

func (s *MemoryAssetStore) CreateAsset(_ context.Context, asset *Asset) error {
	s.storage.mu.Lock()
	defer s.storage.mu.Unlock()

	if asset.ID == "" {
		asset.ID = generateID()
	}
	if asset.CreatedAt.IsZero() {
		asset.CreatedAt = nowUTC()
	}
	stored := *asset
	s.storage.assets[asset.ID] = &stored
	return nil
}

func (s *MemoryAssetStore) GetAsset(_ context.Context, id string) (*Asset, error) {
	s.storage.mu.RLock()
	defer s.storage.mu.RUnlock()

	a, ok := s.storage.assets[id]
	if !ok {
		return nil, fmt.Errorf("asset %s: %w", id, ErrNotFound)
	}
	out := *a
	return &out, nil
}

func (s *MemoryAssetStore) GetAssetsByContent(_ context.Context, contentID string) ([]*Asset, error) {
	s.storage.mu.RLock()
	defer s.storage.mu.RUnlock()

	var result []*Asset
	for _, a := range s.storage.assets {
		if a.ContentID == contentID {
			out := *a
			result = append(result, &out)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.Before(result[j].CreatedAt)
	})
	return result, nil
}

// --- Idempotency Store ---

// MemoryIdempotencyStore is a concurrency-safe in-memory IdempotencyStore.
type MemoryIdempotencyStore struct {
	storage *MemoryStorage
}

func (s *MemoryIdempotencyStore) Claim(_ context.Context, key contracts.IdempotencyKey) (bool, error) {
	s.storage.mu.Lock()
	defer s.storage.mu.Unlock()

	composite := key.Key + ":" + string(key.Scope)

	// Check for existing non-expired key.
	if existing, ok := s.storage.idempotency[composite]; ok {
		if nowUTC().Before(existing.ExpiresAt) {
			return false, nil // already claimed
		}
		// Expired — allow re-claim by replacing.
	}

	s.storage.idempotency[composite] = &idempotencyRecord{
		EntityID:  key.EntityID,
		ExpiresAt: nowUTC().Add(key.TTL),
	}
	return true, nil
}

func (s *MemoryIdempotencyStore) Get(_ context.Context, key string, scope contracts.IdempotencyKeyScope) (string, error) {
	s.storage.mu.RLock()
	defer s.storage.mu.RUnlock()

	composite := key + ":" + string(scope)

	rec, ok := s.storage.idempotency[composite]
	if !ok {
		return "", fmt.Errorf("idempotency key %s: %w", key, ErrNotFound)
	}
	if nowUTC().After(rec.ExpiresAt) {
		return "", fmt.Errorf("idempotency key %s: %w", key, ErrNotFound)
	}
	return rec.EntityID, nil
}

func (s *MemoryIdempotencyStore) Cleanup(_ context.Context) (int, error) {
	s.storage.mu.Lock()
	defer s.storage.mu.Unlock()

	now := nowUTC()
	removed := 0
	for k, rec := range s.storage.idempotency {
		if now.After(rec.ExpiresAt) {
			delete(s.storage.idempotency, k)
			removed++
		}
	}
	return removed, nil
}

// --- Internal types ---

// contentRecord wraps content with its version for optimistic locking.
type contentRecord struct {
	content *contracts.Content
	version int
}

// idempotencyRecord tracks when an idempotency key expires.
type idempotencyRecord struct {
	EntityID  string
	ExpiresAt time.Time
}

// --- Copy helpers ---

func copyContent(c *contracts.Content) *contracts.Content {
	out := *c
	if c.Tags != nil {
		out.Tags = make([]string, len(c.Tags))
		copy(out.Tags, c.Tags)
	}
	return &out
}

func copyTask(t *contracts.PublishTask) *contracts.PublishTask {
	out := *t
	return &out
}

func copyAttempt(a *contracts.PublishAttempt) *contracts.PublishAttempt {
	out := *a
	if a.CompletedAt != nil {
		t := *a.CompletedAt
		out.CompletedAt = &t
	}
	return &out
}

// --- MemoryTx ---

// MemoryTx implements Tx for in-memory stores.
//
// The MVP in-memory transaction executes directly under the shared lock,
// providing serializable isolation. A production implementation would use
// a write-ahead buffer with commit/rollback semantics.
type MemoryTx struct {
	storage *MemoryStorage
}

// Run executes fn within a transactional scope.
//
// The MVP in-memory implementation delegates directly to the stores.
// Each store method handles its own locking. A production implementation
// would use database-level transactions for true atomicity.
func (tx *MemoryTx) Run(ctx context.Context, fn func(scope *TxScope) error) error {
	scope := &TxScope{
		ContentStore:        tx.storage.Content,
		PublishTaskStore:    tx.storage.Tasks,
		PublishAttemptStore: tx.storage.Attempts,
		PlatformPostStore:   tx.storage.Posts,
		AssetStore:          tx.storage.Assets,
	}

	return fn(scope)
}
