// Package handlers implements the HTTP layer. Handlers decode, validate and
// translate; business decisions live in the domain packages.
package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/haqi/gophersocial/internal/api/response"
	"github.com/haqi/gophersocial/internal/apperr"
	"github.com/haqi/gophersocial/internal/auth"
	"github.com/haqi/gophersocial/internal/requestid"
	"github.com/haqi/gophersocial/internal/validation"

	"github.com/google/uuid"
)

type base struct {
	logger  *slog.Logger
	maxBody int64
}

func newBase(logger *slog.Logger, maxBody int64) base {
	return base{logger: logger, maxBody: maxBody}
}

// decode enforces a body size limit, rejects unknown fields and trailing data,
// and maps JSON syntax problems to client errors.
func (b base) decode(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, b.maxBody)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		var tooLarge *http.MaxBytesError
		var typeErr *json.UnmarshalTypeError
		switch {
		case errors.As(err, &tooLarge):
			return apperr.ErrPayloadTooLarge
		case errors.As(err, &typeErr):
			return apperr.Invalid(apperr.FieldError{
				Field:   typeErr.Field,
				Message: "has the wrong type",
			})
		case errors.Is(err, io.EOF):
			return apperr.Wrap(apperr.ErrBadRequest, "request body is required")
		default:
			return apperr.Wrap(apperr.ErrBadRequest, "malformed JSON body: %v", err)
		}
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return apperr.Wrap(apperr.ErrBadRequest, "request body must contain a single JSON object")
	}
	return nil
}

func (b base) validate(dto any) error {
	return validation.Struct(dto)
}

// fail logs unexpected errors with context and writes the mapped response.
func (b base) fail(w http.ResponseWriter, r *http.Request, err error) {
	if !known(err) {
		b.logger.ErrorContext(r.Context(), "request failed",
			"error", err,
			"path", r.URL.Path,
			"request_id", requestid.From(r.Context()),
		)
	}
	response.Error(w, r, err)
}

func known(err error) bool {
	for _, sentinel := range []error{
		apperr.ErrValidation, apperr.ErrBadRequest, apperr.ErrPayloadTooLarge,
		apperr.ErrUnauthorized, apperr.ErrForbidden, apperr.ErrNotFound,
		apperr.ErrConflict, apperr.ErrRateLimited,
	} {
		if errors.Is(err, sentinel) {
			return true
		}
	}
	return false
}

// viewer returns the authenticated user id for optional-auth routes.
func viewer(r *http.Request) *uuid.UUID {
	id, ok := auth.UserIDFrom(r.Context())
	if !ok {
		return nil
	}
	return &id
}
