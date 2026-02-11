package idempotency

import (
	"context"
	"sync"
	"time"
)

const (
	defaultMaxSize = 10_000
	defaultTTL     = 24 * time.Hour
)

// MemoryOption configures a MemoryStore.
type MemoryOption func(*MemoryStore)

// WithMaxSize sets the maximum number of keys to track. When the store is full,
// expired entries are evicted first, then the oldest entry is removed.
func WithMaxSize(n int) MemoryOption {
	return func(s *MemoryStore) {
		if n > 0 {
			s.maxSize = n
		}
	}
}

// WithTTL sets how long keys remain in the store before expiring.
func WithTTL(d time.Duration) MemoryOption {
	return func(s *MemoryStore) {
		if d > 0 {
			s.ttl = d
		}
	}
}

type entry struct {
	key       string
	expiresAt time.Time
}

// MemoryStore is an in-memory idempotency store with TTL-based expiration and
// max size eviction.
type MemoryStore struct {
	mu      sync.Mutex
	entries map[string]time.Time // key -> expiry
	order   []entry              // insertion order for eviction
	maxSize int
	ttl     time.Duration
	now     func() time.Time // for testing
}

// NewMemoryStore creates a new in-memory idempotency store.
func NewMemoryStore(opts ...MemoryOption) *MemoryStore {
	s := &MemoryStore{
		entries: make(map[string]time.Time),
		maxSize: defaultMaxSize,
		ttl:     defaultTTL,
		now:     time.Now,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Contains reports whether the key exists and has not expired.
func (s *MemoryStore) Contains(_ context.Context, key string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	expiresAt, ok := s.entries[key]
	if !ok {
		return false, nil
	}
	if s.now().After(expiresAt) {
		delete(s.entries, key)
		return false, nil
	}
	return true, nil
}

// Add records a key. Returns true if newly added, false if already present.
func (s *MemoryStore) Add(_ context.Context, key string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()

	// Check if key already exists and is not expired.
	if expiresAt, ok := s.entries[key]; ok {
		if now.Before(expiresAt) {
			return false, nil
		}
		// Expired: remove so we can re-add.
		delete(s.entries, key)
	}

	// Evict expired entries if at capacity.
	if len(s.entries) >= s.maxSize {
		s.evictExpired(now)
	}

	// Evict oldest if still at capacity.
	if len(s.entries) >= s.maxSize {
		s.evictOldest()
	}

	expiresAt := now.Add(s.ttl)
	s.entries[key] = expiresAt
	s.order = append(s.order, entry{key: key, expiresAt: expiresAt})
	return true, nil
}

// evictExpired removes all expired entries from the store.
func (s *MemoryStore) evictExpired(now time.Time) {
	alive := s.order[:0]
	for _, e := range s.order {
		if now.After(e.expiresAt) {
			delete(s.entries, e.key)
		} else {
			alive = append(alive, e)
		}
	}
	s.order = alive
}

// evictOldest removes the oldest non-expired entry.
func (s *MemoryStore) evictOldest() {
	for len(s.order) > 0 {
		oldest := s.order[0]
		s.order = s.order[1:]
		if _, ok := s.entries[oldest.key]; ok {
			delete(s.entries, oldest.key)
			return
		}
		// Entry was already removed (expired), continue to next.
	}
}
