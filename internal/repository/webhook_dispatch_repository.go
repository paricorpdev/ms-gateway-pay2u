package repository

import (
	"context"
	"errors"
	"time"

	"paygate/internal/entity"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type webhookDispatchRepository struct{}

func NewWebhookDispatchRepository() WebhookDispatchRepository {
	return &webhookDispatchRepository{}
}

func (r *webhookDispatchRepository) CreateDispatch(ctx context.Context, db *gorm.DB, d *entity.WebhookDispatch) error {
	return db.WithContext(ctx).Create(d).Error
}

func (r *webhookDispatchRepository) ClaimDispatch(ctx context.Context, db *gorm.DB, id uuid.UUID, workerID string, lockDuration time.Duration) (bool, error) {
	now := time.Now()
	lockExpiry := now.Add(lockDuration)

	res := db.WithContext(ctx).
		Model(&entity.WebhookDispatch{}).
		Where("id = ? AND status = ? AND (locked_until IS NULL OR locked_until < ?)", id, entity.WebhookStatusPending, now).
		Updates(map[string]any{
			"locked_until": lockExpiry,
			"locked_by":    workerID,
			"updated_at":   now,
		})

	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}

func (r *webhookDispatchRepository) ReleaseDispatch(ctx context.Context, db *gorm.DB, id uuid.UUID, status string, attempts int, nextRetry *time.Time) error {
	updates := map[string]any{
		"status":        status,
		"attempts":      attempts,
		"next_retry_at": nextRetry,
		"locked_until":  nil,
		"locked_by":     nil,
		"updated_at":    time.Now(),
	}
	return db.WithContext(ctx).Model(&entity.WebhookDispatch{}).Where("id = ?", id).Updates(updates).Error
}

func (r *webhookDispatchRepository) CreateDispatchLog(ctx context.Context, db *gorm.DB, l *entity.WebhookDispatchLog) error {
	return db.WithContext(ctx).Create(l).Error
}

func (r *webhookDispatchRepository) FindDispatchByID(ctx context.Context, db *gorm.DB, id uuid.UUID) (*entity.WebhookDispatch, error) {
	var d entity.WebhookDispatch
	if err := db.WithContext(ctx).Preload("Merchant").Where("id = ?", id).First(&d).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &d, nil
}

func (r *webhookDispatchRepository) FindRecoverableDispatches(ctx context.Context, db *gorm.DB, limit int) ([]*entity.WebhookDispatch, error) {
	now := time.Now()
	var list []*entity.WebhookDispatch

	err := db.WithContext(ctx).
		Where("status = ? AND (next_retry_at IS NULL OR next_retry_at <= ?) AND (locked_until IS NULL OR locked_until < ?)",
			entity.WebhookStatusPending, now, now).
		Order("created_at ASC").
		Limit(limit).
		Find(&list).Error

	return list, err
}
