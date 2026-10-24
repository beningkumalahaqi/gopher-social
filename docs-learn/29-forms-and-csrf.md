# 29. Forms, validation, and CSRF

## Goal

Handle a write request once, reject bad input before it reaches the store, and
protect state-changing requests without making the API awkward to use.

## One middleware owns the body

The learning server wraps every route with `withSession`. It does four things
before a handler runs:

```go
r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
...
if r.Method != http.MethodGet && r.Method != http.MethodHead {
    if !s.csrf.Valid(r) {
        s.logger.Warn("csrf rejection", "path", r.URL.Path)
        http.Error(w, "Invalid or missing CSRF token.", http.StatusForbidden)
        return
    }
}
```

The size limit stops a client from making the process allocate an unbounded
form. The CSRF check happens before parsing, so a rejected request costs almost
nothing. Handlers can then call `r.ParseForm()` without repeating either check.

Putting this in middleware is not just less code. It makes the secure path the
default. A new handler cannot forget the check unless it is registered outside
the wrapped mux, which is visible in one place.

## Double-submit cookies

The platform uses two cookies:

- `learn_session` is the opaque session token, `HttpOnly`, `SameSite=Lax`.
- `learn_csrf` is a random token, readable by JavaScript.

A form includes the token in a hidden field. HTMX includes it in a header.
The middleware compares the submitted value with the cookie using
`subtle.ConstantTimeCompare`.

The cookie is not a bearer credential on its own. An attacker on another origin
can cause a request to be sent, but cannot read the cookie value to put it in
the form or header. `SameSite=Lax` is a second layer, not the only layer.

Why not put the CSRF token inside the session record? That works, but it makes
every form render depend on a server read. The double-submit cookie keeps the
token available at render time and still binds the request to the browser. The
trade is that the cookie must not be writable by a subdomain you do not
control. For this app, it is host-only.

## Parse once, validate once

Login is the canonical form flow:

```go
func (s *Server) handleLoginPost(w http.ResponseWriter, r *http.Request) {
    if err := r.ParseForm(); err != nil {
        s.loginError(w, r, "Check the form and try again.", http.StatusBadRequest)
        return
    }
    login := strings.TrimSpace(r.FormValue("login"))
    password := r.FormValue("password")
    if login == "" || password == "" {
        s.loginError(w, r, "Enter your username or email and password.", http.StatusBadRequest)
        return
    }
    user, err := s.auth.VerifyCredentials(r.Context(), login, password)
    if err != nil {
        s.loginError(w, r, "Those credentials did not match.", http.StatusUnauthorized)
        return
    }
    ...
}
```

Three rules are visible here:

1. Trim user input before checking it.
2. Use one generic message for a missing account and a wrong password.
3. Do not tell the client which part failed.

The password check lives in `auth.Service.VerifyCredentials`, not in the web
handler. The web layer validates shape and presentation. The auth layer owns
credential rules. `VerifyCredentials` was extracted from `Login` so the web
session flow and the JSON API share one implementation.

## Rate limit before the expensive part

Login is limited per client and route. The limiter is checked before the
password hash is computed, because bcrypt is intentionally expensive. A failed
login returns the same 401 whether the account exists or not, and the limiter
applies to both.

The key is the client IP from `middleware.ClientIP`, which honors
`X-Forwarded-For` and `X-Real-Ip` behind a trusted proxy. Do not key on a
user-supplied header without deciding which proxy sets it. If the app is
directly exposed, those headers are attacker-controlled and the limiter becomes
trivially bypassable.

## Errors must be renderable twice

A form handler has two audiences: HTMX and a plain browser. `loginError`
returns a full page for a normal request and a small fragment for HTMX. The
template is the same; only the wrapper differs.

Validation errors are values, not panics and not log-only events. The handler
keeps the submitted login value in the page data so the user does not retype
it. The password is never echoed back.

## Writes that are idempotent where possible

`POST /learn/bookmarks/{slug}` toggles a bookmark, but it is modeled as a
desired state in the template: add shows "Save", a saved lesson shows "Saved".
The store uses `INSERT ... ON CONFLICT DO NOTHING` and `DELETE`, so a double
submit does not create duplicates.

Progress updates use `GREATEST(existing, new)` for reading position, so a
late-arriving request with an older percentage cannot move the user backwards.
Status changes are explicit values (`in_progress`, `completed`), and a
completed lesson is not downgraded by a reading update.

Idempotence is not a substitute for CSRF, but it makes retries safe. Users
double-click, networks retry, and HTMX can resend after a timeout. A write that
is safe to repeat removes a whole class of support tickets.

## Exercises

1. Send a form post without `csrf_token` and confirm the 403 arrives before
   `ParseForm`.
2. Send a 2 MB body and confirm the `MaxBytesReader` error path returns 400.
3. Add a new field to the note form. Validate it on the server and keep the
   submitted value on error.

## What to remember

One middleware for size and CSRF, double-submit cookies with constant-time
comparison, parse once, validate shape in the handler and rules in the domain,
rate limit before expensive work, and make retries harmless.
