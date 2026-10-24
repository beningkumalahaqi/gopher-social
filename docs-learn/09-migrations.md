# 09 — Migrations

## Goal

Change the database schema repeatably, in any environment, without manual SQL.

## Embedded migrations

`migrations/embed.go`:

```go
//go:embed *.sql
var FS embed.FS
```

The SQL files ship inside the binary. `cmd/migrate` runs them with
`golang-migrate`'s `iofs` source driver, so a deploy needs no separate migration
files on disk.

Files are numbered and paired:

```text
000001_users.up.sql
000001_users.down.sql
000002_refresh_tokens.up.sql
000002_refresh_tokens.down.sql
...
000008_search_indexes.up.sql
000008_search_indexes.down.sql
```

`up` applies in order; `down` reverses one migration at a time; `drop` reverses all of them. Never edit an applied
migration in a shared environment. Add a new one.

## Commands

```bash
go run ./cmd/migrate -dir=up
go run ./cmd/migrate -dir=down   # one step
go run ./cmd/migrate -dir=version
go run ./cmd/migrate -dir=drop
```

`drop` removes everything. The integration `TestMain` uses it to start from a
clean schema on `gopherdb_test`. Never point `drop` at production; consider a
guard that refuses when `APP_ENV=production`.

## What the migrations create

1. `users` with `citext` username/email, timestamps, constraints.
2. `refresh_tokens` with hash, family, expiry, revocation.
3. `posts` with status check and `published_at`.
4. `follows` with composite PK and self-follow check.
5. `comments`.
6. `reactions` with composite PK.
7. `notifications`.
8. Full-text search indexes.

Read them in order. Notice:

- `CREATE EXTENSION IF NOT EXISTS citext` at the top.
- `CHECK` constraints encode invariants close to the data.
- `ON DELETE CASCADE` where child rows have no meaning without the parent.
- Indexes are created for the actual query patterns (`follows.follower_id`,
  `posts(author_id, created_at DESC)`, etc.).
- `tsvector` columns are generated, so they cannot drift from `content`.

## Safe schema changes

Additive changes are easy:

```sql
ALTER TABLE users ADD COLUMN website TEXT NOT NULL DEFAULT '';
```

Rewrites and locks are not:

- Adding a column with a volatile default rewrites the table on older Postgres.
  Use a constant default or add nullable, backfill, then set `NOT NULL`.
- `ALTER COLUMN TYPE` rewrites and locks. Plan a shadow column for large tables.
- Creating an index locks writes unless `CONCURRENTLY`. `CONCURRENTLY` cannot run
  inside a transaction; golang-migrate wraps each migration in one by default,
  so use a separate migration with the driver's no-transaction mode or run it
  manually.
- Dropping a column breaks old app versions during a rolling deploy. Expand →
  migrate data → contract in separate releases.

## Down migrations

They exist for development and tests, not as a production rollback strategy.
A down migration that drops a column loses data. Prefer forward fixes and
backups. Still write them, and test them: `make migrate-down && make migrate-up`
should leave a clean schema.

## Migration state

`golang-migrate` stores the version in `schema_migrations`. If a migration
fails halfway, the version is marked dirty. Fix the SQL or the database, then
force the version:

```bash
go run ./cmd/migrate -dir=force -version=7
```

`cmd/migrate` exposes this. Use it only after understanding what is half-applied.

## Test database

`tests/main_test.go`:

```go
database.Migrate(testDBURL, "drop")
database.Migrate(testDBURL, "up")
```

This runs once per package. Tests truncate tables between cases. That is slower
than a transaction-per-test but catches real commit behavior and constraint
enforcement.

## Exercise

1. Add a `website` column to users with a migration pair, update the domain
   model, store scans, DTO, and profile update. Run down/up.
2. Write a migration that adds an index on `posts(status, created_at DESC)`.
   Check `EXPLAIN` before and after.
3. Explain why `CREATE INDEX CONCURRENTLY` needs special handling with
   golang-migrate.
4. Make `cmd/migrate` refuse `-dir=drop` when `APP_ENV=production`.
5. Break a migration intentionally, observe the dirty state, then recover with
   `force`.

Next: [10 — Stores and transactions](10-stores-and-transactions.md)
