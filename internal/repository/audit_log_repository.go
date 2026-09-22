package repository

import (
	"context"

	"paygate/internal/entity"

	"gorm.io/gorm"
)

type auditLogRepository struct {
	db *gorm.DB
}

func NewAuditLogRepository(db *gorm.DB) AuditLogRepository {
	return &auditLogRepository{db: db}
}

func (r *auditLogRepository) SaveInbound(ctx context.Context, log *entity.InboundRequest) error {
	return r.db.WithContext(ctx).Create(log).Error
}

func (r *auditLogRepository) SaveOutbound(ctx context.Context, log *entity.OutboundRequest) error {
	return r.db.WithContext(ctx).Create(log).Error
}
