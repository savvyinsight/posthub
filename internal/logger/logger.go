// Package logger provides structured logging using zap with context propagation.
//
// Logger instances are created via New and passed explicitly to components.
// Contextual fields (request_id, worker_id, publish_job_id, platform) are
// attached via context using With* helpers and extracted with FromContext.
//
// No global mutable state is maintained.
package logger

import (
	"context"
	"os"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// contextKey is unexported to prevent collisions with other packages.
type contextKey struct{}

// loggerKey is the context key used to store and retrieve Logger instances.
var loggerKey = contextKey{}

// Logger wraps zap.Logger with context propagation support.
type Logger struct {
	zl *zap.Logger
}

// New creates a Logger configured for the given level and environment.
// Production environments get JSON output; development gets console output.
// Output is always written to stderr.
func New(level string, environment string) *Logger {
	lvl := parseLevel(level)

	var encCfg zapcore.EncoderConfig
	if environment == "production" {
		encCfg = zap.NewProductionEncoderConfig()
	} else {
		encCfg = zap.NewDevelopmentEncoderConfig()
	}
	encCfg.TimeKey = "ts"
	encCfg.EncodeTime = zapcore.ISO8601TimeEncoder

	var encoder zapcore.Encoder
	if environment == "production" {
		encoder = zapcore.NewJSONEncoder(encCfg)
	} else {
		encoder = zapcore.NewConsoleEncoder(encCfg)
	}

	core := zapcore.NewCore(
		encoder,
		zapcore.AddSync(os.Stderr),
		lvl,
	)

	return &Logger{zl: zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))}
}

// nopLogger is returned by FromContext when no logger is stored.
var nopLogger = &Logger{zl: zap.NewNop()}

// FromContext extracts the Logger from ctx.
// Returns a no-op logger if none is stored.
func FromContext(ctx context.Context) *Logger {
	if l, ok := ctx.Value(loggerKey).(*Logger); ok && l != nil {
		return l
	}
	return nopLogger
}

// WithContext stores the Logger in ctx and returns the new context.
func (l *Logger) WithContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, loggerKey, l)
}

// WithFields returns a new Logger with the given fields attached.
func (l *Logger) WithFields(fields ...zap.Field) *Logger {
	return &Logger{zl: l.zl.With(fields...)}
}

// WithRequestID returns a new Logger with a request_id field.
func WithRequestID(ctx context.Context, requestID string) context.Context {
	l := FromContext(ctx)
	return l.WithFields(zap.String("request_id", requestID)).WithContext(ctx)
}

// WithWorkerID returns a new Logger with a worker_id field.
func WithWorkerID(ctx context.Context, workerID string) context.Context {
	l := FromContext(ctx)
	return l.WithFields(zap.String("worker_id", workerID)).WithContext(ctx)
}

// WithJobID returns a new Logger with a publish_job_id field.
func WithJobID(ctx context.Context, jobID string) context.Context {
	l := FromContext(ctx)
	return l.WithFields(zap.String("publish_job_id", jobID)).WithContext(ctx)
}

// WithPlatform returns a new Logger with a platform field.
func WithPlatform(ctx context.Context, platform string) context.Context {
	l := FromContext(ctx)
	return l.WithFields(zap.String("platform", platform)).WithContext(ctx)
}

// Info logs a message at info level with optional structured fields.
func (l *Logger) Info(msg string, fields ...zap.Field) {
	l.zl.Info(msg, fields...)
}

// Debug logs a message at debug level with optional structured fields.
func (l *Logger) Debug(msg string, fields ...zap.Field) {
	l.zl.Debug(msg, fields...)
}

// Warn logs a message at warn level with optional structured fields.
func (l *Logger) Warn(msg string, fields ...zap.Field) {
	l.zl.Warn(msg, fields...)
}

// Error logs a message at error level with optional structured fields.
func (l *Logger) Error(msg string, fields ...zap.Field) {
	l.zl.Error(msg, fields...)
}

// Sync flushes any buffered log entries. Should be called before exit.
func (l *Logger) Sync() error {
	return l.zl.Sync()
}

// Zap returns the underlying *zap.Logger for interop with zap-aware libraries.
func (l *Logger) Zap() *zap.Logger {
	return l.zl
}

// parseLevel converts a level string to a zap atomic level.
// Defaults to Info for unrecognized values.
func parseLevel(level string) zap.AtomicLevel {
	switch strings.ToLower(level) {
	case "debug":
		return zap.NewAtomicLevelAt(zapcore.DebugLevel)
	case "warn":
		return zap.NewAtomicLevelAt(zapcore.WarnLevel)
	case "error":
		return zap.NewAtomicLevelAt(zapcore.ErrorLevel)
	default:
		return zap.NewAtomicLevelAt(zapcore.InfoLevel)
	}
}
