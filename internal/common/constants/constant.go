package constants

const (
	LocalsRequestID     = "requestId"
	LocalsLogger        = "logger"
	LocalsTransactionID = "transactionId"
	LocalsMerchant      = "merchant"
	LocalsMerchantCode  = "merchantCode"
)

const HeaderRequestID = "X-Request-Id"
const HeaderAPIKey = "X-API-Key"

const (
	StatusSuccess = "success"
	StatusError   = "error"
)

const (
	TransactionStatusPending = "PENDING"
	TransactionStatusSuccess = "SUCCESS"
	TransactionStatusFailed  = "FAILED"
	TransactionStatusExpired = "EXPIRED"
	TransactionStatusRefund  = "REFUND"
	TransactionStatusProcess = "PROCESS"
)

const (
	CurrencyIDR = "IDR"
)

const (
	WebhookEventPaymentSuccess = "payment.success"
	WebhookEventPaymentFailed  = "payment.failed"
	WebhookEventPaymentExpired = "payment.expired"
	WebhookEventPaymentRefund  = "payment.refund"
)

const (
	WebhookStatusPending = "PENDING"
	WebhookStatusSuccess = "SUCCESS"
	WebhookStatusFailed  = "FAILED"
)

const (
	AdminFeeVA    = int64(3500)
	AdminRateQRIS = 0.007
)

// Payment Methods (PayGate Standard)
const (
	PaymentMethodQRIS      = "QRIS"
	PaymentMethodCC        = "CC"
	PaymentMethodVA        = "VA"
	PaymentMethodVABRI     = "VA_BRI"
	PaymentMethodVABNI     = "VA_BNI"
	PaymentMethodVAMandiri = "VA_MANDIRI"
	PaymentMethodVABCA     = "VA_BCA"
	PaymentMethodVAPermata = "VA_PERMATA"
	PaymentMethodVACIMB    = "VA_CIMB"
	PaymentMethodVABSI     = "VA_BSI"
	PaymentMethodVABTN     = "VA_BTN"
	PaymentMethodVADanamon = "VA_DANAMON"
)

// Pay2U Payment Methods
const (
	Pay2UMethodQRIS  = "QRIS-MITRA"
	Pay2UMethodCC    = "CC-MITRA"
	Pay2UMethodVA    = "VA-MITRA"
	// Pay2UMethodVABRI = "BRIVA-MITRA"
	// Pay2UMethodVABNI = "BNIVA-MITRA"
)
