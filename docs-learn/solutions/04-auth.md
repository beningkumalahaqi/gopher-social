# Solutions 04 — Auth

## 4.1 Sessions

```sql
SELECT id, user_agent, created_at, expires_at
FROM refresh_tokens
WHERE user_id = $1 AND revoked_at IS NULL AND expires_at > now()
ORDER BY created_at DESC;
```

The current session is `sid` from the access token. Revoking another session is
`UPDATE refresh_tokens SET revoked_at = now() WHERE id = $1 AND user_id = $2`.
Never accept a session ID without the user ID in the predicate.

## 4.2 Rehash

Parse the PHC string to get `m`, `t`, `p`. If below current constants, after a
successful verify:

```go
hash, _ := password.Hash(plain)
store.UpdatePasswordHash(ctx, userID, hash)
```

Do it asynchronously if login latency matters, but only after success. Never
rehash on a failed login.

## 4.3 Binding

Store `sha256(userAgent + "|" + ipPrefix)` in the token row. On refresh, compare.
Risk: mobile IP changes and browser UA updates log users out. A softer option is
to treat a mismatch as a signal (require re-auth for sensitive actions) rather
than a hard reject.

## 4.4 Denylist

```go
type Denylist struct {
	mu sync.Mutex
	m  map[string]time.Time
}

func (d *Denylist) Add(sid string, until time.Time) { ... }
func (d *Denylist) Has(sid string) bool { ... }
```

Middleware checks `sid` after signature validation. A background goroutine
evicts expired entries. Multi-instance needs Redis; otherwise a logout on one pod
does not affect another.

## 4.5 Reset flow

- `POST /auth/password/forgot` always returns 202, whether or not the email exists.
- Token: 32 random bytes, base64url; store SHA-256 only; expire in 15–30 min.
- `POST /auth/password/reset` consumes the token in a transaction, updates the
  hash, revokes all refresh tokens.
- Single use enforced with `UPDATE ... WHERE used_at IS NULL RETURNING id`.
- Do not log the raw token outside development.
