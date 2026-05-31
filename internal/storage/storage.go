// Package storage defines the database access interfaces.
//
// These interfaces abstract the database layer so that the rest of the
// application depends on contracts, not concrete implementations.
// Actual implementations (sqlc-generated or manual) will live alongside
// these interfaces when the database layer is built.
package storage

import (
	"context"

	"github.com/savvyinsight/posthub/internal/contracts"
)

// ContentStore provides CRUD operations for content.
type ContentStore interface {
	// CreateContent persists new content and returns it with a generated ID.
	CreateContent(ctx context.Context, title, body string, tags []string) (*contracts.Content, error)

	// GetContent retrieves content by ID.
	GetContent(ctx context.Context, id string) (*contracts.Content, error)

	// ListContent retrieves content filtered by status with pagination.
	ListContent(ctx context.Context, status contracts.ContentStatus, limit, offset int) ([]*contracts.Content, error)

	// UpdateContentStatus transitions content to a new status.
	UpdateContentStatus(ctx context.Context, id string, status contracts.ContentStatus) error
}

// PublishTaskStore provides operations for publish tasks.
type PublishTaskStore interface {
	// CreateTask creates a new publish task.
	CreateTask(ctx context.Context, contentID, platform string, maxRetries int) (*contracts.PublishTask, error)

	// GetTask retrieves a publish task by ID.
	GetTask(ctx context.Context, id string) (*contracts.PublishTask, error)

	// GetTasksByContent retrieves all publish tasks for a content item.
	GetTasksByContent(ctx context.Context, contentID string) ([]*contracts.PublishTask, error)

	// UpdateTaskStatus transitions a task to a new status.
	UpdateTaskStatus(ctx context.Context, id string, status contracts.PublishTaskStatus, errMsg string) error
}

// PublishAttemptStore provides operations for publish attempts.
type PublishAttemptStore interface {
	// CreateAttempt records a new publish attempt.
	CreateAttempt(ctx context.Context, taskID string, attemptNumber int) (*contracts.PublishAttempt, error)

	// CompleteAttempt marks an attempt as finished.
	CompleteAttempt(ctx context.Context, id string, status contracts.PublishTaskStatus, errMsg string) error

	// GetAttemptsByTask retrieves all attempts for a task.
	GetAttemptsByTask(ctx context.Context, taskID string) ([]*contracts.PublishAttempt, error)
}

// PlatformPostStore provides operations for platform post records.
type PlatformPostStore interface {
	// CreatePlatformPost records a successful platform publication.
	CreatePlatformPost(ctx context.Context, taskID, platform, platformPostID, platformURL string, response []byte, publishedAt interface{}) error

	// GetByTask retrieves the platform post record for a task.
	GetByTask(ctx context.Context, taskID string) (*PlatformPostRecord, error)
}

// PlatformPostRecord is the storage representation of a successful platform post.
type PlatformPostRecord struct {
	ID             string `json:"id"`
	TaskID         string `json:"task_id"`
	Platform       string `json:"platform"`
	PlatformPostID string `json:"platform_post_id"`
	PlatformURL    string `json:"platform_url,omitempty"`
	Response       []byte `json:"response,omitempty"`
}
