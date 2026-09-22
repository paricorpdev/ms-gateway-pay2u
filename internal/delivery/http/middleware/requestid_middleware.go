package middleware

import (
	"strings"

	"paygate/internal/common/constants"
	"paygate/internal/logger"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

const maxRequestIDLength = 64

func NewRequestID(log *logrus.Logger) fiber.Handler {
	return func(ctx *fiber.Ctx) error {
		requestID := sanitizeRequestID(ctx.Get(constants.HeaderRequestID))
		if requestID == "" {
			requestID = uuid.NewString()
		}

		entry := log.WithFields(logrus.Fields{
			"request_id": requestID,
			"method":     ctx.Method(),
			"path":       ctx.Path(),
		})

		ctx.Locals(constants.LocalsRequestID, requestID)
		uCtx := logger.WithRequestID(logger.WithLogger(ctx.UserContext(), entry), requestID)
		ctx.SetUserContext(uCtx)
		ctx.Set(constants.HeaderRequestID, requestID)

		return ctx.Next()
	}
}

func sanitizeRequestID(value string) string {
	if value == "" {
		return ""
	}
	if len(value) > maxRequestIDLength {
		value = value[:maxRequestIDLength]
	}

	var builder strings.Builder
	builder.Grow(len(value))
	for _, char := range value {
		switch {
		case char >= 'a' && char <= 'z',
			char >= 'A' && char <= 'Z',
			char >= '0' && char <= '9',
			char == '-', char == '_':
			builder.WriteRune(char)
		}
	}
	return builder.String()
}
