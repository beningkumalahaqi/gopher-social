# 38. Allocation and memory pressure

## Goal

Reduce the work the garbage collector does, without turning the code into an
unsafe puzzle. Allocation rate matters more than heap size for a request-driven
server.

## What actually allocates

Every request allocates:

- the request and response writer;
- headers and URL parsing;
- the body buffer when the handler reads it;
- the page data and any slices or maps built for it;
- the rendered HTML buffer;
- database rows and their scanned fields.

Some of this is unavoidable. The goal is to remove the allocations that are
large, repeated, or easy to avoid, not to reach zero.

Use the compiler to see escape decisions:

```bash
go build -gcflags='-m=2' ./internal/learn/... 2>&1 | grep -i 'escapes to heap'
```

A value escapes when it outlives the function, is stored in an interface, or is
captured by a goroutine. Returning a pointer to a local struct is fine if the
caller needs it. Allocating a slice of one element per request is often not.

## Preallocate when the size is known

A slice that grows by append reallocates and copies. If the length is known,
size it once:

```go
hits := make([]SearchHit, 0, len(index.docs))
```

The same applies to maps and string builders. `strings.Builder` grows
geometrically; `Grow` avoids the early copies when the size is predictable.
Do not guess a huge capacity: over-allocating wastes memory and can make the
heap larger than the data warrants.

## Avoid accidental copies

A conversion from `[]byte` to `string` copies:

```go
s := string(b) // allocates unless the compiler proves otherwise
```

If the bytes are already immutable and the string is short-lived, a copy is
usually the right call. Do not use `unsafe.String` to avoid it unless you can
prove the bytes are never mutated and outlive the string. The risk is a
use-after-free or silent data corruption, which is not worth a few nanoseconds.

Similarly, `[]byte(s)` copies. For a write to a network connection, use
`io.WriteString` or a `strings.Reader` instead of converting a large string to
bytes.

## Reuse buffers carefully

A `sync.Pool` can reduce allocation for short-lived, fixed-shape buffers such
as JSON encoders or template buffers:

```go
var bufPool = sync.Pool{
    New: func() any { return new(bytes.Buffer) },
}
buf := bufPool.Get().(*bytes.Buffer)
buf.Reset()
defer bufPool.Put(buf)
```

Rules:

- always `Reset` before use;
- never return a buffer that is still referenced;
- do not pool huge buffers, because the pool can retain memory;
- measure before and after. A pool adds synchronization and can be slower for
  cheap objects.

The template renderer writes to a `strings.Builder` per request. Pooling it
would save one allocation and add a lifetime bug if the string were returned
without copying. The current code chooses clarity, which is correct until a
profile says otherwise.

## Page data and view models

Handlers often build a fresh slice of view structs for each request. That is
fine when the slice is small. It becomes expensive when it duplicates a large
collection for every page.

Options:

- render directly from the domain slice when the fields match;
- reuse a precomputed immutable view for content that never changes;
- paginate earlier so the slice is bounded.

The curriculum is immutable, so the course and module views could be built
once. The handlers currently derive them per request, which is simple and
cheap at this scale. If the catalogue grows, precompute it at startup and keep
the derived data immutable.

## Strings and the curriculum

Lesson bodies are stored as `template.HTML`, which is a string. They are parsed
once and shared by every request. Do not copy a lesson body into page data just
to add a wrapper. Pass the pointer to the document or the existing string.

Search builds lowercase copies of title, text, and code at startup. That
doubles the text memory. It is a deliberate trade: lowercasing on every query
would allocate per request and be slower. At this corpus size the memory is
trivial; at a larger size, consider a case-insensitive index structure or
storing only the normalized form.

## Database rows

`pgx` can scan into a struct or into individual variables. Scanning into
individual variables avoids reflection and temporary maps. For a handful of
columns, it is also clearer.

Watch for these allocation traps:

- selecting `*` and scanning into a map;
- building a query string with `fmt.Sprintf` per row;
- converting every column to `any` before scanning;
- loading all rows when the handler only needs the first.

Prefer a typed `Scan` and a query that selects only the columns used.

## GC tuning is not a substitute

`GOGC` controls the heap target. `GOMEMLIMIT` sets a soft memory limit. Raising
`GOGC` reduces GC frequency and increases memory. Lowering it does the
opposite.

These are global knobs. They do not fix an allocation hot spot, and they can
make latency worse if the process is memory-constrained. Set `GOMEMLIMIT` when
running in a container with a hard limit, so the runtime knows the ceiling.
Then profile the allocations before touching `GOGC`.

## A practical loop

1. Record the allocation profile for the route.
2. Pick the top allocation site.
3. Change one thing.
4. Re-run the benchmark and the profile.
5. Keep the change only if the improvement is measurable and the code is still
   readable.

## Exercises

1. Run an allocation profile for a lesson page and name the top three sites.
2. Preallocate one slice in a handler and measure the difference with
   `-benchmem`.
3. Explain why `unsafe.String` on a reused buffer is unsafe.

## What to remember

Measure allocation rate, preallocate known sizes, avoid needless copies, pool
only with a clear lifetime, and never trade correctness for a micro-optimization
you have not measured.
