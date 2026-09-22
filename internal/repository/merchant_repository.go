package repository

import (
	"context"
	"errors"

	"paygate/internal/entity"
	"paygate/internal/exception"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type merchantRepository struct{}

func NewMerchantRepository() MerchantRepository {
	return &merchantRepository{}
}

func (r *merchantRepository) Create(ctx context.Context, db *gorm.DB, merchant *entity.Merchant) error {
	return db.WithContext(ctx).Create(merchant).Error
}

func (r *merchantRepository) FindByID(ctx context.Context, db *gorm.DB, id uuid.UUID) (*entity.Merchant, error) {
	var m entity.Merchant
	if err := db.WithContext(ctx).Where("id = ?", id).First(&m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, exception.NotFound("merchant not found")
		}
		return nil, err
	}
	return &m, nil
}

func (r *merchantRepository) FindByAPIKey(ctx context.Context, db *gorm.DB, apiKey string) (*entity.Merchant, error) {
	var m entity.Merchant
	if err := db.WithContext(ctx).Where("api_key = ?", apiKey).First(&m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &m, nil
}

func (r *merchantRepository) FindByCode(ctx context.Context, db *gorm.DB, code string) (*entity.Merchant, error) {
	var m entity.Merchant
	if err := db.WithContext(ctx).Where("code = ?", code).First(&m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &m, nil
}

func (r *merchantRepository) FindAll(ctx context.Context, db *gorm.DB, limit, offset int) ([]*entity.Merchant, int64, error) {
	var merchants []*entity.Merchant
	var total int64

	q := db.WithContext(ctx).Model(&entity.Merchant{})
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if limit <= 0 {
		limit = 10
	}
	if offset < 0 {
		offset = 0
	}

	if err := q.Order("created_at DESC").Limit(limit).Offset(offset).Find(&merchants).Error; err != nil {
		return nil, 0, err
	}

	return merchants, total, nil
}

func (r *merchantRepository) Update(ctx context.Context, db *gorm.DB, id uuid.UUID, updates map[string]any) error {
	res := db.WithContext(ctx).Model(&entity.Merchant{}).Where("id = ?", id).Updates(updates)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return exception.NotFound("merchant not found")
	}
	return nil
}
