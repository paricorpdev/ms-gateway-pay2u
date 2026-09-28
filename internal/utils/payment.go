package utils

import (
	"paygate/internal/common/constants"
	"strings"
)

// NormalizePaymentMethod trims whitespace and converts the payment method to uppercase.
func NormalizePaymentMethod(method string) string {
	return strings.ToUpper(strings.TrimSpace(method))
}

// IsValidPaymentMethod validates whether the given payment method string is supported by PayGate.
func IsValidPaymentMethod(method string) bool {
	switch NormalizePaymentMethod(method) {
	case constants.PaymentMethodQRIS, constants.PaymentMethodCC, constants.PaymentMethodVA,
		constants.PaymentMethodVABRI, constants.PaymentMethodVABNI, constants.PaymentMethodVAMandiri,
		constants.PaymentMethodVABCA, constants.PaymentMethodVAPermata, constants.PaymentMethodVACIMB,
		constants.PaymentMethodVABSI, constants.PaymentMethodVABTN, constants.PaymentMethodVADanamon:
		return true
	default:
		return false
	}
}
