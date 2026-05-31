package contracts

import (
	"encoding/json"
	"time"
)

// PublishTaskStatus represents the lifecycle state of a publish task.
type PublishTaskStatus string

const (
	PublishTaskStatusPending    PublishTaskStatus = "pending"
	PublishTaskStatusProcessing PublishTaskStatus = "processing"
	PublishTaskStatusSucceeded  PublishTaskStatus = "succeeded"
	PublishTaskStatusFailed     PublishTaskStatus = "failed"
	PublishTaskStatusRetrying   PublishTaskStatus = "retrying"
	PublishTaskStatusDead       PublishTaskStatus = "dead"
	PublishTaskStatusCancelled  PublishTaskStatus = "cancelled"
)

// PublishTask represents a single publish intent for one content to one platform.
type PublishTask struct {
	ID         string            `json:"id"`
	ContentID  string            `json:"content_id"`
	Platform   string            `json:"platform"`
	Status     PublishTaskStatus `json:"status"`
	MaxRetries int               `json:"max_retries"`
	Error      string            `json:"error,omitempty"`
	CreatedAt  time.Time         `json:"created_at"`
	UpdatedAt  time.Time         `json:"updated_at"`
}

// PublishAttempt represents a single attempt at completing a publish task.
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

// PublishJobStatus represents the status of a queued publish job returned to the client.
type PublishJobStatus struct {
	ID       string            `json:"id"`
	Platform string            `json:"platform"`
	Status   PublishTaskStatus `json:"status"`
}
