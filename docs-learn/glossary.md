# Glossary

Terms as they are used in this repository. Definitions are practical, not
exhaustive.

## A

**Access token** — Short-lived credential sent as `Authorization: Bearer ...`.
Here a 15-minute HS256 JWT. Stateless: the server verifies the signature and
does not look up a session.

**Argon2id** — Memory-hard password hashing function. Winner of the Password
Hashing Competition. Parameters here: 64 MiB, 3 iterations, 2 threads.

**Audience (`aud`)** — JWT claim naming the intended recipient. This API uses
the issuer value and verifies it, so a token from another service is rejected.

**Authentication** — Proving who you are. 401 when it fails.

**Authorization** — Deciding what you may do. 403 when it fails.

## B

**Backpressure** — What a system does when work arrives faster than it can be
processed. Options: block, drop, or reject. The notification queue drops.

**Best-effort delivery** — Work that may be lost under failure. Notifications
are best-effort; a transactional outbox would make them at-least-once.

**Bounded queue** — A channel with fixed capacity. Prevents unbounded memory
growth when producers outrun consumers.

## C

**Cascade** — `ON DELETE CASCADE` removes child rows when the parent is deleted.
Used for comments/reactions/notifications tied to posts.

**`citext`** — Case-insensitive text type. Used for usernames and emails so
`Alice` and `alice` collide on the unique index.

**Claims** — The JSON payload of a JWT (`sub`, `sid`, `iss`, `aud`, `exp`, ...).

**Composite primary key** — A primary key over multiple columns. `follows` uses
`(follower_id, following_id)`; `reactions` uses `(post_id, user_id)`.

**Connection pool** — Reusable set of database connections. `pgxpool` bounds the
number and recycles idle/old connections.

**Context** — `context.Context` carries cancellation, deadlines, and
request-scoped values. Every database call and HTTP handler uses one.

**Cursor (keyset) pagination** — Pagination by "after this row" instead of
offset. Stable under inserts; cannot jump to page N.

## D

**Deadline** — A time by which work must finish. Propagated via context from the
request timeout.

**DTO (Data Transfer Object)** — A struct that defines an HTTP JSON shape. Lives
in `internal/api/dto`, separate from domain models.

**Dirty migration** — A migration that started but failed. `golang-migrate`
records it; `force` resets the version after manual repair.

## E

**Envelope** — The outer JSON object: `{"data": ...}` or `{"error": ...}`.

**Error sentinel** — A package-level error value (`apperr.ErrNotFound`) used with
`errors.Is` to classify failures without string matching.

**`EXPLAIN`** — PostgreSQL command showing a query plan. Use
`EXPLAIN (ANALYZE, BUFFERS)` to see actual time and I/O.

## F

**Fan-out on read** — Compute a feed at request time by joining follows and
posts. This repo's approach.

**Fan-out on write** — Precompute each follower's timeline when a post is
created. Faster reads, expensive writes, celebrity problem.

**Foreign key** — Database constraint ensuring a referenced row exists.
Violations are mapped to 404/400 at the store boundary.

**Full-text search (FTS)** — PostgreSQL `tsvector`/`tsquery` matching with
stemming, stop words, and GIN indexes.

## G

**GIN index** — Inverted index for composite values such as `tsvector` and JSONB.
Used for search.

**`golang-migrate`** — Migration library. Here used as a library with an embedded
`iofs` source and the pgx driver.

**Graceful shutdown** — Stop accepting requests, finish in-flight work, drain
workers, close resources, exit.

## H

**Handler** — `func(http.ResponseWriter, *http.Request)`. In this repo, a thin
transport adapter.

**Health check** — Liveness (`/healthz`) says the process is alive; readiness
(`/readyz`) says it can serve traffic (database reachable).

**HS256** — HMAC-SHA256 JWT signing. Symmetric: one secret signs and verifies.

## I

**Idempotent** — Repeating the request has the same effect. `PUT` reaction,
follow, unfollow, logout, and delete are idempotent here.

**Index** — Database structure that speeds reads at the cost of writes/storage.
Composite indexes match filter + sort order.

**`internal/`** — Go directory whose packages can only be imported by code in the
same module.

## J

**JWT (JSON Web Token)** — Signed, base64url-encoded header.payload.signature.
Not encrypted; do not put secrets in claims.

