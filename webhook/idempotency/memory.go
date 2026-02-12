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

type orderEntry struct {
	key string
	seq uint64
}

type record struct {
	expiresAt time.Time
	pending   bool
	seq       uint64
}

// MemoryStore is an in-memory idempotency store with TTL-based expiration and
// max size eviction.
type MemoryStore struct {
	mu      sync.Mutex
	entries map[string]record // key -> record
	order   []orderEntry      // insertion/update order for eviction
	maxSize int
	ttl     time.Duration
	now     func() time.Time // for testing
	nextSeq uint64
}

// NewMemoryStore creates a new in-memory idempotency store.
func NewMemoryStore(opts ...MemoryOption) *MemoryStore {
	s := &MemoryStore{
		entries: make(map[string]record),
		maxSize: defaultMaxSize,
		ttl:     defaultTTL,
		now:     time.Now,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Acquire atomically reserves a key for processing.
func (s *MemoryStore) Acquire(_ context.Context, key string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	s.evictExpired(now)

	if _, ok := s.entries[key]; ok {
		return false, nil
	}

	s.ensureCapacity(now)
	s.upsert(key, now.Add(s.ttl), true)
	return true, nil
}

// Commit marks a reserved key as processed.
func (s *MemoryStore) Commit(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	s.evictExpired(now)

	if _, ok := s.entries[key]; !ok {
		// Reservation may have expired; treat as no-op.
		return nil
	}

	s.upsert(key, now.Add(s.ttl), false)
	return nil
}

// Release abandons a reserved key so it can be retried.
func (s *MemoryStore) Release(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	rec, ok := s.entries[key]
	if !ok {
		return nil
	}
	if rec.pending {
		delete(s.entries, key)
	}
	return nil
}

func (s *MemoryStore) upsert(key string, expiresAt time.Time, pending bool) {
	s.nextSeq++
	rec := record{
		expiresAt: expiresAt,
		pending:   pending,
		seq:       s.nextSeq,
	}
	s.entries[key] = rec
	s.order = append(s.order, orderEntry{key: key, seq: rec.seq})
}

func (s *MemoryStore) ensureCapacity(now time.Time) {
	if len(s.entries) >= s.maxSize {
		s.evictExpired(now)
	}
	if len(s.entries) >= s.maxSize {
		s.evictOldest()
	}
}

// evictExpired removes all expired entries from the store.
func (s *MemoryStore) evictExpired(now time.Time) {
	alive := s.order[:0]
	for _, e := range s.order {
		rec, ok := s.entries[e.key]
		if !ok {
			continue
		}
		if rec.seq != e.seq {
			// Stale ordering entry.
			continue
		}
		if now.After(rec.expiresAt) {
			delete(s.entries, e.key)
			continue
		}
		alive = append(alive, e)
	}
	s.order = alive
}

// evictOldest removes the oldest active entry.
func (s *MemoryStore) evictOldest() {
	for len(s.order) > 0 {
		oldest := s.order[0]
		s.order = s.order[1:]

		rec, ok := s.entries[oldest.key]
		if !ok {
			continue
		}
		if rec.seq != oldest.seq {
			// Stale ordering entry.
			continue
		}

		delete(s.entries, oldest.key)
		return
	}
}
