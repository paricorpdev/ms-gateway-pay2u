package repository

import (
	"context"
	"errors"
	"strings"

	"paygate/internal/entity"
	"paygate/internal/exception"
	"paygate/internal/model/payload"

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

func (r *merchantRepository) FindAll(ctx context.Context, db *gorm.DB, request payload.MerchantListRequest) ([]*entity.Merchant, int64, error) {
	var merchants []*entity.Merchant
	var total int64

	q := db.WithContext(ctx).Model(&entity.Merchant{})
	if request.Search != "" {
		pattern := "%" + strings.TrimSpace(request.Search) + "%"
		q = q.Where("name ILIKE ? OR code ILIKE ?", pattern, pattern)
	}
	if request.IsActive != nil {
		q = q.Where("is_active = ?", *request.IsActive)
	}
	if request.AdminID != nil {
		q = q.Where("admin_id = ?", *request.AdminID)
	}
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if request.PerPage <= 0 {
		request.PerPage = payload.DefaultPerPage
	}
	if request.PerPage > payload.MaxPerPage {
		request.PerPage = payload.MaxPerPage
	}
	if request.Page <= 0 {
		request.Page = payload.DefaultPage
	}
	if request.SortDir != "asc" && request.SortDir != "ASC" {
		request.SortDir = "DESC"
	}
	sortBy := map[string]string{
		"created_at": "created_at",
		"updated_at": "updated_at",
		"name":       "name",
		"code":       "code",
	}[strings.ToLower(request.SortBy)]
	if sortBy == "" {
		sortBy = "created_at"
	}
	offset := (request.Page - 1) * request.PerPage

	if err := q.Order(sortBy + " " + strings.ToUpper(request.SortDir)).Limit(request.PerPage).Offset(offset).Find(&merchants).Error; err != nil {
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
