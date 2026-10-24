# 23 — Shutdown and health

## Goal

Deploy without dropping requests, and let orchestrators distinguish "alive" from
"ready".

## Signals

`cmd/api` uses `signal.NotifyContext`:

```go
ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
defer stop()
if err := application.Run(ctx); err != nil { ... }
```

- `SIGINT` (Ctrl-C) for local development.
- `SIGTERM` for containers and orchestrators.
- The context is canceled on the first signal. A second signal kills the process
  immediately, which is useful when shutdown hangs.

## Shutdown order

```text
SIGTERM
  → stop accepting new connections (server.Shutdown)
  → wait for in-flight requests up to ShutdownTimeout
  → close notification queue and drain workers
  → close database pool
  → exit 0
```

Why this order:

- Closing the pool first would make in-flight requests fail.
- Draining workers before the pool closes lets them write.
- `server.Shutdown` waits for handlers; if a handler ignores context, it can
  exceed the deadline. The process then exits and the request is lost. That is
  why every handler and query must honor context.

`Run` uses one `shutdownCtx` for both HTTP and workers. If HTTP takes the whole
budget, workers get none. A more forgiving design gives each phase its own
deadline.

## Liveness vs readiness

- `GET /healthz` returns 200 if the process is running. It does not touch the
  database. A liveness probe should restart a hung process, not a process with a
  temporary database blip.
- `GET /readyz` pings PostgreSQL with `READINESS_TIMEOUT`. If it fails, return
  503 so the load balancer stops sending traffic.

Do not make liveness depend on dependencies: a database outage would restart
every instance and make things worse. Do make readiness depend on them.

## Timeouts

| Timeout | Purpose |
| --- | --- |
| `ReadHeaderTimeout` 5s | slowloris protection |
| `ReadTimeout` 15s | whole request read |
| `WriteTimeout` 30s | response write |
| `IdleTimeout` 60s | keep-alive reuse |
| `REQUEST_TIMEOUT` 10s | handler/database deadline |
| `SHUTDOWN_TIMEOUT` 10s | drain budget |
| `READINESS_TIMEOUT` 2s | DB ping |

`WriteTimeout` must exceed `REQUEST_TIMEOUT`, or the server can close the
connection before the handler's deadline fires. Here 30s > 10s.

## Startup

Fail fast:

- config invalid → exit 1 with a clear message
- database unreachable → exit 1
- migrations are a separate command; the API does not migrate on startup

Why not migrate on startup? Multiple replicas would race, and a failed migration
would take down every instance at once. Run migrations as a deploy step.

## Graceful drain and load balancers

`server.Shutdown` stops listeners and waits, but it does not tell the load
balancer to stop sending traffic. In Kubernetes, use a `preStop` sleep or a
readiness gate:

1. Mark not ready.
2. Wait for the load balancer to notice (a few seconds).
3. Send SIGTERM.

Otherwise a request can arrive during the drain and be refused.

## Testing shutdown

The integration suite starts an `app.App` and closes it in `t.Cleanup`. A
dedicated test can:

1. Start the server.
2. Send a request that sleeps.
3. Cancel the context.
4. Assert the request completes and `Run` returns.

This is timing-sensitive; keep the sleep short and the timeout generous.

## Exercise

1. Add a `/healthz` that reports the build version. Should it be public?
2. Make `/readyz` also check the notification queue depth. Is that readiness or
   metrics?
3. Simulate a 15-second handler and a 10-second shutdown. What does the client
   see?
4. Add a `preStop` hook manifest for Kubernetes.
5. Explain why a second SIGTERM should terminate immediately.

Next: [24 — Testing](24-testing.md)
