# 22 — Background workers

## Goal

Move slow, non-critical work off the request path without unbounded queues,
lost shutdown work, or panics on closed channels.

## Why notifications are asynchronous

Creating a comment should not wait for a notification insert. The comment is the
user's action; the notification is a side effect. Enqueueing is O(1) and the
request returns immediately.

The trade-off is delivery semantics: best-effort, at-most-once. If the process
crashes after the comment commits but before the worker writes, the notification
is lost. A production system would use a transactional outbox.

## The queue

```go
type Service struct {
    jobs    chan job
    workers int
    mu      sync.RWMutex
    closed  bool
    started bool
    wg      sync.WaitGroup
}
```

- `jobs` is buffered (`NOTIFICATION_QUEUE_SIZE`, default 256).
- `workers` goroutines range over the channel.
- `enqueue` is non-blocking:

```go
select {
case s.jobs <- j:
default:
    s.logger.Warn("notification queue full; dropping job", ...)
}
```

Why drop instead of block:

- Blocking the handler makes user latency depend on the notification database.
- An unbounded queue trades a latency problem for a memory problem.
- Notifications are not worth backpressuring a comment request.

If you need stronger guarantees, use an outbox table and a poller. The HTTP
request then inserts an outbox row in the same transaction as the comment, and a
worker delivers with retries.

## Worker loop

```go
for j := range s.jobs {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    err := s.store.Insert(ctx, ...)
    cancel()
    if err != nil { log }
}
```

Each job gets a fresh context. The request context is canceled as soon as the
response is written; reusing it would make most jobs fail with
`context.Canceled`.

The job timeout bounds a stuck database call. Without it, one bad job can block a
worker forever.

## Shutdown

```go
s.mu.Lock()
if s.closed { unlock; return nil }
s.closed = true
close(s.jobs)
s.mu.Unlock()

done := make(chan struct{})
go func() { s.wg.Wait(); close(done) }()
select {
case <-done: return nil
case <-ctx.Done(): return ctx.Err()
}
```

Properties:

- `Stop` is idempotent.
- Closing the channel makes workers drain remaining jobs and exit.
- The caller's context bounds the wait, so shutdown cannot hang forever.
- `Enqueue` takes a read lock and checks `closed`; no send on a closed channel.
- `Start` after `Stop` is a no-op.

This is the minimum correct lifecycle. Test it: `TestStopIsIdempotent`,
`TestStopWithoutStart`, `TestEnqueueAfterStopIsSafe`.

## Idempotency

Workers may retry (not implemented here). Make inserts idempotent with a unique
key, for example `(user_id, actor_id, type, post_id, comment_id)`, or an explicit
`event_id`. Without it, a retry creates duplicate notifications.

## Ordering

Multiple workers process jobs concurrently, so notification order is not
guaranteed. If order matters, use one worker per recipient partition or sort by
`created_at` when reading. The inbox sorts by `created_at DESC`, which is enough
for display.

## Observability

Log:

- queue full (warn) with type and capacity,
- job failure (warn) with type and error,
- worker start/stop (info).

Consider metrics: queue depth, jobs processed, jobs dropped, job duration. A
silent drop policy is dangerous without a counter.

## Exercise

1. Replace the drop policy with an outbox table. What is the transaction
   boundary?
2. Add a retry with exponential backoff and a max attempt count.
3. Add a `Dropped()` counter and expose it on `/readyz`? Why not on a public
   endpoint?
4. Make `Stop` drain with a deadline but keep accepting until the deadline.
   What race appears?
5. Send 10,000 jobs to a queue of 10 and observe how many are dropped. Is the
   log volume acceptable?

Next: [23 — Shutdown and health](23-shutdown-and-health.md)
