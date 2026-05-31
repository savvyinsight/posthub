// Transaction interface for multi-store atomic operations.
//
// The callback pattern prevents transaction leaks: the caller cannot
// forget to commit or rollback. The Tx manages the lifecycle.
package storage

import "context"

// Tx provides transactional execution across multiple stores.
//
// Callers pass a function that receives a TxScope. All operations
// through the scope participate in the same transaction. The transaction
// commits when the function returns nil, and rolls back on any error.
//
// Example:
//
//	err := tx.Run(ctx, func(s *storage.TxScope) error {
//	    content, err := s.ContentStore.CreateContent(ctx, "title", "body", nil)
//	    if err != nil {
//	        return err // rolls back
//	    }
//	    _, err = s.PublishTaskStore.CreateTask(ctx, content.ID, "zhihu")
//	    return err // nil commits both
//	})
type Tx interface {
	// Run executes fn within a transaction.
	// Commits on nil return, rolls back on error.
	Run(ctx context.Context, fn func(scope *TxScope) error) error
}

// TxScope provides transaction-scoped access to all stores.
//
// All operations through a TxScope participate in the same transaction.
type TxScope struct {
	ContentStore
	PublishTaskStore
	PublishAttemptStore
	PlatformPostStore
	AssetStore
}
