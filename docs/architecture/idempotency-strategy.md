# Idempotency Strategy

## Goal

Define idempotency for distributed async publishing.

This document is the source of truth for:

- duplicate publish prevention
- retry idempotency
- worker crash recovery
- idempotency keys

---

## Why Idempotency Matters

Without idempotency:

```
worker publishes successfully
    ↓
DB update fails
    ↓
retry publishes duplicate article
```

This WILL happen eventually.

Network failures, worker crashes, DB timeouts all cause this.

---

## Core Principle

Every publish operation must be idempotent.

Executing the same publish multiple times produces the same result as executing it once.

---

## Idempotency Layers

```
┌─────────────────┐
│  Task Creation  │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  Task Execution │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  Platform Call  │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  Result Storage │
└─────────────────┘
```

Each layer needs idempotency.

---

## Task Creation Idempotency

### Problem

User clicks publish twice rapidly.

Two requests create duplicate tasks.

### Solution

Use content_id + platform as idempotency key.

```sql
CONSTRAINT uq_publish_task_content_platform UNIQUE (content_id, platform)
```

### Implementation

```go
func (s *PublishService) CreateTask(ctx context.Context, contentID, platform string) (*PublishTask, error) {
    // Try to create task
    task, err := s.store.CreatePublishTask(ctx, contentID, platform)
    if err != nil {
        // Check if unique constraint violation
        if isUniqueViolation(err) {
            // Task already exists, return existing
            return s.store.GetTaskByContentPlatform(ctx, contentID, platform)
        }
        return nil, err
    }
    return task, nil
}
```

---

## Task Execution Idempotency

### Problem

Worker crashes after publishing but before DB update.

Retry publishes duplicate.

### Solution

Check if already published before publishing.

```go
func (h *PublishHandler) ProcessTask(ctx context.Context, t *asynq.Task) error {
    payload := extractPayload(t)

    // Check if already succeeded
    existing, err := h.store.GetTask(ctx, payload.TaskID)
    if err != nil {
        return err
    }

    if existing.Status == "succeeded" {
        // Already published, skip
        return nil
    }

    // Proceed with publish
    return h.doPublish(ctx, t)
}
```

---

## Platform Call Idempotency

### Problem

Platform API call succeeds but response parsing fails.

Retry calls API again, creates duplicate post.

### Solution

Use platform-specific idempotency mechanisms.

### Zhihu

Zhihu does not provide idempotency keys.

Solution: store platform post ID after first success.

```go
func (a *ZhihuAdapter) Publish(ctx context.Context, doc *Document, creds *Credentials) (*PublishResult, error) {
    // Check if already published
    existing, err := a.getExistingPost(ctx, doc.ID)
    if err == nil && existing != nil {
        // Already published, return existing
        return existing, nil
    }

    // Publish new
    result, err := a.createPost(ctx, doc, creds)
    if err != nil {
        return nil, err
    }

    // Store mapping
    a.storePostMapping(ctx, doc.ID, result.PlatformPostID)

    return result, nil
}
```

### Bilibili

Bilibili does not provide idempotency keys.

Same solution as Zhihu.

---

## Result Storage Idempotency

### Problem

Multiple attempts may try to store result.

### Solution

Use attempt ID as idempotency key.

```go
func (s *Store) StoreResult(ctx context.Context, attemptID string, result *PublishResult) error {
    // Check if result already stored
    existing, err := s.GetResultByAttempt(ctx, attemptID)
    if err == nil && existing != nil {
        // Already stored, skip
        return nil
    }

    // Store new result
    return s.createResult(ctx, attemptID, result)
}
```

---

## Idempotency Keys

### Task Level

```
{content_id}:{platform}
```

Example: `content_123:zhihu`

### Attempt Level

```
{task_id}:{attempt_number}
```

Example: `task_456:1`

### Platform Level

```
{platform}:{content_id}
```

Example: `zhihu:content_123`

---

## Database Schema

### idempotency_keys table

```sql
CREATE TABLE idempotency_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    idempotency_key VARCHAR(200) NOT NULL UNIQUE,
    entity_type VARCHAR(50) NOT NULL,
    entity_id UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_idempotency_key UNIQUE (idempotency_key)
);

CREATE INDEX idx_idempotency_keys_key ON idempotency_keys(idempotency_key);
```

### Cleanup

Old keys cleaned up after 24 hours.

---

## Worker Crash Recovery

### Scenario

```
Worker picks up task
    ↓
Worker publishes to platform
    ↓
Worker crashes before DB update
    ↓
Task timeout expires
    ↓
Asynq retries task
    ↓
New worker picks up task
```

### Recovery Flow

1. new worker loads task
2. checks if platform post exists
3. if exists: update DB with existing result
4. if not exists: publish new

### Implementation

```go
func (h *PublishHandler) recoverFromCrash(ctx context.Context, task *PublishTask) error {
    // Check platform for existing post
    existing, err := h.platform.GetPost(ctx, task.ContentID)
    if err != nil {
        // No existing post, safe to publish
        return h.doPublish(ctx, task)
    }

    // Post exists, update DB
    return h.store.StoreResult(ctx, task.ID, &PublishResult{
        PlatformPostID: existing.ID,
        PlatformURL:    existing.URL,
        PublishedAt:    existing.PublishedAt,
    })
}
```

---

## Testing Idempotency

### Test Cases

1. duplicate task creation returns existing task
2. duplicate publish returns existing result
3. crash recovery detects existing post
4. retry after success skips publish

### Example Test

```go
func TestIdempotentPublish(t *testing.T) {
    // Create task
    task1, err := service.CreateTask(ctx, contentID, "zhihu")
    assert.NoError(t, err)

    // Create duplicate task
    task2, err := service.CreateTask(ctx, contentID, "zhihu")
    assert.NoError(t, err)

    // Should be same task
    assert.Equal(t, task1.ID, task2.ID)
}
```

---

## Monitoring

### Metrics

- idempotency key hits
- idempotency key misses
- duplicate task attempts
- crash recoveries

### Logging

```json
{
  "level": "info",
  "msg": "idempotency key hit",
  "key": "content_123:zhihu",
  "entity_type": "publish_task"
}
```

---

## Non-Goals For MVP

Not included:

- distributed idempotency (cross-instance)
- idempotency key expiration policies
- idempotency key analytics
- cross-platform idempotency
