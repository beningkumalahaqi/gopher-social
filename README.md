# GopherSocial

A production-shaped social media REST API written in Go, built to be read as
much as run. It covers the parts of backend work that tutorials usually skip:
refresh-token rotation with reuse detection, keyset pagination, transactional
writes, bounded background workers, graceful shutdown, and an integration test
suite that exercises the real HTTP surface against PostgreSQL.

The repository doubles as a course. Start with the [README](README.md), then
work through [`docs-learn/`](docs-learn/README.md).

## Stack

| Concern | Choice |
| --- | --- |
| Language | Go 1.26 |
| HTTP | `net/http` + Go 1.22 method/path patterns (no framework) |
| Database | PostgreSQL 16, `pgx/v5` + `pgxpool` |
| Migrations | `golang-migrate` as a library, SQL embedded in the binary |
| Auth | Argon2id passwords, HS256 access JWT, opaque rotating refresh tokens |
| Validation | `go-playground/validator/v10` |
| IDs | UUIDv4 (`google/uuid`) |
| Logging | `log/slog` (JSON in production, text in development) |
| API docs | OpenAPI 3.1 + embedded Swagger UI at `/swagger/` |
| Tests | standard `testing`, `httptest`, real PostgreSQL for integration |

## Quick start

Requirements: Go 1.26+, PostgreSQL 16 on `localhost:5432` with user `admin`
and password `password`.

```bash
cp .env.example .env
make migrate-up               # creates gopherdb if it does not exist
make seed
make run                      # http://localhost:8080
```

Databases are created automatically on first connection, so no manual
`CREATE DATABASE` step is needed. If your server uses different credentials or
a different port, point `DATABASE_URL` in `.env` at it.

Check it:

```bash
curl -s localhost:8080/healthz
curl -s localhost:8080/readyz
```

Open **<http://localhost:8080/swagger/>** for the interactive API docs.

Seeded accounts all use the password `Password123!`:

| Username | Email |
| --- | --- |
| `alice` | `alice@example.com` |
| `bob` | `bob@example.com` |
| `carol` | `carol@example.com` |
| `dave` | `dave@example.com` |
| `erin` | `erin@example.com` |
| `frank` | `frank@example.com` |

```bash
curl -s localhost:8080/api/v1/auth/login \
  -H 'content-type: application/json' \
  -d '{"login":"alice","password":"Password123!"}'
```

## Commands

```bash
make run           # start the API
make build         # compile to bin/api
make test          # unit + integration tests
make test-race     # same suite under the race detector
make lint          # gofmt check + go vet
make migrate-up    # apply embedded migrations
make migrate-down  # roll back one migration
make migrate-reset # roll every migration back
make seed          # destructive dev seed (truncates tables)
make smoke         # end-to-end Python smoke test against a running server
make bench         # pagination benchmarks
```

Direct equivalents:

```bash
go run ./cmd/migrate -dir=up
go run ./cmd/migrate -dir=down
go run ./cmd/migrate -dir=version
go run ./cmd/seed
go run ./cmd/api
python3 scripts/smoke.py
```

## API

All application routes live under `/api/v1`. Responses use two envelopes:

```json
{ "data": { "...": "..." } }
```

```json
{ "data": [ ... ], "meta": { "limit": 20, "offset": 0, "has_more": false } }
```

Errors are always:

```json
{
  "error": {
    "code": "validation_error",
    "message": "validation failed",
    "details": [{ "field": "email", "message": "must be a valid email address" }],
    "request_id": "7f2c..."
  }
}
```

