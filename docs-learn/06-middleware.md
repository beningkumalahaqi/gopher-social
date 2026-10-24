# 06 — Middleware

## Goal

Compose cross-cutting behavior without repeating it in 37 handlers.

## The pattern

A middleware is a function from handler to handler:

```go
type Middleware func(http.Handler) http.Handler
```

Usage:

```go
var h http.Handler = mux
h = middleware.JSONErrors(h)
h = middleware.Timeout(cfg.RequestTimeout)(h)
h = middleware.SecurityHeaders(h)
h = middleware.Recover(logger)(h)
h = middleware.Logger(logger)(h)
h = middleware.RequestID(h)
```

The last one applied is the outermost. A request flows:

```text
RequestID → Logger → Recover → SecurityHeaders → Timeout → JSONErrors → mux → handler
```

and the response flows back in reverse.

## Why this order

- **RequestID outermost**: every log line and error body can include it, even
  for a panic or a 404.
- **Logger next**: it must see the final status, including responses written by
  `Recover` or `JSONErrors`.
- **Recover inside Logger**: a panic becomes a 500, and the access log still
  records it.
- **SecurityHeaders before Timeout**: headers are cheap and should be present
  even if the handler times out.
- **Timeout around the mux**: the handler and its database calls share the
  deadline.
- **JSONErrors innermost**: it only rewrites the plain-text 404/405 produced by
  `ServeMux`; it does not need to wrap business handlers.

## Writing a middleware

```go
func SecurityHeaders(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        h := w.Header()
        h.Set("X-Content-Type-Options", "nosniff")
        h.Set("X-Frame-Options", "DENY")
        h.Set("Referrer-Policy", "no-referrer")
        next.ServeHTTP(w, r)
    })
}
```

Rules:

- Call `next.ServeHTTP` exactly once.
- Do not mutate the request unless you own the copy (`r.WithContext`).
- If you write a response, do not call `next`.
- Keep middleware independent of domain packages.

## Response writer wrappers

Wrapping `ResponseWriter` is how you observe status and bytes. Two pitfalls:

1. **Interfaces are lost.** A wrapper that only implements `Write` and
   `WriteHeader` hides `http.Flusher`, `http.Hijacker`, and `io.ReaderFrom`.
   This app does not stream, so it is acceptable. If you add SSE or WebSockets,
   implement `Unwrap() http.ResponseWriter` or forward the interfaces.
2. **WriteHeader can be called once.** Record the first status; ignore later
   calls. If a handler writes a body without calling `WriteHeader`, the implicit
   status is 200 — set your recorded status to 200 before calling `next`.

## Timeout middleware

```go
func Timeout(d time.Duration) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            ctx, cancel := context.WithTimeout(r.Context(), d)
            defer cancel()
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}
```

Important: this cancels the context but does **not** stop a handler that ignores
it. Database calls honor it because `pgx` takes a context. CPU loops do not.
Also, if the handler writes after the deadline, the client may already be gone;
the write fails silently. A stricter implementation uses `http.TimeoutHandler`,
which buffers and returns 503, at the cost of memory and streaming support.

## Panic recovery

```go
defer func() {
    if v := recover(); v != nil {
        logger.Error("panic recovered", "panic", v, "stack", string(debug.Stack()))
        response.Error(w, r, errors.New("panic"))
    }
}()
```

Two rules:

- Log the stack server-side; never send it to the client.
- If headers were already written, you cannot change the status. Accept the
  truncated response and log loudly.

`http.ErrAbortHandler` is special: re-panic it so the server can close the
connection instead of logging noise.

## Auth as middleware

`RequireAuth` parses the bearer token and puts an `auth.Identity` in context.
`OptionalAuth` does the same but allows anonymous requests. Both live in
`internal/api/middleware/auth.go`. The router applies them per route, so
public endpoints do not pay for token parsing.

## Exercise

1. Add a middleware that rejects requests without `Content-Type:
   application/json` on POST/PATCH/PUT. Where should it sit in the chain?
2. Modify `Logger` to include the response's `Content-Type`.
3. Write a middleware that adds `Cache-Control: no-store` to authenticated
   responses. Why is that a good default for this API?
4. Explain why `Timeout` does not guarantee the client receives a 504.
5. Reorder `Recover` outside `Logger`. What changes in the logs for a panic?

Next: [07 — Errors and envelopes](07-errors-and-envelopes.md)
