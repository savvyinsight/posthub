package contracts

import "testing"

func TestPublishTaskStatus_CanTransitionTo(t *testing.T) {
	tests := []struct {
		name   string
		from   PublishTaskStatus
		to     PublishTaskStatus
		wantOK bool
	}{
		// From pending
		{"pending to processing", PublishTaskStatusPending, PublishTaskStatusProcessing, true},
		{"pending to cancelled", PublishTaskStatusPending, PublishTaskStatusCancelled, true},
		{"pending to succeeded", PublishTaskStatusPending, PublishTaskStatusSucceeded, false},
		{"pending to failed", PublishTaskStatusPending, PublishTaskStatusFailed, false},

		// From processing
		{"processing to succeeded", PublishTaskStatusProcessing, PublishTaskStatusSucceeded, true},
		{"processing to failed", PublishTaskStatusProcessing, PublishTaskStatusFailed, true},
		{"processing to retrying", PublishTaskStatusProcessing, PublishTaskStatusRetrying, true},
		{"processing to pending", PublishTaskStatusProcessing, PublishTaskStatusPending, false},
		{"processing to dead", PublishTaskStatusProcessing, PublishTaskStatusDead, false},

		// From retrying
		{"retrying to processing", PublishTaskStatusRetrying, PublishTaskStatusProcessing, true},
		{"retrying to dead", PublishTaskStatusRetrying, PublishTaskStatusDead, true},
		{"retrying to succeeded", PublishTaskStatusRetrying, PublishTaskStatusSucceeded, false},
		{"retrying to failed", PublishTaskStatusRetrying, PublishTaskStatusFailed, false},

		// Terminal states have no transitions
		{"succeeded to anything", PublishTaskStatusSucceeded, PublishTaskStatusProcessing, false},
		{"failed to anything", PublishTaskStatusFailed, PublishTaskStatusPending, false},
		{"dead to anything", PublishTaskStatusDead, PublishTaskStatusPending, false},
		{"cancelled to anything", PublishTaskStatusCancelled, PublishTaskStatusPending, false},
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

func TestPublishTaskStatus_IsTerminal(t *testing.T) {
	tests := []struct {
		status PublishTaskStatus
		want   bool
	}{
		{PublishTaskStatusPending, false},
		{PublishTaskStatusProcessing, false},
		{PublishTaskStatusRetrying, false},
		{PublishTaskStatusSucceeded, true},
		{PublishTaskStatusFailed, true},
		{PublishTaskStatusDead, true},
		{PublishTaskStatusCancelled, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			if got := tt.status.IsTerminal(); got != tt.want {
				t.Errorf("%s.IsTerminal() = %v, want %v", tt.status, got, tt.want)
			}
		})
	}
}

func TestPublishTaskStatus_IsActive(t *testing.T) {
	tests := []struct {
		status PublishTaskStatus
		want   bool
	}{
		{PublishTaskStatusPending, true},
		{PublishTaskStatusProcessing, true},
		{PublishTaskStatusRetrying, true},
		{PublishTaskStatusSucceeded, false},
		{PublishTaskStatusFailed, false},
		{PublishTaskStatusDead, false},
		{PublishTaskStatusCancelled, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			if got := tt.status.IsActive(); got != tt.want {
				t.Errorf("%s.IsActive() = %v, want %v", tt.status, got, tt.want)
			}
		})
	}
}

func TestPublishTaskStatus_IsTerminalAndActiveMutuallyExclusive(t *testing.T) {
	allStates := []PublishTaskStatus{
		PublishTaskStatusPending,
		PublishTaskStatusProcessing,
		PublishTaskStatusSucceeded,
		PublishTaskStatusFailed,
		PublishTaskStatusRetrying,
		PublishTaskStatusDead,
		PublishTaskStatusCancelled,
	}

	for _, s := range allStates {
		if s.IsTerminal() && s.IsActive() {
			t.Errorf("%s is both terminal and active", s)
		}
	}
}
