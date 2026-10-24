# 05 — Structured logging

## Goal

Produce logs that a machine can query and a human can read, without leaking
secrets or personal data.

## `log/slog`

Go 1.21 added `log/slog`. The app builds one logger in `cmd/api`:

```go
handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel})
if cfg.IsProduction() {
    handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel})
}
logger := slog.New(handler)
```

- Development: text, easier to scan.
- Production: JSON, one object per line, ready for Loki/CloudWatch/Datadog.
- The logger is passed explicitly. There is no package-level `slog.Info`.

## Request logging

`middleware.Logger` wraps the response writer to capture status and bytes:

```go
type statusWriter struct {
    http.ResponseWriter
    status int
    bytes  int
}

func (w *statusWriter) WriteHeader(code int) {
    w.status = code
    w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
    n, err := w.ResponseWriter.Write(b)
    w.bytes += n
    return n, err
}
```

Then one line per request:

```go
logger.Info("http request",
    "method", r.Method,
    "path", r.URL.Path,
    "status", sw.status,
    "bytes", sw.bytes,
    "duration_ms", time.Since(start).Milliseconds(),
    "request_id", requestid.From(r.Context()),
    "remote_addr", r.RemoteAddr,
)
```

Design choices:

- Log after the handler returns, so status and duration are final.
- Use `duration_ms` as a number, not a formatted string, so it can be
  aggregated.
- Do not log query strings by default; they may contain tokens or search terms.
  If you need them, redact.
- Do not log request bodies. Passwords and refresh tokens live there.

## Levels

| Level | Use |
| --- | --- |
| Debug | SQL parameters in local debugging, cache hits |
| Info | startup, shutdown, request completed, state changes |
| Warn | dropped notification, rate limit, recoverable degradation |
| Error | failed operation that needs attention; include `err` |

`logger.Error("create post", "err", err)` keeps the error as a structured
attribute. `slog` renders it with its message; `errors.Is/As` still works if you
keep the value.

## Context

If you need per-request fields beyond the request log, derive a logger:

```go
logger := base.With("request_id", requestid.From(ctx))
```

Do not put a logger in the context by default. Passing it explicitly is clearer
and avoids hidden dependencies. The one exception is middleware that must log
after the handler; there, read the request ID from context.

## PII and secrets

Never log:

- passwords, password hashes, refresh tokens, access tokens
- full email addresses in production (hash or mask if needed)
- `Authorization` headers
- raw request bodies

Do log:

- user IDs (UUIDs), not emails
- route templates or paths with IDs (they are not secret)
- error codes, not stack traces from dependencies

## Startup and shutdown logs

```text
level=INFO msg="server starting" addr=:8080 env=development
level=INFO msg="shutdown signal received" signal=terminated
level=INFO msg="http server stopped"
level=INFO msg="notification workers stopped"
level=INFO msg="database pool closed"
```

If a deploy fails, these lines tell you how far it got.

## Exercise

1. Add `user_id` to the request log when an identity is present. Which
   middleware must run before `Logger` for this to work? (Hint: currently auth
   runs after logging.)
2. Change the production handler to include a `service` attribute on every
   line.
3. Make `Logger` skip `/healthz` and `/readyz` to reduce noise. Is that always
   desirable?
4. Write a log line for a dropped notification with the event type and target
   user ID, but no content.
5. Find one place in the code that logs an error with `logger.Error`. What
   context is missing that would help debug it?

Next: [06 — Middleware](06-middleware.md)
