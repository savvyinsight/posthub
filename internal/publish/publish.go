// Package publish provides the publish orchestration domain.
//
// It defines the types and interfaces for managing the lifecycle of
// publish operations: creating tasks per platform, tracking status,
// and handling partial success.
package publish

import (
	"time"

	"github.com/savvyinsight/posthub/internal/contracts"
)

// PublishIntent represents the user's intent to publish content to one or more platforms.
type PublishIntent struct {
	ContentID string   `json:"content_id"`
	Platforms []string `json:"platforms"`
}

// PublishStatus represents the aggregate publish status for content.
type PublishStatus struct {
	ContentID string                  `json:"content_id"`
	Status    contracts.ContentStatus `json:"status"`
	Tasks     []TaskStatus            `json:"tasks"`
}

// TaskStatus represents the status of a single platform publish task.
type TaskStatus struct {
	TaskID         string                      `json:"task_id"`
	Platform       string                      `json:"platform"`
	Status         contracts.PublishTaskStatus `json:"status"`
	PlatformPostID string                      `json:"platform_post_id,omitempty"`
	PlatformURL    string                      `json:"platform_url,omitempty"`
	Error          string                      `json:"error,omitempty"`
	CreatedAt      time.Time                   `json:"created_at"`
	UpdatedAt      time.Time                   `json:"updated_at"`
}
