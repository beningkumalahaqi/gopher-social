# 10 — Stores and transactions

## Goal

Keep SQL in stores, rules in services, and make multi-step writes atomic.

## Three layers, three jobs

```text
handler  → decode, validate shape, choose status
service  → business rules, authorization, transaction boundary
store    → SQL, scanning, driver error translation
```

A store method should be understandable as a query. If it contains an `if` about
who is allowed to do something, that rule belongs in the service or SQL
visibility clause.

## Example: registration

`auth.Service.Register` must create a user and a refresh token together. If the
token insert fails, the user must not exist.

```go
tx, err := s.db.Begin(ctx)
if err != nil { ... }
defer tx.Rollback(ctx)

user, err := s.users.CreateTx(ctx, tx, ...)
if err != nil { return Session{}, err }

refresh, err := s.refresh.CreateTx(ctx, tx, user.ID, ...)
if err != nil { return Session{}, err }

if err := tx.Commit(ctx); err != nil { ... }
```

Why not `CreateUserWithToken` in one store? Because users and refresh tokens are
different domains. The service composes them; the transaction is the service's
responsibility.

## Visibility in SQL

Draft visibility is not a post-processing filter:

```sql
WHERE p.id = $1
  AND (p.status = 'published' OR p.author_id = $2)
```

If the row is not visible, the store returns `ErrNotFound`. Doing this in SQL
means there is no code path that loads a draft and forgets to check. It also
lets the database use the index.

`AdminByID` exists for PATCH: the author must be able to load their own draft to
edit it. It deliberately omits the published filter, and the service checks
ownership before updating.

## Upserts

Reactions use the composite primary key:

```sql
INSERT INTO reactions (post_id, user_id, type)
VALUES ($1, $2, $3)
ON CONFLICT (post_id, user_id)
DO UPDATE SET type = EXCLUDED.type, updated_at = now()
RETURNING ...
```

This is atomic. A read-then-write in Go would race: two requests could both see
"no reaction" and both insert, with one failing on the unique key. The upsert
also makes `PUT` idempotent, which matches HTTP semantics.

Follows are similar:

```sql
INSERT INTO follows (follower_id, following_id)
VALUES ($1, $2)
ON CONFLICT DO NOTHING
```

The service still checks that the target exists and is not the caller, because
those produce meaningful errors. The `ON CONFLICT` handles the concurrent
duplicate.

## Counting without N+1

Post lists need author, comment count, reaction count, and viewer reaction. The
query joins and aggregates in one round trip:

```sql
SELECT p.id, ..., u.username,
       (SELECT count(*) FROM comments c WHERE c.post_id = p.id) AS comment_count,
       (SELECT count(*) FROM reactions r WHERE r.post_id = p.id) AS reaction_count,
       (SELECT r.type FROM reactions r WHERE r.post_id = p.id AND r.user_id = $viewer) AS viewer_reaction
FROM posts p
JOIN users u ON u.id = p.author_id
```

Correlated subqueries are fine for page-sized result sets. For very large counts,
denormalize counters and update them in the same transaction as the write.

The alternative — load 20 posts, then 20 author queries, then 20 count queries —
is the N+1 problem. It is easy to write and painful under load.

## Authorization in services

```go
post, err := s.store.AdminByID(ctx, id)
if post.AuthorID != userID {
    return apperr.Wrap(apperr.ErrForbidden, "you can only edit your own posts")
}
```

Do not rely on the handler to check ownership. Handlers can be added later, and
one forgotten check is a vulnerability.

## Transactions and external calls

Never send an email or call a third-party API inside a transaction. The
transaction holds locks and a connection while the network is slow. Use an
outbox table or a background job instead.

This repo's notifications are enqueued after the database write, outside the
transaction. That means a crash between commit and enqueue can lose the
notification. It is an accepted trade-off for a learning project; a production
system would use an outbox or transactional enqueue.

## Exercise

1. Move the "cannot follow yourself" check into SQL. What error do you get, and
   is it as clear as the service check?
2. Add a `DELETE /posts/{id}` path that also deletes notifications in the same
   transaction. Is a cascade simpler?
3. Write a store method that returns posts with their latest comment. Avoid
   N+1.
4. Explain why `defer tx.Rollback(ctx)` is safe after a successful commit.
5. Find a read-then-write pattern in the code and decide whether it needs an
   upsert or a `SELECT ... FOR UPDATE`.

Next: [11 — Models and DTOs](11-models-and-dtos.md)
