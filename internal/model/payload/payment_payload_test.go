package payload_test

import (
	"testing"

	"paygate/internal/common/constants"
	"paygate/internal/config"
	"paygate/internal/exception"
	"paygate/internal/model/payload"
)

func validPaymentRequest() *payload.CreatePaymentRequest {
	return &payload.CreatePaymentRequest{
		MerchantReff:  "INV-VALID-001",
		PaymentMethod: constants.PaymentMethodQRIS,
		BillTitle:     "Valid Order",
		CustomerName:  "Budi Santoso",
		CustomerPhone: "081234567890",
		Amount:        50000,
		AmountTotal:   50000,
	}
}

func TestCreatePaymentRequest_Validate_Success(t *testing.T) {
	methods := []string{
		constants.PaymentMethodQRIS,
		constants.PaymentMethodCC,
		constants.PaymentMethodVA,
		constants.PaymentMethodVABRI,
		constants.PaymentMethodVABNI,
		constants.PaymentMethodVAMandiri,
		constants.PaymentMethodVAPermata,
		constants.PaymentMethodVACIMB,
		constants.PaymentMethodVABSI,
		constants.PaymentMethodVABTN,
		constants.PaymentMethodVADanamon,
	}

	for _, method := range methods {
		req := validPaymentRequest()
		req.PaymentMethod = method

		if err := req.Validate(); err != nil {
			t.Fatalf("expected nil for valid payment method %q, got: %v", method, err)
		}
	}
}

func TestCreatePaymentRequest_Validate_NormalizesPaymentMethod(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"  qris  ", "QRIS"},
		{"qris", "QRIS"},
		{"va", "VA"},
		{"cc", "CC"},
		{"va_bri", "VA_BRI"},
		{"va_bni", "VA_BNI"},
		{"va_mandiri", "VA_MANDIRI"},
		{"va_bca", "VA_BCA"},
	}

	for _, tt := range tests {
		req := validPaymentRequest()
		req.PaymentMethod = tt.input
		if err := req.Validate(); err != nil {
			t.Fatalf("unexpected validation error for %q: %v", tt.input, err)
		}
		if req.PaymentMethod != tt.expected {
			t.Errorf("for input %q expected normalized %q, got %q", tt.input, tt.expected, req.PaymentMethod)
		}
	}
}

func TestCreatePaymentRequest_Validate_MissingRequiredFields(t *testing.T) {
	tests := []struct {
		name      string
		modifyReq func(r *payload.CreatePaymentRequest)
	}{
		{
			name: "missing payment_method",
			modifyReq: func(r *payload.CreatePaymentRequest) {
				r.PaymentMethod = ""
			},
		},
		{
			name: "missing merchant_reff",
			modifyReq: func(r *payload.CreatePaymentRequest) {
				r.MerchantReff = ""
			},
		},
		{
			name: "missing bill_title",
			modifyReq: func(r *payload.CreatePaymentRequest) {
				r.BillTitle = ""
			},
		},
		{
			name: "missing customer_name",
			modifyReq: func(r *payload.CreatePaymentRequest) {
				r.CustomerName = ""
			},
		},
		{
			name: "missing customer_phone",
			modifyReq: func(r *payload.CreatePaymentRequest) {
				r.CustomerPhone = ""
			},
		},
		{
			name: "zero amount",
			modifyReq: func(r *payload.CreatePaymentRequest) {
				r.Amount = 0
			},
		},
		{
			name: "negative amount",
			modifyReq: func(r *payload.CreatePaymentRequest) {
				r.Amount = -1000
			},
		},
		{
			name: "invalid customer email",
			modifyReq: func(r *payload.CreatePaymentRequest) {
				r.CustomerEmail = "invalid-email-address"
			},
		},
		{
			name: "invalid redirect url",
			modifyReq: func(r *payload.CreatePaymentRequest) {
				r.RedirectURL = "not-a-valid-url"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := validPaymentRequest()
			tt.modifyReq(req)

			err := req.Validate()
			if err == nil {
				t.Fatalf("expected validation error for test %q, got nil", tt.name)
			}

			appErr, ok := exception.As(err)
			if !ok || appErr.Code != exception.CodeValidation {
				t.Errorf("expected CodeValidation error, got: %v", err)
			}
		})
	}
}

func TestCreatePaymentRequest_Validate_CustomValidator(t *testing.T) {
	req := validPaymentRequest()
	customVal := config.NewValidator()

	if err := req.Validate(customVal); err != nil {
		t.Fatalf("unexpected validation error with custom validator: %v", err)
	}

	// Verify error with custom validator
	req.Amount = 0
	if err := req.Validate(customVal); err == nil {
		t.Fatal("expected validation error with custom validator, got nil")
	}
}
