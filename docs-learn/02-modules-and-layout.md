# 02 — Modules and layout

## Goal

Understand why this repository is shaped the way it is, and how Go's package
system enforces (or fails to enforce) boundaries.

## The module

```go
module github.com/haqi/gophersocial

go 1.26.3
```

The module path is also the import prefix:

```go
import "github.com/haqi/gophersocial/internal/posts"
```

`go.mod` records direct dependencies; `go.sum` records checksums. Run
`go mod tidy` after adding or removing imports. Never edit `go.sum` by hand.

Useful commands:

```bash
go list -m all          # dependency graph
go mod why golang.org/x/crypto/argon2
go mod graph | head
go list ./...           # every package in the module
go list -deps ./cmd/api | wc -l
```

## Why `cmd/` and `internal/`

- `cmd/api`, `cmd/migrate`, `cmd/seed` are entrypoints. Each has a tiny `main`
  that parses flags, loads config, and delegates. Keeping them thin makes the
  real logic testable.
- `internal/` is enforced by the Go toolchain: code outside
  `github.com/haqi/gophersocial` cannot import it. This is a real boundary, not
  a convention.
- Domain packages are siblings, not nested under a generic `pkg/`. `pkg/` adds a
  level without adding meaning.

## Dependency direction

```text
cmd/api ──> internal/app ──> internal/api ──> handlers ──> services ──> stores
                                   │                         │
                                   └──> dto, params          └──> apperr
```

Rules that keep this manageable:

- `internal/api` must not contain SQL.
- Domain packages must not import `net/http`.
- Stores must not call services (no cycles, easier transactions).
- A package may import `apperr`, `pagination`, `database` freely.
- `internal/app` is the only composition root.

Go does not prevent all cycles at the type level, but it does prevent import
cycles. If you feel tempted to import `handlers` from `posts`, the abstraction is
in the wrong place.

## Package naming

- Short, lower-case, no underscores: `posts`, not `post_service`.
- The package name is part of the call site: `posts.Store`, not
  `posts.PostStore`.
- Avoid `util`, `common`, `helpers`. They become junk drawers. This repo has
  `apperr`, `pagination`, `validation` because each has a precise job.
- Do not name a package `http`; it shadows the standard library. That is why the
  HTTP layer is `api`, not `http`.

## Interfaces live with the consumer

Handlers define the interfaces they need:

```go
type PostService interface {
    Create(ctx context.Context, authorID uuid.UUID, in posts.CreateInput) (posts.Post, error)
    // ...
}
```

The implementation (`*posts.Service`) does not know this interface exists. This
is idiomatic Go: accept interfaces, return structs. It also means a handler test
can use a fake without importing a mocking framework.

The same idea appears in `follows.Notifier`: the follows package defines the
small behavior it needs, and `notifications.Service` happens to satisfy it.

## Build tags and generated code

There are none. No `//go:generate`, no code generation, no build tags. This is
deliberate for a teaching repo: everything you run is in the tree. In a larger
project, generated SQL code (sqlc) or mocks can be worth it.

## Exercise

1. Run `go list -deps ./internal/posts` and identify which internal packages it
   depends on. Is that set minimal?
2. Add a temporary import of `internal/api/handlers` inside `internal/posts` and
   run `go build ./...`. Read the cycle error, then revert.
3. Find every interface in the repo with `grep -rn "interface {" internal`.
   For each, note who defines it and who implements it.
4. Explain why `cmd/seed` can import `seeds` but `internal/posts` cannot import
   `cmd/seed`.

Next: [03 — HTTP server and routing](03-http-server-and-routing.md)
