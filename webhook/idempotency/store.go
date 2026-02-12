package idempotency

import "context"

// Store provides event deduplication. Implementations must be safe for
// concurrent use.
type Store interface {
	// Acquire atomically reserves the key for processing.
	Acquire(ctx context.Context, key string) (acquired bool, err error)

	// Commit marks a previously acquired key as processed.
	Commit(ctx context.Context, key string) error

	// Release abandons a previously acquired key so it can be retried.
	Release(ctx context.Context, key string) error
}
