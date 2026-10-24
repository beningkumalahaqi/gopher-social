# 15 — Refresh rotation

## Goal

Keep users logged in without long-lived access tokens, make stolen refresh
tokens detectable, and ensure a database leak does not yield usable tokens.

## Why opaque tokens

A refresh token is not a JWT. It is 32 random bytes:

```text
rt_<base64url(32 random bytes)>
```

The server stores only `SHA-256(token)`. On refresh, it hashes the presented
token and looks it up. Benefits:

- A database dump contains no usable refresh tokens.
- Revocation is a row update, not a denylist of JWTs.
- No claims to validate; the row is the source of truth.

The prefix makes tokens recognizable in logs/secret scanners. Never log the
token itself.

## Schema

```sql
CREATE TABLE refresh_tokens (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash  bytea NOT NULL UNIQUE,
    user_agent  text NOT NULL DEFAULT '',
    expires_at  timestamptz NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),
    revoked_at  timestamptz,
    replaced_by uuid REFERENCES refresh_tokens(id) ON DELETE SET NULL
);
```

- `revoked_at` is nullable; `NULL` means active.
- `replaced_by` records the successor, useful for auditing a chain.
- Index on `user_id` for "revoke all sessions"; index on `expires_at` for cleanup.

## Rotation

Every refresh:

1. Hash the presented token.
2. `SELECT ... FOR UPDATE` the row by hash. The lock serializes concurrent
   refreshes of the same token.
3. If missing → 401.
4. If `revoked_at` is set → **reuse detected**.
5. If expired → 401.
6. Create a new token row.
7. Mark the old row revoked and set `replaced_by`.
8. Commit.
9. Return a new access token and the new refresh token.

```go
stored, err := s.refresh.WithTx(tx).ByHashForUpdate(ctx, hash)
if stored.RevokedAt != nil {
    s.refresh.WithTx(tx).RevokeAllForUser(ctx, stored.UserID)
    tx.Commit(ctx)
    return Session{}, errInvalidRefresh()
}
```

Why `FOR UPDATE`: two requests with the same token can arrive together (client
retry, two tabs). Without a lock, both could read `revoked_at IS NULL`, both
create successors, and both succeed. With the lock, the second sees the revoked
row and triggers reuse handling.

## Reuse detection

A rotated token should never be presented again. If it is, either:

- an attacker stole it and is replaying it, or
- a buggy client is retrying an old token.

Both are handled the same way: revoke **all active sessions for that user** and
return 401. This logs the attacker out too, at the cost of logging the user out
everywhere. For a social app that is acceptable; for a bank you might revoke only
the token chain and force step-up auth.

The response is deliberately indistinguishable from any other invalid refresh
token. Do not tell the caller "reuse detected"; that helps an attacker.

## Logout and password change

- Logout hashes the presented token and sets `revoked_at` if active. It is
  idempotent and returns 204 even for an unknown token.
- Password change verifies the current password, updates the hash, and revokes
  all active sessions. Access tokens remain valid until expiry (≤15 min).

## Cleanup

Expired/revoked rows accumulate. A production job should delete rows where
`expires_at < now() - interval '30 days'`. Do not delete immediately: you lose
the ability to detect reuse of a recently rotated token.

## Client behavior

- Store the refresh token securely.
- Refresh before the access token expires, or on a 401 with code
  `unauthorized`.
- Treat the response as a **pair**: replace both tokens. If you keep the old
  refresh token, the next call triggers reuse detection.
- Serialize refreshes across tabs (e.g. a mutex or a single-flight request).
  Concurrent refreshes are the most common cause of accidental reuse.

## Exercise

1. Simulate two concurrent refreshes with the same token. What statuses do you
   get? Why?
2. Change reuse handling to revoke only the token chain (follow `replaced_by`).
   What are the security trade-offs?
3. Add a cleanup command that deletes old tokens. Which index does it use?
4. Remove `FOR UPDATE` and run the concurrency test repeatedly. Can you produce
   two valid successors?
5. Add a `last_used_at` column and update it on refresh. Does that help
   detection or auditing?

Next: [16 — Auth middleware](16-auth-middleware.md)
