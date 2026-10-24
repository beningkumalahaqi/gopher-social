# 14 — JWT access tokens

## Goal

Issue short-lived, verifiable access tokens without a database lookup on every
request — and understand exactly what that buys and costs.

## What a JWT is

Three base64url segments: header, payload, signature.

```text
eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.
eyJzdWIiOiI4Yz...Iiwic2lkIjoiLi4uIn0.
<signature>
```

The payload is **not encrypted**. Anyone can read it. The signature only proves
it was issued by someone with the secret and has not been modified.

Never put secrets, emails, or personal data in claims.

## Claims used

```go
type AccessClaims struct {
    jwt.RegisteredClaims
    SessionID uuid.UUID `json:"sid"`
}
```

- `sub`: user UUID
- `sid`: refresh-token row UUID (links the access token to a session)
- `iss`: `JWT_ISSUER`
- `aud`: same as issuer
- `iat`, `nbf`, `exp`: issued/not-before/expiry
- `jti`: random UUID, useful for future denylisting

## Signing

HS256 (HMAC-SHA256) with a shared secret. It is symmetric: the same secret signs
and verifies. That is fine for a single service. For multiple services where
verifiers should not be able to mint tokens, use RS256/EdDSA with a private key
for signing and public keys for verification.

The secret must be at least 32 random bytes. `openssl rand -base64 48`.

## Verification

```go
parser := jwt.NewParser(
    jwt.WithValidMethods([]string{"HS256"}),
    jwt.WithIssuer(issuer),
    jwt.WithAudience(issuer),
    jwt.WithExpirationRequired(),
    jwt.WithLeeway(30*time.Second),
)
```

Each option matters:

- **ValidMethods** prevents algorithm confusion. Without it, a token with
  `alg: none` or `alg: HS256` signed with the public key of an RS256 setup can
  be accepted.
- **Issuer/Audience** prevent a token from another environment or service being
  reused.
- **ExpirationRequired** rejects tokens with no `exp`.
- **Leeway** tolerates small clock skew. Keep it small (30s), not minutes.

The key function also checks `token.Method` explicitly. Belt and suspenders.

## TTL

15 minutes. Short enough that a stolen token has limited value, long enough to
avoid constant refreshes. The client refreshes proactively using the refresh
token.

Trade-off: revocation is not instant. Logout and password change revoke refresh
sessions, but the access token remains valid until `exp`. If you need immediate
revocation:

- check `sid` against a denylist/cache in middleware, or
- use opaque session tokens with a lookup, or
- keep TTL very short (1–2 minutes) and accept more refresh traffic.

This project chooses the stateless option and documents the window.

## Clock skew and `nbf`

`NotBefore` is set to `now`. A client whose clock is behind may see the token as
not yet valid; `WithLeeway` absorbs that. Do not set `nbf` in the future unless
you have a reason.

## Token size

Claims are small, so the header is ~200–300 bytes. Do not add roles/permissions
that can grow unbounded; fetch authorization data server-side or keep it
compact.

## Storage on the client

- Browser SPA: an httpOnly, Secure, SameSite cookie is safer against XSS than
  localStorage, but introduces CSRF concerns. This API returns tokens in JSON,
  so the client decides.
- Mobile: platform secure storage (Keychain/Keystore).
- Never put tokens in URLs or logs.

## Exercise

1. Decode a token at jwt.io (use a throwaway secret) and identify every claim.
2. Remove `WithValidMethods` and craft an `alg: none` token. Confirm it is
   rejected when the option is present. Restore it.
3. Change the access TTL to 10 seconds and observe a 401 after expiry. How does
   the client recover?
4. Add a `role` claim and a middleware that requires `admin`. What happens when
   a user's role changes before the token expires?
5. Explain why `aud` should not be a user-controlled value.

Next: [15 — Refresh rotation](15-refresh-rotation.md)
