package middleware

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/haqi/gophersocial/internal/requestid"
)

// JSONErrors converts ServeMux's plain-text 404/405 responses into the API's
// error envelope. Handlers write their own JSON; only the mux-generated bodies
// are intercepted.
func JSONErrors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(&errorWriter{ResponseWriter: w, request: r}, r)
	})
}

type errorWriter struct {
	http.ResponseWriter
	request   *http.Request
	status    int
	intercept bool
}

func (w *errorWriter) WriteHeader(status int) {
	// The learning interface renders HTML error pages; only API routes use the
	// JSON envelope. Swagger assets also keep their own responses.
	if isHTMLRoute(w.request.URL.Path) {
		w.ResponseWriter.WriteHeader(status)
		return
	}
	if status == http.StatusNotFound || status == http.StatusMethodNotAllowed {
		w.status = status
		w.intercept = true
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.ResponseWriter.WriteHeader(status)
		return
	}
	w.ResponseWriter.WriteHeader(status)
}

func (w *errorWriter) Write(p []byte) (int, error) {
	if !w.intercept {
		return w.ResponseWriter.Write(p)
	}
	w.intercept = false
	code, message := "not_found", "resource not found"
	if w.status == http.StatusMethodNotAllowed {
		code, message = "method_not_allowed", "method not allowed for this resource"
	}
	body := map[string]any{"error": map[string]any{
		"code":       code,
		"message":    message,
		"request_id": requestid.From(w.request.Context()),
	}}
	encoded, _ := json.Marshal(body)
	return w.ResponseWriter.Write(append(encoded, '\n'))
}

func isHTMLRoute(path string) bool {
	return path == "/learn" || strings.HasPrefix(path, "/learn/") ||
		path == "/swagger" || strings.HasPrefix(path, "/swagger/")
}

// Unwrap lets http.ResponseController reach the original writer.
func (w *errorWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }
