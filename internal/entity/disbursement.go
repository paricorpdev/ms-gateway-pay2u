package entity

import (
	"time"

	"github.com/google/uuid"
)

type Disbursement struct {
	ID                      uuid.UUID    `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TransactionID           uuid.UUID    `gorm:"type:uuid;not null;uniqueIndex" json:"transaction_id"`
	Transaction             *Transaction `gorm:"foreignKey:TransactionID" json:"transaction,omitempty"`
	MerchantID              uuid.UUID    `gorm:"type:uuid;not null;index" json:"merchant_id"`
	Merchant                *Merchant    `gorm:"foreignKey:MerchantID" json:"merchant,omitempty"`
	RequestID               string       `gorm:"size:255;not null" json:"request_id"`
	MerchantRef             string       `gorm:"size:255;not null" json:"merchant_ref"`
	Provider                string       `gorm:"size:50;not null;default:'pay2u'" json:"provider"`
	ProviderDisbursementRef *string      `gorm:"size:255;uniqueIndex" json:"provider_disbursement_ref,omitempty"`
	Amount                  int64        `gorm:"not null" json:"amount"`
	Currency                string       `gorm:"size:10;not null;default:'IDR'" json:"currency"`

	// Beneficiary snapshot
	BeneficiaryName       string `gorm:"size:255;not null" json:"beneficiary_name"`
	BeneficiaryBankCode   string `gorm:"size:50;not null" json:"beneficiary_bank_code"`
	BeneficiaryAccountRef string `gorm:"size:255;not null" json:"beneficiary_account_ref"`

	Status     string     `gorm:"size:30;not null;default:'WAITING_SETTLEMENT';index" json:"status"`
	EligibleAt *time.Time `gorm:"index" json:"eligible_at,omitempty"`

	Attempts    int        `gorm:"not null;default:0" json:"attempts"`
	MaxAttempts int        `gorm:"not null;default:3" json:"max_attempts"`
	NextRetryAt *time.Time `gorm:"index" json:"next_retry_at,omitempty"`
	LockedUntil *time.Time `gorm:"index" json:"locked_until,omitempty"`
	LockedBy    *string    `gorm:"size:100" json:"locked_by,omitempty"`

	// Provider result
	ProviderCallback JSONB      `gorm:"type:jsonb" json:"provider_callback,omitempty"`
	PaidAt           *time.Time `json:"paid_at,omitempty"`

	CreatedAt time.Time `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt time.Time `gorm:"not null;default:now()" json:"updated_at"`
}

func (Disbursement) TableName() string {
	return "disbursements"
}
