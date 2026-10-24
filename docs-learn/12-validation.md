# 12 — Validation

## Goal

Reject bad input with precise, stable errors — without duplicating rules in
every handler.

## Two kinds of validation

1. **Shape validation** — is this JSON, are required fields present, are lengths
   within bounds? This happens in the handler using `validator`.
2. **Business validation** — is this username taken, is this post visible, can
   this user edit this comment? This happens in the service or SQL.

Do not mix them. A validator tag cannot check the database, and a SQL query
should not be responsible for telling the user that their email is malformed.

## The wrapper

`internal/validation.Struct` runs `go-playground/validator` and converts its
errors into `apperr.ValidationError`:

```go
type FieldError struct {
    Field   string `json:"field"`
    Message string `json:"message"`
}
```

It registers a tag-name function so errors use the JSON name (`display_name`),
not the Go field name (`DisplayName`). That keeps the client contract clean.

## Request structs

```go
type registerRequest struct {
    Username    string `json:"username" validate:"required,min=3,max=30"`
    Email       string `json:"email" validate:"required,email,max=254"`
    Password    string `json:"password" validate:"required,min=8,max=128"`
    DisplayName string `json:"display_name" validate:"omitempty,max=60"`
}
```

Tags are declarative and colocated with the transport shape. The handler does:

```go
if err := h.validate(&req); err != nil {
    h.fail(w, r, err)
    return
}
```

## Pointers for PATCH

```go
type patchPostRequest struct {
    Content  *string `json:"content" validate:"omitempty,max=500"`
    ImageURL *string `json:"image_url" validate:"omitempty,max=500"`
    Status   *string `json:"status" validate:"omitempty,oneof=draft published"`
}
```

A pointer distinguishes:

- field absent → `nil` → do not change
- field present with `""` → non-nil, empty → clear or reject
- field present with a value → update

With a plain `string`, you cannot tell "absent" from "empty". This is the main
reason PATCH request structs use pointers.

`omitempty` on a pointer means "if nil, skip other validations". A non-nil
pointer is validated.

## Custom rules

Not everything fits a tag. `internal/users` has:

```go
func ValidateUsername(username string) error
func ValidateEmail(email string) error
func ValidateHTTPURL(field, raw string) error
```

They return `apperr.Invalid(...)`, so the error shape is identical to validator
failures. Use custom functions when:

- the rule is reused outside HTTP (registration service calls it too)
- the rule needs normalization first
- the error message needs domain context

The registration service calls `ValidateUsername` and `ValidateEmail` again,
after normalization. The handler also validates the request. Duplication is
acceptable here because the service must be safe when called from a CLI or a
test, not just from HTTP.

## Length and Unicode

Go's `len(string)` counts bytes. For user-visible limits, use
`utf8.RuneCountInString` or `len([]rune(s))`:

```go
if len([]rune(content)) > 500 { ... }
```

`validator`'s `max` uses rune count for strings, so tags are usually fine. Custom
code must be explicit.

## Normalization

- Usernames: trim, lowercase.
- Emails: trim, lowercase (the column is `citext`, but normalize anyway so
  tokens and comparisons are consistent).
- Content: trim leading/trailing whitespace; reject empty after trimming.
- Display name: trim; default to username if empty.

Normalize before validating. Otherwise `" Alice "` fails a regex that would
accept `"alice"`.

## Error aggregation

`validator` reports all failed fields in one pass. The API returns all of them:

```json
"details": [
  { "field": "username", "message": "must be at least 3 characters" },
  { "field": "email", "message": "must be a valid email address" }
]
```

Do not stop at the first error; clients hate round trips.

## Exercise

1. Add a `website` field to profile updates with `url` validation. What happens
   for an empty string?
2. Write a custom validator that rejects usernames in a reserved set (`admin`,
   `root`, `me`). Where should it run?
3. Change `max=500` to a rune-aware custom check and test with emoji.
4. Make `PATCH /users/me` reject an empty body. Is that correct for PATCH?
5. Add a test that sends an unknown JSON field and asserts 400.

Next: [13 — Password hashing](13-password-hashing.md)
