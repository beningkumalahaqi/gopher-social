# Exercises 07 — Capstone

Pick one feature and ship it end to end. The point is the whole slice: schema,
SQL, service, HTTP, tests, docs, rollout.

## Option A — Bookmarks

Users save posts privately.

- `POST /posts/{id}/bookmark`, `DELETE`, `GET /bookmarks`
- private by default; no notifications
- keyset pagination by save time
- idempotent writes

## Option B — Blocks

Users block other users.

- blocked users cannot follow, comment, react, or appear in feed/search
- existing follows are removed on block
- every visibility query must respect blocks
- decide what the blocked user sees (404 vs empty)

## Option C — Hashtags

Extract `#tags` from posts and browse them.

- `GET /tags/{tag}/posts`
- normalization, uniqueness, indexing
- backfill existing posts
- avoid regex catastrophic backtracking

## Option D — Notifications preferences

Per-user, per-type opt-out.

- `GET/PATCH /notifications/preferences`
- workers check preferences before insert
- cache preferences or accept a query per job
- defaults for existing users

## Requirements for any option

1. **Design doc** (1 page): API, schema, failure modes, rollout.
2. **Migration** with up/down and indexes.
3. **Store + service** with business rules and error mapping.
4. **Handlers + DTOs** with validation and OpenAPI updates.
5. **Tests**: unit for rules, integration for HTTP + concurrency.
6. **Docs**: API examples and a short architecture note.
7. **Review**: run `make lint`, `make test`, `make test-race`.

## Grading yourself

- Does it work under concurrent requests?
- Is authorization enforced below the handler?
- Are errors consistent (codes, statuses)?
- Did you add an index for every new query pattern?
- Can you roll it back without data loss?
- Did you update the contract and the course docs?

## Stretch

Add one of:

- a background backfill with progress and idempotency
- a feature flag to dark-launch
- a load test and a documented capacity number
- an outbox for a side effect that must not be lost
