// Package logger provides structured logging setup using log/slog.
//
// All components should accept a *slog.Logger rather than creating their own.
// This package only provides the factory function for consistent initialization.
package logger

import (
	"log/slog"
	"os"
	"strings"
)

// New creates a *slog.Logger configured for the given level.
// Output is JSON to stderr for production, text to stderr for development.
func New(level string, environment string) *slog.Logger {
	var lvl slog.Level
	switch strings.ToLower(level) {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level: lvl,
	}

	var handler slog.Handler
	if environment == "production" {
		handler = slog.NewJSONHandler(os.Stderr, opts)
	} else {
		handler = slog.NewTextHandler(os.Stderr, opts)
	}

	return slog.New(handler)
}
