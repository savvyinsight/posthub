package queue

import (
	"context"
	"errors"
	"testing"

	"github.com/savvyinsight/posthub/internal/contracts"
)

func TestMockPublisher_StaticResult(t *testing.T) {
	result := &contracts.PublishResult{
		PlatformPostID: "post-1",
		PlatformURL:    "https://example.com/1",
	}
	m := &MockPublisher{Result: result}

	got, err := m.Publish(context.Background(), "content-1", "twitter")
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if got.PlatformPostID != "post-1" {
		t.Errorf("PlatformPostID = %s, want post-1", got.PlatformPostID)
	}
	if len(m.Calls) != 1 {
		t.Fatalf("Calls = %d, want 1", len(m.Calls))
	}
	if m.Calls[0].ContentID != "content-1" || m.Calls[0].Platform != "twitter" {
		t.Errorf("Call = %+v, want content-1/twitter", m.Calls[0])
	}
}

func TestMockPublisher_StaticError(t *testing.T) {
	wantErr := errors.New("platform down")
	m := &MockPublisher{Err: wantErr}

	_, err := m.Publish(context.Background(), "content-1", "twitter")
	if !errors.Is(err, wantErr) {
		t.Errorf("Publish() error = %v, want %v", err, wantErr)
	}
}

func TestMockPublisher_CustomFn(t *testing.T) {
	callCount := 0
	m := &MockPublisher{
		PublishFn: func(_ context.Context, contentID, platform string) (*contracts.PublishResult, error) {
			callCount++
			if contentID == "" {
				return nil, contracts.NewValidationError("content_id required")
			}
			return &contracts.PublishResult{PlatformPostID: "fn-post"}, nil
		},
	}

	// Success case.
	result, err := m.Publish(context.Background(), "content-1", "twitter")
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if result.PlatformPostID != "fn-post" {
		t.Errorf("PlatformPostID = %s, want fn-post", result.PlatformPostID)
	}

	// Error case.
	_, err = m.Publish(context.Background(), "", "twitter")
	if err == nil {
		t.Error("Publish() with empty contentID should error")
	}

	if callCount != 2 {
		t.Errorf("PublishFn called %d times, want 2", callCount)
	}
	if len(m.Calls) != 2 {
		t.Errorf("Calls = %d, want 2", len(m.Calls))
	}
}

func TestMockPublisher_RecordsMultipleCalls(t *testing.T) {
	m := &MockPublisher{
		Result: &contracts.PublishResult{PlatformPostID: "post"},
	}

	platforms := []string{"twitter", "mastodon", "linkedin"}
	for _, p := range platforms {
		m.Publish(context.Background(), "content-1", p)
	}

	if len(m.Calls) != 3 {
		t.Fatalf("Calls = %d, want 3", len(m.Calls))
	}
	for i, p := range platforms {
		if m.Calls[i].Platform != p {
			t.Errorf("Calls[%d].Platform = %s, want %s", i, m.Calls[i].Platform, p)
		}
	}
}
