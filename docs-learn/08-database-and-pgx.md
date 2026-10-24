# 08 — Database and pgx

## Goal

Use PostgreSQL from Go with a bounded pool, context-aware calls, and a small
abstraction that supports transactions.

## Why pgx

`pgx` is a PostgreSQL driver and toolkit. It speaks the binary protocol, supports
`COPY`, `LISTEN/NOTIFY`, and rich types, and has a connection pool (`pgxpool`).
It is faster and more capable than `database/sql` for PostgreSQL-specific work.
The trade-off is portability: this code will not run on MySQL without changes.

## Opening the pool

```go
cfg, err := pgxpool.ParseConfig(url)
cfg.MaxConns = maxConns
cfg.MinConns = 1
cfg.MaxConnIdleTime = 5 * time.Minute
cfg.MaxConnLifetime = 30 * time.Minute
cfg.HealthCheckPeriod = 30 * time.Second

pool, err := pgxpool.NewWithConfig(ctx, cfg)
// ping with a 5s timeout, close and fail if unreachable
```

Sizing rule of thumb: `MaxConns` per instance × number of instances must stay
below PostgreSQL's `max_connections` minus room for migrations and admin. More
connections is not faster once the database is saturated; it increases context
switching and lock contention.

`MinConns = 1` avoids a cold start on the first request. `MaxConnLifetime`
recycles connections so DNS/credential changes eventually take effect.

## Context is mandatory

Every query takes a context:

```go
row := db.QueryRow(ctx, `SELECT id FROM users WHERE username = $1`, username)
```

The request context carries the deadline from `Timeout` middleware and is
canceled if the client disconnects. Never use `context.Background()` in a store.
Never store a context in a struct.

## Scanning

```go
var u User
err := row.Scan(&u.ID, &u.Username, &u.Email, &u.PasswordHash, ...)
if errors.Is(err, pgx.ErrNoRows) {
    return User{}, apperr.Wrap(apperr.ErrNotFound, "user %s", username)
}
if err != nil {
    return User{}, fmt.Errorf("scan user: %w", err)
}
```

Rules:

- Check `pgx.ErrNoRows` explicitly; it is not a generic failure.
- Scan nullable columns into `*string`, `*time.Time`, or `pgtype.*`. Do not scan
  `NULL` into `string`.
- Select columns explicitly. `SELECT *` breaks when a migration adds a column
  and makes it easy to leak new data.
- Use `rows.Err()` after iterating. A loop that ends early must `rows.Close()`.

## The `Querier` abstraction

```go
type Querier interface {
    Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
    Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
    QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}
```

`*pgxpool.Pool` and `pgx.Tx` both implement it. A store holds a `Querier`, so the
same method can run inside or outside a transaction. This is the smallest
interface that covers the code; it is not a general-purpose database layer.

Do not add `Begin` to `Querier`: a transaction cannot start another transaction,
and services should decide transaction boundaries, not stores.

## Transactions

See `internal/auth/service.go`:

```go
tx, err := s.db.Begin(ctx)
defer tx.Rollback(ctx) // no-op after Commit

user, err := s.users.CreateTx(ctx, tx, ...)
token, err := s.refresh.CreateTx(ctx, tx, ...)
return tx.Commit(ctx)
```

`Rollback` after a successful `Commit` returns `pgx.ErrTxClosed`; ignore it.
Always roll back on every error path, including panics (defer).

## Driver errors

Translate at the store boundary:

```go
if database.IsUniqueViolation(err) {
    return User{}, apperr.Wrap(apperr.ErrConflict, "username or email already exists")
}
if database.IsForeignKeyViolation(err) {
    return Post{}, apperr.Wrap(apperr.ErrNotFound, "author or post no longer exists")
}
```

Do not expose `pgconn.PgError` to handlers. Do log the constraint name at debug
level if you need to distinguish which unique index fired; the client should not
know.

## Connection lifecycle

- `pool.Close()` waits for borrowed connections to return. Call it after the
  HTTP server stops and workers drain.
- A leaked `rows` keeps a connection checked out until the pool's lifetime
  limit. Close rows.
- Long transactions hold locks and connections. Keep them short; never call an
  external service inside one.

## Exercise

1. Add `MaxConnIdleTime` and `MaxConnLifetime` to config instead of hard-coding.
2. Write a query that returns `NULL` for `bio` and scan it correctly.
3. Find a place that ignores `rows.Err()` and fix it.
4. Explain why `Querier` omits `Begin`.
5. Set `MaxConns = 1` and run the integration suite with `-race`. Which test
   exposes the bottleneck first?

Next: [09 — Migrations](09-migrations.md)
