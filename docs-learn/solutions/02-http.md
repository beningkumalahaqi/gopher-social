# Solutions 02 — HTTP

## 2.1 Request ID

```go
var validID = regexp.MustCompile(`^[A-Za-z0-9_-]{8,64}$`)

func sanitize(id string) string {
	if validID.MatchString(id) {
		return id
	}
	return newID()
}
```

Generate with `crypto/rand` (hex) rather than a timestamp: timestamps collide
under load and leak clock information. Set the response header in the same
middleware that puts the ID in context.

Tests: valid passthrough, short rejected, `../` rejected, empty generated.

## 2.2 Content-Type guard

```go
func requireJSON(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost, http.MethodPut, http.MethodPatch:
			ct := r.Header.Get("Content-Type")
			if ct != "" && !strings.HasPrefix(ct, "application/json") {
				writeError(w, 415, "unsupported_media_type")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
```

Allow an empty Content-Type so bodyless POSTs (logout, follow) keep working.
Place it before auth so malformed requests do not hit the database, but after
request ID so the rejection is traceable.

## 2.3 405

`http.ServeMux` sets `Allow` and returns 405. `JSONErrors` wraps the mux; if it
buffered the response and re-emitted headers, `Allow` can be lost. The fix is to
copy headers before writing the envelope, or special-case 405 to pass through.
Test with `httptest.NewRequest(http.MethodDelete, "/api/v1/posts", nil)`.

## 2.4 X-Total-Count

Add `SELECT COUNT(*) FROM posts WHERE status = 'published'` with the same filters.
It is a second full scan. For 100k rows it is tens of milliseconds; at millions
it is unacceptable on every page. Prefer `has_more`. If a UI needs a total, cache
it or maintain a counter table.

## 2.5 Problem+JSON

```json
{
  "type": "https://example.com/problems/validation",
  "title": "Validation failed",
  "status": 422,
  "detail": "content is required",
  "instance": "/api/v1/posts"
}
```

Negotiate on `Accept`. Two formats doubles the contract surface; only do it if
clients need it.
