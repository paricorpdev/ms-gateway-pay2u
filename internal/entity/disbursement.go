package entity

import (
	"time"

	"github.com/google/uuid"
)

type Disbursement struct {
	ID                      uuid.UUID    `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TransactionID           uuid.UUID    `gorm:"type:uuid;not null;uniqueIndex" json:"transaction_id"`
	Transaction             *Transaction `gorm:"foreignKey:TransactionID" json:"transaction,omitempty"`
	MerchantID              uuid.UUID    `gorm:"type:uuid;not null;index;uniqueIndex:uq_disbursements_merchant_ref,priority:1" json:"merchant_id"`
	Merchant                *Merchant    `gorm:"foreignKey:MerchantID" json:"merchant,omitempty"`
	RequestID               string       `gorm:"size:255;not null;uniqueIndex" json:"request_id"`
	MerchantRef             string       `gorm:"column:merchant_ref;size:255;not null;uniqueIndex:uq_disbursements_merchant_ref,priority:2" json:"merchant_ref"`
	Provider                string       `gorm:"size:50;not null;default:'pay2u'" json:"provider"`
	ProviderToken           string       `gorm:"size:255" json:"-"`
	ProviderDisbursementRef *string      `gorm:"size:255;uniqueIndex" json:"provider_disbursement_ref,omitempty"`
	ChannelCode             string       `gorm:"size:50;not null" json:"channel_code"`
	URLListenerMerchant     string       `gorm:"type:text;not null" json:"url_listener_merchant"`
	URLRedirectMerchant     string       `gorm:"type:text" json:"url_redirect_merchant,omitempty"`
	ExpiredMinutes          int          `gorm:"not null;default:30" json:"expired_minutes"`
	Amount                  int64        `gorm:"not null" json:"amount"`
	AmountAdmin             int64        `gorm:"not null;default:0" json:"amount_admin"`
	AmountDiscount          int64        `gorm:"not null;default:0" json:"amount_discount"`
	AmountTotal             int64        `gorm:"not null" json:"amount_total"`
	Currency                string       `gorm:"size:10;not null;default:'IDR'" json:"currency"`

	// Beneficiary snapshot
	BeneficiaryName       string `gorm:"size:255;not null" json:"beneficiary_name"`
	BeneficiaryBankCode   string `gorm:"size:50;not null" json:"beneficiary_bank_code"`
	BeneficiaryBankName   string `gorm:"size:150" json:"beneficiary_bank_name,omitempty"`
	BeneficiaryAccountRef string `gorm:"size:255;not null" json:"beneficiary_account_number"`
	Note                  string `gorm:"type:text" json:"note,omitempty"`

	Status        string     `gorm:"size:30;not null;default:'WAITING_SETTLEMENT';index" json:"status"`
	EligibleAt    *time.Time `gorm:"index" json:"eligible_at,omitempty"`
	InquiryAt     *time.Time `json:"inquiry_at,omitempty"`
	TransferredAt *time.Time `json:"transferred_at,omitempty"`

	Attempts    int        `gorm:"not null;default:0" json:"attempts"`
	MaxAttempts int        `gorm:"not null;default:3" json:"max_attempts"`
	NextRetryAt *time.Time `gorm:"index" json:"next_retry_at,omitempty"`
	LockedUntil *time.Time `gorm:"index" json:"locked_until,omitempty"`
	LockedBy    *string    `gorm:"size:100" json:"locked_by,omitempty"`

	// Provider result
	ProviderResponse JSONB      `gorm:"type:jsonb" json:"provider_response,omitempty"`
	ProviderCallback JSONB      `gorm:"type:jsonb" json:"provider_callback,omitempty"`
	PaidAt           *time.Time `json:"paid_at,omitempty"`

	CreatedAt time.Time `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt time.Time `gorm:"not null;default:now()" json:"updated_at"`
}

func (Disbursement) TableName() string {
	return "disbursements"
}
