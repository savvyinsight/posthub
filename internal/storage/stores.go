// Repository interfaces for the storage layer.
//
// Each store owns a single aggregate root. Stores are concurrency-safe
// and have no business logic — they persist and retrieve data only.
//
// Implementations must accept context for cancellation and deadlines.
// All errors returned should be either storage.ErrNotFound,
// storage.ErrConflict, or wrapped standard errors.
package storage

import (
	"context"

	"github.com/savvyinsight/posthub/internal/contracts"
)

// ContentStore provides CRUD operations for content with optimistic locking.
type ContentStore interface {
	// CreateContent persists new content in draft status.
	CreateContent(ctx context.Context, title, body string, tags []string) (*contracts.Content, error)

	// GetContent retrieves content by ID.
	// Returns ErrNotFound if the ID does not exist.
	GetContent(ctx context.Context, id string) (*contracts.Content, error)

	// ListContent retrieves content filtered by status with pagination.
	ListContent(ctx context.Context, status contracts.ContentStatus, page Pagination) ([]*contracts.Content, PageResult, error)

	// UpdateContent transitions content to a new status.
	// Uses optimistic locking: the caller must pass the version from the
	// last read. Returns ErrVersionConflict if the content was modified
	// since the caller's last read.
	UpdateContent(ctx context.Context, id string, status contracts.ContentStatus, version int) error
}

// PublishTaskStore provides operations for publish tasks.
//
// A publish task represents a single publish intent for one content
// to one platform. The combination (content_id, platform) is unique.
type PublishTaskStore interface {
	// CreateTask creates a new publish task in pending status.
	// Returns ErrConflict if a task for this (contentID, platform) already exists.
	CreateTask(ctx context.Context, contentID, platform string) (*contracts.PublishTask, error)

	// GetTask retrieves a publish task by ID.
	// Returns ErrNotFound if the ID does not exist.
	GetTask(ctx context.Context, id string) (*contracts.PublishTask, error)

	// GetTasksByContent retrieves all publish tasks for a content item.
	GetTasksByContent(ctx context.Context, contentID string) ([]*contracts.PublishTask, error)

	// GetTaskByContentPlatform retrieves a task by its unique (contentID, platform) pair.
	// Returns ErrNotFound if no task exists for this pair.
	GetTaskByContentPlatform(ctx context.Context, contentID, platform string) (*contracts.PublishTask, error)

	// UpdateTaskStatus transitions a task to a new status.
	UpdateTaskStatus(ctx context.Context, id string, status contracts.PublishTaskStatus, errMsg string) error

	// IncrementAttemptCount atomically increments the attempt counter.
	IncrementAttemptCount(ctx context.Context, id string) error
}

// PublishAttemptStore provides operations for publish attempts.
//
// A task may have multiple attempts due to retries. Each attempt
// records its own start time, completion time, and outcome.
type PublishAttemptStore interface {
	// CreateAttempt records a new publish attempt.
	CreateAttempt(ctx context.Context, taskID string, attemptNumber int) (*contracts.PublishAttempt, error)

	// CompleteAttempt marks an attempt as finished with the given status.
	CompleteAttempt(ctx context.Context, id string, status contracts.PublishTaskStatus, errMsg string) error

	// GetAttemptsByTask retrieves all attempts for a task, ordered by attempt number.
	GetAttemptsByTask(ctx context.Context, taskID string) ([]*contracts.PublishAttempt, error)
}

// PlatformPostStore provides operations for platform post records.
//
// A platform post record is created after a successful publish to
// an external platform. It stores the platform-assigned post ID and URL.
type PlatformPostStore interface {
	// CreatePlatformPost records a successful platform publication.
	CreatePlatformPost(ctx context.Context, taskID, platform, platformPostID, platformURL string, response []byte) error

	// GetByTask retrieves the platform post record for a task.
	// Returns ErrNotFound if no record exists for this task.
	GetByTask(ctx context.Context, taskID string) (*PlatformPostRecord, error)
}

// AssetStore provides operations for asset metadata.
//
// Assets are content references (images, videos, documents) that
// may need uploading to platforms during the publish pipeline.
type AssetStore interface {
	// CreateAsset persists asset metadata.
	CreateAsset(ctx context.Context, asset *Asset) error

	// GetAsset retrieves an asset by ID.
	// Returns ErrNotFound if the ID does not exist.
	GetAsset(ctx context.Context, id string) (*Asset, error)

	// GetAssetsByContent retrieves all assets for a content item.
	GetAssetsByContent(ctx context.Context, contentID string) ([]*Asset, error)
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
