# Exercises 05 — Concurrency

## 5.1 Worker pool with retries

**Task:** Extend the notification worker with 3 attempts and exponential backoff
(10ms, 50ms, 250ms). Add a `dead_letter` table for final failures.

**Done when:** A poison job does not block the queue and ends in the dead-letter
table.

**Hint:** Keep the retry loop inside the worker; do not re-enqueue into the same
channel or you can deadlock on a full queue.

## 5.2 Graceful drain test

**Task:** Write a test that enqueues 100 notifications, calls `Stop`, and asserts
all 100 rows exist. Repeat with a 1ms context and assert `Stop` returns an error.

**Done when:** The test is deterministic (no arbitrary sleeps).

**Hint:** A `sync.WaitGroup` or a test-only completion channel helps.

## 5.3 Duplicate reactions

**Task:** Remove the upsert and use `SELECT` then `INSERT`. Run the concurrency
test 100 times. Observe unique violations. Restore the upsert.

**Done when:** You can explain why the upsert is the correct primitive.

## 5.4 Context cancellation

**Task:** Add a test that cancels a request context mid-query and asserts the
handler returns 408/499-like behavior without leaking a connection.

**Done when:** `pgxpool.Stat().AcquiredConns()` returns to baseline.

**Hint:** Use a query with `pg_sleep`.

## 5.5 Fan-in with order

**Task:** Process notifications with 4 workers but preserve per-recipient order.
Design it with a per-user queue or partitioning.

**Done when:** Two notifications for the same user never arrive out of order.

**Hint:** Global order is expensive; per-key order is usually enough.
