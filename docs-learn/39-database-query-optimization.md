# 39. Database query optimization

## Goal

Make the database do less work per request. Most slow pages are not slow
because Go is slow; they are slow because the handler runs too many queries or
the database scans too many rows.

## Count queries before tuning them

A lesson page needs:

1. the profile and its progress rows;
2. the lesson body (already in memory);
3. the note for this lesson;
4. the quiz and exercise summaries for the sidebar or next step.

`requestState` loads the profile activity once, then the handler reads from
memory. That is the right shape: one round trip for the learner's state, not
one query per card.

Add a counter in development and log queries per request. A page with forty
queries is an N+1 bug even if each query is fast. Fix the shape before adding
indexes.

## Read the plan

`EXPLAIN (ANALYZE, BUFFERS)` shows what the planner did and what it cost:

```sql
EXPLAIN (ANALYZE, BUFFERS)
SELECT lesson_slug, status, reading_percent, first_opened_at, last_opened_at, completed_at
FROM learn_lesson_progress
WHERE profile_id = $1
ORDER BY last_opened_at DESC;
```

Look for:

- **Seq Scan** on a large table: the planner could not use an index, or judged
  a scan cheaper. Check the row estimate and the filter.
- **Rows Removed by Filter**: the index returned rows that the filter threw
  away. A more selective index may help.
- **Sort** with an external merge: the ordering is not provided by an index.
- **Nested Loop** with a high loop count: a join is running far more times than
  expected.
- **Buffers: shared read** much larger than the rows returned: the query pages
  in more data than it needs.

The plan is data-dependent. Run it on a database with realistic row counts, not
an empty development database. A sequential scan of twenty rows is not a
problem, and forcing an index there is worse.

## The index follows the query

The progress list filters by `profile_id` and sorts by `last_opened_at DESC`:

```sql
CREATE INDEX learn_progress_recent_idx
    ON learn_lesson_progress (profile_id, last_opened_at DESC);
```

Column order matters. The equality column comes first, then the sort column.
With this index, PostgreSQL can find the profile's rows in the requested order
and stop after the limit. Reverse the order and the index is much less useful
for this query.

The primary key `(profile_id, lesson_slug)` already serves lookups by both
columns. The recent index serves the list. Do not add an index on
`lesson_slug` alone unless a query filters by lesson across profiles, which
this app does not.

Every index costs writes and storage. A table with five indexes is slower to
insert into than a table with one. Add an index because a query needs it, then
verify with the plan that it is used.

## Keyset pagination

`OFFSET` gets slower as the offset grows, because the database must produce and
discard the skipped rows. Keyset pagination remembers the last row:

```sql
SELECT id, created_at
FROM posts
WHERE (created_at, id) < ($1, $2)
ORDER BY created_at DESC, id DESC
LIMIT $3;
```

The tuple comparison matches a composite index. It stays fast at any depth and
does not skip or duplicate rows when new data arrives between pages. The
trade-off is that you cannot jump to page 50, which most feeds do not need.

The API already uses cursor pagination for feeds and explore. The cursor
encodes the last seen sort key, not an offset. Keep it opaque and validate it;
a client that sends a malformed cursor should get a 400, not a database error.

## Batch and join instead of looping

Bad:

```go
for _, id := range ids {
    row := pool.QueryRow(ctx, `SELECT title FROM posts WHERE id = $1`, id)
    // ...
}
```

Good:

```go
rows, err := pool.Query(ctx, `SELECT id, title FROM posts WHERE id = ANY($1)`, ids)
```

One round trip, one plan, one result set. The same applies to inserts: use
`COPY` or a multi-row insert for bulk work, not a loop of single inserts.

For counts, ask whether the UI needs an exact number. `count(*)` on a large
table scans. An approximate count or a maintained counter is often enough for
"12k posts". Do not optimize this until the page is actually slow.

## The connection pool is a queue

`DatabaseMaxConns` bounds concurrent database work. If every request waits for
a connection, the p99 rises even though each query is fast. Signs:

- pool wait time is a large share of request latency;
- active connections are at the maximum;
- the database CPU is not saturated.

Raising the pool size helps only until the database or its CPU becomes the
limit. PostgreSQL serves each connection with a backend process; hundreds of
connections are not free. For serverless, use a pooler in front of the database
and keep the per-instance pool small.

Also set a query timeout. A query without a deadline can hold a connection
until the client gives up. `middleware.Timeout` bounds the request, but the
database should have its own context deadline so the server can cancel the
query and release the connection.

## Transactions should be short

A transaction holds locks and a connection. Do not call an external service,
render a template, or wait on a channel inside it. Do the minimum writes,
commit, then do the slow work.

`MergeGuest` is a good example: it copies rows and deletes the guest profile in
one transaction, then returns. It does not send a notification inside the
transaction. If a notification fails, the merge is still correct.

## Verify with a test

For a query that matters, add a test that:

1. seeds a realistic number of rows;
2. runs the query;
3. asserts the result;
4. optionally asserts the plan uses the expected index.

The plan assertion is brittle across PostgreSQL versions. Prefer measuring the
query duration and row count in a benchmark, and keep the plan review manual.

## Exercises

1. Run `EXPLAIN (ANALYZE, BUFFERS)` on the progress list with and without the
   recent index. Compare the plan, not just the time.
2. Rewrite a loop of queries in one handler as a single query with `ANY`.
3. Add a cursor-paginated query and prove it does not skip rows when a new row
   is inserted between pages.

## What to remember

Count queries, read the plan on realistic data, index for the filter and sort,
paginate by keyset, batch round trips, bound the pool, and keep transactions
short.
