# Rate Limit Strategy

## Goal

Define rate limiting for multi-platform async publishing.

This document is the source of truth for:

- per-platform limits
- per-account limits
- retry windows
- backpressure strategy

---

## Why Rate Limiting Matters

Without rate limiting:

- platform APIs reject requests
- accounts get banned
- publish tasks fail unnecessarily
- system becomes unreliable

---

## Rate Limit Layers

```
┌─────────────────┐
│  Task Enqueue   │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  Rate Limiter   │
│  (per platform) │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  Rate Limiter   │
│  (per account)  │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  Platform API   │
└─────────────────┘
```

---

## Rate Limit Model

```go
type RateLimit struct {
    Platform    string
    AccountID   string
    Operation   string
    MaxRequests int
    Window      time.Duration
}
```

Operations:

- publish: creating posts
- upload: uploading assets
- delete: deleting posts
- read: reading data

---

## Platform Rate Limits

| Platform | Operation | Limit | Window |
|----------|-----------|-------|--------|
| Zhihu | publish | 10/hour | 1 hour |
| Zhihu | upload | 50/hour | 1 hour |
| Zhihu | read | 100/hour | 1 hour |
| Bilibili | publish | 5/hour | 1 hour |
| Bilibili | upload | 20/hour | 1 hour |
| Bilibili | read | 30/hour | 1 hour |
| Weibo | publish | 30/hour | 1 hour |
| Weibo | upload | 100/hour | 1 hour |
| Weibo | read | 200/hour | 1 hour |

---

## Rate Limiter Interface

```go
type RateLimiter interface {
    // Allow checks if a request is allowed
    Allow(ctx context.Context, key string) (bool, error)

    // Reserve reserves a slot for future request
    Reserve(ctx context.Context, key string) (*Reservation, error)

    // Reset resets the rate limiter for a key
    Reset(ctx context.Context, key string) error
}

type Reservation struct {
    Allowed    bool
    Delay      time.Duration
    Limit      int
    Remaining  int
    ResetAt    time.Time
}
```

---

## Implementation: Fixed Window Counter

Using Redis-based fixed window counter.

Note: this is NOT token bucket. It is fixed window counter.

Difference:

- token bucket: smooth refill, allows bursts
- fixed window: resets at window boundary, simpler

For MVP: fixed window is sufficient.

```go
type RedisRateLimiter struct {
    client    *redis.Client
    limits    map[string]*RateLimit
}

func (l *RedisRateLimiter) Allow(ctx context.Context, key string) (bool, error) {
    limit, ok := l.limits[key]
    if !ok {
        return true, nil
    }

    // Lua script for atomic fixed window counter
    script := redis.NewScript(`
        local key = KEYS[1]
        local max = tonumber(ARGV[1])
        local window = tonumber(ARGV[2])

        local current = redis.call('GET', key)
        if current == false then
            redis.call('SETEX', key, window, 1)
            return 1
        end

        if tonumber(current) < max then
            redis.call('INCR', key)
            return 1
        end

        return 0
    `)

    result, err := script.Run(ctx, l.client, []string{
        fmt.Sprintf("rate:%s", key),
        limit.MaxRequests,
        int(limit.Window.Seconds()),
    }).Int()

    if err != nil {
        return false, err
    }

    return result == 1, nil
}
```

---

## Rate Limit Keys

### Key Format

```
rate:{platform}:{account}:{operation}
```

Examples:

- `rate:zhihu:user123:publish`
- `rate:zhihu:user123:upload`
- `rate:bilibili:user456:publish`

### Why Include Operation

Upload limits and publish limits may differ.

Example:

- publish: 10/hour
- upload: 50/hour

Separate keys prevent interference.

### Key Hierarchy

For flexible limiting:

```
rate:{platform}:{account}:{operation}  # most specific
rate:{platform}:{account}              # per account
rate:{platform}                        # per platform
```

---

## Backpressure Strategy

### When Rate Limited

```
Task dequeued
    ↓
Check rate limit
    ↓
If allowed: proceed
If not allowed: re-enqueue with delay
```

### Delay Calculation

```go
func calculateDelay(reservation *Reservation) time.Duration {
    if reservation.Delay > 0 {
        return reservation.Delay
    }
    // Default backoff
    return 30 * time.Second
}
```

---

## Integration with Asynq

### Rate Limit Middleware

