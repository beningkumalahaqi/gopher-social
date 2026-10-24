# Solutions 07 — Capstone

There is no single diff. Here is the review rubric and the traps per option.

## Bookmarks

- Table `bookmarks (user_id, post_id, created_at)` with PK `(user_id, post_id)`.
- `INSERT ... ON CONFLICT DO NOTHING` for idempotency.
- List: join posts, filter `deleted_at IS NULL` and visibility, order by
  `bookmarks.created_at DESC, post_id DESC`.
- Index `(user_id, created_at DESC, post_id DESC)`.
- Do not notify the author; bookmarks are private.
- Trap: returning the bookmark row instead of the post.

## Blocks

- Table `blocks (blocker_id, blocked_id, created_at)`.
- On block: delete follows in both directions inside one transaction.
- Every visibility query needs `NOT EXISTS (SELECT 1 FROM blocks ...)`.
- Feed, search, comments, reactions, profile, followers, notifications.
- Trap: enforcing only in the feed; blocked users still comment.
- Decide 404 vs empty list and document it.

## Hashtags

- `tags (id, name unique)`, `post_tags (post_id, tag_id)`.
- Normalize to lowercase, strip punctuation, cap length, dedupe per post.
- Extract in the service after validation; store in the same transaction.
- Backfill with a batched job; idempotent upserts.
- Index `post_tags (tag_id, post_id)`.
- Trap: regex with nested quantifiers on user input.

## Preferences

- Table `notification_preferences (user_id, type, enabled)`.
- Defaults: missing row means enabled.
- Worker checks before insert; one query per job or a cached map refreshed on
  update.
- API: `GET` returns all types with effective values; `PATCH` upserts.
- Trap: caching forever; invalidate on write or use a short TTL.

## Review checklist

- [ ] Authorization predicate includes the actor, not just the object ID.
- [ ] Unique constraints back every idempotent write.
- [ ] Index supports filter + sort, verified with `EXPLAIN`.
- [ ] Migration has a down path and does not lock a hot table.
- [ ] Integration test covers 401, 403, 404, and success.
- [ ] Concurrency test covers duplicate writes.
- [ ] OpenAPI and `docs/api-examples.md` updated.
- [ ] Rollback does not require a destructive down migration.