| Method | Path | Auth | Purpose |
| --- | --- | --- | --- |
| GET | `/healthz` | – | Liveness |
| GET | `/readyz` | – | Readiness (pings DB) |
| POST | `/api/v1/auth/register` | – | Create account, return session |
| POST | `/api/v1/auth/login` | – | Login with username or email |
| POST | `/api/v1/auth/refresh` | – | Rotate refresh token |
| POST | `/api/v1/auth/logout` | – | Revoke a refresh token |
| GET | `/api/v1/users/me` | Bearer | Private profile |
| PATCH | `/api/v1/users/me` | Bearer | Update display name, bio, avatar |
| PATCH | `/api/v1/users/me/password` | Bearer | Change password, revoke sessions |
| GET | `/api/v1/users/{username}` | – | Public profile + counts |
| GET | `/api/v1/users/{username}/posts` | optional | Posts by author |
| GET | `/api/v1/users/{username}/followers` | – | Followers |
| GET | `/api/v1/users/{username}/following` | – | Following |
| GET | `/api/v1/users/{username}/follow-status` | Bearer | Relationship flags |
| GET | `/api/v1/users/{username}/suggestions` | – | Who to follow |
| POST | `/api/v1/users/{username}/follow` | Bearer | Follow (idempotent) |
| DELETE | `/api/v1/users/{username}/follow` | Bearer | Unfollow (idempotent) |
| POST | `/api/v1/posts` | Bearer | Create post or draft |
| GET | `/api/v1/posts` | optional | List/filter posts (offset) |
| GET | `/api/v1/posts/{postID}` | optional | Single post |
| PATCH | `/api/v1/posts/{postID}` | Bearer | Edit own post / publish |
| DELETE | `/api/v1/posts/{postID}` | Bearer | Delete own post |
| POST | `/api/v1/posts/{postID}/comments` | Bearer | Comment |
| GET | `/api/v1/posts/{postID}/comments` | – | List comments |
| PATCH | `/api/v1/comments/{commentID}` | Bearer | Edit own comment |
| DELETE | `/api/v1/comments/{commentID}` | Bearer | Delete own comment or as post author |
| PUT | `/api/v1/posts/{postID}/reaction` | Bearer | Upsert `like`, `love`, `laugh` |
| DELETE | `/api/v1/posts/{postID}/reaction` | Bearer | Remove reaction |
| GET | `/api/v1/posts/{postID}/reactions` | – | List reactions |
| GET | `/api/v1/posts/{postID}/reaction-summary` | optional | Counts + viewer reaction |
| GET | `/api/v1/feed` | Bearer | Following + own posts (cursor) |
| GET | `/api/v1/explore` | optional | Public posts (cursor) |
| GET | `/api/v1/search/users?q=` | – | Username/display-name search |
| GET | `/api/v1/search/posts?q=` | optional | Full-text post search |
| GET | `/api/v1/notifications` | Bearer | Notification inbox |
| GET | `/api/v1/notifications/unread-count` | Bearer | Unread badge |
| PATCH | `/api/v1/notifications/{notificationID}/read` | Bearer | Mark one read |
| POST | `/api/v1/notifications/read-all` | Bearer | Mark all read |

`optional` auth means anonymous requests work; when a valid bearer token is
present, the response includes viewer-specific fields such as `viewer_reaction`.

Full request/response examples: [`docs/api-examples.md`](docs/api-examples.md).
OpenAPI 3.1 contract: [`docs/openapi.yaml`](docs/openapi.yaml).

### Interactive docs

With the server running, open **<http://localhost:8080/swagger/>**. The Swagger
UI is served by the API itself, with the spec and UI assets embedded in the
binary — no CDN, no extra process. The raw contract is at
`/swagger/openapi.yaml`.

Use **Authorize** with an access token from `/api/v1/auth/login` to call
authenticated endpoints. See [`docs/swagger.md`](docs/swagger.md) for details and
the workflow for keeping the contract in sync.

## Learning interface

The same binary serves a server-rendered course at `/learn`. It reuses the
curriculum in `docs-learn/`, the auth service, and the PostgreSQL pool; it does
not duplicate the API or the Markdown files.

| Path | Purpose |
| --- | --- |
| `/learn` | dashboard with the next lesson and current progress |
| `/learn/courses`, `/learn/courses/{slug}` | course catalogue and module map |
| `/learn/lessons/{slug}` | lesson page with notes, bookmark, and reading progress |
| `/learn/exercises`, `/learn/quizzes` | interactive labs and graded checks |
| `/learn/glossary`, `/learn/search` | definitions and full-corpus search |
| `/learn/progress`, `/learn/review` | stored activity and what needs attention |
| `/learn/playground` | server-side proxy to the Go playground compiler |
| `/learn/api` | live API explorer built from the embedded OpenAPI contract |

A visitor can start as a guest; activity is stored against a hashed cookie and
merged into the account on login. Log in with a seeded user to keep progress
across browsers. The interface works without JavaScript for reading and form
submission; HTMX adds partial updates when it is available.

