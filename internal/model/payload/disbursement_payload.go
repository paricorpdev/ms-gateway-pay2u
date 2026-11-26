package payload

import (
	"paygate/internal/config"
	"paygate/internal/exception"

	"github.com/go-playground/validator/v10"
)

var disbursementValidator = config.NewValidator()

// DisbursementInquiryRequest follows Pay2U's account-inquiry fields and adds
// transaction_id to link the payout to the successful incoming payment.
type DisbursementInquiryRequest struct {
	TransactionID       string `json:"transaction_id" validate:"required,uuid4"`
	MerchantReff        string `json:"merchant_reff" validate:"required,max=255"`
	ChannelCode         string `json:"channel_code" validate:"omitempty,max=50"`
	URLListenerMerchant string `json:"url_listener_merchant" validate:"required,url,max=2048"`
	ReceiverAccount     string `json:"receiver_account_number" validate:"required,max=255"`
	ReceiverName        string `json:"receiver_name" validate:"required,max=255"`
	ReceiverBank        string `json:"receiver_bank" validate:"required,max=50"`
	Amount              int64  `json:"amount" validate:"required,gt=0"`
	AmountAdmin         int64  `json:"amount_admin" validate:"gte=0"`
	AmountDiscount      int64  `json:"amount_discount" validate:"gte=0"`
	AmountTotal         int64  `json:"amount_total" validate:"required,gt=0"`
	Note                string `json:"note" validate:"omitempty,max=1000"`
	ExpiredMinutes      int    `json:"expired_minutes" validate:"gte=0,lte=1440"`
	URLRedirectMerchant string `json:"url_redirect_merchant" validate:"omitempty,url,max=2048"`
}

func (r *DisbursementInquiryRequest) Validate(v ...*validator.Validate) error {
	validate := disbursementValidator
	if len(v) > 0 && v[0] != nil {
		validate = v[0]
	}
	if err := validate.Struct(r); err != nil {
		return exception.FromValidation(err)
	}
	return nil
}

type DisbursementResponse struct {
	ID                      string `json:"id"`
	TransactionID           string `json:"transaction_id"`
	MerchantReff            string `json:"merchant_reff"`
	ChannelCode             string `json:"channel_code"`
	URLListenerMerchant     string `json:"url_listener_merchant"`
	ExpiredMinutes          int    `json:"expired_minutes"`
	Amount                  int64  `json:"amount"`
	AmountAdmin             int64  `json:"amount_admin"`
	AmountDiscount          int64  `json:"amount_discount"`
	AmountTotal             int64  `json:"amount_total"`
	Currency                string `json:"currency"`
	BeneficiaryAccount      string `json:"receiver_account_number"`
	BeneficiaryName         string `json:"receiver_name"`
	BeneficiaryBankCode     string `json:"receiver_bank"`
	BeneficiaryBankName     string `json:"receiver_bank_name,omitempty"`
	Status                  string `json:"status"`
	ProviderDisbursementRef string `json:"provider_reference_no,omitempty"`
	TransferDate            string `json:"transfer_date,omitempty"`
	Note                    string `json:"note,omitempty"`
}
