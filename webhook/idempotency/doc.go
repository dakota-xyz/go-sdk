// Package idempotency provides event deduplication for webhook processing.
//
// The [Store] interface uses an acquire/commit/release protocol to prevent
// duplicate concurrent processing.
//
// [NewMemoryStore] provides an in-memory implementation with TTL-based
// expiration and max-size eviction, suitable for single-process deployments.
//
// For distributed systems, implement the [Store] interface with a shared
// backend such as Redis or a database.
package idempotency
