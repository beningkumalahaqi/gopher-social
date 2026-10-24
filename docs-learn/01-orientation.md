# 01 — Orientation

## Goal

Run the system, trace one request end to end, and know where to look when
something breaks.

## The mental model

GopherSocial is three layers:

1. **Transport** — `internal/api`: routing, middleware, decoding, DTOs, status
   codes. It knows HTTP, not business rules.
2. **Domain** — `internal/auth`, `users`, `posts`, `follows`, `comments`,
   `reactions`, `feed`, `notifications`. It knows rules and SQL, not HTTP.
3. **Platform** — `internal/config`, `database`, `apperr`, `pagination`,
   `validation`, `requestid`. Shared primitives with no feature knowledge.

`internal/app` wires layers 1 and 2 together. `cmd/api` only loads config,
builds the app, and handles process lifecycle.

## Run it

```bash
cp .env.example .env
make migrate-up
make seed
make run
```

In another terminal:

```bash
curl -s localhost:8080/healthz
curl -s localhost:8080/api/v1/explore?limit=2
```

Now log in and make a post:

```bash
TOKEN=$(curl -s -X POST localhost:8080/api/v1/auth/login \
  -H 'content-type: application/json' \
  -d '{"login":"alice","password":"Password123!"}' \
  | python3 -c 'import json,sys;print(json.load(sys.stdin)["data"]["access_token"])')

curl -s -X POST localhost:8080/api/v1/posts \
  -H "Authorization: Bearer $TOKEN" \
  -H 'content-type: application/json' \
  -d '{"content":"My first traced post"}'
```

## Trace one request

Take `POST /api/v1/posts` and follow it:

1. `internal/api/router.go` — route `POST /api/v1/posts` is wrapped in
   `auth(...)` and the global middleware chain.
2. `middleware.RequestID` — adds `X-Request-ID` and a context value.
3. `middleware.Logger` — wraps the writer to capture status and bytes.
4. `middleware.Recover` — turns a panic into a 500.
5. `middleware.Timeout` — puts a deadline on the request context.
6. `middleware.RequireAuth` — parses the bearer token, puts `auth.Identity` in
   context.
7. `handlers.Posts.Create` — decodes JSON, validates, calls the service.
8. `posts.Service.Create` — trims content, sets status, calls the store.
9. `posts.Store.Create` — `INSERT ... RETURNING`, then loads the author summary.
10. `dto.NewPost` — maps the domain post to the JSON shape.
11. `response.Data` — writes `201 {"data": ...}`.

Open each file and find the line. Do not skim; the point is to see the seams.

## Where things go wrong

| Symptom | First place to look |
| --- | --- |
| 404 on a valid path | route pattern in `router.go`, trailing slash, method |
| 401 with a fresh token | `JWT_SECRET` mismatch, expired token, `Authorization` header format |
| 500 with no detail | server logs by `request_id`; the client intentionally gets nothing |
| 422 on a field you did not send | pointer vs value in the request struct, validator tag |
| empty list | filters (draft visibility, follow graph), not pagination |
| test skips | PostgreSQL not running or `TEST_DATABASE_URL` wrong |
| migration says "no change" | already at latest; check `make migrate-version` |

## Exercise

Before moving on, answer without looking:

1. Which package decides that a draft returns 404 instead of 403?
2. Where is the JSON shape of a post defined?
3. What happens to an unknown error returned by a store?
4. Which function opens the database pool?
5. If you delete `middleware.JSONErrors`, what changes for a 404?

Check your answers by reading `internal/posts/store.go`, `internal/api/dto`,
`internal/api/response/response.go`, `internal/app/app.go`, and
`internal/api/middleware/errors.go`.

Next: [02 — Modules and layout](02-modules-and-layout.md)
