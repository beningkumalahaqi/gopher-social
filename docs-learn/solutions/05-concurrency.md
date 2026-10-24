# Solutions 05 — Concurrency

## 5.1 Retries

```go
func (w *Worker) handle(ctx context.Context, n Notification) {
	backoff := []time.Duration{10 * time.Millisecond, 50 * time.Millisecond, 250 * time.Millisecond}
	for attempt := 0; ; attempt++ {
		err := w.store.Insert(ctx, n)
		if err == nil {
			return
		}
		if attempt == len(backoff) {
			_ = w.store.DeadLetter(ctx, n, err)
			return
		}
		select {
		case <-time.After(backoff[attempt]):
		case <-ctx.Done():
			return
		}
	}
}
```

Do not re-enqueue into the same bounded channel: if the queue is full, the worker
blocks and can no longer drain it. Dead-letter rows are inspected out of band.

## 5.2 Drain

`Stop` closes the queue, waits for workers, then returns. Enqueue 100, call Stop,
assert count 100. For the timeout case, pass a context with a 1ms deadline to the
inserts and assert `Stop` returns `context.DeadlineExceeded` (or a wrapped
version) and that the process does not hang.

Determinism: use a `sync.WaitGroup` for enqueues and a completion channel for
workers. No `time.Sleep` in assertions.

## 5.3 Duplicate reactions

Read-then-write has a race: two requests both see no row, both insert, one gets
`23505`. The upsert makes the operation atomic in one statement and is the right
primitive. If you need to know whether it was an insert, use
`RETURNING (xmax = 0)`.

## 5.4 Cancellation

`pgx` cancels the query when the context is done. After the handler returns,
`pool.Stat().AcquiredConns()` should return to baseline. If it does not, a
connection leaked (missing `rows.Close()` or a transaction not rolled back).

## 5.5 Per-user order

Hash the recipient ID to a worker, or keep a map of per-user channels each served
by one goroutine. Global order requires a single consumer and kills throughput.
Per-key order is usually the actual requirement.
