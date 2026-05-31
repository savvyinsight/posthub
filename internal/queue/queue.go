// Package queue provides the async task queue for publish operations.
//
// It defines the Enqueuer/Handler abstractions and their Asynq-backed
// implementations, plus supporting types for idempotency, metrics,
// and dead-letter inspection.
//
// Dependency flow: queue (Layer 1) depends on contracts and logger (Layer 0).
// It does NOT depend on storage or platform directly; those are injected
// via the Publisher and TaskStateStore interfaces defined here.
package queue

import (
	"context"

	"github.com/savvyinsight/posthub/internal/contracts"
)

// TaskType identifies the kind of task in the queue.
const (
	TypePublishContent = "publish:content"
)

// PublishPayload is the data carried by a publish task.
type PublishPayload struct {
	TaskID    string `json:"task_id"`
	ContentID string `json:"content_id"`
	Platform  string `json:"platform"`
}

// EnqueueOptions configures how a task is enqueued.
type EnqueueOptions struct {
	MaxRetry    int
	Queue       string
	Timeout     int // seconds
	RetryPolicy *contracts.RetryPolicy // nil = use default backoff
}

// Enqueuer adds tasks to the queue.
type Enqueuer interface {
	// EnqueuePublish adds a publish task to the queue.
	EnqueuePublish(ctx context.Context, payload PublishPayload, opts EnqueueOptions) error
}

// Handler processes tasks from the queue.
type Handler interface {
	// HandlePublish processes a single publish task.
	HandlePublish(ctx context.Context, payload PublishPayload) error
}
