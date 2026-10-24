// Package cache is the small TTL cache used by the learning platform. It is a
// teaching implementation: bounded, race-safe, with request coalescing, and
// explicit statistics. It is not a replacement for Redis.
package cache

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sync/singleflight"
)

type entry[V any] struct {
	value   V
	expires time.Time
}

// Stats is a point-in-time snapshot.
type Stats struct {
	Hits      uint64
	Misses    uint64
	Expired   uint64
	Evictions uint64
	Entries   int
}

// Cache is a bounded TTL cache with FIFO eviction. Read paths take a read lock;
// expiry removal and eviction take the write lock.
type Cache[K comparable, V any] struct {
	mu        sync.RWMutex
	entries   map[K]entry[V]
	order     []K
	max       int
	ttl       time.Duration
	now       func() time.Time
	hits      atomic.Uint64
	misses    atomic.Uint64
	expired   atomic.Uint64
	evictions atomic.Uint64
	group     singleflight.Group
}

// New builds a cache holding at most max entries for ttl. A non-positive ttl
// disables caching; the cache still coalesces concurrent loads.
func New[K comparable, V any](ttl time.Duration, max int) *Cache[K, V] {
	if max < 1 {
		max = 1
	}
	return &Cache[K, V]{
		entries: make(map[K]entry[V]),
		max:     max,
		ttl:     ttl,
		now:     time.Now,
	}
}

// Get returns a live entry. Expired entries are removed on access.
func (c *Cache[K, V]) Get(key K) (V, bool) {
	var zero V
	if c.ttl <= 0 {
		c.misses.Add(1)
		return zero, false
	}
	c.mu.RLock()
	item, ok := c.entries[key]
	c.mu.RUnlock()
	if !ok {
		c.misses.Add(1)
		return zero, false
	}
	if !c.now().Before(item.expires) {
		c.mu.Lock()
		// Re-check under the write lock: another goroutine may have replaced it.
		if current, exists := c.entries[key]; exists && !c.now().Before(current.expires) {
			delete(c.entries, key)
			c.expired.Add(1)
		}
		c.mu.Unlock()
		c.misses.Add(1)
		return zero, false
	}
	c.hits.Add(1)
	return item.value, true
}

// Set stores a value, evicting expired entries first and then the oldest live
// entries until the bound is respected.
func (c *Cache[K, V]) Set(key K, value V) {
	if c.ttl <= 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.entries[key]; !exists {
		c.order = append(c.order, key)
	}
	c.entries[key] = entry[V]{value: value, expires: c.now().Add(c.ttl)}

	if len(c.entries) <= c.max {
		return
	}
	now := c.now()
	kept := c.order[:0]
	for _, k := range c.order {
		item, exists := c.entries[k]
		if !exists {
			continue
		}
		if !now.Before(item.expires) {
			delete(c.entries, k)
			c.expired.Add(1)
			continue
		}
		kept = append(kept, k)
	}
	c.order = kept
	for len(c.entries) > c.max && len(c.order) > 0 {
		oldest := c.order[0]
		c.order = c.order[1:]
		if _, exists := c.entries[oldest]; exists {
			delete(c.entries, oldest)
			c.evictions.Add(1)
		}
	}
}

// GetOrLoad returns a cached value or calls load once for concurrent callers
// asking for the same key. The double check prevents a stampede when the entry
// expires while several requests are in flight.
func (c *Cache[K, V]) GetOrLoad(ctx context.Context, key K, load func(context.Context) (V, error)) (V, error) {
	if value, ok := c.Get(key); ok {
		return value, nil
	}
	value, err, _ := c.group.Do(keyString(key), func() (any, error) {
		if cached, ok := c.Get(key); ok {
			return cached, nil
		}
		loaded, err := load(ctx)
		if err != nil {
			return nil, err
		}
		c.Set(key, loaded)
		return loaded, nil
	})
	if err != nil {
		var zero V
		return zero, err
	}
	return value.(V), nil
}

// Delete removes one key. It is used when content changes.
func (c *Cache[K, V]) Delete(key K) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, key)
}

// Purge removes every entry.
func (c *Cache[K, V]) Purge() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[K]entry[V])
	c.order = nil
}

// Stats reports counters without blocking readers for long.
func (c *Cache[K, V]) Stats() Stats {
	c.mu.RLock()
	size := len(c.entries)
	c.mu.RUnlock()
	return Stats{
		Hits:      c.hits.Load(),
		Misses:    c.misses.Load(),
		Expired:   c.expired.Load(),
		Evictions: c.evictions.Load(),
		Entries:   size,
	}
}

func keyString[K comparable](key K) string {
	return fmt.Sprint(key)
}
