// Idempotency guard for preventing duplicate task processing.
//
// Uses Redis SETNX for distributed deduplication, with an in-memory
// implementation for tests. Keys are scoped per content+platform
// and auto-expire after a configurable TTL.
package queue

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// IdempotencyGuard prevents duplicate task processing.
//
// Acquire returns true if the key was freshly acquired (not already held).
// Release removes the key, allowing re-processing if needed.
type IdempotencyGuard interface {
	// Acquire attempts to claim the key. Returns true if acquired,
	// false if another holder already has it. Error on infrastructure failure.
	Acquire(ctx context.Context, key string, ttl time.Duration) (acquired bool, err error)
	// Release removes the key so it can be re-acquired.
	Release(ctx context.Context, key string) error
}

// RedisIdempotencyGuard implements IdempotencyGuard using Redis SETNX.
type RedisIdempotencyGuard struct {
	client *redis.Client
	prefix string
}

// NewRedisIdempotencyGuard creates a guard backed by the given Redis client.
func NewRedisIdempotencyGuard(client *redis.Client) *RedisIdempotencyGuard {
	return &RedisIdempotencyGuard{
		client: client,
		prefix: "idempotency:",
	}
}

// Acquire uses SETNX to atomically claim a key with a TTL.
func (g *RedisIdempotencyGuard) Acquire(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	fullKey := g.prefix + key
	ok, err := g.client.SetNX(ctx, fullKey, "1", ttl).Result()
	if err != nil {
		return false, fmt.Errorf("idempotency acquire %s: %w", key, err)
	}
	return ok, nil
}

// Release deletes the key so it can be re-acquired.
func (g *RedisIdempotencyGuard) Release(ctx context.Context, key string) error {
	fullKey := g.prefix + key
	if err := g.client.Del(ctx, fullKey).Err(); err != nil {
		return fmt.Errorf("idempotency release %s: %w", key, err)
	}
	return nil
}

// InMemoryIdempotencyGuard is a non-distributed guard for testing.
//
// Not safe for production use — single-process only, no persistence.
type InMemoryIdempotencyGuard struct {
	mu      sync.Mutex
	entries map[string]time.Time
}

// NewInMemoryIdempotencyGuard creates a guard backed by an in-memory map.
func NewInMemoryIdempotencyGuard() *InMemoryIdempotencyGuard {
	return &InMemoryIdempotencyGuard{
		entries: make(map[string]time.Time),
	}
}

// Acquire checks if the key exists and is not expired. If not, it claims it.
func (g *InMemoryIdempotencyGuard) Acquire(_ context.Context, key string, ttl time.Duration) (bool, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if expiry, ok := g.entries[key]; ok && time.Now().Before(expiry) {
		return false, nil
	}

	g.entries[key] = time.Now().Add(ttl)
	return true, nil
}

// Release removes the key from the map.
func (g *InMemoryIdempotencyGuard) Release(_ context.Context, key string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	delete(g.entries, key)
	return nil
}

// Size returns the number of entries currently held (for testing).
func (g *InMemoryIdempotencyGuard) Size() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return len(g.entries)
}
