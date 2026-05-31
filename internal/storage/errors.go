// Sentinel errors for the storage layer.
//
// Callers should use errors.Is() to check these values.
package storage

import "errors"

var (
	// ErrNotFound indicates the requested entity does not exist.
	ErrNotFound = errors.New("not found")

	// ErrConflict indicates a unique constraint violation (e.g., duplicate task).
	ErrConflict = errors.New("conflict")

	// ErrVersionConflict indicates an optimistic locking failure.
	// The entity was modified by another caller since the last read.
	ErrVersionConflict = errors.New("version conflict")
)
