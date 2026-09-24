package utils

import (
	"math"
	"strings"

	"paygate/internal/common/constants"
)

// CalculateAdminFee calculates the admin fee based on payment method and amount.
// Rules:
// - Virtual Account (BNI, BRI, BSI, BTN, CIMB, Danamon, Mandiri, Permata): Rp 3.500
// - QRIS (QRIS MPM): 0.70% MDR (Bank Indonesia regulation)
// math.Round handles 0.70% MDR precision to integer Rupiah.
func CalculateAdminFee(paymentMethod string, amount int64) int64 {
	method := strings.ToUpper(strings.TrimSpace(paymentMethod))
	if strings.Contains(method, "QRIS") {
		return int64(math.Round(float64(amount) * constants.AdminRateQRIS))
	}

	if isVirtualAccount(method) {
		return constants.AdminFeeVA
	}

	return 0
}

func isVirtualAccount(method string) bool {
	if strings.Contains(method, "VA") || strings.Contains(method, "VIRTUAL") {
		return true
	}
	banks := []string{
		"BNI", "BRI", "BSI", "BTN", "CIMB", "DANAMON", "MANDIRI", "PERMATA",
	}
	for _, bank := range banks {
		if strings.Contains(method, bank) {
			return true
		}
	}
	return false
}
