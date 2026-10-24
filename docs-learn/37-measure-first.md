# 37. Measure before you optimize

## Goal

Replace guesses with a profile, a benchmark, and a latency distribution. The
first rule of performance work is that the bottleneck is rarely where you
think.

## Start with a budget

A performance goal needs a number and a scope:

- "The lesson page renders in under 50 ms at p95 on this machine."
- "A search request allocates less than 200 KB."
- "The server handles 200 requests per second with a p99 under 100 ms."

Without a budget, optimization never ends. With one, you know when to stop and
which changes are worth keeping.

State the workload too. Latency under one request per second is not the same as
latency under a hundred. Averages hide the tail, and the tail is what users
remember.

## Measure the request path

The API logs method, path, status, bytes, and duration for every request. That
is enough to find a slow route, not enough to explain it. Add metrics for:

- request count and error rate by route;
- latency percentiles, not just the mean;
- database query count and duration per request;
- cache hits, misses, and evictions;
- goroutine count and heap size over time.

You do not need a metrics platform to start. A structured log line with a
request id and a duration is already a measurement. Aggregate it offline.

## Benchmarks answer narrow questions

A Go benchmark measures a function with a controlled input:

```go
func BenchmarkSearch(b *testing.B) {
    idx := buildIndex()
    b.ReportAllocs()
    for i := 0; i < b.N; i++ {
        idx.Search("cache", SearchFilters{}, nil)
    }
}
```

Run it with:

```bash
go test ./internal/learn -run '^$' -bench Search -benchmem
```

`-benchmem` reports bytes and allocations per operation. Allocation count often
matters more than bytes: each allocation is work for the garbage collector.

Benchmarks are not load tests. They run one function without HTTP, middleware,
TLS, or a database. Use them to compare two implementations of the same logic,
not to predict production throughput.

## Profiles show where time goes

A CPU profile answers "which functions consumed CPU". A heap profile answers
"which code allocated". An allocation profile is often more useful for a web
server, because garbage collection is driven by allocation rate.

```bash
go test ./internal/learn -run '^$' -bench Search -cpuprofile cpu.out -memprofile mem.out
go tool pprof -top cpu.out
go tool pprof -top -alloc_space mem.out
```

For a running server, `net/http/pprof` can expose profiles on an internal
listener. Do not mount it on the public mux without authentication and a
separate port. A profile endpoint is a debugging tool, not a feature.

Read the profile from the top down. If 60 percent of CPU is in the database
driver, optimizing template rendering is irrelevant. If most allocations come
from one conversion, fix that conversion.

## Load tests need a realistic model

A load generator that requests one cached URL in a tight loop measures the
cache, not the system. Model:

- a mix of routes weighted like real traffic;
- think time between requests;
- authenticated and anonymous users;
- cache hit and miss paths;
- a database with realistic row counts.

Run long enough to reach steady state. The first seconds include connection
setup, JIT-like warmup in some runtimes, and cold caches. Report the steady
state separately.

Watch for coordinated omission: if the generator waits for each response before
sending the next, a slow response reduces the request rate and hides the tail.
Use a fixed arrival rate or open-loop generation when tail latency matters.

## The database is usually the wall

Before optimizing Go code, check:

- how many queries one request runs;
- whether those queries use indexes;
- whether the connection pool is saturated;
- whether a query returns more rows than the handler uses.

`EXPLAIN (ANALYZE, BUFFERS)` is the primary tool. A sequential scan on a small
table is fine; a sequential scan on a large table under load is not. An index
that is never used still costs writes.

Add a query counter in development. A page that runs one query is a different
design from a page that runs forty in a loop.

## Percentiles, not averages

If 99 requests take 10 ms and one takes 2 seconds, the average is about 30 ms.
That number describes no user. Report p50, p95, and p99, and keep the maximum
with its request id.

A p99 that is ten times the p50 usually means a queue, a lock, a garbage
collection pause, or a cold cache. The fix depends on which one, so measure
before changing the code.

## Keep a record

For each optimization, write down:

- the hypothesis;
- the measurement before;
- the change;
- the measurement after;
- the cost (complexity, memory, staleness).

Without the before number, you cannot tell whether the change helped. Without
the cost, the next maintainer cannot decide whether to keep it.

## Exercises

1. Benchmark `SearchIndex.Search` with `-benchmem` and identify the largest
   allocation.
2. Add a query counter to one handler and count queries for a lesson page.
3. Run a load test with a fixed arrival rate and compare p50 and p99.

## What to remember

Set a budget, measure the real path, use benchmarks for narrow comparisons and
profiles for causes, load test with a realistic model, and report the tail.
