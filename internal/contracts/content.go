// Package contracts defines shared types used across the posthub application.
//
// These types represent the canonical domain models that other packages
// depend on. They contain no logic — only data definitions.
package contracts

import "time"

// ContentStatus represents the lifecycle state of content.
type ContentStatus string

const (
	ContentStatusDraft              ContentStatus = "draft"
	ContentStatusReady              ContentStatus = "ready"
	ContentStatusPublishing         ContentStatus = "publishing"
	ContentStatusPublished          ContentStatus = "published"
	ContentStatusPartiallyPublished ContentStatus = "partially_published"
	ContentStatusFailed             ContentStatus = "failed"
	ContentStatusArchived           ContentStatus = "archived"
)

// ValidTransitions returns the set of allowed target states from the current state.
func (s ContentStatus) ValidTransitions() []ContentStatus {
	switch s {
	case ContentStatusDraft:
		return []ContentStatus{ContentStatusReady}
	case ContentStatusReady:
		return []ContentStatus{ContentStatusPublishing}
	case ContentStatusPublishing:
		return []ContentStatus{
			ContentStatusPublished,
			ContentStatusPartiallyPublished,
			ContentStatusFailed,
		}
	case ContentStatusPartiallyPublished:
		return []ContentStatus{ContentStatusPublishing, ContentStatusArchived}
	case ContentStatusPublished:
		return []ContentStatus{ContentStatusArchived}
	case ContentStatusFailed:
		return []ContentStatus{ContentStatusDraft}
	default:
		return nil
	}
}

// CanTransitionTo reports whether the current state can transition to the target.
func (s ContentStatus) CanTransitionTo(target ContentStatus) bool {
	for _, valid := range s.ValidTransitions() {
		if valid == target {
			return true
		}
	}
	return false
}

// IsTerminal reports whether the content is in a terminal state.
func (s ContentStatus) IsTerminal() bool {
	return s == ContentStatusArchived
}

// Content represents the canonical content model stored in the database.
//
// This is the single source of truth for content across all subsystems.
// Tags are stored as JSONB arrays. Metadata holds flexible key-value data
// (source, word count, custom fields) without schema changes.
type Content struct {
	ID        string         `json:"id"`
	Title     string         `json:"title"`
	Body      string         `json:"body"`
	Tags      []string       `json:"tags"`
	Status    ContentStatus  `json:"status"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}
