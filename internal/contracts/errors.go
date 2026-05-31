package contracts

import (
	"fmt"
	"time"
)

// ErrorCode represents a machine-readable error classification.
type ErrorCode string

// Application error codes for classifying failures.
const (
	ErrCodeValidation  ErrorCode = "validation_error"
	ErrCodeNotFound    ErrorCode = "not_found"
	ErrCodeForbidden   ErrorCode = "forbidden"
	ErrCodeInternal    ErrorCode = "internal_error"
	ErrCodeConflict    ErrorCode = "conflict"
	ErrCodeUnavailable ErrorCode = "unavailable"
	ErrCodeRateLimited ErrorCode = "rate_limited"
)

// AppError represents an application-level error with a code and message.
type AppError struct {
	Code    ErrorCode     `json:"error"`
	Message string        `json:"message"`
	Details []ErrorDetail `json:"details,omitempty"`
}

func (e *AppError) Error() string {
	if len(e.Details) > 0 {
		return fmt.Sprintf("%s: %s (%d details)", e.Code, e.Message, len(e.Details))
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// ErrorDetail provides field-level error information.
type ErrorDetail struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// NewValidationError creates a validation AppError.
func NewValidationError(message string, details ...ErrorDetail) *AppError {
	return &AppError{
		Code:    ErrCodeValidation,
		Message: message,
		Details: details,
	}
}

// NewNotFoundError creates a not-found AppError.
func NewNotFoundError(resource string) *AppError {
	return &AppError{
		Code:    ErrCodeNotFound,
		Message: resource + " not found",
	}
}

// NewForbiddenError creates a forbidden AppError.
func NewForbiddenError(message string) *AppError {
	return &AppError{
		Code:    ErrCodeForbidden,
		Message: message,
	}
}

// NewInternalError creates an internal AppError.
func NewInternalError(message string) *AppError {
	return &AppError{
		Code:    ErrCodeInternal,
		Message: message,
	}
}

// NewConflictError creates a conflict AppError.
func NewConflictError(message string) *AppError {
	return &AppError{
		Code:    ErrCodeConflict,
		Message: message,
	}
}

// NewUnavailableError creates an unavailable AppError.
func NewUnavailableError(message string) *AppError {
	return &AppError{
		Code:    ErrCodeUnavailable,
		Message: message,
	}
}

// PlatformError represents an error from an external platform API.
type PlatformError struct {
	Platform   string `json:"platform"`
	StatusCode int    `json:"status_code"`
	Message    string `json:"message"`
	Retryable  bool   `json:"retryable"`
}

func (e *PlatformError) Error() string {
	return fmt.Sprintf("platform %s (HTTP %d): %s [retryable=%v]", e.Platform, e.StatusCode, e.Message, e.Retryable)
}

// IsRetryable reports whether the error is transient and the operation can be retried.
func (e *PlatformError) IsRetryable() bool {
	return e.Retryable
}

// RateLimitError represents a rate limit violation from a platform or the local rate limiter.
type RateLimitError struct {
	Platform   string        `json:"platform"`
	RetryAfter time.Duration `json:"retry_after"`
	Message    string        `json:"message"`
}

func (e *RateLimitError) Error() string {
	if e.RetryAfter > 0 {
		return fmt.Sprintf("rate limited on %s: retry after %v", e.Platform, e.RetryAfter)
	}
	return fmt.Sprintf("rate limited on %s: %s", e.Platform, e.Message)
}
