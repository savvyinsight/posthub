// Publish task and attempt domain types.
//
// PublishTaskStatus and state machine methods are defined in state.go.
// PublishIntent, TaskStatus, and orchestration types are also in state.go.
package contracts

import (
	"encoding/json"
	"time"
)

// PublishTask represents a single publish intent for one content to one platform.
//
// Each task tracks its own retry lifecycle independently.
// The combination of (content_id, platform) is unique — one task per platform per publish.
type PublishTask struct {
	ID           string            `json:"id"`
	ContentID    string            `json:"content_id"`
	Platform     string            `json:"platform"`
	Status       PublishTaskStatus `json:"status"`
	RetryPolicy  RetryPolicy       `json:"retry_policy"`
	AttemptCount int               `json:"attempt_count"`
	Error        string            `json:"error,omitempty"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
}

// PublishAttempt represents a single attempt at completing a publish task.
//
// A task may have multiple attempts due to retries.
// Each attempt records its own start time, completion time, and outcome.
type PublishAttempt struct {
	ID            string            `json:"id"`
	TaskID        string            `json:"task_id"`
	AttemptNumber int               `json:"attempt_number"`
	Status        PublishTaskStatus `json:"status"`
	Error         string            `json:"error,omitempty"`
	StartedAt     time.Time         `json:"started_at"`
	CompletedAt   *time.Time        `json:"completed_at,omitempty"`
}

// PublishResult represents the outcome of a successful publish to a platform.
//
// Stored after a platform adapter successfully publishes content.
// Response holds the raw platform API response for debugging.
type PublishResult struct {
	PlatformPostID string          `json:"platform_post_id"`
	PlatformURL    string          `json:"platform_url,omitempty"`
	PublishedAt    time.Time       `json:"published_at"`
	Response       json.RawMessage `json:"response,omitempty"`
}

// PublishRequest represents the API request to publish content to platforms.
type PublishRequest struct {
	Platforms []string `json:"platforms"`
}
