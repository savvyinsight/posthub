package contracts

import "fmt"

// ErrorCode represents a machine-readable error classification.
type ErrorCode string

const (
	ErrCodeValidation  ErrorCode = "validation_error"
	ErrCodeNotFound    ErrorCode = "not_found"
	ErrCodeForbidden   ErrorCode = "forbidden"
	ErrCodeInternal    ErrorCode = "internal_error"
	ErrCodeConflict    ErrorCode = "conflict"
	ErrCodeUnavailable ErrorCode = "unavailable"
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
