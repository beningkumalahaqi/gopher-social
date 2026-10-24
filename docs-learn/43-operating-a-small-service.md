# 43. Operating a small service

## Goal

Keep the service understandable when it is healthy and diagnosable when it is
not. Operations is part of the design, not a phase after launch.

## Know the process

At startup the API:

1. loads configuration and validates it;
2. opens a bounded PostgreSQL pool and pings it;
3. loads the curriculum, activities, and OpenAPI document;
4. builds the router and the learning server;
5. starts the notifier workers;
6. listens for HTTP.

If any step fails, the process exits with a non-zero status and a structured
log line. That is the right behavior: a partially initialized server is worse
than no server. In a container, the platform restarts it; on Vercel, the
invocation fails and the next request retries the cold start.

Write down the order. During an incident, "which dependency failed first" is
the first question.

## Logs are for questions

A log line should answer a question you will actually ask:

- Which route is slow?
- Which request id produced this error?
- Which user or profile was affected, without logging secrets?
- Did the database reject the query, or did the context time out?

The request logger includes method, path, status, bytes, duration, request id,
and remote address. Errors include the operation and the underlying error. Do
not log passwords, tokens, cookies, or full request bodies.

Use structured fields, not string interpolation. `slog` with a JSON handler
makes the fields searchable. A message like `"failed"` without the error value
is not useful.

## Health and readiness are different

- **Liveness** answers "should this process be restarted?" It should not depend
  on the database. A database outage should not cause a restart loop.
- **Readiness** answers "can this instance serve traffic?" It can check the
  database with a short timeout.

The platform uses them differently. A failed liveness check restarts the
instance. A failed readiness check removes it from the load balancer. Mixing
them causes outages that are hard to explain: a slow database restarts every
instance and makes the outage worse.

## The dashboard

Track a small set of signals:

- request rate by route and status class;
- latency p50, p95, p99;
- error rate, split by 4xx and 5xx;
- database pool active, idle, and waiting;
- database query duration;
- cache hit ratio, misses, evictions;
- goroutine count;
- heap size and GC pause;
- notification queue depth.

Four golden signals are enough to start: traffic, errors, latency, saturation.
Add the rest when a specific question needs them.

Alert on symptoms users feel, not on every internal counter. A high goroutine
count with normal latency is not an incident. A rising 5xx rate is.

## Migrations

Migrations are code. They belong in version control, run in order, and are
reviewed like code. The `cmd/migrate` tool applies them up or down.

Rules:

- every migration has a working down path, or a documented reason it does not;
- migrations are backward compatible during a rolling deploy;
- destructive changes happen after the code stops using the old shape;
- large backfills run separately, in batches, with a way to resume;
- never edit a migration that has already run in production.

A migration that takes a lock on a large table can stall every request. Check
the lock level and run it during a low-traffic window, or use a concurrent index
build.

## Backups and restore

A backup you have never restored is a hope, not a backup. Test a restore into a
fresh database and run the application against it. Verify that migrations,
seed data, and at least one read path work.

Know two numbers:

- **RPO**: how much data you can lose. This is the backup interval.
- **RTO**: how long a restore takes. This is the outage budget.

If the database is managed, confirm that point-in-time recovery is enabled and
that the retention matches the RPO. If it is self-hosted, test the restore
procedure on a schedule.

## Security operations

- Rotate `JWT_SECRET` with a plan for existing tokens. Rotating invalidates
  every access and refresh token unless you support overlapping keys.
- Keep dependencies updated. Run `go list -m -u all` and review changes.
- Run `govulncheck` in CI. A dependency with a known reachable vulnerability is
  an incident waiting to happen.
- Set security headers at the edge or in middleware. The app sets a baseline;
  confirm it survives the proxy.
- Rate limit authentication and expensive endpoints. The limits are per
  instance; a distributed attacker needs a shared limiter or an edge rule.
- Use TLS everywhere, including database connections when the provider
  supports it.

## Deploy checklist

Before:

- tests pass, including `-race` for concurrent code;
- migrations reviewed and backward compatible;
- configuration diff reviewed;
- rollback build identified.

After:

- `/healthz` and `/readyz` return success;
- a read path and a write path are smoke-tested;
- error rate and latency are at baseline;
- logs show no new warnings;
- a synthetic check covers the critical page.

Keep the checklist short enough that it is actually used. A twenty-item list
that people skip is worse than six items they complete.

## Incident notes

When something breaks, write a timeline:

- when it started, in UTC;
- what users saw;
- what changed recently;
- what was ruled out;
- what fixed it;
- what will prevent it.

Do not assign blame. The useful output is a change to code, configuration,
monitoring, or process. If the same incident happens twice, the first fix was
incomplete.

## Cost and capacity

Know the cost drivers:

- database size and IOPS;
- instance count and cold starts;
- egress;
- log volume;
- external calls such as the playground.

A cache that saves a database query but adds a service can increase cost. A
larger instance can be cheaper than a complex optimization. Measure the bill
against the latency budget, not in isolation.

## Exercises

1. Write the startup order and the failure behavior for each step.
2. Define liveness and readiness for this service. Which one checks the
   database?
3. Restore a backup into a fresh database and run one read path.

## What to remember

Make startup explicit, log for questions, separate liveness from readiness,
alert on user-visible symptoms, treat migrations as code, test restores, and
write down what you learned from every incident.
