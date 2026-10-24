// Package params parses HTTP query parameters and path identifiers.
package params

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/haqi/gophersocial/internal/apperr"
	"github.com/haqi/gophersocial/internal/pagination"
)

const (
	DefaultLimit = 20
	MaxLimit     = 100
)

// Pagination reads limit and offset. Malformed values return a 422-style
// validation error instead of silently falling back to defaults.
func Pagination(r *http.Request) (pagination.Page, error) {
	limit, err := intQuery(r, "limit", DefaultLimit)
	if err != nil {
		return pagination.Page{}, err
	}
	offset, err := intQuery(r, "offset", 0)
	if err != nil {
		return pagination.Page{}, err
	}
	if err := pagination.Validate(limit, offset, DefaultLimit, MaxLimit); err != nil {
		return pagination.Page{}, err
	}
	return pagination.Page{Limit: limit, Offset: offset}, nil
}

// Limit reads the limit parameter used by cursor endpoints.
func Limit(r *http.Request) (int, error) {
	limit, err := intQuery(r, "limit", DefaultLimit)
	if err != nil {
		return 0, err
	}
	if err := pagination.Validate(limit, 0, DefaultLimit, MaxLimit); err != nil {
		return 0, err
	}
	return limit, nil
}

// Cursor decodes the cursor query parameter, if present.
func Cursor(r *http.Request) (pagination.Cursor, bool, error) {
	raw := strings.TrimSpace(r.URL.Query().Get("cursor"))
	if raw == "" {
		return pagination.Cursor{}, false, nil
	}
	c, err := pagination.Decode(raw)
	if err != nil {
		return pagination.Cursor{}, false, err
	}
	return c, true, nil
}

// UUID reads a path parameter as a UUID.
func UUID(r *http.Request, name string) (uuid.UUID, error) {
	raw := r.PathValue(name)
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, apperr.Wrap(apperr.ErrBadRequest, "invalid %s", strings.ReplaceAll(name, "ID", " id"))
	}
	return id, nil
}

// OptionalUUID reads a query parameter that may be empty.
func OptionalUUID(r *http.Request, name string) (uuid.UUID, bool, error) {
	raw := strings.TrimSpace(r.URL.Query().Get(name))
	if raw == "" {
		return uuid.Nil, false, nil
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, false, apperr.Invalid(apperr.FieldError{Field: name, Message: "must be a UUID"})
	}
	return id, true, nil
}

// Enum validates a query value against allowed options. Empty means unset.
func Enum(r *http.Request, name string, allowed ...string) (string, error) {
	value := strings.TrimSpace(r.URL.Query().Get(name))
	if value == "" {
		return "", nil
	}
	for _, option := range allowed {
		if value == option {
			return value, nil
		}
	}
	return "", apperr.Invalid(apperr.FieldError{
		Field:   name,
		Message: "must be one of: " + strings.Join(allowed, ", "),
	})
}

func intQuery(r *http.Request, name string, fallback int) (int, error) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, apperr.Invalid(apperr.FieldError{Field: name, Message: "must be an integer"})
	}
	return n, nil
}
