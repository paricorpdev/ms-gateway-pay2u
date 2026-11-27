package payload

import (
	"time"

	"github.com/google/uuid"
)

type AdminDisbursementListRequest struct {
	Page       int
	PerPage    int
	Search     string
	Status     string
	MerchantID string
	Month      int
	Year       int
	AdminID    *uuid.UUID
}

type AdminMerchantSummary struct {
	ID   uuid.UUID `json:"id"`
	Code string    `json:"code"`
	Name string    `json:"name"`
}

// AdminDisbursementResponse is the safe list representation used by the
// dashboard. The beneficiary account and internal callback URLs stay out of
// list responses.
type AdminDisbursementResponse struct {
	ID                      uuid.UUID             `json:"id"`
	TransactionID           uuid.UUID             `json:"transaction_id"`
	RequestID               string                `json:"request_id"`
	MerchantRef             string                `json:"merchant_ref"`
	Provider                string                `json:"provider"`
	ProviderDisbursementRef *string               `json:"provider_disbursement_ref"`
	ChannelCode             string                `json:"channel_code"`
	Amount                  int64                 `json:"amount"`
	AmountAdmin             int64                 `json:"amount_admin"`
	AmountDiscount          int64                 `json:"amount_discount"`
	AmountTotal             int64                 `json:"amount_total"`
	Currency                string                `json:"currency"`
	BeneficiaryName         string                `json:"beneficiary_name"`
	BeneficiaryBankCode     string                `json:"beneficiary_bank_code"`
	BeneficiaryBankName     string                `json:"beneficiary_bank_name"`
	Note                    string                `json:"note"`
	Status                  string                `json:"status"`
	EligibleAt              *time.Time            `json:"eligible_at"`
	InquiryAt               *time.Time            `json:"inquiry_at"`
	TransferredAt           *time.Time            `json:"transferred_at"`
	Attempts                int                   `json:"attempts"`
	MaxAttempts             int                   `json:"max_attempts"`
	PaidAt                  *time.Time            `json:"paid_at"`
	CreatedAt               time.Time             `json:"created_at"`
	UpdatedAt               time.Time             `json:"updated_at"`
	Merchant                *AdminMerchantSummary `json:"merchant"`
}

type AdminDisbursementDetailResponse struct {
	AdminDisbursementResponse
	BeneficiaryAccountNumber string `json:"beneficiary_account_number"`
}
