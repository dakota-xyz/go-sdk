package idempotency

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestMemoryStore_AcquireCommitRelease(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()

	acquired, err := store.Acquire(ctx, "evt_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !acquired {
		t.Fatal("expected first acquire to succeed")
	}

	acquired, err = store.Acquire(ctx, "evt_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if acquired {
		t.Fatal("expected second acquire to fail while reserved")
	}

	if err := store.Release(ctx, "evt_1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	acquired, err = store.Acquire(ctx, "evt_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !acquired {
		t.Fatal("expected acquire to succeed after release")
	}

	if err := store.Commit(ctx, "evt_1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	acquired, err = store.Acquire(ctx, "evt_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if acquired {
		t.Fatal("expected acquire to fail after commit")
	}
}

func TestMemoryStore_TTLExpiration(t *testing.T) {
	ctx := context.Background()
	now := time.Now()

	store := NewMemoryStore(WithTTL(1 * time.Hour))
	store.now = func() time.Time { return now }

	acquired, err := store.Acquire(ctx, "evt_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !acquired {
		t.Fatal("expected acquire to succeed")
	}
	if err := store.Commit(ctx, "evt_1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Advance time past TTL; key should expire and become acquirable again.
	store.now = func() time.Time { return now.Add(2 * time.Hour) }
	acquired, err = store.Acquire(ctx, "evt_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !acquired {
		t.Fatal("expected expired key to be acquirable")
	}
}

func TestMemoryStore_MaxSizeEviction(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore(WithMaxSize(3))

	for _, key := range []string{"a", "b", "c"} {
		acquired, err := store.Acquire(ctx, key)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !acquired {
			t.Fatalf("expected acquire for %q", key)
		}
		if err := store.Commit(ctx, key); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	// This should evict oldest key "a".
	acquired, err := store.Acquire(ctx, "d")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !acquired {
		t.Fatal("expected acquire for key 'd'")
	}
	if err := store.Commit(ctx, "d"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// "a" should be evicted and thus acquirable again.
	acquired, err = store.Acquire(ctx, "a")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !acquired {
		t.Fatal("expected key 'a' to be evicted and acquirable")
	}
}

func TestMemoryStore_ExpiredEvictionBeforeOldest(t *testing.T) {
	ctx := context.Background()
	now := time.Now()

	store := NewMemoryStore(WithMaxSize(2), WithTTL(1*time.Hour))
	store.now = func() time.Time { return now }

	acquired, err := store.Acquire(ctx, "a")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !acquired {
		t.Fatal("expected acquire for key 'a'")
	}
	if err := store.Commit(ctx, "a"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Advance time so "a" expires.
	store.now = func() time.Time { return now.Add(2 * time.Hour) }

	acquired, err = store.Acquire(ctx, "b")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !acquired {
		t.Fatal("expected acquire for key 'b'")
	}
	if err := store.Commit(ctx, "b"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// "a" should have been expired and removed, so "c" can be added.
	acquired, err = store.Acquire(ctx, "c")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !acquired {
		t.Fatal("expected acquire for key 'c'")
	}
}

func TestMemoryStore_AcquireSingleWinner(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()

	var winners atomic.Int32
	var wg sync.WaitGroup

	for range 100 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			acquired, err := store.Acquire(ctx, "evt_1")
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if acquired {
				winners.Add(1)
			}
		}()
	}
	wg.Wait()

	if got := winners.Load(); got != 1 {
		t.Fatalf("expected exactly one winner, got %d", got)
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
			acquired, err := store.Acquire(ctx, key)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if acquired {
				if err := store.Commit(ctx, key); err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
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
