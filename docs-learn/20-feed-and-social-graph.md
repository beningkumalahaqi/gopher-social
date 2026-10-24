# 20 — Feed and social graph

## Goal

Serve a chronological feed from a follow graph without N+1 queries or offset
drift, and understand when fan-out-on-write is worth it.

## The graph

```sql
CREATE TABLE follows (
    follower_id  uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    following_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at   timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (follower_id, following_id),
    CONSTRAINT follows_no_self_follow CHECK (follower_id <> following_id)
);
```

- Composite PK prevents duplicate edges and makes "does A follow B?" an index
  lookup.
- The reverse index (`following_id, created_at DESC, follower_id`) serves
  follower lists and counts.
- Self-follow is blocked in the database and in the service (clearer error).

## Follow is idempotent

```sql
INSERT INTO follows (follower_id, following_id)
VALUES ($1, $2)
ON CONFLICT (follower_id, following_id) DO NOTHING
```

`RowsAffected() == 1` means this request created the edge. Only then enqueue a
notification. Concurrent duplicate requests are harmless: one inserts, the rest
no-op, and only one notification is emitted.

## Feed query

```sql
SELECT ...
FROM posts p JOIN users u ON u.id = p.author_id
WHERE p.status = 'published'
  AND (
    p.author_id = $1
    OR EXISTS (
      SELECT 1 FROM follows f
      WHERE f.follower_id = $1 AND f.following_id = p.author_id
    )
  )
  AND ($2::timestamptz IS NULL OR (p.created_at, p.id) < ($2::timestamptz, $3::uuid))
ORDER BY p.created_at DESC, p.id DESC
LIMIT $4
```

Properties:

- One query, no N+1.
- Includes the viewer's own posts.
- Drafts excluded, even the viewer's own drafts. A feed is a reading surface,
  not a draft manager.
- Keyset pagination on `(created_at, id)`.
- `EXISTS` is efficient with the `follows` PK.

`EXPLAIN (ANALYZE, BUFFERS)` should show an index scan on
`posts_published_created_idx` and a semi-join on `follows`. If it shows a
sequential scan and a sort, the index or statistics are missing.

## Explore

Explore is the public firehose:

```sql
WHERE p.status = 'published'
  AND (cursor condition)
ORDER BY p.created_at DESC, p.id DESC
LIMIT $4
```

No follow join. A viewer token only adds `viewer_reaction` via the correlated
subquery. This is the simplest possible discovery surface.

## Fan-out on read vs write

This design is **fan-out on read**: the feed is computed at request time by
joining follows and posts.

| | Fan-out on read | Fan-out on write |
| --- | --- | --- |
| Write cost | low | high (insert into every follower's timeline) |
| Read cost | join per request | one timeline lookup |
| Freshness | immediate | eventual |
| Celebrity problem | none | one post × millions of followers |
| Storage | none extra | duplicated post IDs per follower |

For a learning project and moderate scale, read fan-out is correct. At millions
of users, a hybrid is common: fan-out on write for normal users, pull for
celebrities.

If you add a timeline table:

```sql
CREATE TABLE timeline (
    user_id uuid,
    post_id uuid,
    created_at timestamptz,
    PRIMARY KEY (user_id, created_at DESC, post_id)
);
```

Writes become a background job; reads become a single index range scan. You then
need backfill, repair, and consistency tooling.

## Counts

Profile stats (`posts`, `followers`, `following`) are computed on demand:

```sql
SELECT
  (SELECT count(*) FROM posts WHERE author_id = u.id AND status = 'published'),
  (SELECT count(*) FROM follows WHERE following_id = u.id),
  (SELECT count(*) FROM follows WHERE follower_id = u.id)
FROM users u WHERE u.id = $1
```

For large profiles, cache or denormalize these. Counters must be updated in the
same transaction as the edge/post change, or they drift.

## Blocking and privacy

This project has no blocks or private accounts. Adding them changes every feed
query and visibility check. Design it as a central `can_view(viewer, post)`
predicate in SQL, not scattered `if` statements.

## Exercise

1. Add a `following_only` flag to explore? Why is that just the feed?
2. Implement "who to follow" using mutual follows. Which indexes help?
3. Add a `timeline` table and a worker that fans out new posts. What happens
   when a user follows someone after posts exist?
4. Measure the feed query with 100k posts and 1k follows. What does `EXPLAIN`
   show?
5. Explain why including the viewer's own posts in the feed is a product
   decision, not a technical one.

Next: [21 — Comments and reactions](21-comments-and-reactions.md)
