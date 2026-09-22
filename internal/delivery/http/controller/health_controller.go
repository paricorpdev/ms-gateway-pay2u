package controller

import (
	"context"
	"time"

	"paygate/internal/delivery/http/response"
	"paygate/internal/exception"

	"github.com/gofiber/fiber/v2"
)

// Checker reports whether one dependency is usable.
type Checker struct {
	Name     string
	Check    func(ctx context.Context) error
	Critical bool
}

// HealthController answers the liveness and readiness probes.
type HealthController struct {
	appName  string
	version  string
	checkers []Checker
	timeout  time.Duration
}

func NewHealthController(appName, version string, timeout time.Duration, checkers ...Checker) *HealthController {
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	return &HealthController{appName: appName, version: version, checkers: checkers, timeout: timeout}
}

// Live handles GET /health/live.
func (c *HealthController) Live(ctx *fiber.Ctx) error {
	return response.OK(ctx, fiber.Map{
		"status":  "ok",
		"service": c.appName,
		"version": c.version,
	})
}

// Ready handles GET /health/ready and verifies every dependency.
func (c *HealthController) Ready(ctx *fiber.Ctx) error {
	checkCtx, cancel := context.WithTimeout(ctx.UserContext(), c.timeout)
	defer cancel()

	details := make(map[string]string, len(c.checkers))
	degraded := false

	for _, checker := range c.checkers {
		if err := checker.Check(checkCtx); err != nil {
			details[checker.Name] = "unavailable: " + err.Error()
			if checker.Critical {
				degraded = true
			}
			continue
		}
		details[checker.Name] = "ok"
	}

	if degraded {
		return exception.Unavailable("one or more dependencies are unavailable").
			WithDetails(details)
	}

	return response.OK(ctx, fiber.Map{
		"status":       "ok",
		"service":      c.appName,
		"version":      c.version,
		"dependencies": details,
	})
}
