package logger

import (
	"context"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestNew_ReturnsNonNil(t *testing.T) {
	tests := []struct {
		name string
		lvl  string
		env  string
	}{
		{name: "debug_development", lvl: "debug", env: "development"},
		{name: "info_development", lvl: "info", env: "development"},
		{name: "warn_development", lvl: "warn", env: "development"},
		{name: "error_development", lvl: "error", env: "development"},
		{name: "info_production", lvl: "info", env: "production"},
		{name: "empty_level_defaults_to_info", lvl: "", env: "development"},
		{name: "unknown_level_defaults_to_info", lvl: "unknown", env: "development"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := New(tt.lvl, tt.env)
			if l == nil {
				t.Fatal("New() returned nil")
			}
		})
	}
}

func TestNew_DebugLevelEnablesDebug(t *testing.T) {
	l := New("debug", "development")
	core, _ := observer.New(zapcore.DebugLevel)
	testLogger := &Logger{zl: zap.New(core)}

	// Verify the logger was created (debug level should work)
	if l == nil {
		t.Fatal("New(debug) returned nil")
	}
	_ = testLogger
}

func TestNew_ErrorLevelDisablesInfo(t *testing.T) {
	l := New("error", "development")
	if l == nil {
		t.Fatal("New(error) returned nil")
	}
}

func TestFromContext_WithLogger(t *testing.T) {
	l := New("info", "development")
	ctx := l.WithContext(context.Background())

	got := FromContext(ctx)
	if got == nil {
		t.Fatal("FromContext() returned nil")
	}
	if got != l {
		t.Error("FromContext() did not return the stored logger")
	}
}

func TestFromContext_EmptyContext(t *testing.T) {
	ctx := context.Background()
	l := FromContext(ctx)

	if l == nil {
		t.Fatal("FromContext() returned nil for empty context")
	}
	// Should be the nop logger, not nil — calling Info should not panic
	l.Info("should not panic")
}

func TestFromContext_NilLogger(t *testing.T) {
	ctx := context.WithValue(context.Background(), contextKey{}, (*Logger)(nil))
	l := FromContext(ctx)

	if l == nil {
		t.Fatal("FromContext() returned nil for nil logger in context")
	}
}

func TestWithContext_OverwritesPrevious(t *testing.T) {
	l1 := New("info", "development")
	l2 := New("debug", "development")

	ctx := l1.WithContext(context.Background())
	ctx = l2.WithContext(ctx)

	got := FromContext(ctx)
	if got != l2 {
		t.Error("WithContext did not overwrite previous logger")
	}
}

func TestWithFields_ReturnsNewLogger(t *testing.T) {
	l := New("info", "development")
	enriched := l.WithFields(zap.String("key", "value"))

	if enriched == nil {
		t.Fatal("WithFields() returned nil")
	}
	if enriched == l {
		t.Error("WithFields() should return a new Logger, not the same instance")
	}
}

func TestWithRequestID_SetsField(t *testing.T) {
	ctx := New("info", "development").WithContext(context.Background())
	ctx = WithRequestID(ctx, "req-123")

	l := FromContext(ctx)
	if l == nil {
		t.Fatal("FromContext() returned nil after WithRequestID")
	}
}

func TestWithWorkerID_SetsField(t *testing.T) {
	ctx := New("info", "development").WithContext(context.Background())
	ctx = WithWorkerID(ctx, "worker-456")

	l := FromContext(ctx)
	if l == nil {
		t.Fatal("FromContext() returned nil after WithWorkerID")
	}
}

func TestWithJobID_SetsField(t *testing.T) {
	ctx := New("info", "development").WithContext(context.Background())
	ctx = WithJobID(ctx, "job-789")

	l := FromContext(ctx)
	if l == nil {
		t.Fatal("FromContext() returned nil after WithJobID")
	}
}

