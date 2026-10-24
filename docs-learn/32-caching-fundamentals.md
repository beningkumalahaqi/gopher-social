# 32. Caching fundamentals

## Goal

Learn the vocabulary and the trade-offs before touching the cache package.
Most caching failures are design failures, not implementation bugs.

## What a cache buys

A cache answers a question without repeating the work that produced the
answer. The work might be a SQL query, a template render, a Markdown parse, or a
network call to another service. The cache stores the result and a rule for how
long it is valid.

The benefit is lower latency and less load on the slow layer. The cost is
memory, complexity, and the risk of serving something that is no longer true.
There is no cache without that risk. The question is whether the risk is
bounded and observable.

## Hit ratio

For a window of requests, define:

- **hits**: requests served from the cache;
- **misses**: requests that had to do the work;
- **hit ratio**: hits divided by hits plus misses.

Hit ratio alone is not a goal. A cache with a 99 percent hit ratio that serves
stale prices is worse than no cache. Track the ratio together with staleness
and the cost of a miss.

Also track **evictions** and **expired removals**. A cache at its size bound
with a high eviction rate is thrashing: it spends work inserting entries that
are removed before they are reused. The fix is a larger bound, a shorter key
space, or no cache.

## Where the platform caches

The learning application uses caching in three places, each with a different
reason:

1. **Static assets** are content-hashed and served with a long immutable cache.
   The URL changes when the bytes change, so staleness is impossible.
2. **The curriculum** is parsed once at startup and kept in memory. It is
   immutable for the life of the process, so no TTL is needed.
3. **Search results** are held briefly in a bounded TTL cache. The key includes
   the query, filters, and the profile's progress fingerprint.

Notice that the first two are not "cache invalidation" problems at all. Their
lifetime is tied to a deploy or a build. The third is a real TTL cache and
needs the rest of this course.

## Cache-aside

The common pattern is cache-aside:

```go
value, ok := cache.Get(key)
if !ok {
    value, err = load(ctx, key)
    if err != nil {
        return err
    }
    cache.Set(key, value)
}
return value
```

The application owns the read path. The cache is passive. This is easy to
reason about and works with any store. It also has three classic failure modes:

- **Stampede**: many callers miss the same key at once and all run `load`.
- **Penetration**: repeated requests for a key that does not exist always miss.
- **Avalanche**: many keys expire at the same moment after a deploy or a bulk
  load.

`GetOrLoad` in `internal/learn/cache` addresses the stampede with
`singleflight`. Penetration is handled by not caching misses here, because the
search index is in memory and a miss is cheap. Avalanche is bounded by the
small corpus and by jitter if TTLs are ever randomized. Do not add machinery
for a failure you cannot observe.

## TTL is a staleness budget

A TTL is not a performance knob. It is the maximum time a user can see data
that has already changed. Choose it from the business requirement:

- A search result can be a few seconds stale without anyone noticing.
- A profile's display name should change immediately after an edit.
- A lesson body should change on deploy, not on a timer.

Write the budget down. "Five minutes" is not a decision until you can say what
may be five minutes old. If nothing may be stale, the cache must be invalidated
on write, or it must not exist.

## Invalidation is the hard part

There are three honest strategies:

1. **Time-based**: expire after a TTL. Simple, bounded staleness, no write-path
   work.
2. **Write-through or write-behind**: update or delete the cache when the
   source changes. Correct only if every writer participates.
3. **Versioned keys**: include a version or content hash in the key. Old
   entries become unreachable and expire naturally.

The failure mode to avoid is a write path that updates the database and forgets
one cache key. That produces data that is correct in one view and wrong in
another, which is harder to debug than a consistently stale view.

For this app, progress and notes are never cached. They are small, indexed
reads, and correctness matters more than the microseconds. Search results are
cached with progress in the key, so a status change produces a new key rather
than an invalidation.

## Measure the hit path

A cache that is not instrumented is a rumor. The cache package exposes:

```go
type Stats struct {
    Hits, Misses, Expired, Evictions uint64
    Entries                          int
}
```

Log or export these counters. If hits are near zero, the cache is costing
memory and adding a failure mode for nothing. If evictions are high, the bound
is too small for the working set. If expired is high relative to hits, the TTL
is shorter than the reuse interval.

## When not to cache

Do not cache:

- data that must be strongly consistent for the operation to be correct;
- per-request values such as CSRF tokens or session lookups that are already
  fast;
- results whose key you cannot enumerate completely;
- anything you cannot invalidate or wait out.

The default should be no cache. Add one when a measurement shows repeated work
with the same inputs and a staleness budget you can state.

## Exercises

1. Write the hit ratio formula and compute it for 80 hits and 20 misses.
2. For each of the three caches in the platform, state its lifetime and its
   invalidation rule.
3. Name one endpoint you would not cache and explain the correctness reason.

## What to remember

A cache trades correctness risk for speed. Know the hit ratio, the staleness
budget, and the invalidation rule before you add one. Instrument it, or you are
guessing.
