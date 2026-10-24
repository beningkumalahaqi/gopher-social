// Package response centralizes JSON envelopes and error-to-status mapping so
// every endpoint fails the same way.
package response

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/haqi/gophersocial/internal/apperr"
	"github.com/haqi/gophersocial/internal/requestid"
)

// ErrorBody is the single error shape used by the API.
type ErrorBody struct {
	Error ErrorDetail `json:"error"`
}

type ErrorDetail struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Details   any    `json:"details,omitempty"`
	RequestID string `json:"request_id,omitempty"`
}

// Data writes a single object envelope: {"data": ...}.
func Data(w http.ResponseWriter, status int, data any) {
	write(w, status, map[string]any{"data": data})
}

// List writes a collection envelope: {"data": [...], "meta": {...}}.
func List(w http.ResponseWriter, status int, data any, meta any) {
	write(w, status, map[string]any{"data": data, "meta": meta})
}

func NoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

// Error maps domain errors to HTTP status codes. Unknown errors are 500s and
// their text is never exposed to the client.
func Error(w http.ResponseWriter, r *http.Request, err error) {
	status, code, message := classify(err)

	var details any
	var invalid *apperr.ValidationError
	if errors.As(err, &invalid) {
		details = invalid.Fields
	}

	body := ErrorBody{Error: ErrorDetail{
		Code:      code,
		Message:   message,
		Details:   details,
		RequestID: requestid.From(r.Context()),
	}}
	write(w, status, body)
}

// JSON writes any value with the correct content type. It is exported for
// middleware (health checks) that do not use the data envelope.
func JSON(w http.ResponseWriter, status int, v any) { write(w, status, v) }

func classify(err error) (status int, code, message string) {
	switch {
	case errors.Is(err, apperr.ErrValidation):
		return http.StatusUnprocessableEntity, "validation_error", "validation failed"
	case errors.Is(err, apperr.ErrBadRequest):
		return http.StatusBadRequest, "bad_request", clean(err, "bad request")
	case errors.Is(err, apperr.ErrPayloadTooLarge):
		return http.StatusRequestEntityTooLarge, "payload_too_large", "request body is too large"
	case errors.Is(err, apperr.ErrUnauthorized):
		return http.StatusUnauthorized, "unauthorized", clean(err, "authentication required")
	case errors.Is(err, apperr.ErrForbidden):
		return http.StatusForbidden, "forbidden", clean(err, "not allowed")
	case errors.Is(err, apperr.ErrNotFound):
		return http.StatusNotFound, "not_found", "resource not found"
	case errors.Is(err, apperr.ErrConflict):
		return http.StatusConflict, "conflict", clean(err, "conflict")
	case errors.Is(err, apperr.ErrRateLimited):
		return http.StatusTooManyRequests, "rate_limited", "too many requests"
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		return http.StatusRequestTimeout, "request_timeout", "request timed out"
	default:
		return http.StatusInternalServerError, "internal_error", "internal server error"
	}
}

// clean removes the sentinel prefix, e.g. "unauthorized: invalid credentials"
// becomes "invalid credentials".
func clean(err error, fallback string) string {
	msg := err.Error()
	if _, rest, ok := strings.Cut(msg, ": "); ok && rest != "" {
		return rest
	}
	if msg == "" {
		return fallback
	}
	return msg
}

func write(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(v); err != nil {
		// The status line is already sent; log at debug level via slog default.
		slog.DebugContext(context.Background(), "encode response", "error", err)
	}
}
