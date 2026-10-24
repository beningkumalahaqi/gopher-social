# 40. HTTP and template rendering performance

## Goal

Reduce the time between a request arriving and the last byte leaving, without
breaking the cache rules from the previous lessons.

## The response is mostly bytes

An HTML page is a few hundred kilobytes at most, but the browser spends time
parsing, fetching assets, and laying out. Server-side work is only one part of
the user-perceived latency. Optimize in this order:

1. Do not send bytes the client does not need.
2. Send them once.
3. Send them compressed.
4. Render them with fewer allocations.

## Render once, cache the result only when it is safe

The lesson body is parsed at startup and shared. The page around it is rendered
per request because it contains the CSRF token, profile, and progress. That is
correct. Caching the whole HTML would require a key per profile and would make
invalidation harder than the render it saves.

What can be cached safely:

- the rendered Markdown body (already in memory);
- the course and module catalogue (immutable for a deploy);
- static assets (content-hashed URLs).

What cannot:

- the page shell with a CSRF token;
- progress and bookmark controls;
- notes.

Do not "optimize" by caching a page that embeds a token. That is how one user
gets another user's token.

## The template is already compiled

There is no parse per request. The remaining template cost is execution:
walking the template tree, escaping values, and writing to the buffer. Keep
templates simple:

- avoid deeply nested `with` and `range` blocks in hot fragments;
- precompute values in Go instead of calling helpers repeatedly;
- do not call a method that queries the database from a template;
- keep the number of included fragments proportional to what changed.

A template that calls `printf` in a loop for a thousand rows is slower than
formatting once in Go. A template that renders a fixed sidebar on every page is
fine; it is the same amount of work as any other markup.

## Buffers and writes

The server renders into a `strings.Builder`, then writes once. One write is
better than many small writes because each write may acquire a lock and, for a
TLS connection, produce a record. The builder grows geometrically, so the
number of copies is logarithmic in the page size.

If pages become large, estimate the size and call `Grow`:

```go
var out strings.Builder
out.Grow(32 << 10)
```

Do not guess wildly. A 1 MB preallocation for a 20 KB page wastes memory on
every request and increases GC pressure.

## Compression

Text responses compress well. Compression costs CPU per response and must be
negotiated with `Accept-Encoding`. A middleware can wrap the writer with
`gzip` or `zstd` when the client supports it and the content type is
compressible.

Rules:

- do not compress images or already-compressed formats;
- set `Vary: Accept-Encoding`;
- skip compression for very small responses, where the header and CPU cost
  exceed the savings;
- compress static assets at build time when possible, so the server only
  selects a precompressed file.

The platform currently serves static assets without dynamic compression. Adding
it is a measurable change: benchmark the middleware and check the CPU cost at
the expected request rate. Do not assume compression is free.

## Keep-alive and connection reuse

`http.Server` has read, write, and idle timeouts. The current server uses 15s,
30s, and 60s. Keep-alive lets a browser reuse a TCP and TLS connection for
multiple requests, which saves handshakes. It also means each idle connection
holds a file descriptor and some memory. The idle timeout bounds that.

Behind a proxy, the proxy may keep connections to the origin alive even when
the client does not. Tune the origin's idle timeout to be longer than the
proxy's, or the proxy will reuse a connection the origin is closing, producing
intermittent 502s.

## HTTP/2 and head-of-line blocking

HTTP/2 multiplexes requests on one connection. It removes the browser's
six-connections-per-host limit and compresses headers. It also introduces
head-of-line blocking at the TCP layer: one lost packet stalls all streams on
that connection. HTTP/3 (QUIC) addresses that, but adds operational
complexity.

For a small server behind a CDN, the CDN usually terminates HTTP/2 and HTTP/3.
The origin can speak HTTP/1.1 to the proxy. Do not add protocol complexity at
the origin unless the proxy cannot handle it.

## Static asset strategy

The platform serves CSS and JavaScript from the Go binary with content-hashed
URLs. This is simple and avoids a separate static host. The trade-offs:

- every instance serves the same bytes, which is fine at this scale;
- a CDN can cache the hashed URL for a year, which removes the origin from the
  hot path;
- a deploy changes the URL, so no purge is needed.

If the asset set grows, move static files to a CDN and keep only the HTML in
Go. Do not serve large files through a handler that also does session work.

## Measure the whole response

Use a load test that requests a real page, not a health check. Measure:

- time to first byte;
- total response time;
- bytes transferred;
- CPU per request;
- allocation per request.

A change that reduces CPU but increases bytes can be a net loss on a slow
network. A change that reduces bytes but adds a blocking call can be a net loss
for latency. Measure both sides.

## Exercises

1. Compare the response size of a lesson page with and without compression.
2. Render the same page with a preallocated builder and measure allocations.
3. Explain why caching a full HTML page with a CSRF token is unsafe.

## What to remember

Render HTML per request when it contains user state, cache only what is
immutable, write once, compress with a vary header, reuse connections with
bounded idle timeouts, and measure bytes and CPU together.
