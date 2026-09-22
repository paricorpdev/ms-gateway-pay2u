package config

import (
	"github.com/gofiber/fiber/v2"
)

// NewFiber builds the HTTP server shell. The error handler is injected by the
// delivery layer so this package stays free of presentation concerns.
func NewFiber(cfg *Config, errorHandler fiber.ErrorHandler) *fiber.App {
	return fiber.New(fiber.Config{
		AppName:                 cfg.App.Name,
		ErrorHandler:            errorHandler,
		Prefork:                 cfg.Web.Prefork,
		BodyLimit:               cfg.Web.BodyLimit,
		ReadTimeout:             cfg.Web.ReadTimeout,
		WriteTimeout:            cfg.Web.WriteTimeout,
		IdleTimeout:             cfg.Web.IdleTimeout,
		DisableStartupMessage:   cfg.App.IsProduction(),
		EnableTrustedProxyCheck: cfg.Web.EnableTrustedProxyCheck,
		TrustedProxies:          cfg.Web.TrustedProxies,
		ProxyHeader:             proxyHeader(cfg),
		Immutable:               false,
	})
}

func proxyHeader(cfg *Config) string {
	if cfg.Web.EnableTrustedProxyCheck && len(cfg.Web.TrustedProxies) > 0 {
		return fiber.HeaderXForwardedFor
	}
	return ""
}
