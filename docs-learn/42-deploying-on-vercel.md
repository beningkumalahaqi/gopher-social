# 42. Deploying on Vercel

## Goal

Deploy the same router and handlers to Vercel's Go runtime without changing the
application architecture or pretending the platform is a full server.

## What Vercel runs

Vercel's Go framework preset detects a root `go.mod` and one of three
entrypoints: `main.go`, `cmd/api/main.go`, or `cmd/server/main.go`. This
repository has `cmd/api/main.go`, and that file already does the right thing:

```go
application, err := app.New(ctx, cfg, logger)
if err != nil {
    logger.Error("startup failed", "error", err)
    os.Exit(1)
}
if err := application.Run(ctx); err != nil {
    logger.Error("server stopped with error", "error", err)
    os.Exit(1)
}
```

Vercel starts the process and proxies HTTP requests to it. The server must listen
on the port in `PORT`. `config.Load` already handles that:

```go
addr := env("HTTP_ADDR", "")
if addr == "" {
    if port := os.Getenv("PORT"); port != "" {
        addr = ":" + port
    } else {
        addr = ":8080"
    }
}
```

No adapter is needed. The platform runs the same `App.Run` that local
development runs, including graceful shutdown and the background workers.

## The configuration

`vercel.json` at the repository root is intentionally small:

```json
{
  "$schema": "https://openapi.vercel.sh/vercel.json",
  "framework": "go"
}
```

The `framework` preset is required. Without it, Vercel may treat the repository
as static files or fail to find the server entrypoint.

Vercel reads the Go version from the `go` directive in `go.mod` and downloads
that toolchain on first use. Check the current Vercel documentation for the
supported versions before changing the directive. If the runtime does not
support the version, the build fails before the application starts.

## Environment

Set these in the Vercel project:

- `DATABASE_URL` pointing at a pooled connection string;
- `JWT_SECRET` with a strong random value;
- `APP_ENV=production`;
- `DATABASE_MAX_CONNS=3` or another small value;
- `LEARN_PLAYGROUND=true` only if outbound calls to `go.dev` are allowed and the
  rate limit is acceptable;
- `LOG_LEVEL=info`.

`config.Load` reads a `.env` file if present, but Vercel should use real
environment variables. Never commit `.env`.

Migrations do not run at request time. Run them from a CI step or a one-off job
with the same `DATABASE_URL`. A deploy that starts serving before the schema is
ready will fail in ways that look like application bugs.

## Connection pooling

Each warm instance has its own `pgxpool`. Vercel can run many instances, so the
total connection count is instances multiplied by `DATABASE_MAX_CONNS`. That is
why the per-instance pool is small and why the database URL should point at a
pooler (PgBouncer, Neon's pooler, Supabase's pooler).

With transaction pooling, session-level features (advisory locks held across
statements, `SET` that must persist, prepared statement names) can behave
differently. `pgx` uses the extended protocol; confirm the pooler supports it in
the mode you choose.

Do not raise `DATABASE_MAX_CONNS` to make a slow query disappear. Fix the query
or the index. A larger pool moves the queue from Go to PostgreSQL, where it is
harder to see.

## What does not fit

- **Long-lived background work.** The application starts notification workers,
  but the platform is request-driven and may freeze an instance between
  requests. Do not rely on an in-process queue for durable delivery. Use a queue
  service or process the work synchronously when the request can afford it.
- **Local disk.** There is no durable local filesystem. Uploads must go to
  object storage.
- **Long-lived streams.** A function has a maximum duration. Use a platform or
  service designed for persistent connections.
- **In-memory state as a source of truth.** A warm instance can disappear. The
  curriculum is rebuilt from the embedded files, and learner state lives in
  PostgreSQL. The search cache is an optimization, never the record.
- **A ticker as a scheduler.** Use the platform's scheduled jobs, not a
  `time.Ticker` that dies with the instance.

The platform is a good fit for request-response pages and JSON endpoints. It is
a poor fit for a long-running worker. Keep that boundary explicit.

## Cold starts

A cold start includes loading the binary, opening the pool, loading the
curriculum, parsing templates, and building the search index. The curriculum
work is proportional to the corpus but small at this size. The database ping is
the part most likely to dominate.

Ways to reduce it:

- keep the dependency graph small;
- avoid network calls during init that are not required to serve;
- keep templates embedded and parsed at startup;
- use a pooler with low connection latency;
- keep the function warm if the plan supports it, but do not depend on warmth.

Measure the cold start in the platform's logs. Do not guess from local runs.

## Health and readiness

`/healthz` and `/readyz` work through the platform proxy. A readiness check that
pings the database on every call adds load. Use it for deploy gating, not for
every user request.

Liveness should not depend on the database. If it does, a database blip
restarts healthy instances and makes the outage worse.

## Static assets and caching

Static files are embedded in the binary and served with content-hashed URLs and
a one-year immutable cache. The platform's edge can cache those responses, so
the origin is not on the hot path for CSS and JavaScript.

HTML responses are `no-store` because they contain a CSRF token and the current
profile's progress. Do not add a shared cache in front of them.

## Deploy and rollback

A deploy should be:

1. Run migrations that are backward compatible.
2. Build and test.
3. Deploy the new build.
4. Smoke-test `/healthz`, `/learn`, and one write path.
5. Watch error rate and latency.

Backward-compatible migrations mean the old and new code can run at the same
time. Add a column before using it. Do not drop a column in the same deploy that
stops reading it.

Rollback is a redeploy of the previous build. If the migration was destructive,
rollback may not be possible. That is why destructive changes are a separate,
later step.

## Exercises

1. Run the app locally with `PORT=3000` and no `HTTP_ADDR`. Confirm it listens on
   port 3000.
2. Set `DATABASE_MAX_CONNS=1` and observe pool wait under concurrent requests.
3. Explain why a background notifier is not durable on this platform.

## What to remember

Reuse the server entrypoint, let the platform preset start it, keep the pool
small behind a pooler, run migrations outside the request path, and do not
pretend a request-driven platform is a worker.
