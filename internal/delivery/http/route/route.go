package route

import (
	"strings"

	"paygate/internal/audit"
	"paygate/internal/config"
	"paygate/internal/delivery/http/controller"
	"paygate/internal/delivery/http/middleware"
	"paygate/internal/exception"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/compress"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/helmet"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/fiber/v2/middleware/monitor"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/swagger"
	"github.com/sirupsen/logrus"
)

type RouteConfig struct {
	App    *fiber.App
	Config *config.Config
	Log    *logrus.Logger

	APIKeyMiddleware    fiber.Handler
	MasterKeyMiddleware fiber.Handler
	AuditWorker         *audit.AuditWorker

	PaymentController  *controller.PaymentController
	MerchantController *controller.MerchantController
	HealthController   *controller.HealthController
}

func (c *RouteConfig) Setup() {
	c.setupMiddleware()
	c.setupHealthRoutes()
	c.setupSwaggerRoutes()
	c.setupPublicRoutes()
	c.setupProtectedRoutes()
	c.setupFallback()
}

func (c *RouteConfig) setupMiddleware() {
	c.App.Use(recover.New(recover.Config{EnableStackTrace: !c.Config.App.IsProduction()}))

	c.App.Use(middleware.NewRequestID(c.Log))

	c.App.Use(cors.New(cors.Config{
		AllowOrigins:     strings.Join(c.Config.Web.CORS.AllowOrigins, ","),
		AllowMethods:     strings.Join(c.Config.Web.CORS.AllowMethods, ","),
		AllowHeaders:     strings.Join(c.Config.Web.CORS.AllowHeaders, ","),
		ExposeHeaders:    strings.Join(c.Config.Web.CORS.ExposeHeaders, ","),
		AllowCredentials: c.Config.Web.CORS.AllowCredentials,
		MaxAge:           c.Config.Web.CORS.MaxAge,
	}))

	c.App.Use(helmet.New(helmet.Config{
		XSSProtection:             "0",
		ContentTypeNosniff:        "nosniff",
		XFrameOptions:             "DENY",
		ReferrerPolicy:            "no-referrer",
		CrossOriginEmbedderPolicy: "require-corp",
		CrossOriginOpenerPolicy:   "same-origin",
	}))

	c.App.Use(compress.New(compress.Config{Level: compress.LevelDefault}))

	if c.AuditWorker != nil {
		c.App.Use(middleware.NewAuditMiddleware(c.AuditWorker))
	}

	c.App.Use(middleware.NewAccessLog())

	if c.Config.Web.RateLimit.Enabled {
		c.App.Use(limiter.New(limiter.Config{
			Max:        c.Config.Web.RateLimit.Max,
			Expiration: c.Config.Web.RateLimit.Expiration,
			Next: func(ctx *fiber.Ctx) bool {
				return strings.HasPrefix(ctx.Path(), "/health")
			},
			LimitReached: rateLimitReached,
		}))
	}

	c.App.Use(middleware.NewTimeout(c.Config.Web.RequestTimeout))
}

func (c *RouteConfig) setupHealthRoutes() {
	health := c.App.Group("/health")
	health.Get("/live", c.HealthController.Live)
	health.Get("/ready", c.HealthController.Ready)

	c.App.Get("/", c.HealthController.Live)

	if c.Config.Web.Metrics.Enabled {
		c.App.Get(c.Config.Web.Metrics.Path, monitor.New(monitor.Config{Title: c.Config.App.Name}))
	}
}

func (c *RouteConfig) setupSwaggerRoutes() {
	v1 := c.App.Group("/api/v1")
	v1.Get("/docs/*", swagger.HandlerDefault)
}

func (c *RouteConfig) setupPublicRoutes() {
	if c.PaymentController != nil {
		v1 := c.App.Group("/api/v1")
		v1.Post("/payments/callback/:provider", c.PaymentController.HandleCallback)
	}
}

func (c *RouteConfig) setupProtectedRoutes() {
	if c.PaymentController != nil {
		var paymentAuthHandlers []fiber.Handler
		if c.APIKeyMiddleware != nil {
			paymentAuthHandlers = append(paymentAuthHandlers, c.APIKeyMiddleware)
		}

		payments := c.App.Group("/api/v1/payments", paymentAuthHandlers...)
		payments.Post("/", c.PaymentController.CreatePayment)
		payments.Get("/:merchant_reff", c.PaymentController.GetPayment)
		payments.Post("/:merchant_reff/refresh", c.PaymentController.RefreshPayment)
	}

	if c.MerchantController != nil {
		var adminAuthHandlers []fiber.Handler
		if c.MasterKeyMiddleware != nil {
			adminAuthHandlers = append(adminAuthHandlers, c.MasterKeyMiddleware)
		} else if c.APIKeyMiddleware != nil {
			adminAuthHandlers = append(adminAuthHandlers, c.APIKeyMiddleware)
		}

		admin := c.App.Group("/api/v1/admin/merchants", adminAuthHandlers...)
		admin.Post("/", c.MerchantController.CreateMerchant)
		admin.Get("/", c.MerchantController.ListMerchants)
		admin.Get("/:uuid", c.MerchantController.GetMerchant)
		admin.Put("/:uuid", c.MerchantController.UpdateMerchant)
		admin.Post("/:uuid/rotate-key", c.MerchantController.RotateAPIKey)
	}
}

func (c *RouteConfig) setupFallback() {
	c.App.Use(func(ctx *fiber.Ctx) error {
		return exception.NotFound("route " + ctx.Method() + " " + ctx.Path() + " does not exist")
	})
}

func rateLimitReached(*fiber.Ctx) error {
	return exception.New(exception.CodeRateLimited, fiber.StatusTooManyRequests,
		"too many requests, please retry later")
}
