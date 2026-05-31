// Publish task state machine and publish orchestration types.
//
// PublishTaskStatus has its own state machine separate from ContentStatus.
// Content tracks aggregate publish state; tasks track per-platform execution state.
//
// State transitions follow the architecture docs in docs/architecture/publish-state-machine.md.
package contracts

import "time"

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

// ValidTransitions returns the set of allowed target states from the current state.
func (s PublishTaskStatus) ValidTransitions() []PublishTaskStatus {
	switch s {
	case PublishTaskStatusPending:
		return []PublishTaskStatus{
			PublishTaskStatusProcessing,
			PublishTaskStatusCancelled,
		}
	case PublishTaskStatusProcessing:
		return []PublishTaskStatus{
			PublishTaskStatusSucceeded,
			PublishTaskStatusFailed,
			PublishTaskStatusRetrying,
		}
	case PublishTaskStatusRetrying:
		return []PublishTaskStatus{
			PublishTaskStatusProcessing,
			PublishTaskStatusDead,
		}
	default:
		// Terminal states: succeeded, failed, dead, cancelled.
		return nil
	}
}

// CanTransitionTo reports whether the current state can transition to the target.
func (s PublishTaskStatus) CanTransitionTo(target PublishTaskStatus) bool {
	for _, valid := range s.ValidTransitions() {
		if valid == target {
			return true
		}
	}
	return false
}

// IsTerminal reports whether the task is in a terminal state.
// Terminal states: succeeded, failed, dead, cancelled.
func (s PublishTaskStatus) IsTerminal() bool {
	return s == PublishTaskStatusSucceeded ||
		s == PublishTaskStatusFailed ||
		s == PublishTaskStatusDead ||
		s == PublishTaskStatusCancelled
}

// IsActive reports whether the task is in an active (in-progress) state.
// Active states: pending, processing, retrying.
func (s PublishTaskStatus) IsActive() bool {
	return s == PublishTaskStatusPending ||
		s == PublishTaskStatusProcessing ||
		s == PublishTaskStatusRetrying
}

// PublishIntent represents the user's intent to publish content to one or more platforms.
type PublishIntent struct {
	ContentID string   `json:"content_id"`
	Platforms []string `json:"platforms"`
}

// TaskStatus represents the status of a single platform publish task.
// Returned to clients as part of publish status responses.
type TaskStatus struct {
	TaskID         string            `json:"task_id"`
	Platform       string            `json:"platform"`
	Status         PublishTaskStatus `json:"status"`
	PlatformPostID string            `json:"platform_post_id,omitempty"`
	PlatformURL    string            `json:"platform_url,omitempty"`
	Error          string            `json:"error,omitempty"`
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
}

// PublishStatus represents the aggregate publish status for content.
type PublishStatus struct {
	ContentID string        `json:"content_id"`
	Status    ContentStatus `json:"status"`
	Tasks     []TaskStatus  `json:"tasks"`
}

// PublishBatchResponse is the API response when content is submitted for publishing.
type PublishBatchResponse struct {
	ContentID string       `json:"content_id"`
	Tasks     []TaskStatus `json:"tasks"`
}
