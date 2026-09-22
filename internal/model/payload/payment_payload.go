package payload

import "time"

// CreatePaymentRequest is the incoming request from internal services.
type CreatePaymentRequest struct {
	IdempotencyKey  string `json:"idempotency_key" validate:"required,max=255"`
	PaymentMethod   string `json:"payment_method" validate:"required,max=50"`
	BillTitle       string `json:"bill_title" validate:"required,max=255"`
	BillDescription string `json:"bill_description" validate:"omitempty,max=1000"`
	CustomerName    string `json:"customer_name" validate:"required,max=255"`
	CustomerPhone   string `json:"customer_phone" validate:"required,max=50"`
	CustomerEmail   string `json:"customer_email" validate:"omitempty,email,max=255"`
	Amount          int64  `json:"amount" validate:"required,gte=1"`
	AmountAdmin     int64  `json:"amount_admin" validate:"gte=0"`
	AmountDiscount  int64  `json:"amount_discount" validate:"gte=0"`
	AmountTotal     int64  `json:"amount_total" validate:"required,gte=1"`
	ExpiredMinutes  int    `json:"expired_minutes" validate:"gte=0"`
	CallbackURL     string `json:"callback_url" validate:"omitempty,url"`
	RedirectURL     string `json:"redirect_url" validate:"omitempty,url"`
}

// PaymentResponse is returned after creating or querying a payment.
type PaymentResponse struct {
	ID             string     `json:"id"`
	IdempotencyKey string     `json:"idempotency_key"`
	Provider       string     `json:"provider"`
	MerchantReff   string     `json:"merchant_reff"`
	Status         string     `json:"status"`
	PaymentMethod  string     `json:"payment_method"`
	PaymentCode    string     `json:"payment_code,omitempty"`
	Amount         int64      `json:"amount"`
	AmountAdmin    int64      `json:"amount_admin"`
	AmountDiscount int64      `json:"amount_discount"`
	AmountTotal    int64      `json:"amount_total"`
	Currency       string     `json:"currency"`
	CustomerName   string     `json:"customer_name,omitempty"`
	PaymentReff    string     `json:"payment_reff,omitempty"`
	ExpiredAt      *time.Time `json:"expired_at,omitempty"`
	PaidAt         *time.Time `json:"paid_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

// CallbackPayload is what Pay2U POSTs to our webhook endpoint.
type CallbackPayload struct {
	Token             string  `json:"token"`
	MerchantReff      string  `json:"merchant_reff"`
	PaymentMethodCode string  `json:"payment_method_code"`
	PaymentCode       string  `json:"payment_code"`
	PaymentReff       string  `json:"payment_reff"`
	PaymentDate       string  `json:"payment_date"`
	BillTitle         string  `json:"bill_title"`
	BillCustomerName  string  `json:"bill_customer_name"`
	BillCurrency      string  `json:"bill_currency"`
	Amount            float64 `json:"amount"`
	AmountAdmin       float64 `json:"amount_admin"`
	AmountDiscount    float64 `json:"amount_discount"`
	AmountTotal       float64 `json:"amount_total"`
	AmountPaid        float64 `json:"amount_paid"`
	Status            int     `json:"status"` // 1=Paid, 2=Process, 3=Refund
}