**`jti`** — JWT ID claim, a unique token identifier. Present but not used for
revocation here.

## K

**Keyset pagination** — See cursor pagination.

## L

**Leeway** — Clock-skew tolerance when validating `exp`/`nbf`. Here 30 seconds.

**Liveness** — See health check.

## M

**Middleware** — A function wrapping an `http.Handler` to add cross-cutting
behavior (logging, auth, timeouts).

**Migration** — Versioned SQL change with `up` and `down` files, applied in order.

**`mTLS`** — Mutual TLS. Not used here; common for service-to-service auth.

## N

**N+1 query** — Loading N rows then issuing one query per row. Avoided with joins
and aggregates.

**Notification family / chain** — The sequence of refresh tokens linked by
`replaced_by`. This repo revokes all user sessions on reuse rather than only the
chain.

## O

**Offset pagination** — `LIMIT/OFFSET`. Simple, jumpable, unstable under
concurrent inserts.

**Opaque token** — A random string with no embedded claims. Refresh tokens are
opaque; only their SHA-256 hash is stored.

**Outbox pattern** — Write an event row in the same transaction as the business
change, then deliver it asynchronously. Guarantees no lost side effects.

## P

**PHC string** — Password hash format: `$argon2id$v=19$m=...$salt$hash`.

**`pgx`** — PostgreSQL driver/toolkit for Go. `pgxpool` is its connection pool.

**PII** — Personally identifiable information. Never log emails, tokens, or
passwords.

**Prepared statement** — Server-side parsed query reused with parameters. pgx
uses the extended protocol by default.

## Q

**`Querier`** — The three-method interface (`Exec`, `Query`, `QueryRow`) shared by
`*pgxpool.Pool` and `pgx.Tx`.

**Query parameterization** — Passing values separately from SQL (`$1`) to prevent
injection.

## R

**Race condition** — Concurrent operations producing an unintended result.
Prevented with unique constraints, upserts, `FOR UPDATE`, and mutexes.

**Race detector** — `go test -race`; finds unsynchronized memory access.

**Rate limiting** — Bounding requests per key/window. Here per-IP on auth
endpoints, in-process fixed window.

**Readiness** — See health check.

**Refresh token** — Long-lived opaque credential used to obtain a new access
token. Rotated on every use.

**Repository / store** — Type that owns SQL for one domain and translates driver
errors.

**Reuse detection** — Treating a rotated refresh token as theft and revoking
sessions.

**Row comparison** — `(created_at, id) < ($1, $2)`, used for keyset pagination.

## S

**`SELECT ... FOR UPDATE`** — Row lock held until transaction end. Serializes
concurrent refreshes of the same token.

**Sentinel error** — See error sentinel.

**`slog`** — Standard library structured logger (`log/slog`).

**`sid`** — Custom JWT claim holding the refresh-token row UUID.

**Slowloris** — Attack holding connections open with partial requests. Mitigated
by `ReadHeaderTimeout`.

**Stateless auth** — No per-request session lookup. Trade-off: revocation is
delayed until token expiry.

**Store** — See repository.

## T

**`timestamptz`** — PostgreSQL timestamp with time zone, stored as UTC instant.

**Token rotation** — Issuing a new refresh token and revoking the old one on each
refresh.

**Transaction** — Atomic unit of database work. `BEGIN ... COMMIT/ROLLBACK`.

**`tsvector` / `tsquery`** — PostgreSQL document and query types for full-text
search.

## U

**Upsert** — `INSERT ... ON CONFLICT ... DO UPDATE`. Atomic create-or-replace.

**UUID** — 128-bit identifier. Used for all primary keys.

## V

**Validation** — Shape/type checks in handlers (`validator`), business rules in
services. 422 for semantic failures, 400 for malformed syntax.

**`validator`** — `go-playground/validator/v10`, tag-based struct validation.

## W

**Worker pool** — Fixed goroutines consuming a queue. Here notification writers.

**Write timeout** — Server limit on response write duration. Must exceed the
request timeout.

## X

**XSS / CSRF** — Browser attack classes relevant to token storage. This API
returns JSON tokens; the client chooses storage.

## Z

**Zero value** — The default value of a Go type. A zero `uuid.UUID` usually
signals a bug, not a valid ID.
