# 03 — HTTP server and routing

## Goal

Build a correct HTTP surface with the standard library: methods, path
parameters, status codes, timeouts, and no framework.

## `http.ServeMux` in Go 1.22+

Patterns can include a method and wildcards:

```go
mux.Handle("GET /api/v1/posts/{postID}", handler)
mux.Handle("POST /api/v1/posts", handler)
```

- `{postID}` matches one path segment and is read with `r.PathValue("postID")`.
- `{path...}` matches the rest of the path (not used here).
- A trailing slash means a subtree (`/static/`). Without it, only the exact path
  matches.
- Registering `GET /x` and `POST /x` separately is normal; a method mismatch
  returns 405 with an `Allow` header automatically.
- Conflicting patterns panic at registration. That is good: fail at startup, not
  on the first request.

`internal/api/router.go` registers 37 endpoints this way. There is no catch-all
`/` route; if there were, it would swallow 404s and make method handling fuzzy.

## Handler shape

```go
func (h *Posts) Create(w http.ResponseWriter, r *http.Request) {
    authorID, ok := auth.UserIDFrom(r.Context())
    if !ok {
        h.fail(w, r, apperr.ErrUnauthorized)
        return
    }
    var req createPostRequest
    if err := h.decode(w, r, &req); err != nil {
        h.fail(w, r, err)
        return
    }
    if err := h.validate(&req); err != nil {
        h.fail(w, r, err)
        return
    }
    post, err := h.svc.Create(r.Context(), authorID, posts.CreateInput{...})
    if err != nil {
        h.fail(w, r, err)
        return
    }
    response.Data(w, http.StatusCreated, dto.NewPost(post))
}
```

Notice:

- No business logic. Validation of shape and types happens here; rules like
  "drafts are private" live in the store.
- Every error path returns immediately. No nested `if` pyramids.
- The context is passed down, never replaced with `context.Background()`.

## Status codes used

| Code | Meaning in this API |
| --- | --- |
| 200 | read/update success |
| 201 | created (register, post, comment) |
| 204 | success with no body (follow, delete, logout) |
| 400 | malformed syntax: JSON, UUID, cursor |
| 401 | missing/invalid credentials |
| 403 | authenticated but not allowed |
| 404 | missing, or hidden draft |
| 405 | wrong method (with `Allow`) |
| 409 | uniqueness conflict |
| 413 | body too large |
| 422 | semantic validation failure |
| 429 | rate limited |
| 500 | unexpected |

The distinction between 400 and 422 matters: a cursor that cannot be decoded is
malformed (400); a `limit` of 101 is well-formed but invalid (422). Pick a rule
and keep it consistent.

## Server timeouts

`internal/app/app.go` configures:

```go
srv := &http.Server{
    Addr:              cfg.HTTPAddr,
    Handler:           handler,
    ReadHeaderTimeout: 5 * time.Second,
    ReadTimeout:       15 * time.Second,
    WriteTimeout:      30 * time.Second,
    IdleTimeout:       60 * time.Second,
}
```

Without `ReadHeaderTimeout`, a slowloris client can hold connections forever.
`WriteTimeout` bounds slow readers. These are not optional in production.

## Request body limits

`http.MaxBytesReader` caps the body before decoding:

```go
r.Body = http.MaxBytesReader(w, r.Body, h.maxBody)
```

Exceeding it produces `*http.MaxBytesError`, which the base handler maps to
`apperr.ErrPayloadTooLarge` (413). Do this at the edge; never read an unbounded
body into memory.

## Testing a route without a server

`httptest.NewRequest` + `httptest.NewRecorder` call a handler directly. But path
values are only populated by the mux. For routes with `{id}`, either use the
real router or set values manually:

```go
req.SetPathValue("postID", id.String())
```

The integration suite uses `httptest.NewServer(app.Handler())`, which exercises
the mux and middleware for real.

## Exercise

1. Add `GET /api/v1/ping` returning `{"data":{"pong":true}}` without touching
   existing handlers. Which file changes?
2. Request `DELETE /healthz` and inspect the status and `Allow` header. Which
   component produced the body?
3. Change `ReadHeaderTimeout` to 1 nanosecond and send a request with curl. What
   do you observe? Revert.
4. Explain why `http.MaxBytesReader` needs the `ResponseWriter`.
5. Find a route that is intentionally public and one that is optionally
   authenticated. What differs in the router?

Next: [04 — Configuration](04-configuration.md)
