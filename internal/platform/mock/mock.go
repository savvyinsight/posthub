// Package mock provides a configurable mock platform adapter for testing
// and MVP demonstration of the publish workflow.
//
// The mock adapter simulates all platform operations (validate, upload,
// publish, delete) with configurable success/failure behaviors. It tracks
// all invocations for test assertions.
//
// No real external API calls are made.
package mock

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/savvyinsight/posthub/internal/contracts"
	"github.com/savvyinsight/posthub/internal/platform"
	"github.com/savvyinsight/posthub/internal/transform"
)

// Behavior controls how the mock platform responds to operations.
type Behavior struct {
	// ValidateErr is returned by Validate. Nil means validation passes.
	ValidateErr error

	// PublishErr is returned by Publish. Nil means publish succeeds.
	PublishErr error

	// UploadErr is returned by UploadAssets. Nil means upload succeeds.
	UploadErr error

	// Retryable indicates whether PublishErr is retryable.
	// Only meaningful when PublishErr is set.
	Retryable bool

	// AttemptsBeforeSuccess is the number of failed attempts before Publish
	// starts returning success. Set to 0 for immediate success.
	// Only applies when PublishErr is set — after this many calls, the error
	// is cleared and subsequent calls succeed.
	AttemptsBeforeSuccess int

	// PublishResult is the result returned on successful publish.
	// If nil, a default result is generated.
	PublishResult *platform.PublishResult
}

// PublishCall records a single invocation of Publish for test assertions.
type PublishCall struct {
	DocTitle string
	Tags     []string
	At       time.Time
}

// MockPlatform implements platform.Platform with configurable behavior.
//
// Thread-safe: all methods may be called concurrently.
type MockPlatform struct {
	name     string
	behavior Behavior
	mu       sync.Mutex
	calls    []PublishCall
	attempt  int
}

// Compile-time interface check.
var _ platform.Platform = (*MockPlatform)(nil)

// New creates a MockPlatform with the given name and behavior.
func New(name string, b Behavior) *MockPlatform {
	return &MockPlatform{
		name:     name,
		behavior: b,
	}
}

// NewSuccessPlatform creates a mock that always succeeds.
func NewSuccessPlatform(name string) *MockPlatform {
	return New(name, Behavior{})
}

// NewRetryablePlatform creates a mock that fails failCount times then succeeds.
func NewRetryablePlatform(name string, failCount int) *MockPlatform {
	return New(name, Behavior{
		PublishErr: &contracts.PlatformError{
			Platform:   name,
			StatusCode: 503,
			Message:    "service unavailable",
			Retryable:  true,
		},
		Retryable:             true,
		AttemptsBeforeSuccess: failCount,
	})
}

// NewPermanentFailPlatform creates a mock that always fails with a non-retryable error.
func NewPermanentFailPlatform(name, errMsg string) *MockPlatform {
	return New(name, Behavior{
		PublishErr: &contracts.PlatformError{
			Platform:   name,
			StatusCode: 422,
			Message:    errMsg,
			Retryable:  false,
		},
		Retryable: false,
	})
}

// NewValidationFailPlatform creates a mock that fails validation.
func NewValidationFailPlatform(name, errMsg string) *MockPlatform {
	return New(name, Behavior{
		ValidateErr: fmt.Errorf("validation failed: %s", errMsg),
	})
}

// Name returns the platform identifier.
func (m *MockPlatform) Name() string {
	return m.name
}

// Validate checks if content meets the platform's requirements.
// Returns Behavior.ValidateErr if set; nil otherwise.
func (m *MockPlatform) Validate(doc *transform.Document) error {
	return m.behavior.ValidateErr
}

// UploadAssets simulates asset uploading.
// Returns Behavior.UploadErr if set; passes through assets otherwise.
func (m *MockPlatform) UploadAssets(_ context.Context, assets []transform.AssetRef) ([]transform.AssetRef, error) {
	if m.behavior.UploadErr != nil {
		return nil, m.behavior.UploadErr
	}
	return assets, nil
}

// Publish simulates publishing content to an external platform.
//
// Tracks each call for test assertions. Behavior depends on configuration:
//   - No PublishErr → immediate success with default or configured PublishResult
//   - PublishErr + AttemptsBeforeSuccess → fails N times, then succeeds
//   - PublishErr + no AttemptsBeforeSuccess → always fails
func (m *MockPlatform) Publish(_ context.Context, doc *transform.Document, _ *platform.Credentials) (*platform.PublishResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Record the call.
	m.calls = append(m.calls, PublishCall{
		DocTitle: doc.Title,
		Tags:     doc.Tags,
		At:       time.Now().UTC(),
	})
	m.attempt++

	// Check if we should simulate failure.
	if m.behavior.PublishErr != nil {
		if m.behavior.AttemptsBeforeSuccess > 0 && m.attempt > m.behavior.AttemptsBeforeSuccess {
			// Past the failure window — fall through to success.
		} else {
			return nil, m.behavior.PublishErr
		}
	}

	// Return configured or default result.
	if m.behavior.PublishResult != nil {
		return m.behavior.PublishResult, nil
	}

	return &platform.PublishResult{
		PlatformPostID: fmt.Sprintf("mock-post-%d", m.attempt),
		PlatformURL:    fmt.Sprintf("https://%s.example.com/posts/%d", m.name, m.attempt),
		PublishedAt:    time.Now().UTC(),
		Response:       json.RawMessage(`{"status":"ok"}`),
	}, nil
}

// Delete simulates deleting published content. Always succeeds.
func (m *MockPlatform) Delete(_ context.Context, _ string, _ *platform.Credentials) error {
	return nil
}

// Capabilities returns permissive default capabilities suitable for testing.
func (m *MockPlatform) Capabilities() platform.Capabilities {
	return platform.Capabilities{
		SupportedNodes: []transform.NodeType{
			transform.NodeHeading,
			transform.NodeParagraph,
			transform.NodeCodeBlock,
			transform.NodeBlockQuote,
			transform.NodeList,
			transform.NodeImage,
			transform.NodeHorizontalRule,
		},
		MaxTitleLength: 500,
		MaxBodyLength:  100000,
		MaxTags:        10,
		MaxImages:      20,
		RequiresCover:  false,
		SupportsVideo:  true,
		AuthType:       platform.AuthTypeOAuth2,
	}
}

// Calls returns a copy of all recorded Publish calls.
func (m *MockPlatform) Calls() []PublishCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]PublishCall, len(m.calls))
	copy(out, m.calls)
	return out
}

// AttemptCount returns the number of Publish invocations.
func (m *MockPlatform) AttemptCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.attempt
}

// Reset clears all recorded calls and resets the attempt counter.
func (m *MockPlatform) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = nil
	m.attempt = 0
}
