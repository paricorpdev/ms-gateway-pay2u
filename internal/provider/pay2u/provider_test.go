package pay2u

import (
	"testing"
)

func TestGetBillingURL(t *testing.T) {
	cfg := Config{
		BaseURL:        "https://api-dev.pay2u.co.id",
		BillingVAURL:   "/va/submit",
		BillingQRISURL: "/qris/submit",
		BillingCCURL:   "/cc/submit",
	}
	p := NewProvider(&Client{config: cfg})

	tests := []struct {
		methodCode string
		expected   string
	}{
		{"BRIVA-MITRA", "https://api-dev.pay2u.co.id/va/submit"},
		{"BNIVA-MITRA", "https://api-dev.pay2u.co.id/va/submit"},
		{"VA-MITRA", "https://api-dev.pay2u.co.id/va/submit"},
		{"QRIS-MITRA", "https://api-dev.pay2u.co.id/qris/submit"},
		{"CC-MITRA", "https://api-dev.pay2u.co.id/cc/submit"},
		{"UNKNOWN", "https://api-dev.pay2u.co.id/va/submit"},
	}

	for _, tt := range tests {
		actual := p.getBillingURL(tt.methodCode)
		if actual != tt.expected {
			t.Errorf("getBillingURL(%q) = %q; want %q", tt.methodCode, actual, tt.expected)
		}
	}
}

func TestParseCallback(t *testing.T) {
	p := NewProvider(nil)

	payload := []byte(`{
		"token": "TOKEN123",
		"merchant_reff": "X0487831911188",
		"payment_method_code": "VA-MITRA",
		"payment_code": "1392570141172029",
		"payment_reff": "PAYREFF999",
		"payment_date": "2026-09-22 15:09:32",
		"amount_paid": 22000,
		"status": 1
	}`)

	res, err := p.ParseCallback(payload)
	if err != nil {
		t.Fatalf("unexpected error parsing callback: %v", err)
	}

	if res.MerchantReff != "X0487831911188" {
		t.Errorf("MerchantReff = %q; want X0487831911188", res.MerchantReff)
	}
	if res.ProviderToken != "TOKEN123" {
		t.Errorf("ProviderToken = %q; want TOKEN123", res.ProviderToken)
	}
	if res.PaymentReff != "PAYREFF999" {
		t.Errorf("PaymentReff = %q; want PAYREFF999", res.PaymentReff)
	}
	if res.AmountPaid != 22000 {
		t.Errorf("AmountPaid = %d; want 22000", res.AmountPaid)
	}
	if res.Status != 1 {
		t.Errorf("Status = %d; want 1", res.Status)
	}
}
