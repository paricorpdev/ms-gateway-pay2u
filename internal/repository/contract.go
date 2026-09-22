package repository

import (
	"context"
	"time"

	"paygate/internal/entity"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type TransactionRepository interface {
	Create(ctx context.Context, db *gorm.DB, tx *entity.Transaction) error
	Save(ctx context.Context, db *gorm.DB, tx *entity.Transaction) error
	FindByID(ctx context.Context, db *gorm.DB, id uuid.UUID) (*entity.Transaction, error)
	FindByIdempotencyKey(ctx context.Context, db *gorm.DB, key string) (*entity.Transaction, error)
	FindByMerchantReff(ctx context.Context, db *gorm.DB, reff string) (*entity.Transaction, error)
	FindByProviderToken(ctx context.Context, db *gorm.DB, token string) (*entity.Transaction, error)
	UpdateStatus(ctx context.Context, db *gorm.DB, id uuid.UUID, status string, updates map[string]any) error
}

type CacheRepository interface {
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Get(ctx context.Context, key string) ([]byte, error)
	Delete(ctx context.Context, key string) error
}

type AuditLogRepository interface {
	SaveInbound(ctx context.Context, log *entity.InboundRequest) error
	SaveOutbound(ctx context.Context, log *entity.OutboundRequest) error
}

type MerchantRepository interface {
	Create(ctx context.Context, db *gorm.DB, merchant *entity.Merchant) error
	FindByID(ctx context.Context, db *gorm.DB, id uuid.UUID) (*entity.Merchant, error)
	FindByAPIKey(ctx context.Context, db *gorm.DB, apiKey string) (*entity.Merchant, error)
	FindByCode(ctx context.Context, db *gorm.DB, code string) (*entity.Merchant, error)
	FindAll(ctx context.Context, db *gorm.DB, limit, offset int) ([]*entity.Merchant, int64, error)
	Update(ctx context.Context, db *gorm.DB, id uuid.UUID, updates map[string]any) error
}

var (
	_ TransactionRepository = (*transactionRepository)(nil)
	_ CacheRepository       = (*redisRepository)(nil)
	_ AuditLogRepository   = (*auditLogRepository)(nil)
	_ MerchantRepository   = (*merchantRepository)(nil)
)
