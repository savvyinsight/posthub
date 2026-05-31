// Publisher interface and test mock for platform publishing.
//
// Publisher abstracts the actual publish operation so the queue layer
// never imports concrete platform adapters. The MockPublisher is
// exported for use by other packages' tests.
package queue

import (
	"context"

	"github.com/savvyinsight/posthub/internal/contracts"
)

// Publisher publishes content to a specific platform.
//
// Implementations are injected at startup by cmd/worker.
// The queue layer does not depend on concrete platform adapters.
type Publisher interface {
	// Publish sends content to a platform and returns the result.
	Publish(ctx context.Context, contentID, platform string) (*contracts.PublishResult, error)
}

// MockPublisher is a test double that returns predetermined results.
//
// Set PublishFn for custom behavior, or set Result/Err for static responses.
// Every invocation is recorded in Calls for assertion.
type MockPublisher struct {
	// PublishFn, if set, overrides the default Result/Err behavior.
	PublishFn func(ctx context.Context, contentID, platform string) (*contracts.PublishResult, error)
	// Result is returned when PublishFn is nil.
	Result *contracts.PublishResult
	// Err is returned when PublishFn is nil.
	Err error
	// Calls records each invocation for test assertions.
	Calls []PublishCall
}

// PublishCall records the arguments of a single Publish invocation.
type PublishCall struct {
	ContentID string
	Platform  string
}

// Publish implements Publisher. It records the call and returns
// either PublishFn's result or the static Result/Err.
func (m *MockPublisher) Publish(ctx context.Context, contentID, platform string) (*contracts.PublishResult, error) {
	m.Calls = append(m.Calls, PublishCall{ContentID: contentID, Platform: platform})
	if m.PublishFn != nil {
		return m.PublishFn(ctx, contentID, platform)
	}
	return m.Result, m.Err
}
