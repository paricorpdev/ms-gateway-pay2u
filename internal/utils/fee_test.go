package utils_test

import (
	"testing"

	"paygate/internal/utils"
)

func TestCalculateAdminFee(t *testing.T) {
	tests := []struct {
		name          string
		paymentMethod string
		amount        int64
		expectedFee   int64
	}{
		{"VA BNI", "BNI", 50000, 3500},
		{"VA BRI", "BRI", 50000, 3500},
		{"VA BSI", "BSI", 50000, 3500},
		{"VA BTN", "BTN", 50000, 3500},
		{"VA CIMB", "CIMB", 50000, 3500},
		{"VA Danamon", "Danamon", 50000, 3500},
		{"VA Mandiri", "Mandiri", 50000, 3500},
		{"VA Permata", "Permata", 50000, 3500},
		{"VA-MITRA", "VA-MITRA", 50000, 3500},
		{"BRIVA-MITRA", "BRIVA-MITRA", 50000, 3500},
		{"BNIVA-MITRA", "BNIVA-MITRA", 50000, 3500},
		{"VA prefix", "VA-MANDIRI", 50000, 3500},
		{"Virtual Account whitespace", "Virtual Account", 50000, 3500},
		{"QRIS 100k", "QRIS", 100000, 700},
		{"QRIS MPM 50k", "QRIS MPM", 50000, 350},
		{"QRIS-MITRA 75k", "QRIS-MITRA", 75000, 525},
		{"QRIS-MPM 200k", "QRIS-MPM", 200000, 1400},
		{"CC MITRA no admin fee", "CC-MITRA", 100000, 0},
		{"Unknown method", "UNKNOWN", 100000, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := utils.CalculateAdminFee(tt.paymentMethod, tt.amount)
			if actual != tt.expectedFee {
				t.Errorf("CalculateAdminFee(%q, %d) = %d; want %d", tt.paymentMethod, tt.amount, actual, tt.expectedFee)
			}
		})
	}
}
