# Learn Go by building GopherSocial

This is a course embedded in a working codebase. It assumes you can read Go and
have built something small, but it does not assume you have shipped a backend.
By the end you should be able to explain every moving part of this repository,
and rebuild a smaller version without looking.

The code is the source of truth. Each module points at real files; read them
with the module open. Exercises ask you to change behavior, not just answer
questions. Solutions are sketches so you still have to do the work.

The course has four parts: the API (01 to 25), the server-rendered interface
(26 to 31), caching (32 to 36), and performance with deployment (37 to 43).
See the [roadmap](00-roadmap.md) for study paths.

## How to use this

1. Get the API running (`README.md` → Quick start).
2. Read modules in order, or follow a path in the roadmap. The first modules
   are foundations; the rest follow the request path and then the harder
   guarantees.
3. After each module, do the matching exercise file. Keep a branch per exercise
   so you can compare.
4. Run `make test` before and after every change. If the integration tests skip,
   your PostgreSQL is not reachable — fix that first.
5. Use the glossary when a term is unfamiliar.

Suggested pace: two modules and one exercise per sitting. The capstone takes a
weekend.

## Modules

### Foundations

| # | Module | You will learn | Code |
| --- | --- | --- | --- |
| 01 | [Orientation](01-orientation.md) | how to run, trace, and change the project | `cmd/`, `internal/app` |
| 02 | [Modules and layout](02-modules-and-layout.md) | `go.mod`, `internal/`, package boundaries | `go.mod`, tree |
| 03 | [HTTP server and routing](03-http-server-and-routing.md) | `net/http`, Go 1.22 patterns, server timeouts | `internal/api/router.go` |
| 04 | [Configuration](04-configuration.md) | env parsing, fail-fast validation, secrets | `internal/config` |
| 05 | [Structured logging](05-structured-logging.md) | `slog`, request logs, levels, PII | `internal/api/middleware/middleware.go` |

### HTTP engineering

| # | Module | You will learn | Code |
| --- | --- | --- | --- |
| 06 | [Middleware](06-middleware.md) | composition, ordering, response capture, panic recovery | `internal/api/middleware/` |
| 07 | [Errors and envelopes](07-errors-and-envelopes.md) | sentinels, wrapping, stable codes, request IDs | `internal/apperr`, `internal/api/response` |
| 08 | [Database and pgx](08-database-and-pgx.md) | pools, contexts, scanning, `Querier` | `internal/database` |
| 09 | [Migrations](09-migrations.md) | embedded SQL, up/down, safe schema changes | `migrations/`, `cmd/migrate` |
| 10 | [Stores and transactions](10-stores-and-transactions.md) | repository pattern, rollback, unique violations | `internal/auth/service.go` |
| 11 | [Models and DTOs](11-models-and-dtos.md) | domain/API separation, deliberate JSON | `internal/api/dto` |
| 12 | [Validation](12-validation.md) | tags, custom rules, field errors, Unicode | `internal/validation`, `internal/users` |

### Identity

| # | Module | You will learn | Code |
| --- | --- | --- | --- |
| 13 | [Password hashing](13-password-hashing.md) | Argon2id, salts, PHC, timing defense | `internal/auth/password.go` |
| 14 | [JWT access tokens](14-jwt-access-tokens.md) | claims, signing, algorithm confusion, TTL | `internal/auth/token.go` |
| 15 | [Refresh rotation](15-refresh-rotation.md) | opaque tokens, hashing, rotation, reuse detection | `internal/auth/service.go` |
| 16 | [Auth middleware](16-auth-middleware.md) | bearer parsing, context identity, 401 vs 403 | `internal/api/middleware/auth.go` |

### Features

| # | Module | You will learn | Code |
| --- | --- | --- | --- |
| 17 | [Handlers](17-handlers.md) | strict decoding, limits, status codes, thin handlers | `internal/api/handlers/` |
| 18 | [Pagination](18-pagination.md) | offset vs keyset, cursors, stable order, indexes | `internal/pagination`, `internal/posts/store.go` |
| 19 | [Search](19-search.md) | `tsvector`, GIN, ranking, safe queries | `migrations/000008_search_indexes.up.sql` |
| 20 | [Feed and social graph](20-feed-and-social-graph.md) | joins, visibility, fan-out trade-offs | `internal/posts/store.go`, `internal/follows` |
| 21 | [Comments and reactions](21-comments-and-reactions.md) | permissions, upserts, composite keys, races | `internal/comments`, `internal/reactions` |

### Reliability

| # | Module | You will learn | Code |
| --- | --- | --- | --- |
| 22 | [Background workers](22-background-workers.md) | bounded queues, drop policy, draining, idempotency | `internal/notifications` |
| 23 | [Shutdown and health](23-shutdown-and-health.md) | signals, ordered shutdown, probes | `internal/app/app.go` |
| 24 | [Testing](24-testing.md) | unit, integration, race, fuzz, benchmarks | `*_test.go`, `tests/` |
| 25 | [Production checklist](25-production-checklist.md) | security, observability, deployment, scaling | everything |

