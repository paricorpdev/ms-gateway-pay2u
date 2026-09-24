package entity

import (
	"time"

	"github.com/google/uuid"
)

const (
	WebhookStatusPending = "PENDING"
	WebhookStatusSuccess = "SUCCESS"
	WebhookStatusFailed  = "FAILED"
)

const (
	WebhookEventPaymentSuccess = "payment.success"
	WebhookEventPaymentFailed  = "payment.failed"
	WebhookEventPaymentExpired = "payment.expired"
	WebhookEventPaymentRefund  = "payment.refund"
)

type WebhookDispatch struct {
	ID            uuid.UUID    `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TransactionID uuid.UUID    `gorm:"type:uuid;not null;index" json:"transaction_id"`
	Transaction   *Transaction `gorm:"foreignKey:TransactionID" json:"transaction,omitempty"`
	MerchantID    uuid.UUID    `gorm:"type:uuid;not null;index" json:"merchant_id"`
	Merchant      *Merchant    `gorm:"foreignKey:MerchantID" json:"merchant,omitempty"`
	TargetURL     string       `gorm:"type:text;not null" json:"target_url"`
	EventType     string       `gorm:"size:50;not null;default:'payment.success'" json:"event_type"`
	Payload       JSONB        `gorm:"type:jsonb;not null" json:"payload"`
	Status        string       `gorm:"size:20;not null;default:'PENDING';index" json:"status"`
	Attempts      int          `gorm:"not null;default:0" json:"attempts"`
	MaxAttempts   int          `gorm:"not null;default:3" json:"max_attempts"`
	NextRetryAt   *time.Time   `gorm:"index" json:"next_retry_at,omitempty"`
	LockedUntil   *time.Time   `gorm:"index" json:"locked_until,omitempty"`
	LockedBy      *string      `gorm:"size:100" json:"locked_by,omitempty"`
	CreatedAt     time.Time    `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt     time.Time    `gorm:"not null;default:now()" json:"updated_at"`
}

func (WebhookDispatch) TableName() string { return "webhook_dispatches" }

type WebhookDispatchLog struct {
	ID              uuid.UUID        `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	DispatchID      uuid.UUID        `gorm:"type:uuid;not null;index" json:"dispatch_id"`
	Dispatch        *WebhookDispatch `gorm:"foreignKey:DispatchID" json:"dispatch,omitempty"`
	AttemptNumber   int              `gorm:"not null" json:"attempt_number"`
	HTTPStatus      *int             `json:"http_status,omitempty"`
	ResponsePayload *string          `gorm:"type:text" json:"response_payload,omitempty"`
	ErrorMessage    *string          `gorm:"type:text" json:"error_message,omitempty"`
	LatencyMs       int64            `gorm:"not null;default:0" json:"latency_ms"`
	DispatchedAt    time.Time        `gorm:"not null;default:now()" json:"dispatched_at"`
}

func (WebhookDispatchLog) TableName() string { return "webhook_dispatch_logs" }
