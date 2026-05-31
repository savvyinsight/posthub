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
    MaxRequests int
    Window      time.Duration
}
```

---

## Platform Rate Limits

| Platform | Publish Limit | API Limit | Window |
|----------|--------------|-----------|--------|
| Zhihu | 10/hour | 100/hour | 1 hour |
| Bilibili | 5/hour | 30/hour | 1 hour |
| Weibo | 30/hour | 200/hour | 1 hour |

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

## Implementation: Token Bucket

Using Redis-based token bucket.

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

    // Lua script for atomic token bucket
    script := redis.NewScript(`
        local key = KEYS[1]
        local max = tonumber(ARGV[1])
        local window = tonumber(ARGV[2])
        local now = tonumber(ARGV[3])

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
        time.Now().Unix(),
    }).Int()

    if err != nil {
        return false, err
    }

    return result == 1, nil
}
```

---

## Rate Limit Keys

### Per Platform

```
rate:platform:{platform_name}
```

Example: `rate:platform:zhihu`

### Per Account

```
rate:account:{account_id}
```

Example: `rate:account:user123`

### Per Platform-Account

```
rate:{platform_name}:{account_id}
```

Example: `rate:zhihu:user123`

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
func RateLimitMiddleware(limiter RateLimiter) asynq.MiddlewareFunc {
    return func(ctx context.Context, t *asynq.Task, h asynq.Handler) error {
        key := extractRateLimitKey(t)

        allowed, err := limiter.Allow(ctx, key)
        if err != nil {
            return err
        }

        if !allowed {
            // Re-enqueue with delay
            return asynq.SkipRetry
        }

        return h.ProcessTask(ctx, t)
    }
}
```

### Task Options

```go
asynq.NewTask(TypePublishContent, payload,
    asynq.MaxRetry(3),
    asynq.RateLimit(10, time.Hour),  // 10 per hour
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

## Non-Goals For MVP

Not included:

- adaptive rate limiting
- cross-instance coordination
- priority-based rate limiting
- rate limit prediction
- dynamic rate adjustment