The course itself starts at [`docs-learn/00-roadmap.md`](docs-learn/00-roadmap.md).

### Deploying on Vercel

Vercel's Go framework preset detects `cmd/api/main.go` and runs the server on
`$PORT`, which `config.Load` already honors. `vercel.json` sets the framework to
`go`. Set `DATABASE_URL`, `JWT_SECRET`, and `APP_ENV` in the project, keep
`DATABASE_MAX_CONNS` small, and run migrations outside the request path. See
[`docs-learn/42-deploying-on-vercel.md`](docs-learn/42-deploying-on-vercel.md)
for the constraints.

## Layout

```text
cmd/
  api/            HTTP entrypoint
  migrate/        migration CLI
  seed/           seed CLI
internal/
  api/            router, handlers, DTOs, middleware, params, envelopes
  app/            composition root + lifecycle
  apperr/         domain error sentinels + validation errors
  auth/           passwords, JWTs, refresh sessions, auth service
  comments/       comment store/service
  config/         env loading + validation
  database/       pgxpool, querier abstraction, migration runner
  feed/           feed/explore composition
  follows/        social graph
  notifications/  store, service, bounded worker queue
  pagination/     offset + keyset primitives
  posts/          post store/service
  reactions/      reaction store/service
  requestid/      request ID context
  users/          user store/service
migrations/       embedded SQL migrations
seeds/            embedded dev seed
tests/            HTTP integration suite against PostgreSQL
scripts/          smoke test + DB helpers
docs/             architecture, API examples, OpenAPI, Swagger guide
docs-learn/       the course
internal/learn/   curriculum, renderer, search, activities, cache, pgstore
internal/learn/web/  HTML handlers, templates, static assets
```

## Design decisions worth knowing

- **No framework.** `http.ServeMux` with Go 1.22 patterns is enough for 40
  routes and keeps middleware explicit.
- **Explicit wiring.** `internal/app` constructs every store, service, and
  handler. No DI container, no `init()` magic.
- **Stores take a `Querier`.** Both `*pgxpool.Pool` and `pgx.Tx` satisfy it, so
  multi-step writes can be transactional without leaking `pgx` into services.
- **Domain models carry no JSON tags.** HTTP shapes live in `internal/api/dto`.
  This prevents a database refactor from silently changing the public API.
- **Errors are sentinels.** Stores return `apperr.ErrNotFound` etc.; one mapping
  function turns them into status codes. Unknown errors become opaque 500s.
- **Refresh tokens are opaque and hashed.** The database never stores a usable
  token. Rotation plus reuse detection revokes all of the user's active sessions.
- **The notification queue is bounded.** When full, work is dropped and logged
  rather than blocking an HTTP request. Shutdown drains what is queued.
- **Integration tests use a real database.** Concurrency guarantees live in SQL;
  mocking PostgreSQL would test the mock.

More: [`docs/architecture.md`](docs/architecture.md).

## Testing

```bash
make test        # includes tests/ against gopherdb_test
make test-race
make bench
```

The integration suite runs migrations once in `TestMain`, truncates between
tests, starts the real `app.App` behind `httptest.NewServer`, and covers:

- full HTTP lifecycle: register → post → follow → comment → react → notify
- draft visibility and ownership rules
- refresh rotation, reuse detection, logout, password change
- concurrent reactions and follows (one row, one notification)
- cursor pagination walking every post exactly once
- validation failures and error envelopes

Override the test database with `TEST_DATABASE_URL`. If PostgreSQL is
unreachable the integration tests skip instead of failing, so `go test ./...`
still works on a laptop without Docker.

## Course

[`docs-learn/README.md`](docs-learn/README.md) maps 43 modules, 35 exercises,
runnable examples, and a glossary onto this codebase. The first course covers
the API; the second covers the server-rendered learning interface, caching, and
performance. Each module points at real files and ends with questions or an
exercise. The exercises are designed to be done in a branch; solutions are
sketches, not drop-in patches.

The ordered map and study paths are in
[`docs-learn/00-roadmap.md`](docs-learn/00-roadmap.md).

## License

Released under the [MIT License](LICENSE). Use it, fork it, teach with it.
