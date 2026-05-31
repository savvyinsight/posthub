package mock

import (
	"context"
	"errors"
	"testing"

	"github.com/savvyinsight/posthub/internal/contracts"
	"github.com/savvyinsight/posthub/internal/platform"
	"github.com/savvyinsight/posthub/internal/transform"
)

func TestMockPlatform_Name(t *testing.T) {
	m := New("zhihu", Behavior{})
	if m.Name() != "zhihu" {
		t.Errorf("Name() = %s, want zhihu", m.Name())
	}
}

func TestMockPlatform_Validate_Success(t *testing.T) {
	m := NewSuccessPlatform("test")
	doc := &transform.Document{Title: "Hello"}

	if err := m.Validate(doc); err != nil {
		t.Errorf("Validate() error = %v, want nil", err)
	}
}

func TestMockPlatform_Validate_Failure(t *testing.T) {
	m := NewValidationFailPlatform("test", "title too long")
	doc := &transform.Document{Title: "Hello"}

	err := m.Validate(doc)
	if err == nil {
		t.Fatal("Validate() error = nil, want error")
	}
	if !contains(err.Error(), "title too long") {
		t.Errorf("Validate() error = %q, want to contain 'title too long'", err.Error())
	}
}

func TestMockPlatform_UploadAssets_Success(t *testing.T) {
	m := NewSuccessPlatform("test")
	assets := []transform.AssetRef{
		{URL: "https://example.com/img.png", Type: transform.AssetTypeImage},
	}

	got, err := m.UploadAssets(context.Background(), assets)
	if err != nil {
		t.Fatalf("UploadAssets() error = %v", err)
	}
	if len(got) != len(assets) {
		t.Errorf("UploadAssets() returned %d assets, want %d", len(got), len(assets))
	}
}

func TestMockPlatform_UploadAssets_Error(t *testing.T) {
	m := New("test", Behavior{UploadErr: errors.New("upload failed")})

	_, err := m.UploadAssets(context.Background(), nil)
	if err == nil {
		t.Fatal("UploadAssets() error = nil, want error")
	}
}

func TestMockPlatform_Publish_Success(t *testing.T) {
	m := NewSuccessPlatform("test")
	doc := &transform.Document{Title: "Hello", Tags: []string{"go"}}

	result, err := m.Publish(context.Background(), doc, &platform.Credentials{})
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if result == nil {
		t.Fatal("Publish() result = nil, want non-nil")
	}
	if result.PlatformPostID == "" {
		t.Error("Publish() result.PlatformPostID is empty")
	}
	if result.PlatformURL == "" {
		t.Error("Publish() result.PlatformURL is empty")
	}
}

func TestMockPlatform_Publish_RetryableThenSucceed(t *testing.T) {
	m := NewRetryablePlatform("test", 2)
	doc := &transform.Document{Title: "Hello"}

	// First two attempts should fail.
	for i := 0; i < 2; i++ {
		_, err := m.Publish(context.Background(), doc, &platform.Credentials{})
		if err == nil {
			t.Fatalf("Publish() attempt %d: error = nil, want retryable error", i+1)
		}
		var platErr *contracts.PlatformError
		if !errors.As(err, &platErr) {
			t.Fatalf("Publish() attempt %d: error is not PlatformError: %v", i+1, err)
		}
		if !platErr.IsRetryable() {
			t.Errorf("Publish() attempt %d: error.Retryable = false, want true", i+1)
		}
	}

	// Third attempt should succeed.
	result, err := m.Publish(context.Background(), doc, &platform.Credentials{})
	if err != nil {
		t.Fatalf("Publish() attempt 3: error = %v, want nil", err)
	}
	if result == nil {
		t.Fatal("Publish() attempt 3: result = nil, want non-nil")
	}
}

func TestMockPlatform_Publish_PermanentFail(t *testing.T) {
	m := NewPermanentFailPlatform("test", "content rejected")
	doc := &transform.Document{Title: "Hello"}

	for i := 0; i < 3; i++ {
		_, err := m.Publish(context.Background(), doc, &platform.Credentials{})
		if err == nil {
			t.Fatalf("Publish() attempt %d: error = nil, want permanent error", i+1)
		}
		var platErr *contracts.PlatformError
		if errors.As(err, &platErr) {
			if platErr.IsRetryable() {
				t.Errorf("Publish() attempt %d: error.Retryable = true, want false", i+1)
			}
		}
	}
}

func TestMockPlatform_Publish_CallTracking(t *testing.T) {
	m := NewSuccessPlatform("test")

	docs := []*transform.Document{
		{Title: "Post 1", Tags: []string{"go"}},
		{Title: "Post 2", Tags: []string{"rust"}},
	}
	for _, doc := range docs {
		_, _ = m.Publish(context.Background(), doc, &platform.Credentials{})
	}

	calls := m.Calls()
	if len(calls) != 2 {
		t.Fatalf("len(Calls()) = %d, want 2", len(calls))
	}
	if calls[0].DocTitle != "Post 1" {
		t.Errorf("Calls()[0].DocTitle = %s, want 'Post 1'", calls[0].DocTitle)
	}
	if calls[1].DocTitle != "Post 2" {
		t.Errorf("Calls()[1].DocTitle = %s, want 'Post 2'", calls[1].DocTitle)
	}
	if m.AttemptCount() != 2 {
		t.Errorf("AttemptCount() = %d, want 2", m.AttemptCount())
	}
}

func TestMockPlatform_Publish_Reset(t *testing.T) {
	m := NewSuccessPlatform("test")
	doc := &transform.Document{Title: "Hello"}

	_, _ = m.Publish(context.Background(), doc, &platform.Credentials{})
	if m.AttemptCount() != 1 {
		t.Fatalf("AttemptCount() before reset = %d, want 1", m.AttemptCount())
	}

	m.Reset()
	if m.AttemptCount() != 0 {
		t.Errorf("AttemptCount() after reset = %d, want 0", m.AttemptCount())
	}
	if len(m.Calls()) != 0 {
		t.Errorf("len(Calls()) after reset = %d, want 0", len(m.Calls()))
	}
}

func TestMockPlatform_Delete(t *testing.T) {
	m := NewSuccessPlatform("test")
	if err := m.Delete(context.Background(), "post-123", &platform.Credentials{}); err != nil {
		t.Errorf("Delete() error = %v, want nil", err)
	}
}

func TestMockPlatform_Capabilities(t *testing.T) {
	m := NewSuccessPlatform("test")
	caps := m.Capabilities()

	if caps.MaxTitleLength <= 0 {
		t.Error("Capabilities().MaxTitleLength <= 0")
	}
	if caps.MaxBodyLength <= 0 {
		t.Error("Capabilities().MaxBodyLength <= 0")
	}
	if len(caps.SupportedNodes) == 0 {
		t.Error("Capabilities().SupportedNodes is empty")
	}
}

func TestMockPlatform_ImplementsInterface(t *testing.T) {
	var _ platform.Platform = (*MockPlatform)(nil)
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