### Server-rendered interface

Build the learning platform you are reading. These lessons assume the router,
middleware, and database modules above.

| # | Module | You will learn | Code |
| --- | --- | --- | --- |
| 26 | [Template architecture](26-template-architecture.md) | startup parsing, page data, escaping, buffered render | `internal/learn/web` |
| 27 | [Layouts and fragments](27-layouts-and-fragments.md) | the shell, named fragments, two response modes | `internal/learn/web/templates` |
| 28 | [HTMX contract](28-htmx-contract.md) | targeted updates that work without JavaScript | `templates/partials.html`, `static/learn.js` |
| 29 | [Forms and CSRF](29-forms-and-csrf.md) | double-submit tokens, limits, validation | `internal/learn/web/server.go` |
| 30 | [Content pipeline](30-content-pipeline.md) | Markdown, sanitizing, metadata, diagnostics | `internal/learn/markdown.go` |
| 31 | [Search over your corpus](31-search-over-your-corpus.md) | in-memory index, ranking, filters | `internal/learn/search.go` |

### Caching

| # | Module | You will learn | Code |
| --- | --- | --- | --- |
| 32 | [Caching fundamentals](32-caching-fundamentals.md) | hit ratio, staleness budget, cache-aside | `internal/learn/cache` |
| 33 | [HTTP caching](33-http-caching.md) | immutable URLs, validators, `Vary` | `internal/learn/web/server.go` |
| 34 | [An in-process cache](34-in-process-cache.md) | bounds, locks, expiry, FIFO eviction | `internal/learn/cache/cache.go` |
| 35 | [Invalidation and coalescing](35-invalidation-and-coalescing.md) | delete on write, versioned keys, singleflight | `internal/learn/cache` |
| 36 | [Distributed caches](36-distributed-caches.md) | serialization, network failure, operational cost | no code, by design |

### Performance and deployment

| # | Module | You will learn | Code |
| --- | --- | --- | --- |
| 37 | [Measure first](37-measure-first.md) | budgets, benchmarks, profiles, percentiles | `internal/learn` tests |
| 38 | [Allocations and memory](38-allocations-and-memory.md) | escape analysis, preallocation, GC limits | `internal/learn/web/render.go` |
| 39 | [Database query optimization](39-database-query-optimization.md) | plans, indexes, keyset pagination, pools | `migrations/000009_learning` |
| 40 | [HTTP and rendering](40-http-and-rendering-performance.md) | bytes, compression, keep-alive, assets | `internal/app/app.go` |
| 41 | [Concurrency under load](41-concurrency-under-load.md) | bounds, queues, leaks, shutdown | `internal/app/app.go` |
| 42 | [Deploying on Vercel](42-deploying-on-vercel.md) | the server preset, pooling, platform limits | `cmd/api/main.go`, `vercel.json` |
| 43 | [Operating a small service](43-operating-a-small-service.md) | logs, probes, migrations, backups, incidents | `internal/app`, `cmd/` |

The ordered map, study paths, and file index are in the
[roadmap](00-roadmap.md).

## Exercises

| File | Focus | Count |
| --- | --- | --- |
| [01-basics](exercises/01-basics.md) | tooling, types, packages | 5 |
| [02-http](exercises/02-http.md) | routing, middleware, errors | 5 |
| [03-database](exercises/03-database.md) | SQL, transactions, migrations | 5 |
| [04-auth](exercises/04-auth.md) | passwords, JWTs, sessions | 5 |
| [05-concurrency](exercises/05-concurrency.md) | workers, races, contexts | 5 |
| [06-testing](exercises/06-testing.md) | unit, integration, fuzz | 5 |
| [07-capstone](exercises/07-capstone.md) | design and ship a feature | 5 + project |

Solutions: [`solutions/`](solutions/README.md). Read them only after trying.

## Runnable examples

Each is a standalone `main` package. They do not need the database; the cursor
example reuses `internal/pagination` so the encoding matches the API.

```bash
go run ./docs-learn/examples/01-hello-server
go run ./docs-learn/examples/02-middleware
go run ./docs-learn/examples/03-context-timeout
go run ./docs-learn/examples/04-worker-pool
go run ./docs-learn/examples/05-strict-json
go run ./docs-learn/examples/06-cursor-pagination
```

They are deliberately small and commented where the idea matters. Use them to
experiment without touching the API.

## Reference

- [Glossary](glossary.md)
- [Swagger UI guide](../docs/swagger.md)
- [Architecture](../docs/architecture.md)
- [API examples](../docs/api-examples.md)
- [OpenAPI](../docs/openapi.yaml)

## A note on scope

This is a teaching backend, not a finished product. It has no media uploads,
email delivery, blocking, moderation, or horizontal rate limiting. Those are
called out where relevant so you know what is intentionally missing. The point
is the shape of the engineering, not feature count.
