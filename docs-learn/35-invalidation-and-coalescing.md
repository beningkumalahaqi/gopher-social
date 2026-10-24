# 35. Invalidation and request coalescing

## Goal

Keep cached values correct when data changes, and keep a cold key from turning
one slow request into a hundred.

## Three invalidation strategies

### Expire on a timer

The cache entry has a TTL. After it passes, the next read misses and reloads.
This is simple and bounded: the user can see data at most one TTL old. It does
not require the writer to know about the cache.

Use it when stale data is acceptable and the reload is cheap.

### Delete on write

The writer removes the key after a successful database write:

```go
if err := store.Update(ctx, id, value); err != nil {
    return err
}
cache.Delete(key)
```

This gives read-after-write freshness for readers that miss next. It has two
failure modes. If the delete fails or is forgotten, the old value survives until
the TTL. If a reader repopulates the key from an old snapshot between the write
and the delete, the stale value can win.

The usual mitigation is delete after commit, not before, and accept that the
TTL is still the backstop. There is no atomic "write database and delete cache"
across two systems.

### Version the key

Include a version in the key:

```go
key := "lesson:" + slug + ":v" + strconv.FormatInt(version, 10)
```

A write increments the version. Old keys become unreachable and expire on
their own. No delete is needed, and a racing reader cannot repopulate the
current key with old data.

The cost is that you need a reliable version source. A row version column, a
content hash, or a deploy ID all work. An in-memory counter does not work across
processes.

## The platform's choice

Search results include a fingerprint of the profile's progress in the key. A
progress change produces a new key. The old entry is unreachable and expires.
No write path calls `Delete`, so no handler can forget to.

This is a good fit because progress changes are frequent and the result is
cheap to recompute. It would be a poor fit for a large object where most of the
value does not depend on progress, because every change would duplicate the
whole result.

Notes and progress pages are not cached at all. Their reads are indexed and
small, and a stale completion count is a visible correctness bug.

## Coalescing a cold key

Without coalescing, a cold key under concurrency looks like this:

```text
request 1: miss -> query (slow)
request 2: miss -> query (slow)
request 3: miss -> query (slow)
```

With `singleflight`, one caller runs the loader and the others wait:

```go
value, err, _ := group.Do(key, func() (any, error) {
    if cached, ok := cache.Get(key); ok {
        return cached, nil
    }
    return load(ctx)
})
```

The double check is essential. A caller can miss, then wait for a flight that
was started by another caller who already populated the cache. Without the
second `Get`, that caller would load again after the flight completes.

Coalescing is not caching. It only helps requests that overlap in time. A
request one second after the flight completes will miss again unless the value
was stored with a TTL.

## Coalescing shares a context

The loader receives the context of the caller that started the flight. If that
request is canceled, all waiters get the cancellation, even if their own
contexts are still alive. For a request-scoped loader, this is usually the
desired behaviour: if the originator gave up, the work is probably not worth
finishing. For a background refresh, use a detached context with its own
timeout.

## Stampede at expiry

Even with coalescing, all keys that expire at the same instant can send one
flight each. If the backing store has a limit, that is still a burst.

Options:

- Add jitter to TTLs so keys do not expire together.
- Refresh hot keys before expiry (stale-while-revalidate).
- Serve a slightly stale value while one goroutine refreshes.
- Cap concurrent loads with a semaphore.

Do not add all four. Pick the one that matches the failure you can observe. For
this corpus, jitter is enough if expiry bursts ever matter.

## Errors and negative caching

A failed load is not stored. That is the right default: a transient database
error should not make every user wait for the TTL. But it means a broken
dependency receives one load attempt per request, even with coalescing,
because each flight ends in an error and the next request starts a new one.

A negative cache stores "not found" or "failed" for a short time. It protects
the dependency but can hide a recovery. If you add one, make the TTL much
shorter than the positive TTL and count negative hits separately.

## Test invalidation directly

The test that matters is not "the value is cached". It is:

1. Load a value.
2. Change the source.
3. Read again.
4. Assert the new value, or assert the documented staleness window.

Also test two concurrent readers of a cold key and assert the loader ran once.
`go test -race` should pass on both.

## Exercises

1. Add a version column to a small table and use it in a cache key. What must
   happen for the version to change?
2. Write a test where two goroutines call `GetOrLoad` on a cold key and the
   loader increments a counter. Assert the counter is one.
3. Explain why deleting a key before the database commit is unsafe.

## What to remember

TTL bounds staleness, delete-on-write is easy to forget, versioned keys avoid
the race, and coalescing collapses concurrent misses but shares one context and
never caches errors.
