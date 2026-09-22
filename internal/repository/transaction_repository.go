package repository

import (
	"context"
	"errors"

	"paygate/internal/entity"
	"paygate/internal/exception"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type transactionRepository struct{}

func NewTransactionRepository() TransactionRepository {
	return &transactionRepository{}
}

func (r *transactionRepository) Create(ctx context.Context, db *gorm.DB, tx *entity.Transaction) error {
	return db.WithContext(ctx).Create(tx).Error
}

func (r *transactionRepository) Save(ctx context.Context, db *gorm.DB, tx *entity.Transaction) error {
	return db.WithContext(ctx).Save(tx).Error
}

func (r *transactionRepository) FindByID(ctx context.Context, db *gorm.DB, id uuid.UUID) (*entity.Transaction, error) {
	var tx entity.Transaction
	if err := db.WithContext(ctx).Where("id = ?", id).First(&tx).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, exception.NotFound("transaction not found")
		}
		return nil, err
	}
	return &tx, nil
}

func (r *transactionRepository) FindByIdempotencyKey(ctx context.Context, db *gorm.DB, key string) (*entity.Transaction, error) {
	var tx entity.Transaction
	if err := db.WithContext(ctx).Where("idempotency_key = ?", key).First(&tx).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &tx, nil
}

func (r *transactionRepository) FindByMerchantReff(ctx context.Context, db *gorm.DB, reff string) (*entity.Transaction, error) {
	var tx entity.Transaction
	if err := db.WithContext(ctx).Where("merchant_reff = ?", reff).First(&tx).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &tx, nil
}

func (r *transactionRepository) FindByProviderToken(ctx context.Context, db *gorm.DB, token string) (*entity.Transaction, error) {
	var tx entity.Transaction
	if err := db.WithContext(ctx).Where("provider_token = ?", token).First(&tx).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &tx, nil
}

func (r *transactionRepository) UpdateStatus(ctx context.Context, db *gorm.DB, id uuid.UUID, status string, updates map[string]any) error {
	if updates == nil {
		updates = make(map[string]any)
	}
	updates["status"] = status
	return db.WithContext(ctx).Model(&entity.Transaction{}).Where("id = ?", id).Updates(updates).Error
}
