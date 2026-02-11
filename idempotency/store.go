package idempotency

import "context"

// Store provides event deduplication. Implementations must be safe for
// concurrent use.
type Store interface {
	// Contains reports whether the key has been seen before.
	Contains(ctx context.Context, key string) (bool, error)

	// Add records a key. Returns true if the key was newly added, false if
	// it was already present.
	Add(ctx context.Context, key string) (added bool, err error)
}
