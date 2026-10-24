// Package middleware contains transport-level concerns. None of it knows about
// business rules.
package middleware

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/haqi/gophersocial/internal/api/response"
	"github.com/haqi/gophersocial/internal/requestid"
)

// RequestID reads or creates a correlation id and always echoes it.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := requestid.Sanitize(r.Header.Get("X-Request-Id"))
		if id == "" {
			id = requestid.New()
		}
		w.Header().Set("X-Request-Id", id)
		next.ServeHTTP(w, r.WithContext(requestid.With(r.Context(), id)))
	})
}

// statusRecorder captures the status code for logging without hiding the
// optional interfaces of the underlying writer.
type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (r *statusRecorder) WriteHeader(status int) {
	if r.status != 0 {
		return
	}
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Write(p []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	n, err := r.ResponseWriter.Write(p)
	r.bytes += n
	return n, err
}

func (r *statusRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Logger emits one structured line per request.
func Logger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w}
			ctx, slot := withIdentitySlot(r.Context())
			next.ServeHTTP(rec, r.WithContext(ctx))
			if rec.status == 0 {
				rec.status = http.StatusOK
			}

			attrs := []any{
				"method", r.Method,
				"path", r.URL.Path,
				"status", rec.status,
				"bytes", rec.bytes,
				"duration_ms", time.Since(start).Milliseconds(),
				"request_id", requestid.From(r.Context()),
				"remote_addr", clientIP(r),
			}
			if slot.ok {
				attrs = append(attrs, "user_id", slot.identity.UserID.String())
			}

			if rec.status >= 500 {
				logger.Error("http request", attrs...)
			} else {
				logger.Info("http request", attrs...)
			}
		})
	}
}

// Recover turns a panic into a logged 500. It sits inside RequestID and Logger
// so the response still carries a request id.
func Recover(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					logger.Error("panic recovered",
						"panic", rec,
						"request_id", requestid.From(r.Context()),
						"stack", string(debug.Stack()),
					)
					response.Error(w, r, fmt.Errorf("panic recovered"))
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// Timeout bounds handler work. Handlers must still respect ctx; this only sets
// the deadline and documents the contract.
func Timeout(d time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), d)
			defer cancel()
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// SecurityHeaders sets conservative defaults. A JSON API needs no inline
// scripts, framing or MIME sniffing.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}

// ClientIP returns the best available client address. Behind a platform proxy
// the forwarded header is the only useful value; it is spoofable when the app is
// exposed directly, so rate limits behind a proxy must also be enforced at the
// edge.
func ClientIP(r *http.Request) string {
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		if first, _, ok := strings.Cut(forwarded, ","); ok {
			return strings.TrimSpace(first)
		}
		return strings.TrimSpace(forwarded)
	}
	if real := strings.TrimSpace(r.Header.Get("X-Real-Ip")); real != "" {
		return real
	}
	host := r.RemoteAddr
	if i := strings.LastIndex(host, ":"); i > 0 {
		host = host[:i]
	}
	return host
}

func clientIP(r *http.Request) string { return ClientIP(r) }
