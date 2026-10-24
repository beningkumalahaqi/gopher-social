# 21 — Comments and reactions

## Goal

Model one-to-many comments and one-per-user reactions with correct permissions
and race-free writes.

## Comments

```sql
CREATE TABLE comments (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    post_id    uuid NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    author_id  uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    content    text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT comments_content_length CHECK (char_length(content) BETWEEN 1 AND 1000)
);
```

Permissions:

- **Create**: any authenticated user, but only on a published post they can see.
  `posts.ByID(ctx, postID, nil)` returns 404 for drafts and missing posts.
- **Update**: only the comment author. The service checks; the store also scopes
  the `UPDATE` by `author_id`.
- **Delete**: the comment author **or** the post author (moderation). The service
  loads the comment with `PostAuthorID` and checks both.

Returning 403 for "comment exists but you cannot edit" is fine: the comment is
public. Returning 404 for a comment on a hidden draft is consistent with post
visibility.

## Reactions

```sql
CREATE TABLE reactions (
    post_id    uuid NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    user_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type       text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (post_id, user_id),
    CONSTRAINT reactions_type_valid CHECK (type IN ('like', 'love', 'laugh'))
);
```

One row per user per post. Changing a reaction updates the row; it does not
insert a second one.

### Upsert

```sql
INSERT INTO reactions (post_id, user_id, type)
VALUES ($1, $2, $3)
ON CONFLICT (post_id, user_id)
DO UPDATE SET type = EXCLUDED.type, updated_at = now()
```

`PUT` semantics: idempotent create-or-replace. `DELETE` removes the row and is
also idempotent.

### Notification only on real change

```go
previous, _ := s.store.Get(ctx, postID, userID)
s.store.Upsert(...)
changed := previous == nil || *previous != t
if changed && post.AuthorID != userID { enqueue }
```

Without the `changed` check, a client that re-sends the same reaction would spam
the author. The read-then-write is not atomic, but the worst case is a duplicate
notification, not data corruption. If that matters, use `RETURNING` with a
comparison in SQL or a unique notification key.

### Counts

The summary query aggregates in one round trip:

```sql
SELECT count(*),
       count(*) FILTER (WHERE type = 'like'),
       count(*) FILTER (WHERE type = 'love'),
       count(*) FILTER (WHERE type = 'laugh'),
       (SELECT type FROM reactions WHERE post_id = $1 AND user_id = $2)
FROM reactions WHERE post_id = $1
```

`FILTER` is standard SQL and clearer than `SUM(CASE WHEN ...)`.

## Race conditions

The integration suite hammers both endpoints concurrently:

- 24 goroutines `PUT` different reaction types for the same user/post.
- 16 goroutines follow the same user.

Assertions:

- exactly one reaction row,
- exactly one follow row,
- exactly one follow notification,
- all requests return success.

Why the upsert wins: the database serializes conflicting inserts on the unique
key. A Go-level "check then insert" would let two requests both see no row and
both try to insert; one gets a unique violation, which you would have to treat
as success anyway.

## Counts and drift

`comment_count` and `reaction_count` are computed by subqueries in post queries.
They cannot drift because they are not stored. The cost is per-row subqueries;
for large pages, denormalize and update counters in the same transaction as the
write, then add a reconciliation job.

## Pagination

Comments use offset pagination ordered by `created_at ASC, id ASC` (conversation
order). Reactions are ordered by `created_at DESC, user_id`.
They are bounded and rarely deep. For a very active post, switch comments to
keyset.

## Exercise

1. Add emoji reactions. What changes in the check constraint, DTO enum, and
   validation?
2. Make reaction notifications fire only when the type changes from nothing to
   something, not between types. Is that better UX?
3. Add a `GET /comments/{id}` endpoint. Who may see a comment on a draft?
4. Implement comment editing history. Which table and indexes?
5. Run the concurrency test with `-race` and `-count=10`. Does it stay green?

Next: [22 — Background workers](22-background-workers.md)
