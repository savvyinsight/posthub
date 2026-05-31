package contracts

import (
	"testing"
	"time"
)

func TestDefaultRetryPolicy(t *testing.T) {
	p := DefaultRetryPolicy()

	if p.MaxRetries != 3 {
		t.Errorf("MaxRetries = %d, want 3", p.MaxRetries)
	}
	if p.InitialDelay != 1*time.Second {
		t.Errorf("InitialDelay = %v, want 1s", p.InitialDelay)
	}
	if p.MaxDelay != 5*time.Minute {
		t.Errorf("MaxDelay = %v, want 5m", p.MaxDelay)
	}
	if p.Multiplier != 2.0 {
		t.Errorf("Multiplier = %f, want 2.0", p.Multiplier)
	}
	if !p.Jitter {
		t.Error("Jitter should be true by default")
	}

	if err := p.Validate(); err != nil {
		t.Errorf("DefaultRetryPolicy().Validate() error: %v", err)
	}
}

func TestRetryPolicy_Validate(t *testing.T) {
	tests := []struct {
		name    string
		policy  RetryPolicy
		wantErr bool
	}{
		{
			name:    "valid default",
			policy:  DefaultRetryPolicy(),
			wantErr: false,
		},
		{
			name:    "valid zero retries",
			policy:  RetryPolicy{MaxRetries: 0, InitialDelay: time.Second, MaxDelay: time.Minute, Multiplier: 2.0},
			wantErr: false,
		},
		{
			name:    "valid no jitter",
			policy:  RetryPolicy{MaxRetries: 3, InitialDelay: time.Second, MaxDelay: time.Minute, Multiplier: 2.0, Jitter: false},
			wantErr: false,
		},
		{
			name:    "invalid negative retries",
			policy:  RetryPolicy{MaxRetries: -1, InitialDelay: time.Second, MaxDelay: time.Minute, Multiplier: 2.0},
			wantErr: true,
		},
		{
			name:    "invalid negative initial delay",
			policy:  RetryPolicy{MaxRetries: 3, InitialDelay: -time.Second, MaxDelay: time.Minute, Multiplier: 2.0},
			wantErr: true,
		},
		{
			name:    "invalid negative max delay",
			policy:  RetryPolicy{MaxRetries: 3, InitialDelay: time.Second, MaxDelay: -time.Minute, Multiplier: 2.0},
			wantErr: true,
		},
		{
			name:    "invalid multiplier below 1",
			policy:  RetryPolicy{MaxRetries: 3, InitialDelay: time.Second, MaxDelay: time.Minute, Multiplier: 0.5},
			wantErr: true,
		},
		{
			name:    "invalid initial > max delay",
			policy:  RetryPolicy{MaxRetries: 3, InitialDelay: 10 * time.Minute, MaxDelay: time.Minute, Multiplier: 2.0},
			wantErr: true,
		},
		{
			name:    "valid zero delays (no backoff)",
			policy:  RetryPolicy{MaxRetries: 3, InitialDelay: 0, MaxDelay: 0, Multiplier: 1.0},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.policy.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRetryPolicy_CalculateBackoff(t *testing.T) {
	p := RetryPolicy{
		MaxRetries:   3,
		InitialDelay: 1 * time.Second,
		MaxDelay:     30 * time.Second,
		Multiplier:   2.0,
		Jitter:       false, // deterministic for testing
	}

	tests := []struct {
		attempt int
		want    time.Duration
	}{
		{0, 1 * time.Second},  // 1s * 2^0 = 1s
		{1, 2 * time.Second},  // 1s * 2^1 = 2s
		{2, 4 * time.Second},  // 1s * 2^2 = 4s
		{3, 8 * time.Second},  // 1s * 2^3 = 8s
		{4, 16 * time.Second}, // 1s * 2^4 = 16s
		{5, 30 * time.Second}, // 1s * 2^5 = 32s, capped at 30s
		{10, 30 * time.Second}, // capped at max
	}

	for _, tt := range tests {
		t.Run("attempt_"+itoa(tt.attempt), func(t *testing.T) {
			got := p.CalculateBackoff(tt.attempt)
			if got != tt.want {
				t.Errorf("CalculateBackoff(%d) = %v, want %v", tt.attempt, got, tt.want)
			}
		})
	}
}

func TestRetryPolicy_CalculateBackoff_NegativeAttempt(t *testing.T) {
	p := RetryPolicy{
		InitialDelay: 1 * time.Second,
		MaxDelay:     30 * time.Second,
		Multiplier:   2.0,
		Jitter:       false,
	}

	got := p.CalculateBackoff(-1)
	want := 1 * time.Second
	if got != want {
		t.Errorf("CalculateBackoff(-1) = %v, want %v", got, want)
	}
}

func TestRetryPolicy_CalculateBackoff_WithJitter(t *testing.T) {
	p := RetryPolicy{
		InitialDelay: 1 * time.Second,
		MaxDelay:     5 * time.Minute,
		Multiplier:   2.0,
		Jitter:       true,
	}

	// With jitter, result should be between base and base+25%
	base := 1 * time.Second
	maxWithJitter := time.Duration(float64(base) * 1.25)

	got := p.CalculateBackoff(0)
	if got < base || got > maxWithJitter {
		t.Errorf("CalculateBackoff(0) with jitter = %v, want between %v and %v", got, base, maxWithJitter)
	}
}

func TestRetryPolicy_CalculateBackoff_ZeroDelay(t *testing.T) {
	p := RetryPolicy{
		InitialDelay: 0,
		MaxDelay:     0,
		Multiplier:   1.0,
		Jitter:       false,
	}

	got := p.CalculateBackoff(5)
	if got != 0 {
		t.Errorf("CalculateBackoff(5) with zero delay = %v, want 0", got)
	}
}

func TestIdempotencyKey_Validate(t *testing.T) {
	tests := []struct {
		name    string
		key     IdempotencyKey
		wantErr bool
	}{
		{
			name:    "valid task key",
			key:     IdempotencyKey{Key: "content_123:zhihu", Scope: IdempotencyScopeTask, EntityID: "task_1"},
			wantErr: false,
		},
		{
			name:    "valid attempt key",
			key:     IdempotencyKey{Key: "task_1:1", Scope: IdempotencyScopeAttempt, EntityID: "attempt_1"},
			wantErr: false,
		},
		{
			name:    "valid result key",
			key:     IdempotencyKey{Key: "task_1:attempt_1", Scope: IdempotencyScopeResult, EntityID: "result_1"},
			wantErr: false,
		},
		{
			name:    "invalid empty key",
			key:     IdempotencyKey{Key: "", Scope: IdempotencyScopeTask, EntityID: "task_1"},
			wantErr: true,
		},
		{
			name:    "invalid empty scope",
			key:     IdempotencyKey{Key: "k", Scope: "", EntityID: "task_1"},
			wantErr: true,
		},
		{
			name:    "invalid scope",
			key:     IdempotencyKey{Key: "k", Scope: "invalid", EntityID: "task_1"},
			wantErr: true,
		},
		{
			name:    "invalid empty entity_id",
			key:     IdempotencyKey{Key: "k", Scope: IdempotencyScopeTask, EntityID: ""},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.key.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// itoa is a minimal int-to-string helper for test names.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	if neg {
		s = "-" + s
	}
	return s
}
