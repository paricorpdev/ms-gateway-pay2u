package repository

import (
	"context"
	"errors"

	"paygate/internal/entity"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type disbursementRepository struct{}

func NewDisbursementRepository() DisbursementRepository {
	return &disbursementRepository{}
}

func (r *disbursementRepository) Create(ctx context.Context, db *gorm.DB, disbursement *entity.Disbursement) error {
	return db.WithContext(ctx).Create(disbursement).Error
}

func (r *disbursementRepository) FindByMerchantAndTransactionID(ctx context.Context, db *gorm.DB, merchantID, transactionID uuid.UUID) (*entity.Disbursement, error) {
	return r.find(ctx, db, false, "merchant_id = ? AND transaction_id = ?", merchantID, transactionID)
}

func (r *disbursementRepository) FindByMerchantAndRef(ctx context.Context, db *gorm.DB, merchantID uuid.UUID, merchantRef string) (*entity.Disbursement, error) {
	return r.find(ctx, db, false, "merchant_id = ? AND merchant_ref = ?", merchantID, merchantRef)
}

func (r *disbursementRepository) FindByMerchantAndRefForUpdate(ctx context.Context, db *gorm.DB, merchantID uuid.UUID, merchantRef string) (*entity.Disbursement, error) {
	return r.find(ctx, db, true, "merchant_id = ? AND merchant_ref = ?", merchantID, merchantRef)
}

func (r *disbursementRepository) FindByProviderTokenAndRef(ctx context.Context, db *gorm.DB, token, merchantRef string) (*entity.Disbursement, error) {
	return r.find(ctx, db, false, "provider_token = ? AND merchant_ref = ?", token, merchantRef)
}

func (r *disbursementRepository) FindByProviderTokenAndRefForUpdate(ctx context.Context, db *gorm.DB, token, merchantRef string) (*entity.Disbursement, error) {
	return r.find(ctx, db, true, "provider_token = ? AND merchant_ref = ?", token, merchantRef)
}

func (r *disbursementRepository) Update(ctx context.Context, db *gorm.DB, id uuid.UUID, updates map[string]any) error {
	return db.WithContext(ctx).Model(&entity.Disbursement{}).Where("id = ?", id).Updates(updates).Error
}

func (r *disbursementRepository) find(ctx context.Context, db *gorm.DB, forUpdate bool, query string, args ...any) (*entity.Disbursement, error) {
	var disbursement entity.Disbursement
	result := db.WithContext(ctx)
	if forUpdate {
		result = result.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	if err := result.Where(query, args...).First(&disbursement).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &disbursement, nil
}
