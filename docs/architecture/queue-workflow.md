# Queue Workflow

## Goal

Define the task queue system using Asynq.

This document is the source of truth for:

- Asynq integration
- task lifecycle
- worker processing
- failure handling

---

## Why Asynq

Do not build custom queue infrastructure.

Asynq provides:

- Go-native
- Redis-based
- built-in retries
- built-in dead letter queue
- scheduling support
- production-tested

---

## Architecture

```
API Server
    ↓
Asynq Client
    ↓
Redis
    ↓
Asynq Worker
    ↓
Task Handler
    ↓
Platform Publisher
```

---

## Task Types

### Publish Task

```go
const TypePublishContent = "publish:content"

type PublishTaskPayload struct {
    TaskID    string `json:"task_id"`
    ContentID string `json:"content_id"`
    Platform  string `json:"platform"`
}
```

---

## Task Lifecycle

```
Enqueue
    ↓
Pending (in Redis)
    ↓
Processing (worker picked up)
    ↓
Success / Retry / Dead
```

### States

- pending: task in queue
- processing: worker executing
- succeeded: task completed
- retrying: will retry after delay
- dead: max retries exceeded

---

## Enqueue Task

```go
func (q *Queue) EnqueuePublishTask(ctx context.Context, task *PublishTask) error {
    payload, err := json.Marshal(PublishTaskPayload{
        TaskID:    task.ID,
        ContentID: task.ContentID,
        Platform:  task.Platform,
    })
    if err != nil {
        return err
    }

    _, err = q.client.EnqueueContext(ctx,
        asynq.NewTask(TypePublishContent, payload),
        asynq.MaxRetry(task.MaxRetries),
        asynq.Queue("publish"),
    )
    return err
}
```

---

## Task Handler

```go
type PublishHandler struct {
    store     *storage.Queries
    registry  *platform.Registry
    transform *transform.Pipeline
}

func (h *PublishHandler) ProcessTask(ctx context.Context, t *asynq.Task) error {
    var payload PublishTaskPayload
    if err := json.Unmarshal(t.Payload(), &payload); err != nil {
        return fmt.Errorf("unmarshal payload: %w", err)
    }

    // Load content
    content, err := h.store.GetContent(ctx, payload.ContentID)
    if err != nil {
        return fmt.Errorf("get content: %w", err)
    }

    // Get publisher
    publisher, err := h.registry.Get(payload.Platform)
    if err != nil {
        return fmt.Errorf("get publisher: %w", err)
    }

    // Parse and transform
    doc, err := h.transform.Parse(content.Body)
    if err != nil {
        return fmt.Errorf("parse content: %w", err)
    }

    payload_doc, err := publisher.Renderer().Render(doc)
    if err != nil {
        return fmt.Errorf("render content: %w", err)
    }

    // Upload assets if needed
    if err := h.processAssets(ctx, payload_doc, publisher); err != nil {
        return fmt.Errorf("process assets: %w", err)
    }

    // Publish
    result, err := publisher.Publish(ctx, payload_doc)
    if err != nil {
        return fmt.Errorf("publish: %w", err)
    }

    // Store result
    if err := h.storeResult(ctx, payload.TaskID, result); err != nil {
        return fmt.Errorf("store result: %w", err)
    }

    return nil
}
```

---

## Worker Setup

```go
func NewWorker(cfg *config.Config, handler *PublishHandler) *asynq.Server {
    srv := asynq.NewServer(
        asynq.RedisClientOpt{Addr: cfg.RedisURL},
        asynq.Config{
            Concurrency: cfg.WorkerConcurrency,
            Queues: map[string]int{
                "publish": 6,
                "default": 4,
            },
        },
    )
    return srv
}

func main() {
    mux := asynq.NewServeMux()
    mux.HandleFunc(TypePublishContent, handler.ProcessTask)

    if err := srv.Run(mux); err != nil {
        log.Fatal(err)
    }
}
```

---

## Retry Behavior

### Asynq Built-in Retries

Asynq handles:

- automatic retry on error
- exponential backoff
- max retry limit
- dead letter queue

### Configuration

```go
asynq.NewTask(TypePublishContent, payload,
    asynq.MaxRetry(3),
    asynq.Timeout(30*time.Second),
    asynq.Deadline(time.Now().Add(5*time.Minute)),
)
```

### Error Classification

```go
func (h *PublishHandler) ProcessTask(ctx context.Context, t *asynq.Task) error {
    err := h.doPublish(ctx, t)
    if err != nil {
        // Permanent error: don't retry
        if isPermanentError(err) {
            return fmt.Errorf("permanent: %w", asynq.SkipRetry)
        }
        // Retryable error: Asynq will retry
        return err
    }
    return nil
}
```

---

## Dead Letter Queue

Asynq automatically moves failed tasks to DLQ after max retries.

### Inspect Dead Tasks

```go
func (q *Queue) ListDeadTasks(ctx context) ([]*asynq.TaskInfo, error) {
    inspector := asynq.NewInspector(asynq.RedisClientOpt{Addr: q.redisURL})
    return inspector.ListDeadTasks("publish")
}
```

### Retry Dead Task

```go
func (q *Queue) RetryDeadTask(ctx context, taskID string) error {
    inspector := asynq.NewInspector(asynq.RedisClientOpt{Addr: q.redisURL})
    return inspector.RunTask("publish", taskID)
}
```

---

## Queue Configuration

### Queues

| Queue | Priority | Description |
|-------|----------|-------------|
| publish | high | publish tasks |
| default | low | other tasks |

### Concurrency

- total: 10 workers (configurable)
- publish queue: 6 workers
- default queue: 4 workers

---

## Monitoring

### Asynq Built-in

Asynq provides:

- web UI for monitoring
- CLI for inspection
- metrics export

### Metrics to Track

- queue depth
- processing time
- success rate
- failure rate
- dead task count

### Logging

Each task logs:

- task_id
- content_id
- platform
- status transitions
- error details
- processing duration

---

## Graceful Shutdown

Asynq handles graceful shutdown:

```go
srv := asynq.NewServer(...)

// Asynq waits for active tasks to complete
srv.Shutdown()
```

---

## Configuration

```env
REDIS_URL=redis://localhost:6379
WORKER_CONCURRENCY=10
```

---

## Non-Goals For MVP

Not included:

- priority queues (beyond basic)
- delayed tasks
- task scheduling
- distributed workers across machines
- task chaining
