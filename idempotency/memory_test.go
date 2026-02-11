package idempotency

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestMemoryStore_AddAndContains(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()

	// Key should not exist initially.
	exists, err := store.Contains(ctx, "evt_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exists {
		t.Error("expected key to not exist")
	}

	// Add key.
	added, err := store.Add(ctx, "evt_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !added {
		t.Error("expected key to be newly added")
	}

	// Key should exist now.
	exists, err = store.Contains(ctx, "evt_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !exists {
		t.Error("expected key to exist")
	}

	// Adding again should return false.
	added, err = store.Add(ctx, "evt_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if added {
		t.Error("expected duplicate key to not be added")
	}
}

func TestMemoryStore_TTLExpiration(t *testing.T) {
	ctx := context.Background()
	now := time.Now()

	store := NewMemoryStore(WithTTL(1 * time.Hour))
	store.now = func() time.Time { return now }

	added, err := store.Add(ctx, "evt_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !added {
		t.Error("expected key to be added")
	}

	// Advance time past TTL.
	store.now = func() time.Time { return now.Add(2 * time.Hour) }

	// Contains should return false for expired key.
	exists, err := store.Contains(ctx, "evt_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exists {
		t.Error("expected expired key to not exist")
	}

	// Add should succeed for expired key.
	added, err = store.Add(ctx, "evt_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !added {
		t.Error("expected expired key to be re-added")
	}
}

func TestMemoryStore_MaxSizeEviction(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore(WithMaxSize(3))

	for i := range 3 {
		key := string(rune('a' + i))
		added, err := store.Add(ctx, key)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !added {
			t.Errorf("expected key %q to be added", key)
		}
	}

	// Adding a 4th key should evict the oldest ("a").
	added, err := store.Add(ctx, "d")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !added {
		t.Error("expected key 'd' to be added")
	}

	exists, err := store.Contains(ctx, "a")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exists {
		t.Error("expected 'a' to be evicted")
	}

	// "b" and "c" should still exist.
	for _, key := range []string{"b", "c", "d"} {
		exists, err := store.Contains(ctx, key)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !exists {
			t.Errorf("expected key %q to still exist", key)
		}
	}
}

func TestMemoryStore_ExpiredEvictionBeforeOldest(t *testing.T) {
	ctx := context.Background()
	now := time.Now()

	store := NewMemoryStore(WithMaxSize(2), WithTTL(1*time.Hour))
	store.now = func() time.Time { return now }

	store.Add(ctx, "a")

	// Advance time so "a" expires.
	store.now = func() time.Time { return now.Add(2 * time.Hour) }

	store.Add(ctx, "b")

	// Store should have evicted "a" (expired) and have room for "b".
	// Adding "c" should evict "b" (oldest non-expired), not panic.
	added, err := store.Add(ctx, "c")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !added {
		t.Error("expected 'c' to be added")
	}
}

func TestMemoryStore_ConcurrentAccess(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore(WithMaxSize(100))

	var wg sync.WaitGroup
	for i := range 50 {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			key := string(rune('A' + n%26))
			store.Add(ctx, key)
			store.Contains(ctx, key)
		}(i)
	}
	wg.Wait()
}

func TestMemoryStore_DefaultOptions(t *testing.T) {
	store := NewMemoryStore()

	if store.maxSize != defaultMaxSize {
		t.Errorf("got maxSize %d, want %d", store.maxSize, defaultMaxSize)
	}
	if store.ttl != defaultTTL {
		t.Errorf("got ttl %v, want %v", store.ttl, defaultTTL)
	}
}

func TestMemoryStore_InvalidOptions(t *testing.T) {
	store := NewMemoryStore(WithMaxSize(-1), WithTTL(-1*time.Second))

	// Should keep defaults for invalid values.
	if store.maxSize != defaultMaxSize {
		t.Errorf("got maxSize %d, want %d", store.maxSize, defaultMaxSize)
	}
	if store.ttl != defaultTTL {
		t.Errorf("got ttl %v, want %v", store.ttl, defaultTTL)
	}
}
