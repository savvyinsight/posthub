# Error Handling

## Goal

Define error handling strategy for the content distribution system.

This document is the source of truth for:

- error classification
- error propagation
- error logging
- error recovery

---

## Error Categories

### Validation Errors

Cause: invalid input from client.

Examples:

- missing required field
- field too long
- invalid format

HTTP Status: 400

Action: return error to client, do not process.

---

### Not Found Errors

Cause: resource does not exist.

Examples:

- content ID not found
- job ID not found

HTTP Status: 404

Action: return error to client.

---

### Business Logic Errors

Cause: operation not allowed in current state.

Examples:

- update non-draft content
- publish non-ready content
- delete published content

HTTP Status: 403

Action: return error to client.

---

### Platform Errors

Cause: external platform API failure.

Sub-categories:

#### Retryable Platform Errors

- network timeout
- connection refused
- HTTP 429 (rate limit)
- HTTP 5xx (server error)

Action: retry with backoff.

#### Permanent Platform Errors

- HTTP 400 (bad request)
- HTTP 401 (unauthorized)
- HTTP 403 (forbidden)
- HTTP 404 (not found)

Action: mark job as failed, do not retry.

---

### Internal Errors

Cause: system failure.

Examples:

- database connection failure
- Redis connection failure
- unexpected panic

HTTP Status: 500

Action: log error, return generic error to client.

---

## Error Types

### Application Error

```go
type AppError struct {
    Code    string `json:"error"`
    Message string `json:"message"`
    Details []ErrorDetail `json:"details,omitempty"`
}

type ErrorDetail struct {
    Field   string `json:"field"`
    Message string `json:"message"`
}
```

### Platform Error

```go
type PlatformError struct {
    Platform    string
    StatusCode  int
    Message     string
    Retryable   bool
}
```

---

## Error Handling Flow

### API Layer

```
Request → Handler → Service → Repository
    ↓
Error occurs
    ↓
Wrap in AppError
    ↓
Return to client
```

### Worker Layer

```
Job → Worker → Publisher → Platform API
    ↓
Error occurs
    ↓
Classify error
    ↓
If retryable: retry
If permanent: fail job
```

---

## Error Propagation

### Do

- wrap errors with context
- preserve error chain
- log at each layer

### Don't

- swallow errors
- return generic errors
- expose internal details

---

## Error Wrapping

```go
// Good
if err != nil {
    return fmt.Errorf("failed to publish to %s: %w", platform, err)
}

// Bad
if err != nil {
    return err
}
```

---

## Error Logging

### What to Log

- error message
- error type
- stack trace (for internal errors)
- request context
- job context

### What Not to Log

- client passwords
- API tokens
- full request bodies (may contain sensitive data)

### Log Format

```json
{
  "level": "error",
  "msg": "failed to publish content",
  "error": "connection timeout",
  "job_id": "job_xxx",
  "content_id": "content_xxx",
  "platform": "zhihu",
  "retry_count": 1
}
```

---

## Error Recovery

### Database Errors

- connection pool exhaustion: wait and retry
- query timeout: retry once
- constraint violation: return validation error

### Redis Errors

- connection failure: retry with backoff
- queue empty: block and wait

### Platform Errors

- network timeout: retry
- rate limit: wait and retry
- auth failure: refresh token and retry
- permanent failure: fail job

---

## Circuit Breaker

For platform publishers:

### States

- closed: normal operation
- open: failing, reject requests
- half-open: testing recovery

### Configuration

```go
type CircuitBreakerConfig struct {
    Threshold    int           // failures before open
    Timeout      time.Duration // time before half-open
    MaxRequests  int           // requests in half-open
}
```

### Behavior

- 5 consecutive failures: open circuit
- 30 seconds timeout: half-open
- 1 success: close circuit

---

## Panic Recovery

### API Handlers

```go
func recoverMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        defer func() {
            if err := recover(); err != nil {
                log.Error("panic recovered", "error", err)
                http.Error(w, "internal error", 500)
            }
        }()
        next.ServeHTTP(w, r)
    })
}
```

### Workers

```go
func (w *Worker) process(job *PublishJob) (err error) {
    defer func() {
        if r := recover(); r != nil {
            err = fmt.Errorf("panic: %v", r)
        }
    }()
    // process job
}
```

---

## Retry Strategy

### Exponential Backoff

```
delay = base * 2^attempt + jitter
```

### Configuration

- base: 1 second
- max: 5 minutes
- jitter: random 0-1 second

### Example

```
attempt 0: 1s
attempt 1: 2s
attempt 2: 4s
attempt 3: fail
```

---

## Dead Letter Queue

### Purpose

Store jobs that cannot be processed.

### When to Use

- max retries exceeded
- permanent failure after retry

### Operations

- inspect failed jobs
- manually retry
- archive old jobs

---

## Error Response Examples

### Validation Error

```json
{
  "error": "validation_error",
  "message": "invalid request",
  "details": [
    {
      "field": "title",
      "message": "title is required"
    },
    {
      "field": "body",
      "message": "body must be at least 1 character"
    }
  ]
}
```

### Not Found Error

```json
{
  "error": "not_found",
  "message": "content not found"
}
```

### Internal Error

```json
{
  "error": "internal_error",
  "message": "something went wrong"
}
```

---

## Non-Goals For MVP

Not included:

- error analytics
- error alerting
- error dashboards
- automated recovery
- error aggregation
