# Queue Workflow

## Goal

Define the job queue system for asynchronous publish processing.

This document is the source of truth for:

- queue architecture
- job lifecycle
- worker processing
- failure handling

---

## Queue Architecture

```
API Server
    ↓
Redis Queue
    ↓
Worker Pool
    ↓
Platform Publishers
```

Components:

- **Redis**: message broker
- **API Server**: job producer
- **Worker Pool**: job consumers
- **Platform Publishers**: job executors

---

## Job Model

```go
type PublishJob struct {
    ID          string    `json:"id"`
    ContentID   string    `json:"content_id"`
    Platform    string    `json:"platform"`
    Status      string    `json:"status"`
    RetryCount  int       `json:"retry_count"`
    MaxRetries  int       `json:"max_retries"`
    Error       string    `json:"error,omitempty"`
    CreatedAt   time.Time `json:"created_at"`
    UpdatedAt   time.Time `json:"updated_at"`
}
```

---

## Job States

```
queued
    ↓
processing
    ↓
success
    OR
failed → retrying → processing
    OR
failed → dead
```

### State Definitions

- **queued**: job is in Redis queue
- **processing**: worker is executing job
- **success**: job completed successfully
- **failed**: job failed permanently
- **retrying**: job will be retried
- **dead**: job exceeded max retries

---

## Queue Operations

### Enqueue Job

```go
func EnqueueJob(ctx context.Context, job *PublishJob) error {
    data, _ := json.Marshal(job)
    return redis.LPush(ctx, "publish:queue", data).Err()
}
```

### Dequeue Job

```go
func DequeueJob(ctx context.Context) (*PublishJob, error) {
    result, err := redis.BRPop(ctx, 0, "publish:queue").Result()
    if err != nil {
        return nil, err
    }
    var job PublishJob
    json.Unmarshal([]byte(result[1]), &job)
    return &job, nil
}
```

### Queue Name

- main queue: `publish:queue`
- dead letter queue: `publish:dead`

---

## Worker Pool

### Worker Lifecycle

```
Start
    ↓
Dequeue Job
    ↓
Process Job
    ↓
Update Status
    ↓
Loop
```

### Worker Configuration

```go
type WorkerConfig struct {
    Concurrency    int
    MaxRetries     int
    RetryDelay     time.Duration
    ShutdownTimeout time.Duration
}
```

### Concurrency

- default: 5 workers
- configurable via environment
- each worker runs in separate goroutine

---

## Job Processing

### Processing Flow

```
Dequeue
    ↓
Load Content from DB
    ↓
Get Publisher from Registry
    ↓
Publisher.Authenticate()
    ↓
Publisher.Validate()
    ↓
Publisher.Publish()
    ↓
Store Result
    ↓
Update Job Status
```

### Processing Timeout

- default: 30 seconds per job
- configurable per platform
- timeout = permanent failure

---

## Retry Logic

### Retryable Failures

- network timeout
- connection refused
- HTTP 429 (rate limit)
- HTTP 5xx (server error)

### Permanent Failures

- HTTP 4xx (client error)
- validation failure
- authentication failure

### Retry Implementation

```go
func (w *Worker) processWithRetry(job *PublishJob) error {
    for i := 0; i <= job.MaxRetries; i++ {
        err := w.process(job)
        if err == nil {
            return nil
        }
        if !isRetryable(err) {
            return err
        }
        job.RetryCount = i + 1
        delay := calculateBackoff(i)
        time.Sleep(delay)
    }
    return ErrMaxRetriesExceeded
}
```

### Backoff Strategy

```
delay = baseDelay * 2^retryCount
```

- base delay: 1 second
- max delay: 5 minutes
- jitter: +/- 10%

---

## Dead Letter Queue

### When Job Moves to DLQ

- max retries exceeded
- permanent failure after retry

### DLQ Operations

- inspect failed jobs
- manually retry jobs
- archive old jobs

### DLQ Retention

- default: 7 days
- configurable

---

## Idempotency

### Duplicate Prevention

Each job has unique ID.

Before processing:

```
Check if job_id exists in processed set
    ↓
If exists: skip
If not exists: process and add to set
```

### Processed Set

- Redis set: `publish:processed`
- TTL: 24 hours
- auto-cleanup

---

## Monitoring

### Metrics to Track

- queue depth
- processing time
- success rate
- failure rate
- retry rate

### Logging

Each job logs:

- job_id
- content_id
- platform
- status transitions
- error details
- processing duration

---

## Graceful Shutdown

### Shutdown Flow

```
Receive SIGTERM
    ↓
Stop accepting new jobs
    ↓
Wait for current jobs to complete
    ↓
Timeout: force stop
    ↓
Exit
```

### Shutdown Timeout

- default: 30 seconds
- configurable

---

## Non-Goals For MVP

Not included:

- priority queues
- delayed jobs
- job scheduling
- queue sharding
- distributed workers
