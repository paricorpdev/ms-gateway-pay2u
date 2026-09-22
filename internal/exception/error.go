package exception

import (
	"errors"
	"fmt"
	"net/http"
)

type Code string

const (
	CodeBadRequest   Code = "BAD_REQUEST"
	CodeValidation   Code = "VALIDATION_ERROR"
	CodeUnauthorized Code = "UNAUTHORIZED"
	CodeForbidden    Code = "FORBIDDEN"
	CodeNotFound     Code = "NOT_FOUND"
	CodeConflict     Code = "CONFLICT"
	CodeTooLarge     Code = "PAYLOAD_TOO_LARGE"
	CodeUnsupported  Code = "UNSUPPORTED_MEDIA_TYPE"
	CodeRateLimited  Code = "TOO_MANY_REQUESTS"
	CodeTimeout      Code = "REQUEST_TIMEOUT"
	CodeInternal     Code = "INTERNAL_SERVER_ERROR"
	CodeUnavailable  Code = "SERVICE_UNAVAILABLE"
)

type AppError struct {
	Code    Code
	Status  int
	Message string
	Details any
	Err     error
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *AppError) Unwrap() error { return e.Err }

func (e *AppError) WithDetails(details any) *AppError {
	e.Details = details
	return e
}

func (e *AppError) Wrap(err error) *AppError {
	e.Err = err
	return e
}

func New(code Code, status int, message string) *AppError {
	return &AppError{Code: code, Status: status, Message: message}
}

func BadRequest(message string) *AppError {
	return New(CodeBadRequest, http.StatusBadRequest, message)
}

func Validation(message string) *AppError {
	return New(CodeValidation, http.StatusUnprocessableEntity, message)
}

func Unauthorized(message string) *AppError {
	return New(CodeUnauthorized, http.StatusUnauthorized, message)
}

func Forbidden(message string) *AppError {
	return New(CodeForbidden, http.StatusForbidden, message)
}

func NotFound(message string) *AppError {
	return New(CodeNotFound, http.StatusNotFound, message)
}

func Conflict(message string) *AppError {
	return New(CodeConflict, http.StatusConflict, message)
}

func TooLarge(message string) *AppError {
	return New(CodeTooLarge, http.StatusRequestEntityTooLarge, message)
}

func UnsupportedMedia(message string) *AppError {
	return New(CodeUnsupported, http.StatusUnsupportedMediaType, message)
}

func Timeout(message string) *AppError {
	return New(CodeTimeout, http.StatusRequestTimeout, message)
}

func Unavailable(message string) *AppError {
	return New(CodeUnavailable, http.StatusServiceUnavailable, message)
}

func Internal(err error) *AppError {
	return &AppError{
		Code:    CodeInternal,
		Status:  http.StatusInternalServerError,
		Message: "internal server error",
		Err:     err,
	}
}

func As(err error) (*AppError, bool) {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr, true
	}
	return nil, false
}

func From(err error) *AppError {
	if err == nil {
		return nil
	}
	if appErr, ok := As(err); ok {
		return appErr
	}
	return Internal(err)
}

func IsNotFound(err error) bool {
	appErr, ok := As(err)
	return ok && appErr.Code == CodeNotFound
}
