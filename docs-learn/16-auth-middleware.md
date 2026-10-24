# 16 — Auth middleware

## Goal

Turn an `Authorization: Bearer ...` header into a typed identity in context,
with correct 401/403 behavior and no business logic.

## Parsing

```go
header := strings.TrimSpace(r.Header.Get("Authorization"))
scheme, raw, ok := strings.Cut(header, " ")
if !ok || !strings.EqualFold(scheme, "Bearer") || strings.TrimSpace(raw) == "" {
    return apperr.Wrap(apperr.ErrUnauthorized, "missing or malformed bearer token")
}
```

- Scheme comparison is case-insensitive (`Bearer`, `bearer`).
- Exactly one space is required by this parser; multiple spaces fail. That is
  acceptable and strict.
- Empty token fails before hitting the parser.

Then `ParseAccessToken` validates signature, algorithm, issuer, audience, and
expiry. Any failure becomes the same 401 message; do not tell the client which
check failed.

## Identity in context

```go
type Identity struct {
    UserID    uuid.UUID
    SessionID uuid.UUID
}

func WithIdentity(ctx context.Context, id Identity) context.Context
func IdentityFrom(ctx context.Context) (Identity, bool)
func UserIDFrom(ctx context.Context) (uuid.UUID, bool)
```

Use an unexported key type to avoid collisions:

```go
type contextKey struct{}
```

Context values are for request-scoped data, not dependencies. Do not put a
database, logger, or config in context.

## RequireAuth vs OptionalAuth

- `RequireAuth` rejects missing/invalid tokens with 401 and always attaches an
  identity.
- `OptionalAuth` allows anonymous requests, but a **present and invalid** token
  is still rejected. Silently ignoring a bad token hides client bugs and can
  surprise users with a logged-out view.

Routes choose explicitly in `router.go`:

```go
mux.Handle("GET /api/v1/feed", auth(d.Feed.Feed))
mux.Handle("GET /api/v1/explore", opt(d.Feed.Explore))
```

## 401 vs 403

- **401**: no valid identity. The client should authenticate or refresh.
- **403**: valid identity, but not allowed. Re-authenticating will not help.

Examples:

- No token on `/feed` → 401.
- Bob edits Alice's post → 403 (he can see it).
- Bob reads Alice's draft → 404 (existence is hidden).

## Why auth is per-route

Authentication is not free: JWT parsing, UUID parsing, and (in the future) a
denylist lookup. Public endpoints like `/explore`, `/users/{name}`, and
`/healthz` should not pay for it. Per-route middleware also makes the policy
visible in one file.

## Logging identity

The outer logger runs before auth, so it cannot read the identity from the
request context directly. `internal/api/middleware/auth.go` uses a small
`identitySlot` pointer created by the logger and filled by auth. This is a
deliberate exception to "no mutable context values": it lets the access log
include `user_id` without moving auth outside the router.

If you dislike the slot, alternatives are:

- log identity inside handlers (duplicated),
- log in an outer middleware that parses the token again (wasteful),
- emit a separate auth log line (two lines per request).

## Security notes

- Do not trust `X-User-ID` or any client-supplied identity header.
- Do not accept tokens from query strings; they end up in logs and Referer.
- Reject tokens with `alg: none` (the parser does).
- Keep the signing secret out of logs and error messages.
- If you add a denylist, check `sid` after signature validation, and cache it.

## Exercise

1. Add a middleware that requires the `sid` claim to be present. It already is;
   where is it checked?
2. Make `OptionalAuth` ignore invalid tokens instead of rejecting. What breaks
   in the client contract?
3. Add an `admin` check based on a claim. Why is a database role safer?
4. Write a test for `Authorization: bearer <valid>` (lowercase) and
   `Bearer  <valid>` (double space).
5. Explain why `IdentityFrom` returns `(Identity, bool)` instead of a zero
   value on failure.

Next: [17 — Handlers](17-handlers.md)
