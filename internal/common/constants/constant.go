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

