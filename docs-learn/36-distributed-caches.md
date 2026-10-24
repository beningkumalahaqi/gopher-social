# 36. Distributed caches and their costs

## Goal

Decide when a remote cache is justified, and understand what changes when the
cache is a network service instead of a map in your process.

## What changes when the cache is remote

An in-process cache has no network, no serialization, and no independent
failure domain. A remote cache adds all three:

- Every read is a network round trip unless the client has its own local tier.
- Every value must be encoded and decoded.
- The cache can be unavailable while the database is healthy, or healthy while
  the database is unavailable.

The last point is subtle. A cache is not a backup. If the database is down, a
cache hit may still serve a page, but writes fail and the user sees an
inconsistent system. Do not let a cache mask a database outage during
debugging.

## The case for Redis or a similar service

A remote cache makes sense when:

1. Multiple processes must see the same cached value.
2. The value is expensive to compute and larger than a process should hold.
3. You need a shared TTL, atomic counters, or pub/sub invalidation.
4. The working set does not fit comfortably in each process's memory.

The platform has one process and a small corpus. A remote cache would add a
service to operate, secure, monitor, and back up, to avoid work that is already
sub-millisecond. That is a bad trade. Do not add Redis because a tutorial did.

## The case against, in detail

### Serialization is part of the contract

`encoding/json` is convenient but it loses types: integers become floats unless
you use a decoder with `UseNumber`, times become strings, and nil versus zero
can be ambiguous. `gob` is fast but Go-specific and version-sensitive. Protobuf
or MessagePack reduce size at the cost of a schema step.

Whatever you choose, version the payload. A deploy that changes a struct must
not read old values as if they were new. Include a schema version in the key or
the envelope, and discard incompatible entries.

### The network can fail in the middle

A read that times out may have succeeded on the server. A write that times out
may have applied. Retrying a non-idempotent operation can duplicate it. Set
timeouts on connect, read, and write separately, and decide whether a cache
error is fatal or a fallthrough to the source.

For a cache-aside read, falling through to the database is usually right. For a
write that updates the cache, falling through can leave the cache stale. Prefer
delete-on-write with a TTL backstop over a read-modify-write on a remote value.

### Hot keys and skew

One key can receive a large share of traffic. A single Redis node serves that
key from one thread, and the network link to that node becomes the bottleneck.
Client-side caching, key sharding, or a local tier reduces the pressure.

Sharding a key changes the key space and complicates invalidation. Measure the
distribution before sharding. A hot key is a design signal: either the value
should be local, or the key is too coarse and should be split.

### Operational cost

A remote cache needs:

- authentication and network policy;
- memory limits and eviction policy;
- persistence decisions (yes, some caches need it);
- monitoring for latency, hit ratio, evictions, and memory;
- a plan for failover and for losing the whole cache.

Losing the cache should be a performance event, not a correctness event. If the
system cannot survive an empty cache, the cache is holding primary data.

## A safe adoption path

If the in-process cache is no longer enough:

1. Add a local tier first. It removes most network reads and is easy to reason
   about.
2. Measure the remaining miss rate and the cost of a miss.
3. Add the remote tier only for keys that are shared and expensive.
4. Keep the same key versioning and invalidation rules across tiers.
5. Test with the remote cache stopped. Requests should slow down, not fail.

A two-tier cache has a new failure: the local tier can serve a value that the
remote tier has already invalidated. Bound the local TTL to the maximum
staleness you accept, and never let it exceed the remote TTL without a
versioned key.

## Consistency models in plain terms

- **Read-your-writes**: a user sees their own update immediately. Requires
  invalidating or versioning the key on the write path.
- **Monotonic reads**: a user never sees an older value after a newer one.
  Requires session affinity or a version check.
- **Eventual consistency**: all readers converge within a bounded time. A TTL
  cache is eventually consistent by construction.

Choose the weakest model the feature can tolerate, and write it down. "It is
probably fine" is not a consistency model.

## Exercises

1. List the failure modes a remote cache adds that an in-process cache does
   not. For each, state whether the request should fail or fall through.
2. Design a key schema for a shared course catalogue with a version and an
   environment prefix.
3. Explain why losing the cache must not lose data.

## What to remember

A remote cache buys shared state at the cost of a network, serialization, and a
new failure domain. Add it when multiple processes or a large working set
justify it, version the payload, bound local staleness, and make an empty cache
survivable.
