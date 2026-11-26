package provider

import "context"

const (
	RequestTypeOAuthToken   = "OAUTH_TOKEN"
	RequestTypeTxToken      = "TX_TOKEN"
	RequestTypeSubmitBill   = "SUBMIT_BILL"
	RequestTypeGetBill      = "GET_BILL"
	RequestTypeBankInquiry  = "BANK_ACCOUNT_INQUIRY"
	RequestTypeBankTransfer = "BANK_ACCOUNT_TRANSFER"
	RequestTypeWebhookAck   = "WEBHOOK_ACK"
)

type APIError struct {
	Code        string
	Description string
}

func (e *APIError) Error() string {
	return "provider error " + e.Code + ": " + e.Description
}

// OutcomeUnknownError marks a transfer request whose response did not establish
// whether the provider accepted it. Such requests must not be submitted again.
type OutcomeUnknownError struct {
	Err error
}

func (e *OutcomeUnknownError) Error() string { return e.Err.Error() }
func (e *OutcomeUnknownError) Unwrap() error { return e.Err }

type BillRequest struct {
	MerchantReff      string
	PaymentMethodCode string
	CallbackURL       string
	RedirectURL       string
	BillTitle         string
	BillDescription   string
	CustomerName      string
	CustomerPhone     string
	CustomerEmail     string
	Currency          string
	Amount            int64
	AmountAdmin       int64
	AmountDiscount    int64
	AmountTotal       int64
	ExpiredMinutes    int
}

type BillResult struct {
	Token             string
	MerchantReff      string
	PaymentMethodCode string
	PaymentCode       string
	AmountTotal       int64
	ExpiredMinutes    int
	Status            int
	PaymentReff       string
	PaymentDate       string
}

type CallbackResult struct {
	MerchantReff  string
	ProviderToken string
	PaymentReff   string
	PaymentDate   string
	AmountPaid    int64
	Status        int
}

type BankTransferRequest struct {
	Token                 string `json:"token"`
	MerchantReff          string `json:"merchant_reff"`
	ChannelCode           string `json:"channel_code"`
	URLListenerMerchant   string `json:"url_listener_merchant"`
	URLRedirectMerchant   string `json:"url_redirect_merchant,omitempty"`
	ReceiverAccountNumber string `json:"receiver_account_number"`
	ReceiverName          string `json:"receiver_name"`
	ReceiverBank          string `json:"receiver_bank"`
	Amount                int64  `json:"amount"`
	AmountAdmin           int64  `json:"amount_admin"`
	AmountDiscount        int64  `json:"amount_discount"`
	AmountTotal           int64  `json:"amount_total"`
	Note                  string `json:"note"`
	ExpiredMinutes        int    `json:"expired_minutes"`
}

type BankInquiryResult struct {
	Token                 string
	MerchantReff          string
	ReceiverAccountNumber string
	ReceiverName          string
	ReceiverBank          string
	ReceiverBankName      string
	AmountTotal           int64
	RawResponse           []byte
}

type BankTransferResult struct {
	Token                 string
	MerchantReff          string
	ReceiverAccountNumber string
	ReceiverName          string
	ReceiverBank          string
	ReceiverBankName      string
	AmountTotal           int64
	ReferenceNo           string
	TransactionDate       string
	RawResponse           []byte
}

type BankTransferCallbackResult struct {
	Token                 string
	MerchantReff          string
	ChannelCode           string
	ReceiverAccountNumber string
	ReceiverName          string
	ReceiverBank          string
	ReceiverBankName      string
	Amount                *int64
	AmountAdmin           *int64
	AmountDiscount        *int64
	AmountTotal           *int64
	AmountPaid            int64
	Note                  string
	Status                int
	TransferDate          string
	ReferenceNo           string
	ExpiredMinutes        *int
}

type DisbursementProvider interface {
	AccountInquiry(ctx context.Context, req *BankTransferRequest) (*BankInquiryResult, error)
	AccountTransfer(ctx context.Context, req *BankTransferRequest) (*BankTransferResult, error)
	ParseTransferCallback(raw []byte) (*BankTransferCallbackResult, error)
}

type PaymentProvider interface {
	Name() string
	CreateBill(ctx context.Context, req *BillRequest) (*BillResult, error)
	GetBill(ctx context.Context, token, methodCode string) (*BillResult, error)
	ParseCallback(payload []byte) (*CallbackResult, error)
}
