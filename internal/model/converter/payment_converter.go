package converter

import (
	"paygate/internal/entity"
	"paygate/internal/model/payload"
)

// ToPaymentResponse maps a transaction entity onto its API representation.
func ToPaymentResponse(tx *entity.Transaction) *payload.PaymentResponse {
	return &payload.PaymentResponse{
		ID:           tx.ID.String(),
		Provider:     tx.Provider,
		MerchantReff:   tx.MerchantReff,
		Status:         tx.Status,
		PaymentMethod:  tx.PaymentMethod,
		PaymentCode:    tx.PaymentCode,
		Amount:         tx.Amount,
		AmountAdmin:    tx.AmountAdmin,
		AmountDiscount: tx.AmountDiscount,
		AmountTotal:    tx.AmountTotal,
		Currency:       tx.Currency,
		CustomerName:   tx.CustomerName,
		PaymentReff:    tx.PaymentReff,
		ExpiredAt:      tx.ExpiredAt,
		PaidAt:         tx.PaidAt,
		CreatedAt:      tx.CreatedAt,
	}
}
