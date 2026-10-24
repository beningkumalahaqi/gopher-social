# 07 — Errors and envelopes

## Goal

Make every failure predictable for clients and debuggable for operators.

## The envelope

Success:

```json
{ "data": { "id": "..." } }
```

Collection:

```json
{ "data": [], "meta": { "limit": 20, "offset": 0, "has_more": false } }
```

Failure:

```json
{
  "error": {
    "code": "not_found",
    "message": "resource not found",
    "details": null,
    "request_id": "..."
  }
}
```

`code` is stable and machine-readable. `message` is human-readable and may
change. Clients should branch on status and `code`, never on `message`.

## Sentinels

`internal/apperr` defines the categories:

```go
var (
    ErrBadRequest       = errors.New("bad request")
    ErrValidation       = errors.New("validation failed")
    ErrUnauthorized     = errors.New("unauthorized")
    ErrForbidden        = errors.New("forbidden")
    ErrNotFound         = errors.New("not found")
    ErrConflict         = errors.New("conflict")
    ErrRateLimited      = errors.New("rate limited")
    ErrPayloadTooLarge  = errors.New("payload too large")
)
```

Wrap with context:

```go
return apperr.Wrap(apperr.ErrNotFound, "post %s", id)
```

`Wrap` preserves the chain (`%w`), so `errors.Is(err, apperr.ErrNotFound)` is
true. The message becomes `not found: post <id>`; `response.clean` strips the
sentinel prefix for the client, leaving `post <id>`.

## Validation errors

Validation needs field-level detail, which a sentinel cannot carry:

```go
type FieldError struct {
    Field   string `json:"field"`
    Message string `json:"message"`
}

type ValidationError struct {
    Fields []FieldError
}

func (e *ValidationError) Error() string { return "validation failed" }
func (e *ValidationError) Is(target error) bool { return target == ErrValidation }
```

`errors.As` extracts the fields in `response.Error`. The client gets:

```json
{
  "error": {
    "code": "validation_error",
    "message": "validation failed",
    "details": [{ "field": "email", "message": "must be a valid email address" }]
  }
}
```

## One mapping function

```go
switch {
case errors.Is(err, apperr.ErrValidation):
    return 422, "validation_error", "validation failed"
case errors.Is(err, apperr.ErrNotFound):
    return 404, "not_found", "resource not found"
// ...
default:
    return 500, "internal_error", "internal server error"
}
```

Benefits:

- Handlers never choose status codes for domain errors.
- Unknown errors are opaque. No SQL, no stack, no table names leak.
- Adding a category is one place plus tests.

## 400 vs 422 vs 404 vs 403

- **400**: the request cannot be parsed (bad JSON, bad UUID, bad cursor).
- **422**: parsed but semantically invalid (limit 101, unknown status, short
  password).
- **401**: no valid identity.
- **403**: identity is valid but lacks permission, and the resource's existence
  is not secret.
- **404**: missing, or intentionally hidden (another user's draft). Returning
  404 for hidden drafts avoids confirming that a draft exists.

## Request IDs

`requestid` middleware generates a random hex ID, stores it in context, and sets
`X-Request-ID`. Every error includes it. Logs include it. When a user reports a
failure, ask for the ID; you can find the exact request without guessing.

Do not trust a client-supplied request ID without validation. This app always
generates its own. If you want to propagate one, accept it only if it matches a
strict pattern and cap its length.

## Errors at boundaries

- **Handlers** translate decode/validation failures into `apperr`.
- **Services** enforce rules and return sentinels.
- **Stores** translate driver errors: `pgx.ErrNoRows` → `ErrNotFound`, unique
  violation → `ErrConflict`, FK violation → `ErrBadRequest`/`ErrNotFound`.
- **Middleware** maps panics and unknown errors to 500.

Never compare error strings. Never return `nil, nil`. Never log and return the
same error at every layer; log once, where you have the most context (usually
the handler or middleware).

## Exercise

1. Add `ErrTooManyRequests`? There is already `ErrRateLimited`. Explain why a
   new sentinel is unnecessary.
2. Make `GET /posts/{id}` return 403 for another user's draft instead of 404.
   What information does that leak? Revert.
3. Add a `details` field for a conflict error that names the conflicting field.
   Where should it be constructed?
4. Find a store method that returns a raw `pgx` error. Wrap it correctly.
5. Write a test asserting that a 500 response does not contain the string
   `pq:` or a table name.

Next: [08 — Database and pgx](08-database-and-pgx.md)