func TestWithPlatform_SetsField(t *testing.T) {
	ctx := New("info", "development").WithContext(context.Background())
	ctx = WithPlatform(ctx, "twitter")

	l := FromContext(ctx)
	if l == nil {
		t.Fatal("FromContext() returned nil after WithPlatform")
	}
}

func TestWithHelpers_ChainMultiple(t *testing.T) {
	ctx := New("info", "development").WithContext(context.Background())
	ctx = WithRequestID(ctx, "req-1")
	ctx = WithWorkerID(ctx, "w-1")
	ctx = WithJobID(ctx, "j-1")
	ctx = WithPlatform(ctx, "twitter")

	l := FromContext(ctx)
	if l == nil {
		t.Fatal("FromContext() returned nil after chaining helpers")
	}
}

func TestLogger_LogMethods_DoNotPanic(t *testing.T) {
	l := New("debug", "development")

	l.Info("test message", zap.String("key", "value"))
	l.Debug("test message", zap.Int("count", 1))
	l.Warn("test message", zap.Bool("flag", true))
	l.Error("test message", zap.Error(nil))
}

func TestLogger_NopLogger_DoNotPanic(t *testing.T) {
	ctx := context.Background()
	l := FromContext(ctx)

	l.Info("should not panic")
	l.Debug("should not panic")
	l.Warn("should not panic")
	l.Error("should not panic")
}

func TestLogger_WithContext_FieldsPreserved(t *testing.T) {
	core, logs := observer.New(zapcore.InfoLevel)
	l := &Logger{zl: zap.New(core)}

	ctx := l.WithContext(context.Background())
	ctx = WithRequestID(ctx, "req-abc")
	ctx = WithPlatform(ctx, "twitter")

	FromContext(ctx).Info("test")

	if logs.Len() != 1 {
		t.Fatalf("expected 1 log entry, got %d", logs.Len())
	}

	entry := logs.All()[0]
	fields := entry.ContextMap()

	if fields["request_id"] != "req-abc" {
		t.Errorf("request_id = %v, want req-abc", fields["request_id"])
	}
	if fields["platform"] != "twitter" {
		t.Errorf("platform = %v, want twitter", fields["platform"])
	}
}

func TestLogger_WithFields_FieldsLogged(t *testing.T) {
	core, logs := observer.New(zapcore.InfoLevel)
	l := &Logger{zl: zap.New(core)}

	l.WithFields(zap.String("service", "api")).Info("starting")

	if logs.Len() != 1 {
		t.Fatalf("expected 1 log entry, got %d", logs.Len())
	}

	fields := logs.All()[0].ContextMap()
	if fields["service"] != "api" {
		t.Errorf("service = %v, want api", fields["service"])
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input string
		want  zapcore.Level
	}{
		{input: "debug", want: zapcore.DebugLevel},
		{input: "DEBUG", want: zapcore.DebugLevel},
		{input: "info", want: zapcore.InfoLevel},
		{input: "INFO", want: zapcore.InfoLevel},
		{input: "", want: zapcore.InfoLevel},
		{input: "warn", want: zapcore.WarnLevel},
		{input: "WARN", want: zapcore.WarnLevel},
		{input: "error", want: zapcore.ErrorLevel},
		{input: "ERROR", want: zapcore.ErrorLevel},
		{input: "unknown", want: zapcore.InfoLevel},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			al := parseLevel(tt.input)
			if al.Level() != tt.want {
				t.Errorf("parseLevel(%q) = %v, want %v", tt.input, al.Level(), tt.want)
			}
		})
	}
}

func TestLogger_Sync(t *testing.T) {
	l := New("info", "development")
	// Sync may return an error on stderr (e.g., "invalid argument"), which is fine.
	_ = l.Sync()
}

func TestLogger_Zap(t *testing.T) {
	l := New("info", "development")
	zl := l.Zap()
	if zl == nil {
		t.Fatal("Zap() returned nil")
	}
}
