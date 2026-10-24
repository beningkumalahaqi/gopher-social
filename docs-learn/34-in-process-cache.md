# 34. An in-process cache in Go

## Goal

Read the cache used by the platform and understand every decision in it:
bounds, locking, expiry, eviction, and coalescing.

## Generics keep the call site honest

The cache is generic over key and value:

```go
type Cache[K comparable, V any] struct {
    mu      sync.RWMutex
    entries map[K]entry[V]
    order   []K
    max     int
    ttl     time.Duration
    now     func() time.Time
    group   singleflight.Group
}
```

`K comparable` is required by the map. `V any` avoids `interface{}` casts at
every call site. The zero value is not usable; `New` initializes the map and
normalizes a non-positive maximum to one.

The `now func() time.Time` field looks like over-engineering until you test
expiry. Tests can advance a fake clock without sleeping. In production it is
`time.Now`.

## Reads take a read lock

`Get` is the hot path:

```go
c.mu.RLock()
item, ok := c.entries[key]
c.mu.RUnlock()
if !ok {
    c.misses.Add(1)
    return zero, false
}
```

The lock is held only for the map lookup. Expiry is checked after unlocking
because a hit is the common case and does not need a write lock.

An expired entry is removed lazily, under the write lock, with a re-check:

```go
if !c.now().Before(item.expires) {
    c.mu.Lock()
    if current, exists := c.entries[key]; exists && !c.now().Before(current.expires) {
        delete(c.entries, key)
        c.expired.Add(1)
    }
    c.mu.Unlock()
    ...
}
```

The re-check matters. Between the read unlock and the write lock, another
goroutine may have replaced the entry with a fresh value. Deleting without
re-checking would remove the new value.

## Writes evict, but not by recency

`Set` appends a new key to `order` and stores the entry. If the map is over the
bound, it first drops expired keys, then removes from the front of `order`:

```go
for len(c.entries) > c.max && len(c.order) > 0 {
    oldest := c.order[0]
    c.order = c.order[1:]
    if _, exists := c.entries[oldest]; exists {
        delete(c.entries, oldest)
        c.evictions.Add(1)
    }
}
```

This is FIFO, not LRU. Reading a key does not move it. For a small cache with a
short TTL, FIFO is predictable and cheap. For a workload where a few keys are
hot and many are cold, FIFO can evict a hot key while a cold key survives.
That is a reason to measure the eviction rate, not a reason to assume LRU is
always better.

`order` can contain stale keys after `Delete` because deleting from the middle
of a slice is O(n). The eviction loop skips keys that are no longer in the map.
Memory for stale keys is reclaimed when they reach the front.

## Expiry is lazy

There is no background janitor. An expired entry occupies memory until it is
read or until a `Set` triggers a sweep. For a cache of a few hundred entries,
that is fine. For a cache of millions, a janitor or a size-bounded eviction
policy is required.

Lazy expiry also means `Stats.Entries` can include expired entries that have
not been touched. If you need an exact live count, sweep under the lock first,
and accept the pause.

## Counters are atomic

`hits`, `misses`, `expired`, and `evictions` are `atomic.Uint64`. Updating them
under the mutex would also work, but the read path releases the lock before
incrementing. Atomics keep the counters correct without holding the lock
across the increment.

`Stats` reads the size under a read lock and the counters atomically. The
snapshot is not a single point in time: a hit can occur between reading size
and reading hits. That is acceptable for observability. Do not use it for
billing.

## Coalescing is the second half

`GetOrLoad` wraps `singleflight.Group`:

```go
value, err, _ := c.group.Do(keyString(key), func() (any, error) {
    if cached, ok := c.Get(key); ok {
        return cached, nil
    }
    loaded, err := load(ctx)
    ...
})
```

The double check inside the flight handles the case where the value was
populated between the first miss and the start of the flight. Concurrent
callers for the same key wait for one load and receive its result.

`keyString` uses `fmt.Sprint`. That is convenient for string keys and risky for
structs with ambiguous formatting. If keys can collide in their string form,
write an explicit key function.

Two details are easy to miss:

- The context passed to `load` belongs to whichever caller started the flight.
  If that caller cancels, every waiter receives the cancellation. For
  request-scoped loads, this is usually acceptable, but it is a real coupling.
- Errors are not cached. Every new caller retries the load. That is correct
  for transient failures, but it also means a failing dependency is hammered
  by every request. Add a short negative cache only with a deliberate policy.

## Exercises

1. Write a test that advances the fake clock past the TTL and asserts a miss.
2. Explain why `Set` must re-check `len(c.entries) <= c.max` before sweeping.
3. Construct a workload where FIFO evicts a hot key. Would LRU fix it?

## What to remember

Hold locks briefly, re-check under the write lock, make expiry and eviction
explicit, count what you cannot see, and know that coalescing shares one
caller's context and never caches errors.
