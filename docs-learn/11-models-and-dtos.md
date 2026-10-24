# 11 — Models and DTOs

## Goal

Separate what the database stores from what the API exposes, and make every
field in a response a deliberate decision.

## Two sets of structs

Domain (`internal/users/model.go`):

```go
type User struct {
    ID           uuid.UUID
    Username     string
    Email        string
    PasswordHash string
    DisplayName  string
    Bio          string
    AvatarURL    *string
    CreatedAt    time.Time
    UpdatedAt    time.Time
}
```

DTO (`internal/api/dto/dto.go`):

```go
type PrivateUser struct {
    User
    Email string `json:"email"`
}
```

The domain struct has no JSON tags. `encoding/json` would serialize exported
fields by name (`PasswordHash`!) if it were passed directly. The DTO is a
whitelist.

## Why not tags on the domain model?

It seems convenient, until:

- A store adds `DeletedAt` and it silently appears in API responses.
- A handler needs a different shape (public vs private profile).
- A field is renamed in the database and the API contract changes by accident.
- Tests need to assert JSON separately from domain behavior.

Tags couple storage, transport, and business concerns. The mapping layer is
small and explicit.

## Mapping functions

```go
func NewUser(u users.Public) User { ... }
func NewPrivateUser(u users.User) PrivateUser { ... }
func NewPost(p posts.Post) Post { ... }
```

Each takes a domain value and returns a DTO. There is no reflection. If a field
is missing, the compiler or a test catches it.

## Public vs private

- `GET /users/{username}` returns `Profile`: public fields + stats. No email.
- `GET /users/me` returns `PrivateUser`: includes email.
- `Post.Author` is `UserSummary`: id, username, display name, avatar. No bio,
  no email, no timestamps.

This is not just privacy; it keeps list responses small. A feed of 20 posts
should not embed 20 full profiles.

## Pointers and null

- `*string` means the database column is nullable and JSON should emit `null`.
- `string` with `omitempty` would omit the field entirely, which changes the
  shape. Prefer explicit `null` for optional fields.
- `*time.Time` for `published_at` and `read_at`.
- A zero `uuid.UUID` should never be serialized; it usually indicates a bug.

Be consistent: clients should be able to rely on a field being present with
`null` rather than absent.

## Time

Use `time.Time` in UTC. PostgreSQL `timestamptz` stores an instant; pgx scans it
with a location. The DTO marshals RFC3339. Do not send Unix timestamps and
RFC3339 in different endpoints.

## Evolving the API

- Additive fields are safe. Clients ignore unknown fields.
- Removing or renaming a field is breaking. Version the API (`/v2`) or keep both
  for a deprecation window.
- Changing a type (`int` → `string`) is breaking even if it looks harmless.
- Adding a new enum value can break exhaustive clients. Document it.

Keep `docs/openapi.yaml` in sync. A contract test that validates responses
against the schema is a good next step.

## Exercise

1. Add `website` to the domain user, migration, store, DTO, and PATCH endpoint.
   Which layers must change?
2. Make `GET /users/{username}` accidentally include email by changing the DTO.
   Confirm the integration test or a new test catches it.
3. Design a `PostDetail` DTO that includes the latest three comments. Would you
   add it to the existing endpoint or a new one?
4. Explain why `AvatarURL *string` is better than `string` with `omitempty` for
   this API.
5. Add a `json` tag to a domain struct temporarily and observe what leaks. Do
   not commit it.

Next: [12 — Validation](12-validation.md)