```go
func RateLimitMiddleware(limiter RateLimiter, client *asynq.Client) asynq.MiddlewareFunc {
    return func(ctx context.Context, t *asynq.Task, h asynq.Handler) error {
        key := extractRateLimitKey(t)

        reservation, err := limiter.Reserve(ctx, key)
        if err != nil {
            return err
        }

        if !reservation.Allowed {
            // Re-enqueue with delay, preserve retry count
            delay := calculateDelay(reservation)
            _, err := client.EnqueueContext(ctx, t,
                asynq.ProcessIn(delay),
                asynq.Retention(24*time.Hour),
            )
            if err != nil {
                return fmt.Errorf("re-enqueue rate limited task: %w", err)
            }
            // Return nil to indicate task was handled (re-enqueued)
            return nil
        }

        return h.ProcessTask(ctx, t)
    }
}
```

Important: do NOT use asynq.SkipRetry for rate limiting.

SkipRetry removes task from retry lifecycle.

Rate limiting is NOT task failure.

Correct behavior: re-enqueue with delay, preserve retry count.

### Task Options

```go
asynq.NewTask(TypePublishContent, payload,
    asynq.MaxRetry(3),
    asynq.Retention(24*time.Hour),
)
```

---

## Rate Limit Response Handling

### Platform Returns 429

```go
func handleRateLimitError(err error) error {
    if isRateLimitError(err) {
        // Extract retry-after header
        retryAfter := extractRetryAfter(err)
        return &RateLimitError{
            RetryAfter: retryAfter,
        }
    }
    return err
}
```

### Rate Limit Error

```go
type RateLimitError struct {
    RetryAfter time.Duration
}

func (e *RateLimitError) Error() string {
    return fmt.Sprintf("rate limited, retry after %v", e.RetryAfter)
}
```

---

## Monitoring

### Metrics

- requests allowed
- requests denied
- current usage
- reset time

### Logging

```json
{
  "level": "warn",
  "msg": "rate limited",
  "platform": "zhihu",
  "key": "rate:platform:zhihu",
  "limit": 10,
  "window": "1h"
}
```

---

## Configuration

```yaml
rate_limits:
  zhihu:
    publish:
      max: 10
      window: 1h
    api:
      max: 100
      window: 1h
  bilibili:
    publish:
      max: 5
      window: 1h
    api:
      max: 30
      window: 1h
  weibo:
    publish:
      max: 30
      window: 1h
    api:
      max: 200
      window: 1h
```

---

## Graceful Degradation

When rate limit exceeded:

1. log the event
2. re-enqueue task with delay
3. do not fail task
4. update metrics

---

## Concurrency Control

Rate limiting alone is insufficient.

Problem:

```
100 workers simultaneously
all pass rate check
API explodes
```

Solution: concurrency limits per platform.

### Configuration

```yaml
platform_workers:
  zhihu: 2
  bilibili: 1
  weibo: 3
```

### Implementation

Use semaphore per platform:

```go
type PlatformConcurrencyLimiter struct {
    semaphores map[string]chan struct{}
}

func NewPlatformConcurrencyLimiter(limits map[string]int) *PlatformConcurrencyLimiter {
    semaphores := make(map[string]chan struct{})
    for platform, limit := range limits {
        semaphores[platform] = make(chan struct{}, limit)
    }
    return &PlatformConcurrencyLimiter{semaphores: semaphores}
}

func (l *PlatformConcurrencyLimiter) Acquire(ctx context.Context, platform string) error {
    sem, ok := l.semaphores[platform]
    if !ok {
        return nil
    }

    select {
    case sem <- struct{}{}:
        return nil
    case <-ctx.Done():
        return ctx.Err()
    }
}

func (l *PlatformConcurrencyLimiter) Release(platform string) {
    sem, ok := l.semaphores[platform]
    if !ok {
        return
    }
    <-sem
}
```

### Integration

```go
func (h *PublishHandler) ProcessTask(ctx context.Context, t *asynq.Task) error {
    payload := extractPayload(t)

    // Acquire concurrency slot
    if err := h.concurrency.Acquire(ctx, payload.Platform); err != nil {
        return err
    }
    defer h.concurrency.Release(payload.Platform)

    // Check rate limit
    allowed, err := h.rateLimiter.Allow(ctx, extractRateLimitKey(t))
    if err != nil {
        return err
    }
    if !allowed {
        return requeueWithDelay(ctx, h.client, t)
    }

    // Process task
    return h.doPublish(ctx, t)
}
```

Both rate limits AND concurrency limits are needed.

---

## Non-Goals For MVP

Not included:

- adaptive rate limiting
- cross-instance coordination
- priority-based rate limiting
- rate limit prediction
- dynamic rate adjustment
