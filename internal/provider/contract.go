package provider

import "context"

const (
	RequestTypeOAuthToken = "OAUTH_TOKEN"
	RequestTypeTxToken    = "TX_TOKEN"
	RequestTypeSubmitBill = "SUBMIT_BILL"
	RequestTypeGetBill    = "GET_BILL"
	RequestTypeWebhookAck = "WEBHOOK_ACK"
)

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

type PaymentProvider interface {
	Name() string
	CreateBill(ctx context.Context, req *BillRequest) (*BillResult, error)
	GetBill(ctx context.Context, token, methodCode string) (*BillResult, error)
	ParseCallback(payload []byte) (*CallbackResult, error)
}
