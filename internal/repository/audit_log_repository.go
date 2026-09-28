package repository

import (
	"context"
	"time"

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

func (r *auditLogRepository) DeleteInboundBefore(ctx context.Context, cutoff time.Time, limit int) (int64, error) {
	if limit <= 0 {
		limit = 1000
	}
	query := `
		DELETE FROM inbound_requests
		WHERE id IN (
			SELECT id FROM inbound_requests
			WHERE created_at < ?
			ORDER BY created_at ASC
			LIMIT ?
		)
	`
	res := r.db.WithContext(ctx).Exec(query, cutoff, limit)
	return res.RowsAffected, res.Error
}

func (r *auditLogRepository) DeleteOutboundBefore(ctx context.Context, cutoff time.Time, limit int) (int64, error) {
	if limit <= 0 {
		limit = 1000
	}
	query := `
		DELETE FROM outbound_requests
		WHERE id IN (
			SELECT id FROM outbound_requests
			WHERE created_at < ?
			ORDER BY created_at ASC
			LIMIT ?
		)
	`
	res := r.db.WithContext(ctx).Exec(query, cutoff, limit)
	return res.RowsAffected, res.Error
}
