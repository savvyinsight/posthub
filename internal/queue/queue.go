// Package queue defines the task queue abstraction.
//
// The queue is responsible for enqueueing publish tasks and providing
// the handler interface that workers implement. The actual Asynq
// implementation will be added when Redis integration is built.
package queue

import "context"

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
	MaxRetry int
	Queue    string
	Timeout  int // seconds
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
