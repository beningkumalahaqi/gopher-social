# 18 — Pagination

## Goal

Return large collections in pages without duplicates, skips, or unbounded
queries — and choose the right strategy per endpoint.

## Two strategies

| | Offset | Keyset (cursor) |
| --- | --- | --- |
| Query | `LIMIT n OFFSET m` | `WHERE (created_at, id) < (?, ?)` |
| Jump to page N | easy | not possible |
| Stable under inserts | no | yes |
| Cost at deep pages | scans/skips m rows | index seek |
| Used by | lists, search, followers | feed, explore |

## Offset pagination

```sql
SELECT ... ORDER BY created_at DESC, id DESC LIMIT $1 OFFSET $2
```

Fetch `limit+1` rows to compute `has_more` without `COUNT(*)`:

```go
out, _ := collectPosts(rows, limit)
hasMore := len(out) > limit
if hasMore { out = out[:limit] }
```

Response metadata:

```json
{
  "limit": 20,
  "offset": 0,
  "has_more": true,
  "next_offset": 20
}
```

Validation: `1 ≤ limit ≤ 100`, `offset ≥ 0`. Malformed numbers are 422, not
silently defaulted. `limit=0` is rejected.

Why offset is fine here: post lists are filtered and bounded; users rarely jump
deep. It is simple and supports "page 3".

Why offset is wrong for feeds: a new post shifts every row. The client asks for
offset 20 and sees a post it already saw, or misses one. Under active inserts,
duplicates are guaranteed eventually.

## Keyset pagination

Order by a unique, stable tuple: `(created_at DESC, id DESC)`. The cursor encodes
the last row's values:

```text
base64url("2026-10-24T10:00:00.123456789Z|<uuid>")
```

Query:

```sql
WHERE ($2::timestamptz IS NULL OR (p.created_at, p.id) < ($2::timestamptz, $3::uuid))
ORDER BY p.created_at DESC, p.id DESC
LIMIT $4
```

Row comparison is lexicographic: older timestamp, or same timestamp with a
smaller UUID. `id` breaks ties, so the order is total and no row is skipped.

Response:

```json
{ "limit": 20, "next_cursor": "MjAyNi0xMC0yNFQxMDowMDowMFp8..." }
```

`next_cursor` is `null` on the last page. The client must treat it as opaque and
send it back unchanged.

## Why encode the cursor

- Clients cannot easily construct invalid positions.
- You can change the internal format without breaking clients.
- It signals "do not interpret this".

The cost: you must validate it. `pagination.Decode` checks base64, timestamp,
and UUID, returning 400 on failure. An unsigned cursor is fine here because it
contains no secret and cannot grant access; the query still applies visibility
rules. If cursors encoded authorization decisions, sign them.

## Indexes

For the feed query, the database needs:

```sql
CREATE INDEX posts_published_created_idx
    ON posts (created_at DESC, id DESC) WHERE status = 'published';
CREATE INDEX posts_author_created_idx
    ON posts (author_id, created_at DESC, id DESC);
-- follows has PRIMARY KEY (follower_id, following_id), which covers the join.
```

Without the composite index, Postgres may sort the whole table. Check with
`EXPLAIN (ANALYZE, BUFFERS)`.

For filtered lists (`author_id`, `status`), a composite index in the same order
as the filter and sort helps:

```sql
CREATE INDEX posts_author_created_idx ON posts (author_id, created_at DESC, id DESC);
```

## Limits and abuse

- Always cap `limit`. Without a cap, `?limit=1000000` is a memory exhaustion
  vector.
- Do not allow client-controlled `ORDER BY`; map an enum to fixed SQL.
- Deep offset is still expensive. If you need deep navigation, switch to keyset
  or restrict the maximum offset.

## Testing pagination

The integration test creates 5 posts and walks `limit=2` until `next_cursor` is
null. It asserts:

- every post appears exactly once,
- no duplicates across pages,
- the total equals the number created.

It also checks that `limit=0`, `limit=101`, `offset=-1`, and a bad cursor fail
with the right status.

## Exercise

1. Add a `before` timestamp filter to explore. Does it compose with the cursor?
2. Change the cursor to include `status`. Why is that unnecessary for explore?
3. Write a benchmark for `pagination.Decode` and `Encode`.
4. The post list service already rejects offsets above 10,000. Should the
   cursor endpoints have a similar guard?
5. Explain why `(created_at, id)` must be unique for keyset pagination.

Next: [19 — Search](19-search.md)
