# Solutions 06 — Testing

## 6.1 No leaks

```go
func TestInternalErrorDoesNotLeak(t *testing.T) {
	h := newHandlerWithFailingService(errors.New("pq: relation users does not exist"))
	rec := httptest.NewRecorder()
	h.Create(rec, httptest.NewRequest(http.MethodPost, "/posts", body))
	if strings.Contains(rec.Body.String(), "pq:") {
		t.Fatal("driver error leaked")
	}
}
```

Also assert the body contains only `code` and `message`, and that the log line
contains the request ID so operators can correlate.

## 6.2 Fuzz

Seeds:

```go
f.Add("")
f.Add("not-base64")
f.Add(base64.RawURLEncoding.EncodeToString([]byte("no-separator")))
f.Add(base64.RawURLEncoding.EncodeToString([]byte("2024-01-01T00:00:00Z|not-a-uuid")))
f.Add(validCursor)
```

Run `go test ./internal/pagination -fuzz FuzzCursorDecode -fuzztime 30s`. The
property: never panic, and any returned cursor round-trips.

## 6.3 Table tests

One table, one assertion block. Each case: name, body, want status, want code,
want field. Use `t.Run`. For oversized bodies, generate `strings.Repeat("a", 2<<20)`.
For unknown fields, assert the decoder rejects rather than silently ignoring.

## 6.4 Parallel integration

- Do not truncate in `TestMain`.
- Generate unique emails/usernames per test (`uuid.NewString()`).
- Clean up with `DELETE FROM users WHERE email LIKE 'test-%'` in `t.Cleanup`,
  scoped to the test's prefix.
- Share the pool; do not share rows.
- Run with `-parallel 4 -count=3` until green.

## 6.5 Benchmark

```go
func BenchmarkFeed(b *testing.B) {
	seed(b, 10_000, 100)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = store.Feed(ctx, userID, 20, pagination.Cursor{})
	}
}
```

Run `go test ./internal/posts -bench Feed -benchmem -run '^$'`. Compare with the
index dropped (`DROP INDEX` in a transaction you roll back). Report p50/p95 from
a load test, not just `ns/op`.
