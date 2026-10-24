# Exercises 01 — Basics

Do these in a branch. Run `go test ./...` after each. Solutions are sketches in
`solutions/01-basics.md`.

## 1.1 Trace a request

**Task:** Write a one-page trace of `POST /api/v1/posts/{postID}/comments`, from
router to SQL and back. Name every function and the file it lives in.

**Done when:** You can explain where the 404 for a draft comes from without
opening the code.

**Hint:** Start at `internal/api/router.go`; follow the `Comments.Create` handler.

## 1.2 Add a health detail

**Task:** Add `GET /api/v1/version` returning `{"data":{"version":"dev"}}`. Read
the version from a new `VERSION` config value with default `dev`.

**Done when:** The endpoint works without auth, has a test, and `VERSION` is in
`.env.example`.

**Hint:** The route does not belong in `handlers.Health` if it grows; a tiny
handler is fine.

## 1.3 Package boundary

**Task:** Move `NormalizeUsername` from `internal/users` to a new package
`internal/names`. Update imports.

**Done when:** `go build ./...` passes and no import cycle exists.

**Question:** Did the move improve anything? If not, revert it. Not every
refactor is progress.

## 1.4 Error classification

**Task:** Add `apperr.ErrGone` mapped to 410. Return it from
`GET /posts/{id}` when a post is soft-deleted. You will need a `deleted_at`
column or a separate table.

**Done when:** The error envelope has `code: "gone"` and a test covers it.

**Hint:** Do not expose whether a draft or a deleted post exists differently
unless the product requires it.

## 1.5 Module hygiene

**Task:** Run:

```bash
go mod tidy
go list -m all | wc -l
go vet ./...
gofmt -l .
```

Remove one dependency you can replace with the standard library. Justify it.

**Done when:** The build and tests pass and the diff is small.

**Hint:** `google/uuid` is not worth replacing. Look at convenience libraries
instead.
