package queue

import (
	"context"
	"testing"
	"time"
)

func TestInMemoryIdempotencyGuard_Acquire_NewKey(t *testing.T) {
	g := NewInMemoryIdempotencyGuard()

	acquired, err := g.Acquire(context.Background(), "key-1", 5*time.Minute)
	if err != nil {
		t.Fatalf("Acquire() error = %v", err)
	}
	if !acquired {
		t.Error("Acquire() = false, want true for new key")
	}
}

func TestInMemoryIdempotencyGuard_Acquire_Duplicate(t *testing.T) {
	g := NewInMemoryIdempotencyGuard()

	acquired, err := g.Acquire(context.Background(), "key-1", 5*time.Minute)
	if err != nil || !acquired {
		t.Fatalf("first Acquire() = (%v, %v), want (true, nil)", acquired, err)
	}

	acquired, err = g.Acquire(context.Background(), "key-1", 5*time.Minute)
	if err != nil {
		t.Fatalf("second Acquire() error = %v", err)
	}
	if acquired {
		t.Error("second Acquire() = true, want false for duplicate key")
	}
}

func TestInMemoryIdempotencyGuard_Acquire_DifferentKeys(t *testing.T) {
	g := NewInMemoryIdempotencyGuard()

	acquired1, _ := g.Acquire(context.Background(), "key-1", 5*time.Minute)
	acquired2, _ := g.Acquire(context.Background(), "key-2", 5*time.Minute)

	if !acquired1 || !acquired2 {
		t.Errorf("both keys should be acquired: key-1=%v, key-2=%v", acquired1, acquired2)
	}
}

func TestInMemoryIdempotencyGuard_Acquire_ExpiredKey(t *testing.T) {
	g := NewInMemoryIdempotencyGuard()

	// Acquire with a very short TTL.
	acquired, _ := g.Acquire(context.Background(), "key-1", 1*time.Millisecond)
	if !acquired {
		t.Fatal("first Acquire() should succeed")
	}

	// Wait for expiration.
	time.Sleep(5 * time.Millisecond)

	acquired, err := g.Acquire(context.Background(), "key-1", 5*time.Minute)
	if err != nil {
		t.Fatalf("Acquire() after expiry error = %v", err)
	}
	if !acquired {
		t.Error("Acquire() after expiry = false, want true")
	}
}

func TestInMemoryIdempotencyGuard_Release(t *testing.T) {
	g := NewInMemoryIdempotencyGuard()

	g.Acquire(context.Background(), "key-1", 5*time.Minute)

	if err := g.Release(context.Background(), "key-1"); err != nil {
		t.Fatalf("Release() error = %v", err)
	}

	// Should be acquirable again.
	acquired, _ := g.Acquire(context.Background(), "key-1", 5*time.Minute)
	if !acquired {
		t.Error("Acquire() after Release() = false, want true")
	}
}

func TestInMemoryIdempotencyGuard_Release_NonExistent(t *testing.T) {
	g := NewInMemoryIdempotencyGuard()

	// Releasing a non-existent key should not error.
	err := g.Release(context.Background(), "missing-key")
	if err != nil {
		t.Fatalf("Release() error = %v, want nil", err)
	}
}

func TestInMemoryIdempotencyGuard_Size(t *testing.T) {
	g := NewInMemoryIdempotencyGuard()

	if g.Size() != 0 {
		t.Errorf("initial Size() = %d, want 0", g.Size())
	}

	g.Acquire(context.Background(), "key-1", 5*time.Minute)
	g.Acquire(context.Background(), "key-2", 5*time.Minute)

	if g.Size() != 2 {
		t.Errorf("Size() = %d, want 2", g.Size())
	}

	g.Release(context.Background(), "key-1")

	if g.Size() != 1 {
		t.Errorf("Size() after release = %d, want 1", g.Size())
	}
}

// Compile-time interface checks.
var (
	_ IdempotencyGuard = (*RedisIdempotencyGuard)(nil)
	_ IdempotencyGuard = (*InMemoryIdempotencyGuard)(nil)
)
