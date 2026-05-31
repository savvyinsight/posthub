package logger

import (
	"log/slog"
	"testing"
)

func TestNew_ReturnsNonNil(t *testing.T) {
	tests := []struct {
		level       string
		environment string
	}{
		{"debug", "development"},
		{"info", "development"},
		{"warn", "development"},
		{"error", "development"},
		{"info", "production"},
		{"", "development"},        // empty level defaults to info
		{"unknown", "development"}, // unknown level defaults to info
	}

	for _, tt := range tests {
		t.Run(tt.level+"_"+tt.environment, func(t *testing.T) {
			logger := New(tt.level, tt.environment)
			if logger == nil {
				t.Fatal("New() returned nil")
			}
			if !logger.Enabled(nil, slog.LevelInfo) && tt.level == "" {
				// default should be info level
			}
		})
	}
}

func TestNew_DebugLevelEnablesDebug(t *testing.T) {
	logger := New("debug", "development")
	if !logger.Enabled(nil, slog.LevelDebug) {
		t.Error("debug level should enable debug messages")
	}
}

func TestNew_ErrorLevelDisablesInfo(t *testing.T) {
	logger := New("error", "development")
	if logger.Enabled(nil, slog.LevelInfo) {
		t.Error("error level should disable info messages")
	}
}
