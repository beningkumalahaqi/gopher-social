# 25 — Production checklist

This project is a teaching backend, but the gaps between it and a production
service are concrete. Use this as a review checklist, not a badge.

## Security

- [ ] TLS everywhere; redirect HTTP to HTTPS. Set HSTS at the edge.
- [ ] `JWT_SECRET` from a secret manager, ≥32 random bytes, rotated with a plan.
- [ ] Passwords hashed with Argon2id; parameters tuned to ~100–500 ms.
- [ ] Rate limits at the edge for all routes, not just auth. The in-process
      limiter does not survive multiple replicas.
- [ ] CORS configured for known origins if a browser client exists.
- [ ] Security headers reviewed (`CSP`, `HSTS`, `X-Content-Type-Options`).
- [ ] No secrets or PII in logs. Redact `Authorization`, cookies, emails.
- [ ] SQL always parameterized. No string concatenation.
- [ ] Authorization checks in services/SQL, not only handlers.
- [ ] Request body limits and timeouts on every route.
- [ ] Dependency scanning (`govulncheck`) in CI.
- [ ] Backups tested by restoring, not just scheduled.

## Identity

- [ ] Access token TTL short (≤15 min); refresh rotation with reuse detection.
- [ ] Refresh tokens hashed at rest; logout and password change revoke sessions.
- [ ] Account enumeration resisted (same error/timing for unknown user).
- [ ] Breach-password check at registration.
- [ ] Email verification and password reset (not implemented here).
- [ ] Optional MFA for sensitive actions.
- [ ] Admin roles checked server-side, not from a JWT claim that can go stale.

## Data

- [ ] Migrations run as a separate deploy step, with a lock to prevent races.
- [ ] Backward-compatible schema changes (expand/contract) for rolling deploys.
- [ ] Indexes verified with `EXPLAIN` on real data sizes.
- [ ] Connection pool sized against `max_connections`.
- [ ] Slow query logging and `pg_stat_statements` enabled.
- [ ] Retention policy for refresh tokens, notifications, and soft-deleted data.
- [ ] PII inventory and deletion path (GDPR/CCPA).
- [ ] Read replicas for heavy reads if needed; beware replica lag after writes.

## Reliability

- [ ] `/healthz` liveness independent of dependencies.
- [ ] `/readyz` checks the database and any critical dependency.
- [ ] Graceful shutdown with a preStop delay for load balancers.
- [ ] Timeouts: server, request, database, worker jobs.
- [ ] Retries with jitter for idempotent operations only.
- [ ] Circuit breakers for external services.
- [ ] Outbox or queue for side effects that must not be lost.
- [ ] Backpressure policy documented (drop, block, or reject).
- [ ] Load test at expected peak; find the knee before users do.

## Observability

- [ ] Structured JSON logs with request ID, user ID, route, status, duration.
- [ ] Metrics: request rate, error rate, latency percentiles, pool saturation,
      queue depth, job failures, DB connections.
- [ ] Traces across HTTP and SQL (OpenTelemetry).
- [ ] Dashboards for the four golden signals.
- [ ] Alerts on symptoms (error rate, latency, saturation), not every cause.
- [ ] Runbooks for database down, queue full, migration failure, secret rotation.

## Deployment

- [ ] Reproducible build (`-trimpath`, pinned toolchain, module checksums).
- [ ] Container runs as non-root, read-only filesystem, no shell.
- [ ] Config validated at startup; missing env fails fast.
- [ ] Migrations before or after rollout, never concurrently from every replica.
- [ ] Rolling deploy with readiness gates; keep old version compatible.
- [ ] Rollback plan that does not rely on down migrations.
- [ ] Resource requests/limits and horizontal pod autoscaling.
- [ ] Pod disruption budget and anti-affinity for availability.

## API contract

- [ ] OpenAPI is generated or validated in CI.
- [ ] Error codes are stable; messages are not part of the contract.
- [ ] Pagination limits enforced; no unbounded queries.
- [ ] Deprecation policy and versioning.
- [ ] Idempotency keys for unsafe retries (POST payments, etc.).
- [ ] Consistent timestamps (UTC, RFC3339) and IDs (UUID).

## Testing

- [ ] Unit tests for rules and parsers.
- [ ] Integration tests against real PostgreSQL.
- [ ] `-race` in CI.
- [ ] Fuzz parsers.
- [ ] Contract tests for JSON shapes.
- [ ] Load tests for feed and search.
- [ ] Failure injection: DB down, queue full, slow query.

## Things this repo intentionally omits

- media upload/storage and image processing
- email delivery (verification, reset)
- blocking, muting, private accounts
- moderation and reporting
- timeline fan-out and ranking
- distributed rate limiting
- multi-region and read replicas
- soft deletes and audit logs
- full observability stack

Each is a project of its own. The point of the checklist is to know what you are
choosing not to build.

## Final exercise

Pick one omission and design it end to end:

1. API surface and status codes.
2. Schema and indexes.
3. Transaction boundaries and failure modes.
4. Background work and idempotency.
5. Tests, including concurrency.
6. Rollout and rollback.

Write the design doc before the code. That is the job.
