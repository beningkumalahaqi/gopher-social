# Exercises 04 — Auth

## 4.1 Session list

**Task:** Add `GET /api/v1/auth/sessions` returning active refresh sessions
(user agent, created, expires, current flag). Add `DELETE /auth/sessions/{id}`.

**Done when:** A user can revoke another device without logging out everywhere.

**Hint:** `sid` in the access token identifies the current session.

## 4.2 Rehash on login

**Task:** If a stored hash uses weaker parameters than the current constants,
rehash the password after a successful login and update the row.

**Done when:** Old hashes upgrade transparently; tests cover it.

**Hint:** Expose the parsed parameters from `password.go`.

## 4.3 Refresh token binding

**Task:** Bind refresh tokens to the user agent or IP prefix. Reject a refresh
from a different context.

**Done when:** You can explain the false-positive risk (mobile networks, browser
updates).

**Hint:** Store a hash of the binding, not the raw value.

## 4.4 Access token denylist

**Task:** Revoke access immediately on logout by storing `sid` in a short-TTL
denylist checked in middleware.

**Done when:** A logged-out access token gets 401 before its `exp`.

**Hint:** Use an in-memory map with expiry for the exercise; discuss Redis for
multiple instances.

## 4.5 Password reset

**Task:** Design and implement a reset flow: request by email, single-use token,
expiry, revoke sessions.

**Done when:** The token is hashed at rest, single-use, and expires. No account
enumeration in the request endpoint.

**Hint:** Do not send email in this project; log the token in development only.
