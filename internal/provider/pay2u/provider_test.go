package pay2u

import (
	"context"
	"net/http"
	"net/http/httptest"
	"paygate/internal/utils"
	"testing"
	"time"
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

func TestResolveListenerURL(t *testing.T) {
	tests := []struct {
		name           string
		baseURL        string
		reqCallbackURL string
		expected       string
	}{
		{
			name:           "uses paygate callback base url when configured",
			baseURL:        "https://paygate.example.com",
			reqCallbackURL: "https://merchant.example.com/callback",
			expected:       "https://paygate.example.com/api/v1/payments/callback/pay2u",
		},
		{
			name:           "strips trailing slash from callback base url",
			baseURL:        "https://paygate.example.com/",
			reqCallbackURL: "",
			expected:       "https://paygate.example.com/api/v1/payments/callback/pay2u",
		},
		{
			name:           "falls back to request callback url when base url is empty",
			baseURL:        "",
			reqCallbackURL: "https://merchant.example.com/callback",
			expected:       "https://merchant.example.com/callback",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewProvider(&Client{config: Config{CallbackBaseURL: tt.baseURL}})
			actual := p.resolveListenerURL(tt.reqCallbackURL)
			if actual != tt.expected {
				t.Errorf("resolveListenerURL(%q) = %q; want %q", tt.reqCallbackURL, actual, tt.expected)
			}
		})
	}
}

func TestGetBill_ParsePaymentsArray(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/oauth" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access_token":"mock-token","expires_in":3600}`))
			return
		}
		if r.URL.Path == "/billing/get" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"rc": "00",
				"rd": "Success",
				"data": {
					"token": "AILRWULCE1TGLRFYLGPX",
					"merchant_reff": "ref-1790222645904-5585",
					"payment_method_code": "QRIS-MITRA",
					"payment_code": "MOCK-CODE",
					"amount_total": 78500,
					"payments": [
						{
							"payment_reff": "TW2026092487",
							"payment_date": "2026-09-24 11:25:01",
							"status": 1
						}
					]
				}
			}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	cfg := Config{
		BaseURL:       ts.URL,
		OAuthURL:      "/oauth",
		BillingGetURL: "/billing/get",
		Timeout:       2 * time.Second,
	}
	p := NewProvider(NewClient(cfg, nil, nil))

	res, err := p.GetBill(context.Background(), "AILRWULCE1TGLRFYLGPX", "QRIS-MITRA")
	if err != nil {
		t.Fatalf("unexpected error getting bill: %v", err)
	}

	if res.Status != 1 {
		t.Errorf("expected Status 1 (SUCCESS), got %d", res.Status)
	}
	if res.PaymentReff != "TW2026092487" {
		t.Errorf("expected PaymentReff TW2026092487, got %q", res.PaymentReff)
	}
	if res.PaymentDate != "2026-09-24 11:25:01" {
		t.Errorf("expected PaymentDate '2026-09-24 11:25:01', got %q", res.PaymentDate)
	}
}

func TestResolveProviderMethod(t *testing.T) {
	p := NewProvider(nil)

	tests := []struct {
		input    string
		expected string
	}{
		{"QRIS", "QRIS-MITRA"},
		{"qris", "QRIS-MITRA"},
		{"CC", "CC-MITRA"},
		{"cc", "CC-MITRA"},
		{"VA", "VA-MITRA"},
		{"va", "VA-MITRA"},
		{"VA_BRI", "VA-MITRA"},
		{"va_bri", "VA-MITRA"},
		{"VA_BNI", "VA-MITRA"},
		{"va_bni", "VA-MITRA"},
		{"VA_MANDIRI", "VA-MITRA"},
		{"VA_BCA", "VA-MITRA"},
		// Pass-through if already Pay2U code
		{"BRIVA-MITRA", "BRIVA-MITRA"},
		{"BNIVA-MITRA", "BNIVA-MITRA"},
		{"QRIS-MITRA", "QRIS-MITRA"},
		{"CC-MITRA", "CC-MITRA"},
		{"VA-MITRA", "VA-MITRA"},
	}

	for _, tt := range tests {
		actual := p.resolveProviderMethod(tt.input)
		if actual != tt.expected {
			t.Errorf("resolveProviderMethod(%q) = %q; want %q", tt.input, actual, tt.expected)
		}
	}
}

func TestNormalizePaymentMethod(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"  qris  ", "QRIS"},
		{"qris", "QRIS"},
		{"va", "VA"},
		{"cc", "CC"},
		{"va_bri", "VA_BRI"},
		{"  va_mandiri  ", "VA_MANDIRI"},
	}

	for _, tt := range tests {
		actual := utils.NormalizePaymentMethod(tt.input)
		if actual != tt.expected {
			t.Errorf("NormalizePaymentMethod(%q) = %q; want %q", tt.input, actual, tt.expected)
		}
	}
}

func TestIsValidPaymentMethod(t *testing.T) {
	valid := []string{
		"QRIS", "qris", "  qris  ",
		"CC", "cc",
		"VA", "va",
		"VA_BRI", "va_bri",
		"VA_BNI", "va_bni",
		"VA_MANDIRI", "va_mandiri",
		"VA_BCA", "va_bca",
		"VA_PERMATA", "va_permata",
		"VA_CIMB", "va_cimb",
		"VA_BSI", "va_bsi",
		"VA_BTN", "va_btn",
		"VA_DANAMON", "va_danamon",
	}

	for _, m := range valid {
		if !utils.IsValidPaymentMethod(m) {
			t.Errorf("expected IsValidPaymentMethod(%q) to be true", m)
		}
	}

	invalid := []string{
		"", "   ", "BITCOIN", "PAYPAL", "GOPAY",
		"QRIS-MITRA", "VA-MITRA", "CC-MITRA", "BRIVA-MITRA", "BNIVA-MITRA",
		"BRI", "BNI", "MANDIRI", "BCA",
	}

	for _, m := range invalid {
		if utils.IsValidPaymentMethod(m) {
			t.Errorf("expected IsValidPaymentMethod(%q) to be false", m)
		}
	}
}

