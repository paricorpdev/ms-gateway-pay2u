package middleware

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"
)

// NewTimeout bounds how long a handler may take.
func NewTimeout(timeout time.Duration) fiber.Handler {
	if timeout <= 0 {
		return func(ctx *fiber.Ctx) error { return ctx.Next() }
	}

	return func(ctx *fiber.Ctx) error {
		timedCtx, cancel := context.WithTimeout(ctx.UserContext(), timeout)
		defer cancel()

		ctx.SetUserContext(timedCtx)
		return ctx.Next()
	}
}
