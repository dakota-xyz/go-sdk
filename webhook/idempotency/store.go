package idempotency

import "context"

// Store provides event deduplication. Implementations must be safe for
// concurrent use.
type Store interface {
	// Acquire atomically reserves the key for processing.
	//
	// It returns true only if the key is not currently reserved or committed.
	// If the key has been committed and has not yet expired, Acquire must return
	// false. If the key has expired, Acquire may return true.
	Acquire(ctx context.Context, key string) (acquired bool, err error)

	// Commit marks a previously acquired key as processed.
	//
	// Commit should be idempotent: calling Commit multiple times for the same key
	// should not return an error. After Commit succeeds, Acquire should return
	// false until the key expires.
	Commit(ctx context.Context, key string) error

	// Release abandons a previously acquired key so it can be retried.
	//
	// Release should be idempotent and safe to call for missing keys. After
	// Release succeeds, Acquire should return true unless another worker has
	// acquired or committed the key.
	Release(ctx context.Context, key string) error
}
