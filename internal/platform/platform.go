// Package platform defines the platform abstraction layer.
//
// Every social platform implements the Platform interface.
// Adding a new platform requires:
//  1. Implement the Platform interface
//  2. Register the adapter in the Registry
//  3. Configure credentials
//
// No changes to the core system.
package platform

import (
	"context"
	"encoding/json"
	"time"

	"github.com/savvyinsight/posthub/internal/transform"
)

// AuthType classifies how a platform authenticates.
type AuthType string

const (
	AuthTypeOAuth2 AuthType = "oauth2"
	AuthTypeCookie AuthType = "cookie"
	AuthTypeAPIKey AuthType = "api_key"
)

// Capabilities describes what a platform supports.
type Capabilities struct {
	SupportedNodes []transform.NodeType `json:"supported_nodes"`
	MaxTitleLength int                  `json:"max_title_length"`
	MaxBodyLength  int                  `json:"max_body_length"`
	MaxTags        int                  `json:"max_tags"`
	MaxImages      int                  `json:"max_images"`
	RequiresCover  bool                 `json:"requires_cover"`
	SupportsVideo  bool                 `json:"supports_video"`
	AuthType       AuthType             `json:"auth_type"`
}

// PublishResult is the outcome of a successful publish.
type PublishResult struct {
	PlatformPostID string          `json:"platform_post_id"`
	PlatformURL    string          `json:"platform_url,omitempty"`
	PublishedAt    time.Time       `json:"published_at"`
	Response       json.RawMessage `json:"response,omitempty"`
}

// Credentials holds authentication material for a platform.
type Credentials struct {
	// OAuth2 fields
	AccessToken  string    `json:"access_token,omitempty"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	Expiry       time.Time `json:"expiry,omitempty"`

	// Cookie fields
	Cookie string `json:"cookie,omitempty"`

	// API key fields
	APIKey string `json:"api_key,omitempty"`
}

// Platform is the interface every social platform adapter must implement.
type Platform interface {
	// Name returns the platform identifier (e.g., "zhihu", "bilibili").
	Name() string

	// Validate checks if content meets the platform's requirements.
	Validate(doc *transform.Document) error

	// UploadAssets uploads assets and returns platform-specific references.
	UploadAssets(ctx context.Context, assets []transform.AssetRef) ([]transform.AssetRef, error)

	// Publish publishes content to the platform.
	Publish(ctx context.Context, doc *transform.Document, creds *Credentials) (*PublishResult, error)

	// Delete removes published content from the platform.
	Delete(ctx context.Context, postID string, creds *Credentials) error

	// Capabilities returns the platform's capabilities.
	Capabilities() Capabilities
}
