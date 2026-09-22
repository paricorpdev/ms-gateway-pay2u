package middleware

import (
	"time"

	"paygate/internal/logger"

	"github.com/gofiber/fiber/v2"
)

// NewAccessLog emits one structured line per completed request, reusing the
// request-scoped logger so it shares the correlation id.
func NewAccessLog() fiber.Handler {
	return func(ctx *fiber.Ctx) error {
		start := time.Now()

		err := ctx.Next()

		entry := logger.FromContext(ctx.UserContext()).WithFields(map[string]any{
			"status":      ctx.Response().StatusCode(),
			"duration_ms": time.Since(start).Milliseconds(),
			"ip":          ctx.IP(),
			"bytes":       len(ctx.Response().Body()),
		})

		if err != nil || ctx.Response().StatusCode() >= fiber.StatusInternalServerError {
			entry.Warn("request completed with error")
			return err
		}

		entry.Info("request completed")
		return nil
	}
}
