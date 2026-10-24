package cache

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestGetExpires(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	c := New[string, int](time.Minute, 4)
	c.now = func() time.Time { return now }

	c.Set("a", 1)
	if v, ok := c.Get("a"); !ok || v != 1 {
		t.Fatalf("Get(a) = %d, %v; want 1, true", v, ok)
	}

	now = now.Add(time.Minute)
	if _, ok := c.Get("a"); ok {
		t.Fatal("Get(a) after TTL returned a value")
	}
	if got := c.Stats().Expired; got != 1 {
		t.Fatalf("expired = %d; want 1", got)
	}
}

func TestEvictionIsBounded(t *testing.T) {
	c := New[string, int](time.Minute, 2)
	c.Set("a", 1)
	c.Set("b", 2)
	c.Set("c", 3)

	if _, ok := c.Get("a"); ok {
		t.Fatal("oldest entry was not evicted")
	}
	if _, ok := c.Get("c"); !ok {
		t.Fatal("newest entry is missing")
	}
	stats := c.Stats()
	if stats.Evictions != 1 || stats.Entries != 2 {
		t.Fatalf("stats = %+v; want 1 eviction and 2 entries", stats)
	}
}

func TestGetOrLoadCoalesces(t *testing.T) {
	c := New[string, int](time.Minute, 4)
	var calls atomic.Int32
	started := make(chan struct{})
	release := make(chan struct{})

	load := func(context.Context) (int, error) {
		calls.Add(1)
		close(started)
		<-release
		return 42, nil
	}

	const workers = 16
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			value, err := c.GetOrLoad(context.Background(), "key", load)
			if err != nil || value != 42 {
				t.Errorf("GetOrLoad = %d, %v; want 42, nil", value, err)
			}
		}()
	}

	select {
	case <-started:
	case <-time.After(time.Second):
		close(release)
		t.Fatal("loader did not start")
	}
	close(release)
	wg.Wait()

	if got := calls.Load(); got != 1 {
		t.Fatalf("loader called %d times; want 1", got)
	}
}

func TestGetOrLoadDoesNotCacheErrors(t *testing.T) {
	c := New[string, int](time.Minute, 4)
	var calls atomic.Int32
	load := func(context.Context) (int, error) {
		calls.Add(1)
		return 0, errors.New("boom")
	}

	for i := 0; i < 2; i++ {
		if _, err := c.GetOrLoad(context.Background(), "key", load); err == nil {
			t.Fatal("expected an error")
		}
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("loader called %d times; want 2", got)
	}
}

func TestDeleteAndPurge(t *testing.T) {
	c := New[string, int](time.Minute, 4)
	c.Set("a", 1)
	c.Set("b", 2)
	c.Delete("a")
	if _, ok := c.Get("a"); ok {
		t.Fatal("deleted key is still present")
	}
	c.Purge()
	if got := c.Stats().Entries; got != 0 {
		t.Fatalf("entries after purge = %d; want 0", got)
	}
}
