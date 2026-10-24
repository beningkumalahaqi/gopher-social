# 19 — Search

## Goal

Search text in PostgreSQL without adding Elasticsearch, while keeping queries
fast and injection-safe.

## Two search shapes

- **Users**: prefix match on username, plus full-text on username + display name.
- **Posts**: full-text on content, published only.

## Full-text search basics

PostgreSQL turns text into a `tsvector` (normalized lexemes) and a query into a
`tsquery`. The `@@` operator asks "does this document match this query".

```sql
SELECT ...
FROM posts
WHERE to_tsvector('english', content) @@ websearch_to_tsquery('english', $1)
```

- `english` applies stemming and stop words (`running` matches `run`).
- `simple` does no stemming; it is better for usernames and exact tokens.
- `websearch_to_tsquery` accepts user-friendly syntax (`golang -rust`, quoted
  phrases) and never errors on malformed input. Prefer it to `to_tsquery`,
  which raises syntax errors.

## Indexes

The migration creates expression indexes:

```sql
CREATE INDEX posts_search_idx ON posts
    USING GIN (to_tsvector('english', content));

CREATE INDEX users_search_idx ON users
    USING GIN (to_tsvector('simple', username || ' ' || display_name));
```

The query must use the **same expression** for the index to be used. If you
write `to_tsvector('simple', content)` against the `english` index, Postgres
falls back to a sequential scan. Generated columns avoid this class of mistake:

```sql
ALTER TABLE posts ADD COLUMN search tsvector
  GENERATED ALWAYS AS (to_tsvector('english', content)) STORED;
CREATE INDEX posts_search_idx ON posts USING GIN (search);
```

This repo uses expression indexes to keep the schema smaller; either is fine,
but be consistent.

## Users query

```sql
WHERE username LIKE $1
   OR to_tsvector('simple', username || ' ' || display_name) @@ websearch_to_tsquery('simple', $2)
ORDER BY username ASC
LIMIT $3 OFFSET $4
```

`$1` is `query + "%"`, so `ali` matches `alice`. The OR means a user can match by
prefix even if FTS does not match. The `LIKE` prefix can use a B-tree index on
`username`; FTS uses the GIN index.

Parameterized queries prevent SQL injection. Never concatenate the query into
SQL. `websearch_to_tsquery` also handles quotes and operators safely.

## Posts query

```sql
WHERE p.status = 'published'
  AND to_tsvector('english', p.content) @@ websearch_to_tsquery('english', $2)
ORDER BY p.created_at DESC, p.id DESC
LIMIT $3 OFFSET $4
```

Search returns published posts only, even to their author. Drafts are private
working state, not searchable content. If you want author-only draft search, add
`AND (p.status = 'published' OR p.author_id = $viewer)`.

## Ranking

This query orders by recency, not relevance. For relevance:

```sql
ORDER BY ts_rank_cd(to_tsvector('english', p.content), query) DESC, p.created_at DESC
```

Ranking adds CPU and makes cursor pagination harder (rank is not a stable unique
key). For a social feed, recency is usually the right default.

## Validation

- `q` must be 2–100 characters. Shorter queries produce noisy matches and can be
  expensive.
- Empty/whitespace → 422.
- `limit`/`offset` use the same bounds as other lists.

## Search at scale

- GIN indexes are large and slow to update. For write-heavy tables, consider a
  separate search replica or an external engine.
- `to_tsvector` at query time recomputes for every row unless the index matches.
- For typo tolerance, use `pg_trgm` with a trigram index, or an external engine.
- For multilingual content, choose the right text search configuration per row
  or store a language column.

## Exercise

1. Add `pg_trgm` and make user search tolerate one typo. Which index do you
   need?
2. Change posts search to rank by relevance, then design a stable cursor for
   it. Why is `(rank, id)` problematic?
3. Add a generated `search` column and switch the query. Compare `EXPLAIN`.
4. Make drafts searchable by their author only. What changes in SQL and tests?
5. Try `q='` and `q='golang &'`. Does the API return 422 or an empty page? Why?

Next: [20 — Feed and social graph](20-feed-and-social-graph.md)
