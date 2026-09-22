package response

import (
	"context"
	"errors"
	"net/http"
	"time"

	"paygate/internal/common/constants"
	"paygate/internal/exception"
	"paygate/internal/logger"
	"paygate/internal/model/payload"

	"github.com/gofiber/fiber/v2"
)

// RequestID returns the correlation id attached by the request-id middleware.
func RequestID(ctx *fiber.Ctx) string {
	if id, ok := ctx.Locals(constants.LocalsRequestID).(string); ok {
		return id
	}
	return ""
}

// JSON writes a success envelope.
func JSON(ctx *fiber.Ctx, status int, data any) error {
	return ctx.Status(status).JSON(payload.Response{
		Status:       constants.StatusSuccess,
		StatusCode:   status,
		ResponseData: data,
		RequestID:    RequestID(ctx),
	})
}

// OK writes 200 with a payload.
func OK(ctx *fiber.Ctx, data any) error { return JSON(ctx, fiber.StatusOK, data) }

// Created writes 201 with a payload.
func Created(ctx *fiber.Ctx, data any) error { return JSON(ctx, fiber.StatusCreated, data) }

// NoContent writes 204 with no body.
func NoContent(ctx *fiber.Ctx) error { return ctx.SendStatus(fiber.StatusNoContent) }

// NewErrorHandler builds the Fiber error handler. Every error returned by a
// handler or middleware lands here, so error shaping happens exactly once.
func NewErrorHandler() fiber.ErrorHandler {
	return func(ctx *fiber.Ctx, err error) error {
		appErr := normalize(err)

		log := logger.FromContext(ctx.UserContext()).WithFields(map[string]any{
			"status": appErr.Status,
			"code":   string(appErr.Code),
			"method": ctx.Method(),
			"path":   ctx.Path(),
		})

		// 5xx means we broke something; 4xx is the caller's mistake.
		switch {
		case appErr.Status >= http.StatusInternalServerError:
			log.WithError(err).Error(appErr.Message)
		default:
			log.WithError(err).Warn(appErr.Message)
		}

		body := payload.ResponseError{
			Status:     constants.StatusError,
			StatusCode: appErr.Status,
			RequestID:  RequestID(ctx),
			Error: payload.ErrorDetail{
				Code:      string(appErr.Code),
				Message:   appErr.Message,
				Details:   appErr.Details,
				Path:      ctx.Path(),
				Timestamp: time.Now().UTC().Format(time.RFC3339),
			},
		}

		return ctx.Status(appErr.Status).JSON(body)
	}
}

// normalize maps anything that can reach the handler onto an AppError.
func normalize(err error) *exception.AppError {
	if err == nil {
		return exception.Internal(errors.New("nil error reached the error handler"))
	}

	if appErr, ok := exception.As(err); ok {
		return appErr
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return exception.Timeout("request timed out").Wrap(err)
	}
	if errors.Is(err, context.Canceled) {
		return exception.New(exception.CodeBadRequest, 499, "client closed request").Wrap(err)
	}

	// Raised by Fiber itself: unmatched routes, body limit, the rate limiter.
	var fiberErr *fiber.Error
	if errors.As(err, &fiberErr) {
		return &exception.AppError{
			Code:    codeForStatus(fiberErr.Code),
			Status:  fiberErr.Code,
			Message: fiberErr.Message,
			Err:     err,
		}
	}

	return exception.Internal(err)
}

func codeForStatus(status int) exception.Code {
	switch status {
	case http.StatusBadRequest:
		return exception.CodeBadRequest
	case http.StatusUnauthorized:
		return exception.CodeUnauthorized
	case http.StatusForbidden:
		return exception.CodeForbidden
	case http.StatusNotFound:
		return exception.CodeNotFound
	case http.StatusConflict:
		return exception.CodeConflict
	case http.StatusRequestEntityTooLarge:
		return exception.CodeTooLarge
	case http.StatusUnsupportedMediaType:
		return exception.CodeUnsupported
	case http.StatusTooManyRequests:
		return exception.CodeRateLimited
	case http.StatusRequestTimeout, http.StatusGatewayTimeout:
		return exception.CodeTimeout
	case http.StatusServiceUnavailable:
		return exception.CodeUnavailable
	default:
		if status >= http.StatusInternalServerError {
			return exception.CodeInternal
		}
		return exception.CodeBadRequest
	}
}
