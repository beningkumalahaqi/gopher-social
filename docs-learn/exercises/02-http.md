# Exercises 02 — HTTP

## 2.1 Request ID propagation

**Task:** Accept a client `X-Request-ID` only if it matches
`^[A-Za-z0-9_-]{8,64}$`. Otherwise generate one. Add tests for valid, too short,
and invalid characters.

**Done when:** The response always has an ID, and logs match it.

**Hint:** `requestid.Sanitize` already exists; tighten it.

## 2.2 Content-Type guard

**Task:** Reject `POST`, `PATCH`, and `PUT` without
`Content-Type: application/json` with 415. Allow requests with no body.

**Done when:** Existing tests pass, and a new middleware test covers the guard.

**Hint:** Where in the chain should this run? Before or after auth? Explain.

## 2.3 405 envelope

**Task:** Send `DELETE /api/v1/posts` and inspect the response. Ensure the
`Allow` header survives `JSONErrors`. Add an integration test.

**Done when:** Status is 405, body is the error envelope, `Allow` contains
`GET, POST`.

**Hint:** `errorWriter` forwards headers; check whether `Allow` is set before
`WriteHeader`.

## 2.4 Pagination headers

**Task:** Add `X-Total-Count` to `GET /api/v1/posts`. This requires a `COUNT(*)`
query. Measure the cost on 100k rows.

**Done when:** You can explain why the repo does not do this by default.

**Hint:** `has_more` avoids the count; total counts are expensive and often
unnecessary.

## 2.5 Problem+JSON

**Task:** Add an optional `Accept: application/problem+json` response format
(RFC 9457) while keeping the current envelope for other clients.

**Done when:** The same error yields both shapes, with tests.

**Hint:** This is an API design exercise; consider whether two formats are worth
the maintenance.
