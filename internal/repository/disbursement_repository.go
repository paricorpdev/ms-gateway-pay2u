package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"paygate/internal/entity"
	"paygate/internal/model/payload"

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

func (r *disbursementRepository) ListAdmin(ctx context.Context, db *gorm.DB, request payload.AdminDisbursementListRequest, merchantID *uuid.UUID, start, end *time.Time) ([]entity.Disbursement, int64, error) {
	var rows []entity.Disbursement
	var total int64
	query := db.WithContext(ctx).Model(&entity.Disbursement{}).Preload("Merchant")
	query = filterDisbursementsByAdmin(query, request.AdminID)

	if request.Search != "" {
		pattern := "%" + strings.TrimSpace(request.Search) + "%"
		query = query.Where(`
			request_id ILIKE ? OR merchant_ref ILIKE ? OR beneficiary_name ILIKE ? OR
			beneficiary_account_ref ILIKE ? OR beneficiary_bank_name ILIKE ? OR
			COALESCE(provider_disbursement_ref, '') ILIKE ?
		`, pattern, pattern, pattern, pattern, pattern, pattern)
	}
	if request.Status != "" {
		query = query.Where("status = ?", request.Status)
	}
	if merchantID != nil {
		query = query.Where("merchant_id = ?", *merchantID)
	}
	if start != nil && end != nil {
		query = query.Where("created_at >= ? AND created_at < ?", *start, *end)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := query.Order("created_at DESC").Limit(request.PerPage).Offset((request.Page - 1) * request.PerPage).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func (r *disbursementRepository) FindAdminByID(ctx context.Context, db *gorm.DB, id uuid.UUID, adminID *uuid.UUID) (*entity.Disbursement, error) {
	var row entity.Disbursement
	query := db.WithContext(ctx).Preload("Merchant").Where("disbursements.id = ?", id)
	query = filterDisbursementsByAdmin(query, adminID)
	if err := query.First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
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

func filterDisbursementsByAdmin(query *gorm.DB, adminID *uuid.UUID) *gorm.DB {
	if adminID == nil {
		return query
	}
	return query.Where(`EXISTS (
		SELECT 1 FROM merchants m
		WHERE m.id = disbursements.merchant_id AND m.admin_id = ?
	)`, *adminID)
}
