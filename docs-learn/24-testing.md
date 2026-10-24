# 24 — Testing

## Goal

Test behavior at the right level, keep tests fast and deterministic, and use the
race detector where it matters.

## The pyramid in this repo

| Level | Location | What it proves |
| --- | --- | --- |
| Unit | `internal/*/**_test.go` | pure logic: hashing, tokens, pagination, validation |
| Handler | `internal/api/handlers/auth_test.go` | decode, validate, status, DTO, error envelope |
| Middleware | `internal/api/middleware/middleware_test.go` | auth, rate limit, identity |
| Integration | `tests/` | real HTTP + real PostgreSQL + concurrency |
| Fuzz | `internal/pagination` | decoder never panics on arbitrary input |
| Benchmark | `internal/pagination` | encode/decode allocation and speed |

Most tests are unit tests. The integration suite is small but covers the paths
where mocks lie: SQL, constraints, transactions, and races.

## Unit tests

Table-driven:

```go
func TestValidateUsername(t *testing.T) {
    valid := []string{"abc", "alice_99"}
    for _, username := range valid {
        if err := ValidateUsername(username); err != nil { ... }
    }
    invalid := []string{"", "ab", "Alice", "has space"}
    for _, username := range invalid {
        if err := ValidateUsername(username); err == nil { ... }
    }
}
```

No testify. `t.Fatalf` and `t.Errorf` are enough. Subtests (`t.Run`) make failures
readable.

## Handler tests

Fakes implement the handler's consumer interface:

```go
type fakeAuth struct{ session auth.Session; err error }
func (f *fakeAuth) Register(...) (auth.Session, error) { ... }
```

Then call the handler directly with `httptest`. Assert:

- status code
- JSON shape
- service called (or not)
- no secret leaked
- validation details

These tests do not need a database and run in milliseconds (Argon2 is not
involved because the fake skips it).

## Integration tests

`tests/main_test.go`:

```go
func TestMain(m *testing.M) {
    // connect; if unavailable, set a flag and skip
    database.Migrate(url, "drop")
    database.Migrate(url, "up")
    os.Exit(m.Run())
}
```

Each test:

1. Truncates all tables (`TRUNCATE users ... CASCADE`).
2. Builds a real `app.App` with a test config.
3. Starts `httptest.NewServer(app.Handler())`.
4. Starts notification workers.
5. Cleans up server, app, and pool.

Why a real database:

- SQL errors, constraints, and transaction behavior are the bugs you care about.
- Concurrency guarantees (upsert, `FOR UPDATE`) cannot be tested with a mock.
- The test database is separate (`gopherdb_test`), so dev data is safe.

Why skip when unavailable: `go test ./...` should work on a laptop without
Docker. The skip is explicit and prints the reason. CI must have PostgreSQL, or
the suite is silently weaker — add a CI check that fails if the tests skip.

## Test data

Use unique usernames per test or truncate. This suite truncates, which is simple
but serializes tests within a package. For parallel tests, use unique prefixes
and avoid truncation, or run each test in a transaction (harder with HTTP).

## Race detector

```bash
go test -race ./...
```

Run it before merging anything that touches workers, shared maps, or goroutines.
The notification service uses a mutex and a WaitGroup; the race detector is the
cheap way to validate the locking.

## Fuzzing

```go
func FuzzCursorDecode(f *testing.F) {
    f.Add("")
    f.Add("not-base64")
    f.Fuzz(func(t *testing.T, raw string) {
        _, _ = pagination.Decode(raw) // must not panic
    })
}
```

Fuzzing is good for parsers: cursors, JSON, headers. It finds panics, not logic
errors.

## Benchmarks

```bash
go test ./internal/pagination -run '^$' -bench . -benchmem
```

Look at `ns/op` and `allocs/op`. A cursor encode should not allocate a dozen
times; if it does, check string concatenation and base64 usage.

## Flaky tests

Common causes:

- sleeping instead of polling (notification workers),
- shared database state,
- time-dependent assertions,
- port conflicts.

The notification assertions poll for up to 5 seconds. That is acceptable but
slow; a test hook that signals completion would be faster and deterministic.
Never use a fixed `time.Sleep` as the only synchronization.

## What not to test

- Standard library behavior (`json.Marshal`).
- Every getter.
- Exact log strings (brittle). Test behavior and structured fields if needed.
- Implementation details that make refactoring painful.

## Exercise

1. Add a test that a 500 response does not leak SQL. How do you force a 500?
2. Add a fuzz test for `params.UUID`.
3. Write a benchmark for `Cursor.Encode` and improve allocations.
4. Make the integration suite fail (not skip) when `CI=true` and the DB is
   missing.
5. Add a test for concurrent password changes. What should happen?

Next: [25 — Production checklist](25-production-checklist.md)
