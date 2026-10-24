# Exercises 03 — Database

## 3.1 Soft delete

**Task:** Add `deleted_at` to posts. All reads filter it out; delete sets it
instead of removing the row. Update indexes and tests.

**Done when:** Deleted posts are invisible everywhere, including search, feed,
comments, and reactions.

**Hint:** Partial indexes (`WHERE deleted_at IS NULL`) keep performance.

## 3.2 Transaction rollback

**Task:** Write an integration test that forces the refresh-token insert to fail
during registration (e.g. duplicate hash via a test hook). Assert no user row
exists.

**Done when:** The test fails if you remove the transaction.

**Hint:** You may need to inject a store or use a deliberate constraint
violation.

## 3.3 Counter table

**Task:** Denormalize `reaction_count` and `comment_count` on posts, updated in
the same transaction as the write. Add a reconciliation query.

**Done when:** Concurrent reaction tests still show the correct count.

**Hint:** `UPDATE posts SET reaction_count = reaction_count + 1 WHERE id = $1`
inside the same transaction as the reaction upsert.

## 3.4 Query plan

**Task:** Seed 100k posts and 10k follows. Run `EXPLAIN (ANALYZE, BUFFERS)` for
feed and explore. Add any missing index.

**Done when:** Both use an index scan, not a sequential scan + sort.

**Hint:** Generate data with `generate_series`; do not commit it.

## 3.5 Migration safety

**Task:** Add a `NOT NULL` column to `users` with a default, then rewrite it as
the safe three-step process (nullable → backfill → constraint). Compare lock
behavior on a large table.

**Done when:** You can explain why the naive version can lock writes.

**Hint:** PostgreSQL 11+ avoids rewrites for constant defaults, but older
versions and volatile defaults do not.
