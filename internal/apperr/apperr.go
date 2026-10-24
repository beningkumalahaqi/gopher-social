package apperr

import (
	"errors"
	"fmt"
)

// Sentinel errors describe categories that the HTTP layer can translate into
// status codes. Callers use errors.Is, never string comparison.
var (
	ErrNotFound        = errors.New("not found")
	ErrConflict        = errors.New("conflict")
	ErrForbidden       = errors.New("forbidden")
	ErrUnauthorized    = errors.New("unauthorized")
	ErrBadRequest      = errors.New("bad request")
	ErrPayloadTooLarge = errors.New("payload too large")
	ErrRateLimited     = errors.New("rate limited")
)

// FieldError describes one invalid input field.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ValidationError groups field errors and unwraps to ErrValidation so a single
// errors.Is check catches every validation failure.
type ValidationError struct {
	Fields []FieldError
}

func (e *ValidationError) Error() string {
	if len(e.Fields) == 0 {
		return ErrValidation.Error()
	}
	return fmt.Sprintf("%s: %s", ErrValidation, e.Fields[0].Field)
}

func (e *ValidationError) Unwrap() error { return ErrValidation }

// ErrValidation is returned by every business-level validation failure.
var ErrValidation = errors.New("validation failed")

// Wrap adds context while preserving the sentinel for errors.Is.
func Wrap(kind error, format string, args ...any) error {
	return fmt.Errorf("%w: %s", kind, fmt.Sprintf(format, args...))
}

// Invalid builds a ValidationError from field/message pairs.
func Invalid(fields ...FieldError) error {
	return &ValidationError{Fields: fields}
}
