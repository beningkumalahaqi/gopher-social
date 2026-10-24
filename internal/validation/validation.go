// Package validation adapts go-playground/validator to the project's
// apperr.ValidationError shape.
package validation

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"

	"github.com/haqi/gophersocial/internal/apperr"
)

var instance = newValidator()

func newValidator() *validator.Validate {
	v := validator.New(validator.WithRequiredStructEnabled())
	v.RegisterTagNameFunc(func(field reflect.StructField) string {
		name := strings.SplitN(field.Tag.Get("json"), ",", 2)[0]
		if name == "-" || name == "" {
			return field.Name
		}
		return name
	})
	return v
}

// Struct validates a DTO and returns an apperr.ValidationError with one entry
// per failed field.
func Struct(dto any) error {
	err := instance.Struct(dto)
	if err == nil {
		return nil
	}
	var invalid *validator.InvalidValidationError
	if errors.As(err, &invalid) {
		return fmt.Errorf("validate %T: %w", dto, invalid)
	}

	var fieldErrors validator.ValidationErrors
	if !errors.As(err, &fieldErrors) {
		return fmt.Errorf("validate: %w", err)
	}

	fields := make([]apperr.FieldError, 0, len(fieldErrors))
	for _, fe := range fieldErrors {
		fields = append(fields, apperr.FieldError{
			Field:   fe.Field(),
			Message: message(fe),
		})
	}
	return &apperr.ValidationError{Fields: fields}
}

func message(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return "is required"
	case "email":
		return "must be a valid email address"
	case "min":
		return "must be at least " + fe.Param() + " characters"
	case "max":
		return "must be at most " + fe.Param() + " characters"
	case "oneof":
		return "must be one of: " + fe.Param()
	case "gte":
		return "must be greater than or equal to " + fe.Param()
	case "lte":
		return "must be less than or equal to " + fe.Param()
	case "url":
		return "must be a valid URL"
	default:
		return "is invalid"
	}
}
