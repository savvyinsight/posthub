package contracts

import "testing"

func TestContentStatus_CanTransitionTo(t *testing.T) {
	tests := []struct {
		name   string
		from   ContentStatus
		to     ContentStatus
		wantOK bool
	}{
		{"draft to ready", ContentStatusDraft, ContentStatusReady, true},
		{"draft to publishing", ContentStatusDraft, ContentStatusPublishing, false},
		{"ready to publishing", ContentStatusReady, ContentStatusPublishing, true},
		{"ready to draft", ContentStatusReady, ContentStatusDraft, false},
		{"publishing to published", ContentStatusPublishing, ContentStatusPublished, true},
		{"publishing to partially_published", ContentStatusPublishing, ContentStatusPartiallyPublished, true},
		{"publishing to failed", ContentStatusPublishing, ContentStatusFailed, true},
		{"publishing to draft", ContentStatusPublishing, ContentStatusDraft, false},
		{"partially_published to publishing", ContentStatusPartiallyPublished, ContentStatusPublishing, true},
		{"partially_published to archived", ContentStatusPartiallyPublished, ContentStatusArchived, true},
		{"published to archived", ContentStatusPublished, ContentStatusArchived, true},
		{"published to draft", ContentStatusPublished, ContentStatusDraft, false},
		{"failed to draft", ContentStatusFailed, ContentStatusDraft, true},
		{"failed to ready", ContentStatusFailed, ContentStatusReady, false},
		{"archived to anything", ContentStatusArchived, ContentStatusDraft, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.from.CanTransitionTo(tt.to)
			if got != tt.wantOK {
				t.Errorf("%s -> %s: got %v, want %v", tt.from, tt.to, got, tt.wantOK)
			}
		})
	}
}

func TestContentStatus_IsTerminal(t *testing.T) {
	tests := []struct {
		status ContentStatus
		want   bool
	}{
		{ContentStatusDraft, false},
		{ContentStatusReady, false},
		{ContentStatusPublishing, false},
		{ContentStatusPublished, false},
		{ContentStatusPartiallyPublished, false},
		{ContentStatusFailed, false},
		{ContentStatusArchived, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			if got := tt.status.IsTerminal(); got != tt.want {
				t.Errorf("%s.IsTerminal() = %v, want %v", tt.status, got, tt.want)
			}
		})
	}
}

func TestContent_MetadataOptional(t *testing.T) {
	c := Content{
		ID:     "test-1",
		Title:  "Test",
		Body:   "body",
		Status: ContentStatusDraft,
	}

	if c.Metadata != nil {
		t.Errorf("Metadata should be nil when not set, got %v", c.Metadata)
	}

	c.Metadata = map[string]any{
		"source":     "web",
		"word_count": 1500,
	}

	if c.Metadata["source"] != "web" {
		t.Errorf("Metadata[source] = %v, want web", c.Metadata["source"])
	}
}
