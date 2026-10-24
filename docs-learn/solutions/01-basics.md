# Solutions 01 — Basics

## 1.1 Trace

The route is registered in `internal/api/router.go` as
`POST /api/v1/posts/{postID}/comments`, wrapped by `RequireAuth`. The handler in
`internal/api/handlers/comments.go` parses `postID` with `uuid.Parse`, decodes the
body, validates it, then calls `comments.Service.Create`.

The service checks the post exists and is visible. The store query joins or
selects the post with `deleted_at IS NULL` and `status = 'published'` (drafts are
visible only to their author). A missing or invisible post maps to
`apperr.ErrNotFound`, which the response layer renders as 404 `not_found`.

Key point: the 404 is a service rule, not a handler check.

## 1.2 Version endpoint

- Add `Version string` to `internal/config.Config`, loaded from `VERSION`, default
  `dev`.
- Add a handler that writes `{"data":{"version": ...}}`.
- Register outside the auth group.
- Test with `httptest` and assert `Content-Type`.
- Add `VERSION=dev` to `.env.example`.

## 1.3 Package move

Move the function and its tests. Keep the exported name. Update call sites with
`grep -rn NormalizeUsername`. If the new package needs config or errors from
`users`, it is not a clean boundary; keep it where it was.

The honest answer to "did it improve anything" is often "no".

## 1.4 410 Gone

- Add `ErrGone` to `internal/apperr`, plus a `Code` string (`gone`).
- Map it in `internal/api/response` (or wherever status mapping lives).
- Store: `SELECT ..., deleted_at FROM posts WHERE id = $1`; return `ErrGone` when
  `deleted_at.Valid`.
- Do not leak drafts as gone unless the product wants that.

## 1.5 Hygiene

Typical removable dependency: a tiny helper library used once. `go mod tidy`
removes unused modules. Do not remove `pgx`, `validator`, `uuid`, or `jwt`.
Measure before/after with `go build -o /dev/null ./...` if you care.
