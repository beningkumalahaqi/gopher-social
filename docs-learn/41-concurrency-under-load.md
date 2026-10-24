# 41. Concurrency under load

## Goal

Keep latency predictable when many requests, background jobs, and database
connections compete for the same process.

## The server is already concurrent

`net/http` runs each connection in its own goroutine. A handler that blocks
holds a goroutine, but not a thread. The runtime schedules other goroutines on
the available CPUs. Concurrency is not something you add at the handler level;
it is the default, and the job is to bound it.

Unbounded concurrency looks like:

- one goroutine per incoming request;
- one database query per request with no pool limit;
- one background job per event;
- one retry loop per failure.

Each of these is fine alone. Together they multiply.

## Bound the expensive layer

The database pool is the first bound. `DatabaseMaxConns` limits how many
queries run at once. When the pool is full, requests wait. That wait is visible
as latency, which is better than overwhelming PostgreSQL and making every query
slow.

The second bound is the request timeout. `middleware.Timeout` cancels the
request context after the configured duration. A handler that respects the
context stops waiting for the database and returns. A handler that ignores the
context keeps working after the client is gone, wasting capacity.

The third bound is a semaphore for work that is not covered by the pool, such
as calls to an external compile service. The playground uses a rate limiter per
client. A global semaphore would also cap total concurrent outbound calls:

```go
sem := make(chan struct{}, 8)

select {
case sem <- struct{}{}:
    defer func() { <-sem }()
case <-ctx.Done():
    return ctx.Err()
}
```

Without the `select`, a burst of requests waits forever even after their
contexts are canceled.

## Worker pools need a queue policy

The notification worker pool has a bounded queue. When the queue is full, the
producer must choose:

- block until there is room (backpressure);
- drop the event;
- return an error to the caller;
- spill to durable storage.

Each choice is a product decision. Blocking is correct when the event matters
and the caller can wait. Dropping is correct for low-value notifications.
Returning an error is correct when the caller can retry. Spilling is correct
when the event must survive a restart but cannot block the request.

What you must not do is use an unbounded channel. It turns a slow consumer into
unbounded memory growth, and the process dies far from the actual cause.

## Avoid goroutine leaks

A goroutine leaks when it waits forever on a channel or a lock. Common causes:

- sending to an unbuffered channel with no receiver;
- receiving from a channel that is never closed;
- a `time.Ticker` that is never stopped;
- a context that is never canceled;
- a worker that returns without draining its input.

Every goroutine you start should have a path to exit. Prefer passing a context
and selecting on `ctx.Done()`. In tests, use `goleak` or compare
`runtime.NumGoroutine` before and after a workload.

## Do not share mutable state without a reason

The curriculum and search index are immutable after startup, so they are safe
to share without locks. The cache uses a mutex and atomics. The session store is
backed by PostgreSQL, so there is no in-process map to protect.

If you add an in-memory map keyed by user, you now own its locking and its
eviction. Prefer a store with explicit concurrency semantics. If you must keep a
map, guard every access, including reads, and document the lock order.

## Graceful shutdown is a concurrency feature

The server stops accepting new connections, then waits for in-flight requests
up to `ShutdownTimeout`. Workers stop, the queue drains or is abandoned, and the
database pool closes. The order matters:

1. Stop accepting.
2. Cancel background work.
3. Wait for requests and workers with a deadline.
4. Close the pool.

Closing the pool before requests finish produces errors in the logs and failed
requests during a deploy. Waiting forever produces a process that never exits.
The deadline is the compromise.

## Load tests reveal the shape

Run a test at increasing concurrency and plot latency and throughput. You will
usually see:

- a linear region where more concurrency means more throughput;
- a knee where a shared resource saturates;
- a region where more concurrency increases latency and decreases throughput.

Find the knee. Set timeouts and pool sizes around it, not around the maximum
your laptop can survive. A server that is stable at 70 percent of the knee is
more useful than one that collapses at 100 percent.

Watch goroutine count during the test. A steady increase that does not come
down after the load stops is a leak or a queue that never drains.

## Exercises

1. Add a global semaphore to the playground and test behavior when it is full.
2. Run a load test with a low database pool size and observe pool wait time.
3. Find one goroutine in the codebase and write down its exit condition.

## What to remember

Concurrency is the default, so bound every expensive layer, give every
goroutine an exit, choose a queue policy deliberately, and shut down in the
right order.
