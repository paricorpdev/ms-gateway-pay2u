package exception

import (
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"
)

type FieldError struct {
	Field   string `json:"field"`
	Rule    string `json:"rule"`
	Param   string `json:"param,omitempty"`
	Message string `json:"message"`
}

func FromValidation(err error) *AppError {
	if err == nil {
		return nil
	}

	var invalid *validator.InvalidValidationError
	if As := asInvalid(err, &invalid); As {
		return Internal(err)
	}

	validationErrors, ok := err.(validator.ValidationErrors)
	if !ok {
		return BadRequest("invalid request body").Wrap(err)
	}

	fields := make([]FieldError, 0, len(validationErrors))
	for _, fieldErr := range validationErrors {
		fields = append(fields, FieldError{
			Field:   fieldName(fieldErr),
			Rule:    fieldErr.Tag(),
			Param:   fieldErr.Param(),
			Message: message(fieldErr),
		})
	}

	return Validation("request validation failed").WithDetails(fields)
}

func asInvalid(err error, target **validator.InvalidValidationError) bool {
	invalid, ok := err.(*validator.InvalidValidationError)
	if ok {
		*target = invalid
	}
	return ok
}

func fieldName(fieldErr validator.FieldError) string {
	namespace := fieldErr.Namespace()
	if idx := strings.Index(namespace, "."); idx >= 0 {
		namespace = namespace[idx+1:]
	}
	if namespace == "" {
		return fieldErr.Field()
	}
	return namespace
}

func message(fieldErr validator.FieldError) string {
	field := fieldName(fieldErr)
	switch fieldErr.Tag() {
	case "required":
		return fmt.Sprintf("%s is required", field)
	case "email":
		return fmt.Sprintf("%s must be a valid email address", field)
	case "uuid", "uuid4":
		return fmt.Sprintf("%s must be a valid UUID", field)
	case "min":
		return fmt.Sprintf("%s must be at least %s", field, fieldErr.Param())
	case "max":
		return fmt.Sprintf("%s must be at most %s", field, fieldErr.Param())
	case "gte":
		return fmt.Sprintf("%s must be greater than or equal to %s", field, fieldErr.Param())
	case "lte":
		return fmt.Sprintf("%s must be less than or equal to %s", field, fieldErr.Param())
	case "oneof":
		return fmt.Sprintf("%s must be one of [%s]", field, fieldErr.Param())
	default:
		return fmt.Sprintf("%s failed the %q rule", field, fieldErr.Tag())
	}
}
