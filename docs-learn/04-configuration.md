# 04 — Configuration

## Goal

Load configuration once, validate it at startup, and pass it explicitly. No
package should read `os.Getenv` except `internal/config`.

## The contract

`internal/config.Config` is a plain struct. `Load()` resolves values in this
order:

1. Defaults in code.
2. `.env` in the working directory, if present.
3. Real environment variables (they win).

Then `Validate()` checks the result and returns one error listing every problem.

```bash
go run ./cmd/api
# invalid configuration: JWT_SECRET is required
```

Failing at startup is much better than a nil map panic on the first request.

## Durations, not magic numbers

Env values use `time.ParseDuration`:

```bash
ACCESS_TOKEN_TTL=15m
REFRESH_TOKEN_TTL=720h
REQUEST_TIMEOUT=10s
```

Code receives `time.Duration`, never strings. Parsing happens once.

## Secrets

- `JWT_SECRET` must be at least 32 characters. Generate one:

  ```bash
  openssl rand -base64 48
  ```

- Never commit `.env`. `.gitignore` already excludes it. Commit
  `.env.example` with placeholders.
- In production, inject secrets through the platform's secret store, not a
  file in the image.
- Rotating the JWT secret invalidates every access token immediately. Plan for
  a short overlap if you need zero downtime (key ID + multiple keys), or accept
  a 15-minute re-login storm.

## Database URL

`pgxpool` accepts a URL:

```text
postgresql://user:password@host:port/database?sslmode=disable
```

For production, use `sslmode=require` (or stricter) and a least-privilege user.
The app needs DDL only for migrations, not at runtime. Run migrations as a
separate job with a migration user; the API user needs `SELECT/INSERT/UPDATE/
DELETE` only.

## Tuning knobs

| Variable | Default | Effect |
| --- | --- | --- |
| `DATABASE_MAX_CONNS` | 10 | pool ceiling; keep `instances × conns < Postgres max_connections` |
| `REQUEST_TIMEOUT` | 10s | handler/database deadline |
| `SHUTDOWN_TIMEOUT` | 10s | how long to drain HTTP and workers |
| `MAX_BODY_BYTES` | 1048576 | request body cap |
| `AUTH_RATE_LIMIT` / `AUTH_RATE_WINDOW` | 10 / 1m | per-IP auth throttle |
| `NOTIFICATION_QUEUE_SIZE` / `NOTIFICATION_WORKERS` | 256 / 2 | worker capacity |
| `READINESS_TIMEOUT` | 2s | DB ping deadline for `/readyz` |
| `LOG_LEVEL` | info | debug/info/warn/error |

## Testing config

`internal/config/config_test.go` covers defaults, overrides, invalid durations,
and the secret length rule. Tests that need a config use a literal struct
instead of calling `Load()`, so they do not depend on the developer's `.env`.
That is why `tests/main_test.go` constructs `config.Config{...}` directly.

If you add a field:

1. Add it to `Config`.
2. Parse it in `Load` with a default.
3. Validate it.
4. Add it to `.env.example`.
5. Add a test for the invalid case.
6. Update this table.

## Exercise

1. Add `FEATURE_COMMENTS_ENABLED` (bool, default true) to config and log it at
   startup. Which parser helper do you need?
2. Make `Load` reject an `HTTP_ADDR` without a colon. Is that validation worth
   it?
3. Change the default `DATABASE_MAX_CONNS` to 1 and run the integration suite.
   What fails, and why?
4. Explain why `loadDotEnv` refuses to override an existing environment
   variable.
5. Read `config_test.go` and add a case for `JWT_SECRET` of exactly 32
   characters.

Next: [05 — Structured logging](05-structured-logging.md)
