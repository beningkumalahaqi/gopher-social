# 33. HTTP caching and validators

## Goal

Use the cache that already exists between your server and the user, without
serving one person's page to another.

## The response is the cache policy

HTTP caching is controlled by headers on the response. The important ones:

- `Cache-Control: public, max-age=3600` lets shared caches store the response.
- `Cache-Control: private, max-age=60` allows only the browser to store it.
- `Cache-Control: no-store` forbids storage.
- `ETag` identifies a specific representation.
- `Last-Modified` gives a weaker validator based on time.
- `Vary` lists request headers that change the representation.

If you set none of them, caches may still apply heuristic freshness. Be
explicit. A response with no policy is not "uncached"; it is cached according
to rules you did not choose.

## Static assets: immutable by URL

The platform serves `/learn/static/app.css?v=<hash>`. The hash is derived from
the file contents at startup. The response sets a one-year immutable cache:

```go
w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
```

This is safe because the URL changes when the bytes change. A new deploy
produces a new hash and a new URL. The old URL keeps working for clients that
still hold the old HTML, and no one sees a mixed version.

`immutable` tells the browser not to revalidate even on reload. Use it only
with a content-addressed URL. If the URL is stable, `immutable` turns a bad
deploy into a bug that survives Ctrl+F5.

## HTML: private or no-store

Lesson pages contain a CSRF token, the current profile's progress, and notes.
They must not be stored by a shared cache. The safe default is:

```go
Cache-Control: private, no-store
```

`no-store` is stronger than `no-cache`. `no-cache` allows storage but requires
revalidation before reuse. `no-store` forbids writing the response at all.

Some pages could be public: the course catalogue has no per-user data. Even
there, the layout shows progress and a session identity. Splitting the page
into a public shell and a private fragment would be required before adding
shared caching. Do not set `public` on a response that contains a cookie-scoped
value.

## Validators and 304 responses

For a public, expensive representation, send an `ETag` and handle
`If-None-Match`:

```go
sum := sha256.Sum256(body)
etag := `"` + hex.EncodeToString(sum[:8]) + `"`
w.Header().Set("ETag", etag)
if r.Header.Get("If-None-Match") == etag {
    w.WriteHeader(http.StatusNotModified)
    return
}
w.Header().Set("Cache-Control", "public, max-age=60")
```

The client sends the tag it holds. If the representation is unchanged, the
server returns 304 with no body. The client reuses its copy. This saves
bandwidth, not server render time, because the server still produced the body
to compute the hash. For a static file, hash the file once. For a dynamic page,
compute the validator from the inputs, not from the finished output.

`Last-Modified` is easier but weaker. It has one-second resolution and depends
on a reliable modification time. Prefer `ETag` when correctness matters.

## Vary is part of the key

If the representation depends on `Accept-Encoding`, `Accept-Language`, or a
custom header, list it:

```go
w.Header().Add("Vary", "Accept-Encoding")
```

A shared cache keys on the URL plus the listed headers. Forgetting `Vary`
causes one variant to be served to clients that asked for another. This is the
classic compressed/uncompressed bug.

Cookies are not varied automatically. A shared cache may store a response that
was generated with a session cookie unless the response says `private` or
`no-store`. This is why authenticated HTML should not be public.

## The platform's current policy

Static assets are immutable and content-hashed. HTML pages are session-scoped
and are not marked public. The JSON API sets no explicit cache headers, so the
safe assumption is that intermediaries may apply heuristic caching to GET
responses. If the API is exposed publicly, add `Cache-Control: no-store` to
authenticated endpoints and an explicit policy to public ones such as
`/api/v1/explore`.

The lesson is not to memorize a header set. It is to decide, for each response,
who may store it, for how long, and under which request variations.

## Exercises

1. Inspect the headers for `/learn/static/app.css` and `/learn/lessons/01-orientation`.
   Explain the difference.
2. Add `Vary: Accept-Encoding` to a compressed response and describe what a
   shared cache does with it.
3. Compute an ETag for a public JSON response and return 304 on a matching
   `If-None-Match`.

## What to remember

Caching is a response policy. Content-hash the URL before using `immutable`.
Keep user-specific HTML private or unstored. Use validators for public
representations, and vary on every header that changes the body.
