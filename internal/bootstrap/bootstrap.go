package bootstrap

import (
	"context"
	"time"

	"paygate/internal/audit"
	"paygate/internal/config"
	"paygate/internal/delivery/http/controller"
	"paygate/internal/delivery/http/middleware"
	"paygate/internal/delivery/http/route"
	"paygate/internal/provider"
	"paygate/internal/provider/pay2u"
	"paygate/internal/repository"
	"paygate/internal/usecase"
	"paygate/internal/webhook"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	fiberredis "github.com/gofiber/storage/redis/v3"
	goredis "github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

var Version = "dev"

type Dependencies struct {
	Config   *config.Config
	Log      *logrus.Logger
	DB       *gorm.DB
	Redis    *fiberredis.Storage
	Validate *validator.Validate
	App      *fiber.App
}

type Container struct {
	Payment     usecase.PaymentService
	Merchant    usecase.MerchantService
	AuditWorker *audit.AuditWorker
	Dispatcher  webhook.Dispatcher
}

func NewContainer(deps *Dependencies) *Container {
	var auditWorker *audit.AuditWorker
	if deps.DB != nil {
		auditRepo := repository.NewAuditLogRepository(deps.DB)
		auditWorker = audit.NewWorker(auditRepo, deps.Log, 1000)
	}

	var cache repository.CacheRepository
	if deps.Redis != nil {
		cache = repository.NewCacheRepository(deps.Redis)
	}

	// Load Configuration Provider Pay2u, etc.
	
	pay2uCfg := pay2u.Config{
		BaseURL:         deps.Config.Pay2U.BaseURL(),
		OAuthURL:        deps.Config.Pay2U.OAuthURL,
		TokenURL:        deps.Config.Pay2U.TokenURL,
		BillingVAURL:    deps.Config.Pay2U.BillingVAURL,
		BillingQRISURL:  deps.Config.Pay2U.BillingQRISURL,
		BillingCCURL:    deps.Config.Pay2U.BillingCCURL,
		BillingGetURL:   deps.Config.Pay2U.BillingGetURL,
		ClientID:        deps.Config.Pay2U.ClientID,
		ClientSecret:    deps.Config.Pay2U.ClientSecret,
		MerchantCode:    deps.Config.Pay2U.MerchantCode,
		MerchantUser:    deps.Config.Pay2U.MerchantUser,
		MerchantPass:    deps.Config.Pay2U.MerchantPass,
		MerchantDomain:  deps.Config.Pay2U.MerchantDomain,
		Timeout:         deps.Config.Pay2U.Timeout,
		OAuthTokenTTL:   deps.Config.Pay2U.OAuthTokenTTL,
		CallbackBaseURL: deps.Config.Pay2U.CallbackBaseURL,
	}

	pay2uClient := pay2u.NewClient(pay2uCfg, cache, auditWorker)
	pay2uProvider := pay2u.NewProvider(pay2uClient)

	providers := map[string]provider.PaymentProvider{
		"pay2u": pay2uProvider,
	}

	txRepo := repository.NewTransactionRepository()
	merchantRepo := repository.NewMerchantRepository()
	merchantUC := usecase.NewMerchantUseCase(deps.DB, merchantRepo, cache)

	var redisClient goredis.UniversalClient
	if deps.Redis != nil {
		redisClient = deps.Redis.Conn()
	}

	dispatchRepo := repository.NewWebhookDispatchRepository()
	var dispatcher webhook.Dispatcher
	if deps.DB != nil {
		dispatcher = webhook.NewDispatcher(deps.DB, redisClient, dispatchRepo, deps.Log)
		dispatcher.Start(context.Background())
	}

	paymentUC := usecase.NewPaymentUseCase(
		deps.DB,
		deps.Validate,
		txRepo,
		merchantRepo,
		dispatchRepo,
		dispatcher,
		providers,
	)

	return &Container{
		Payment:     paymentUC,
		Merchant:    merchantUC,
		AuditWorker: auditWorker,
		Dispatcher:  dispatcher,
	}
}

func HTTP(deps *Dependencies, container *Container) {
	routeConfig := route.RouteConfig{
		App:                 deps.App,
		Config:              deps.Config,
		Log:                 deps.Log,
		PaymentController:   controller.NewPaymentController(container.Payment),
		MerchantController:  controller.NewMerchantController(container.Merchant),
		HealthController:    newHealthController(deps),
		APIKeyMiddleware:    middleware.NewMerchantAPIKey(container.Merchant),
		MasterKeyMiddleware: middleware.NewMasterAPIKey(deps.Config.APIKey),
		AuditWorker:         container.AuditWorker,
	}
	routeConfig.Setup()
}

func newHealthController(deps *Dependencies) *controller.HealthController {
	var checkers []controller.Checker

	if deps.DB != nil {
		checkers = append(checkers, controller.Checker{
			Name:     "postgres",
			Critical: true,
			Check:    func(ctx context.Context) error { return config.PingDatabase(ctx, deps.DB) },
		})
	}

	if deps.Redis != nil {
		checkers = append(checkers, controller.Checker{
			Name:     "redis",
			Critical: true,
			Check:    func(ctx context.Context) error { return config.PingRedis(ctx, deps.Redis) },
		})
	}

	return controller.NewHealthController(deps.Config.App.Name, Version, 2*time.Second, checkers...)
}
