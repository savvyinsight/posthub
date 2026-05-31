// Shared types for the storage layer.
//
// Pagination, optimistic locking, and ID generation used across all stores.
package storage

import (
	"crypto/rand"
	"fmt"
	"time"
)

// Pagination holds offset-based pagination parameters.
type Pagination struct {
	Limit  int
	Offset int
}

// Normalize applies defaults and caps to pagination parameters.
func (p Pagination) Normalize() Pagination {
	if p.Limit <= 0 {
		p.Limit = 20
	}
	if p.Limit > 100 {
		p.Limit = 100
	}
	if p.Offset < 0 {
		p.Offset = 0
	}
	return p
}

// PageResult holds pagination metadata returned alongside query results.
type PageResult struct {
	Total  int
	Limit  int
	Offset int
}

// Versioned wraps any entity with a version counter for optimistic locking.
//
// Callers read the version on fetch and pass it back on update.
// The store rejects updates when the stored version differs,
// preventing lost updates from concurrent writers.
type Versioned[T any] struct {
	Entity  T
	Version int
}

// Asset represents stored asset metadata.
//
// Assets are content references (images, videos, documents) that may
// need uploading to platforms during the publish pipeline.
type Asset struct {
	ID          string    `json:"id"`
	ContentID   string    `json:"content_id"`
	Type        string    `json:"type"`
	URL         string    `json:"url"`
	ContentType string    `json:"content_type,omitempty"`
	Size        int64     `json:"size,omitempty"`
	Checksum    string    `json:"checksum,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// generateID produces a UUID v4 string without external dependencies.
func generateID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	// Set UUID v4 variant and version bits.
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// nowUTC returns the current UTC time.
// Extracted for testability.
var nowUTC = func() time.Time {
	return time.Now().UTC()
}
