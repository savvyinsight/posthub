package platform

import (
	"context"
	"testing"

	"github.com/savvyinsight/posthub/internal/transform"
)

// mockPlatform implements Platform for testing.
type mockPlatform struct {
	name string
}

func (m *mockPlatform) Name() string                           { return m.name }
func (m *mockPlatform) Validate(doc *transform.Document) error { return nil }
func (m *mockPlatform) UploadAssets(ctx context.Context, assets []transform.AssetRef) ([]transform.AssetRef, error) {
	return assets, nil
}
func (m *mockPlatform) Publish(ctx context.Context, doc *transform.Document, creds *Credentials) (*PublishResult, error) {
	return &PublishResult{}, nil
}
func (m *mockPlatform) Delete(ctx context.Context, postID string, creds *Credentials) error {
	return nil
}
func (m *mockPlatform) Capabilities() Capabilities { return Capabilities{} }

func TestRegistry_RegisterAndGet(t *testing.T) {
	r := NewRegistry()
	p := &mockPlatform{name: "zhihu"}

	r.Register(p)

	got, err := r.Get("zhihu")
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if got.Name() != "zhihu" {
		t.Errorf("Get().Name() = %s, want zhihu", got.Name())
	}
}

func TestRegistry_GetNotFound(t *testing.T) {
	r := NewRegistry()

	_, err := r.Get("nonexistent")
	if err == nil {
		t.Error("expected error for unregistered platform")
	}
}

func TestRegistry_DuplicatePanics(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockPlatform{name: "zhihu"})

	defer func() {
		if recover() == nil {
			t.Error("expected panic for duplicate registration")
		}
	}()
	r.Register(&mockPlatform{name: "zhihu"})
}

func TestRegistry_List(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockPlatform{name: "bilibili"})
	r.Register(&mockPlatform{name: "zhihu"})
	r.Register(&mockPlatform{name: "weibo"})

	names := r.List()
	if len(names) != 3 {
		t.Fatalf("len(List()) = %d, want 3", len(names))
	}
	// Should be sorted
	expected := []string{"bilibili", "weibo", "zhihu"}
	for i, name := range names {
		if name != expected[i] {
			t.Errorf("List()[%d] = %s, want %s", i, name, expected[i])
		}
	}
}

func TestRegistry_Has(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockPlatform{name: "zhihu"})

	if !r.Has("zhihu") {
		t.Error("Has(zhihu) = false, want true")
	}
	if r.Has("bilibili") {
		t.Error("Has(bilibili) = true, want false")
	}
}

func TestRegistry_Count(t *testing.T) {
	r := NewRegistry()
	if r.Count() != 0 {
		t.Errorf("Count() = %d, want 0", r.Count())
	}

	r.Register(&mockPlatform{name: "zhihu"})
	r.Register(&mockPlatform{name: "bilibili"})
	if r.Count() != 2 {
		t.Errorf("Count() = %d, want 2", r.Count())
	}
}
