package config

import (
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
)

// NewValidator builds the shared struct validator. Field names in errors come
// from the json tag, so clients see the identifiers they sent.
func NewValidator() *validator.Validate {
	validate := validator.New(validator.WithRequiredStructEnabled())

	validate.RegisterTagNameFunc(func(field reflect.StructField) string {
		name := strings.SplitN(field.Tag.Get("json"), ",", 2)[0]
		if name == "" {
			name = strings.SplitN(field.Tag.Get("form"), ",", 2)[0]
		}
		if name == "-" {
			return ""
		}
		return name
	})

	return validate
}
