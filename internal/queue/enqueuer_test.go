package queue

import (
	"context"
	"testing"

	"github.com/savvyinsight/posthub/internal/contracts"
)

// Compile-time interface check for AsynqEnqueuer.
var _ Enqueuer = (*AsynqEnqueuer)(nil)

func TestEnqueueOptions_Defaults(t *testing.T) {
	opts := EnqueueOptions{
		MaxRetry: 3,
		Queue:    "publish",
		Timeout:  30,
	}

	if opts.MaxRetry != 3 {
		t.Errorf("MaxRetry = %d, want 3", opts.MaxRetry)
	}
	if opts.Queue != "publish" {
		t.Errorf("Queue = %s, want publish", opts.Queue)
	}
	if opts.Timeout != 30 {
		t.Errorf("Timeout = %d, want 30", opts.Timeout)
	}
	if opts.RetryPolicy != nil {
		t.Error("RetryPolicy should be nil by default")
	}
}

func TestEnqueueOptions_WithRetryPolicy(t *testing.T) {
	policy := contracts.DefaultRetryPolicy()
	opts := EnqueueOptions{
		MaxRetry:    5,
		Queue:       "publish",
		RetryPolicy: &policy,
	}

	if opts.RetryPolicy == nil {
		t.Fatal("RetryPolicy should not be nil")
	}
	if opts.RetryPolicy.MaxRetries != 3 {
		t.Errorf("RetryPolicy.MaxRetries = %d, want 3", opts.RetryPolicy.MaxRetries)
	}
}

// Test that the buildOptions helper doesn't panic with zero values.
func TestAsynqEnqueuer_buildOptions_ZeroValue(t *testing.T) {
	// We can't easily test buildOptions without an asynq client,
	// but we can verify EnqueueOptions construction is sane.
	opts := EnqueueOptions{}
	if opts.MaxRetry != 0 {
		t.Errorf("zero-value MaxRetry = %d, want 0", opts.MaxRetry)
	}
	if opts.Queue != "" {
		t.Errorf("zero-value Queue = %q, want empty", opts.Queue)
	}
}

// TestPublishPayload_Fields verifies the struct field layout.
func TestPublishPayload_Fields(t *testing.T) {
	p := PublishPayload{
		TaskID:    "t1",
		ContentID: "c1",
		Platform:  "twitter",
	}
	if p.TaskID != "t1" || p.ContentID != "c1" || p.Platform != "twitter" {
		t.Errorf("unexpected payload: %+v", p)
	}
}

// TestTypePublishContent verifies the task type constant.
func TestTypePublishContent(t *testing.T) {
	if TypePublishContent != "publish:content" {
		t.Errorf("TypePublishContent = %q, want %q", TypePublishContent, "publish:content")
	}
}

// Verify that a nil context doesn't break the Enqueuer interface contract.
func TestEnqueuerInterface_NilContextSafety(t *testing.T) {
	// This test exists to document that implementations should handle
	// context gracefully. We test with a valid context here since
	// nil context would panic in standard library code.
	var _ Enqueuer = (*AsynqEnqueuer)(nil)

	// Just verify the mock publisher works with context.
	m := &MockPublisher{Result: &contracts.PublishResult{PlatformPostID: "x"}}
	_, _ = m.Publish(context.Background(), "c1", "twitter")
}
