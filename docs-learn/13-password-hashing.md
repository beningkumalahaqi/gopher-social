# 13 — Password hashing

## Goal

Store passwords so that a database leak does not hand attackers usable
credentials, and so that login timing does not reveal which accounts exist.

## Never encrypt passwords

Encryption is reversible; password storage must not be. Use a slow, salted,
memory-hard hash. This project uses Argon2id, the winner of the Password Hashing
Competition and the current default recommendation.

## Parameters

```go
const (
    argonMemory  = 64 * 1024 // KiB = 64 MiB
    argonTime    = 3
    argonThreads = 2
    argonKeyLen  = 32
    argonSaltLen = 16
)
```

- **Memory** makes GPU/ASIC cracking expensive.
- **Time** is iterations.
- **Threads** is parallelism within one hash.
- **Salt** is random per password, so identical passwords produce different
  hashes and rainbow tables fail.

Tune for your hardware: a login should take roughly 100–500 ms. Measure, do not
guess. Store the parameters in the hash so you can raise them later and rehash on
next login.

## PHC format

```text
$argon2id$v=19$m=65536,t=3,p=2$<base64 salt>$<base64 hash>
```

`decodeHash` parses all fields and rejects:

- wrong algorithm (`argon2i`, `bcrypt`)
- unsupported version
- missing/zero parameters
- salt shorter than 8 bytes
- hash shorter than 16 bytes

This matters: accepting arbitrary parameters from the database can turn a
compromised row into a denial-of-service (e.g. `m=4GiB`).

## Verification

```go
got := argon2.IDKey(password, salt, params.time, params.memory, params.threads, uint32(len(want)))
return subtle.ConstantTimeCompare(got, want) == 1
```

`subtle.ConstantTimeCompare` avoids leaking how many bytes matched. The hash
itself is not secret, but constant-time comparison is cheap and removes a class
of bugs.

## Timing attacks on login

If an unknown user returns immediately and a known user takes 200 ms, an attacker
can enumerate accounts. The fix:

```go
user, err := s.users.ByLogin(ctx, login)
if errors.Is(err, apperr.ErrNotFound) {
    verifyDummy(in.Password)
    return Session{}, errInvalidCredentials()
}
```

`verifyDummy` runs the same Argon2id work against a fixed hash. The response body
and status are identical for unknown user and wrong password.

Caveats:

- Network jitter dominates; timing defenses are probabilistic, not perfect.
- Rate limiting is still required. This app limits auth endpoints per IP.
- Do not log the login identifier at info level.

## Password policy

Current rules:

- 8–128 characters.
- No composition rules (uppercase/digit/symbol). Length beats composition.
- Change password requires the current password and rejects reuse of the same
  password.
- Registration does not enforce a breach-list check. In production, check
  against Have I Been Pwned's k-anonymity API or a local list.

Do not silently truncate passwords. Argon2 handles arbitrary length, but the
128-character cap prevents memory abuse.

## Upgrading hashes

Because the parameters are encoded, you can:

1. Raise the constants.
2. On successful login, if the stored parameters are below current, rehash the
   plaintext and update the row.
3. Optionally rehash in a background job.

This repo does not implement rehashing; the hook is in `Login` after a
successful `VerifyPassword`.

## What not to do

- Do not use SHA-256/MD5 alone.
- Do not use a global salt.
- Do not compare hashes with `==`.
- Do not return different errors for "no such user" and "wrong password".
- Do not put password hashes in DTOs or logs.
- Do not hash on the client and call it security; the server must hash.

## Exercise

1. Lower `argonMemory` to 8 MiB and measure login latency. Why is this a bad
   production choice?
2. Implement rehash-on-login when parameters differ. Which function must expose
   the stored parameters?
3. Add a test that the same password hashed twice produces different strings but
   both verify.
4. Time 20 logins for an existing and a missing user. Is the difference within
   noise?
5. Explain why `dummyHash` must be a valid Argon2id string with the same
   parameters, not a random constant.

Next: [14 — JWT access tokens](14-jwt-access-tokens.md)
