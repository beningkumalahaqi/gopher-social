# 17 — Handlers

## Goal

Write handlers that are boring: decode, validate, call one service method, map
the result, return. Every interesting decision lives elsewhere.

## The base helper

`internal/api/handlers/base.go` centralizes:

- body size limit (`http.MaxBytesReader`)
- strict JSON decoding (`DisallowUnknownFields`, single object, no trailing data)
- validation
- error response mapping

```go
func (b base) decode(w http.ResponseWriter, r *http.Request, dst any) error {
    r.Body = http.MaxBytesReader(w, r.Body, b.maxBody)
    dec := json.NewDecoder(r.Body)
    dec.DisallowUnknownFields()
    if err := dec.Decode(dst); err != nil { ... }
    if dec.Decode(&struct{}{}) != io.EOF { return ... trailing data ... }
    return nil
}
```

Why strict:

- A typo like `"emial"` fails loudly instead of being ignored and producing a
  confusing validation error.
- Extra fields can be an attempt to set server-controlled state (`"is_admin"`).
- Trailing JSON is almost always a client bug.

The trade-off: clients cannot send extra fields for forward compatibility. For a
public API, you might allow unknown fields; for this one, strict is safer.

## Status codes by action

```go
response.Data(w, http.StatusCreated, dto.NewPost(post)) // create
response.Data(w, http.StatusOK, dto.NewPost(post))      // read/update
response.NoContent(w)                                  // delete/follow/logout
```

Do not return 200 with `{"success":true}` for deletes. 204 is unambiguous and
saves bytes.

## Thin handlers

A handler should not:

- open transactions
- write SQL
- hash passwords
- decide draft visibility
- send notifications directly

It may:

- read path/query params
- decode and validate the body
- get the identity
- call a service
- choose a DTO and status

If a handler grows past ~40 lines, a rule is probably in the wrong place.

## Errors

Every handler uses `h.fail(w, r, err)`, which calls `response.Error`. Do not
write error JSON inline; one mapping function keeps codes consistent.

For expected failures, return a sentinel from the service. For programmer
errors, return a wrapped `fmt.Errorf`; it becomes a 500 and is logged.

## Query parameters

`internal/api/params` centralizes parsing:

```go
page, err := params.Pagination(r)
status, err := params.Enum(r, "status", "draft", "published")
id, err := params.UUID(r, "postID")
```

Consistent rules:

- `limit` default 20, range 1–100.
- `offset` default 0, non-negative.
- Unknown enum values → 422.
- Malformed UUID/cursor → 400.

Handlers should not call `strconv.Atoi` directly; otherwise each endpoint
invents its own error behavior.

## Example: comments

```go
func (h *Comments) Create(w http.ResponseWriter, r *http.Request) {
    userID, ok := auth.UserIDFrom(r.Context())
    if !ok { h.fail(w, r, apperr.ErrUnauthorized); return }

    postID, err := params.UUID(r, "postID")
    if err != nil { h.fail(w, r, err); return }

    var req commentRequest
    if err := h.decode(w, r, &req); err != nil { h.fail(w, r, err); return }
    if err := h.validate(&req); err != nil { h.fail(w, r, err); return }

    comment, err := h.svc.Create(r.Context(), postID, userID, req.Content)
    if err != nil { h.fail(w, r, err); return }

    response.Data(w, http.StatusCreated, dto.NewComment(comment))
}
```

Notice the rhythm: guard, parse, decode, validate, call, map. Every early return
is one line. This is easy to review and hard to get subtly wrong.

## Handler tests

Handlers are tested with fakes for the service interfaces:

```go
h := handlers.NewAuth(logger, maxBody, fakeAuth)
req := httptest.NewRequest(...)
rec := httptest.NewRecorder()
h.Register(rec, req)
```

This tests decoding, validation, status, and DTO mapping without a database. The
integration suite covers the full path.

## Exercise

1. Add `GET /api/v1/posts/{postID}/comments/count`. Does it need a service
   method or can it reuse an existing one?
2. Make `decode` accept unknown fields and explain the risk.
3. Return 200 instead of 204 for DELETE and update the tests. Which clients
   break?
4. Add a `Location` header on post creation. What URL should it contain?
5. Find a handler that calls two services. Is that a sign of a missing
   orchestration service?

Next: [18 — Pagination](18-pagination.md)
