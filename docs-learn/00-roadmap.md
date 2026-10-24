# Roadmap

This file maps the whole course. It is not a lesson; it is the order and the
reason for the order. Each lesson is a separate Markdown file in this directory
and a page at `/learn/lessons/<slug>` in the running application.

## Who this is for

You can read Go and have built something small. You do not need production
experience. The first course uses the GopherSocial API as the specimen: a real
router, real SQL, real auth, and a test suite that runs against PostgreSQL.

The second course builds the interface you are reading. It uses the same
repository, so every example points at code that exists and runs.

## Course 1: Backend engineering (lessons 01 to 25)

Build and operate the API. The order follows the request path and then the
guarantees that are easy to get wrong.

| Module | Lessons | Focus |
| --- | --- | --- |
| Foundations | 01 to 05 | run the system, package layout, routing, config, logging |
| HTTP engineering | 06 to 12 | middleware, errors, pgx, migrations, transactions, DTOs, validation |
| Identity | 13 to 16 | password hashing, JWTs, refresh rotation, auth middleware |
| Features | 17 to 21 | handlers, pagination, search, social graph, comments and reactions |
| Reliability | 22 to 25 | workers, shutdown and health, testing, production checklist |

Start at [01. Orientation](01-orientation.md). Do the matching exercise after
each module.

## Course 2: Server-rendered interface (lessons 26 to 31)

Build HTML on the server, then add interactivity without a client framework.

| # | Lesson | What you build |
| --- | --- | --- |
| 26 | [Template architecture](26-template-architecture.md) | startup parsing, page data, escaping, buffered render |
| 27 | [Layouts and fragments](27-layouts-and-fragments.md) | the shell, named fragments, two response modes |
| 28 | [HTMX contract](28-htmx-contract.md) | targeted updates that work without JavaScript |
| 29 | [Forms and CSRF](29-forms-and-csrf.md) | one parse, server validation, double-submit tokens |
| 30 | [Content pipeline](30-content-pipeline.md) | Markdown to sanitized HTML, metadata, diagnostics |
| 31 | [Search over your corpus](31-search-over-your-corpus.md) | an in-memory index with explainable ranking |

The interface lives in `internal/learn/web`. Content lives in `docs-learn/`.

## Course 3: Caching (lessons 32 to 36)

Learn the trade before the tool.

| # | Lesson | Core idea |
| --- | --- | --- |
| 32 | [Caching fundamentals](32-caching-fundamentals.md) | hit ratio, staleness budget, cache-aside |
| 33 | [HTTP caching](33-http-caching.md) | immutable URLs, validators, `Vary`, private responses |
| 34 | [An in-process cache](34-in-process-cache.md) | bounds, locks, lazy expiry, FIFO eviction |
| 35 | [Invalidation and coalescing](35-invalidation-and-coalescing.md) | delete on write, versioned keys, singleflight |
| 36 | [Distributed caches](36-distributed-caches.md) | serialization, network failure, operational cost |

The teaching cache is `internal/learn/cache`. The platform uses it for search
results and uses content hashes for static assets.

## Course 4: Performance and deployment (lessons 37 to 43)

Measure, then change one thing at a time, then run it.

| # | Lesson | Core idea |
| --- | --- | --- |
| 37 | [Measure first](37-measure-first.md) | budgets, benchmarks, profiles, percentiles |
| 38 | [Allocations and memory](38-allocations-and-memory.md) | escape analysis, preallocation, GC limits |
| 39 | [Database query optimization](39-database-query-optimization.md) | plans, indexes, keyset pagination, pool limits |
| 40 | [HTTP and rendering](40-http-and-rendering-performance.md) | bytes, compression, keep-alive, static assets |
| 41 | [Concurrency under load](41-concurrency-under-load.md) | bounds, queues, leaks, shutdown |
| 42 | [Deploying on Vercel](42-deploying-on-vercel.md) | the server preset, pooling, platform limits |
| 43 | [Operating a small service](43-operating-a-small-service.md) | logs, probes, migrations, backups, incidents |

## Suggested paths

**Full course.** 01 to 43 in order, one exercise per sitting. The capstone in
`exercises/07-capstone.md` is the final project.

**Interface only.** Read 01, 03, 06, 07, then 26 to 31. You need the router and
middleware context, not the auth internals.

**Performance only.** Read 08, 10, 18, 24, then 32 to 41. The database and
pagination lessons are prerequisites for the query work.

**Deployment only.** Read 04, 09, 23, then 42 and 43.

## What you should be able to do at the end

- Trace a request from the router through middleware, a handler, a store, and
  back as HTML or JSON.
- Explain why a cache key includes the profile's progress and why a CSRF token
  cannot be cached with the page.
- Read a query plan and decide whether an index will help.
- Measure a page before and after a change, and say which number moved.
- Deploy the same binary as a process or a serverless function, and explain the
  trade-offs of each.

## Where things live

| Path | Contents |
| --- | --- |
| `internal/learn/web` | HTTP handlers, templates, static assets |
| `internal/learn` | curriculum loader, Markdown renderer, search index, activities |
| `internal/learn/pgstore` | learner activity persistence |
| `internal/learn/cache` | bounded TTL cache with request coalescing |
| `internal/learn/apispec` | OpenAPI catalogue for the API explorer |
| `docs-learn` | lessons, exercises, solutions, glossary, examples |
| `migrations/000009_learning` | learning tables and indexes |
| `cmd/api/main.go` | server entrypoint used by the Vercel Go preset |

## A note on order

The numbers are a path, not a dependency graph. A later lesson may reference an
earlier one, but every lesson states its prerequisites in the first paragraph.
If you already know the prerequisite, skip it and keep going.
