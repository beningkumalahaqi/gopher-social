# Exercises 06 — Testing

## 6.1 500 without leaks

**Task:** Add a handler test that forces a service error and asserts the body is
`internal_error` with no SQL, table name, or stack trace.

**Done when:** The test fails if you change `response.Error` to expose `err.Error()`.

## 6.2 Fuzz the cursor

**Task:** Extend `FuzzCursorDecode` with seeds for valid cursors, huge strings,
and Unicode. Run for 30 seconds.

**Done when:** No panic and no excessive allocation.

## 6.3 Handler table tests

**Task:** Add table-driven tests for `POST /posts` covering: empty content, 501
runes, invalid status, unknown field, oversized body.

**Done when:** Each case asserts status, code, and field name.

## 6.4 Integration isolation

**Task:** Make the integration suite run tests in parallel without truncating
each other. Use unique usernames and clean up by prefix.

**Done when:** `go test ./tests -parallel 4 -count=3` is green.

**Hint:** Truncation is the enemy of parallelism; per-test data is the fix.

## 6.5 Benchmark the feed

**Task:** Add a benchmark that seeds 10k posts and 100 follows, then measures
feed query time. Compare with and without the composite index.

**Done when:** You have numbers, not opinions.

**Hint:** Keep the benchmark out of normal `go test` runs with `-run '^$'`.
