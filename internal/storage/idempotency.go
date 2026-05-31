// Idempotency store for duplicate operation prevention.
//
// The idempotency store prevents duplicate operations across the
// publish pipeline. Keys are scoped by entity type (task, attempt, result)
// and automatically expire after their TTL.
//
// See docs/architecture/idempotency-strategy.md.
package storage

import (
	"context"

	"github.com/savvyinsight/posthub/internal/contracts"
)

// IdempotencyStore provides operations for idempotency key management.
//
// Keys are used to prevent duplicate publish operations. Each key
// maps to the entity ID that was created or processed.
type IdempotencyStore interface {
	// Claim attempts to claim an idempotency key.
	//
	// Returns (true, nil) if the key was successfully claimed (new key).
	// Returns (false, nil) if the key already exists (duplicate operation).
	// Returns (false, err) on storage errors.
	Claim(ctx context.Context, key contracts.IdempotencyKey) (bool, error)

	// Get retrieves the entity ID associated with an idempotency key.
	// Returns ErrNotFound if the key does not exist or has expired.
	Get(ctx context.Context, key string, scope contracts.IdempotencyKeyScope) (string, error)

	// Cleanup removes expired idempotency keys.
	// Should be called periodically to prevent unbounded growth.
	Cleanup(ctx context.Context) (int, error)
}
