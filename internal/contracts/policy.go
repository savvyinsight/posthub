// Retry policy and idempotency key structures for async publish operations.
//
// RetryPolicy defines backoff strategy for failed publish attempts.
// IdempotencyKey prevents duplicate operations across the publish pipeline.
package contracts

import (
	"fmt"
	"math"
	"math/rand"
	"time"
)

// RetryPolicy defines how failed publish tasks are retried.
//
// Uses exponential backoff: delay = min(InitialDelay * Multiplier^attempt, MaxDelay) + jitter.
// Defaults (via DefaultRetryPolicy): 3 retries, 1s initial, 5m max, 2x multiplier, jitter on.
type RetryPolicy struct {
	MaxRetries   int           `json:"max_retries"`
	InitialDelay time.Duration `json:"initial_delay"`
	MaxDelay     time.Duration `json:"max_delay"`
	Multiplier   float64       `json:"multiplier"`
	Jitter       bool          `json:"jitter"`
}

// DefaultRetryPolicy returns a production-ready retry policy.
//
// 3 retries, 1s initial delay, 5m max delay, 2x multiplier, jitter enabled.
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxRetries:   3,
		InitialDelay: 1 * time.Second,
		MaxDelay:     5 * time.Minute,
		Multiplier:   2.0,
		Jitter:       true,
	}
}

// Validate checks that the policy has valid configuration.
func (p RetryPolicy) Validate() error {
	if p.MaxRetries < 0 {
		return fmt.Errorf("max_retries must be >= 0, got %d", p.MaxRetries)
	}
	if p.InitialDelay < 0 {
		return fmt.Errorf("initial_delay must be >= 0, got %v", p.InitialDelay)
	}
	if p.MaxDelay < 0 {
		return fmt.Errorf("max_delay must be >= 0, got %v", p.MaxDelay)
	}
	if p.Multiplier < 1.0 {
		return fmt.Errorf("multiplier must be >= 1.0, got %f", p.Multiplier)
	}
	if p.MaxDelay > 0 && p.InitialDelay > p.MaxDelay {
		return fmt.Errorf("initial_delay (%v) must be <= max_delay (%v)", p.InitialDelay, p.MaxDelay)
	}
	return nil
}

// CalculateBackoff computes the backoff delay for a given attempt number.
// Attempt is zero-indexed (0 = first retry).
func (p RetryPolicy) CalculateBackoff(attempt int) time.Duration {
	if attempt < 0 {
		attempt = 0
	}

	delay := float64(p.InitialDelay) * math.Pow(p.Multiplier, float64(attempt))

	if p.MaxDelay > 0 && time.Duration(delay) > p.MaxDelay {
		delay = float64(p.MaxDelay)
	}

	if p.Jitter {
		// Add up to 25% random jitter to prevent thundering herd.
		jitter := delay * 0.25 * rand.Float64() //nolint:gosec // non-crypto jitter is intentional
		delay += jitter
	}

	return time.Duration(delay)
}

// IdempotencyKeyScope classifies what entity an idempotency key protects.
type IdempotencyKeyScope string

const (
	IdempotencyScopeTask    IdempotencyKeyScope = "task"
	IdempotencyScopeAttempt IdempotencyKeyScope = "attempt"
	IdempotencyScopeResult  IdempotencyKeyScope = "result"
)

// IdempotencyKey prevents duplicate operations in the publish pipeline.
//
// Key format varies by scope:
//   - task:    "{content_id}:{platform}"
//   - attempt: "{task_id}:{attempt_number}"
//   - result:  "{task_id}:{attempt_id}"
//
// See docs/architecture/idempotency-strategy.md.
type IdempotencyKey struct {
	Key       string              `json:"key"`
	Scope     IdempotencyKeyScope `json:"scope"`
	EntityID  string              `json:"entity_id"`
	CreatedAt time.Time           `json:"created_at"`
	TTL       time.Duration       `json:"ttl"`
}

// Validate checks that the key has required fields set.
func (k IdempotencyKey) Validate() error {
	if k.Key == "" {
		return fmt.Errorf("idempotency key must not be empty")
	}
	if k.Scope == "" {
		return fmt.Errorf("idempotency scope must not be empty")
	}
	switch k.Scope {
	case IdempotencyScopeTask, IdempotencyScopeAttempt, IdempotencyScopeResult:
		// valid
	default:
		return fmt.Errorf("invalid idempotency scope: %s", k.Scope)
	}
	if k.EntityID == "" {
		return fmt.Errorf("entity_id must not be empty")
	}
	return nil
}
