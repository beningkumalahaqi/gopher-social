# Solutions 03 — Database

## 3.1 Soft delete

```sql
ALTER TABLE posts ADD COLUMN deleted_at timestamptz;
CREATE INDEX posts_live_created_idx
  ON posts (created_at DESC, id DESC)
  WHERE deleted_at IS NULL AND status = 'published';
```

Every read adds `AND deleted_at IS NULL`. Delete becomes
`UPDATE posts SET deleted_at = now() WHERE id = $1 AND author_id = $2`.
Comments and reactions can stay; they are unreachable through the post. Decide
whether direct comment reads must also check the parent.

## 3.2 Rollback test

Wrap registration in `BEGIN`. Insert user, then insert refresh token. To force a
failure, use a test store wrapper that returns an error on the token insert, or
pre-insert a row with the same `token_hash`. After the error, assert
`SELECT count(*) FROM users WHERE email = $1` is 0.

If you remove the transaction, the user row remains and the test fails. That is
the point.

## 3.3 Counters

```sql
BEGIN;
INSERT INTO reactions (post_id, user_id, type) VALUES ($1,$2,$3)
ON CONFLICT (post_id, user_id) DO UPDATE SET type = EXCLUDED.type;
-- only increment when the insert actually added a row
UPDATE posts SET reaction_count = reaction_count + 1 WHERE id = $1;
COMMIT;
```

The hard part is not double counting on conflict. Use `RETURNING (xmax = 0) AS
inserted` or compare the old type first. Reconciliation:

```sql
UPDATE posts p SET reaction_count = (
  SELECT count(*) FROM reactions r WHERE r.post_id = p.id
);
```

Run it periodically, not per request.

## 3.4 Plans

Feed query shape:

```sql
SELECT ... FROM posts p
JOIN follows f ON f.following_id = p.author_id
WHERE f.follower_id = $1 AND p.status = 'published'
ORDER BY p.created_at DESC, p.id DESC
LIMIT $2;
```

Index `follows (follower_id, following_id)` and
`posts (author_id, status, created_at DESC, id DESC)`. Explore orders by
`created_at` across all authors; a BRIN or btree on `(status, created_at DESC)`
helps. Verify with `EXPLAIN (ANALYZE, BUFFERS)`, not by guessing.

## 3.5 Safe NOT NULL

1. `ADD COLUMN x text` (nullable, fast).
2. Backfill in batches: `UPDATE ... WHERE x IS NULL LIMIT 1000` in a loop.
3. `ALTER COLUMN x SET NOT NULL` (takes a brief lock; use a check constraint
   `NOT VALID` then `VALIDATE CONSTRAINT` for large tables).

A constant default on PG 11+ avoids a rewrite, but it still takes an ACCESS
EXCLUSIVE lock briefly. On hot tables, prefer the three-step form.
