package middleware

import (
	"context"
	"net/http/httptest"
	"testing"

	"paygate/internal/common/constants"
	"paygate/internal/delivery/http/response"
	"paygate/internal/entity"
	"paygate/internal/exception"
	"paygate/internal/model/payload"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type mockMerchantService struct {
	validKey string
	merchant *entity.Merchant
}

func (m *mockMerchantService) ValidateAPIKey(ctx context.Context, apiKey string) (*entity.Merchant, error) {
	if apiKey == m.validKey {
		return m.merchant, nil
	}
	return nil, exception.Unauthorized("invalid API key")
}

func (m *mockMerchantService) CreateMerchant(ctx context.Context, req *payload.CreateMerchantRequest) (*payload.MerchantResponse, error) {
	return nil, nil
}
func (m *mockMerchantService) GetMerchant(ctx context.Context, id uuid.UUID) (*payload.MerchantResponse, error) {
	return nil, nil
}
func (m *mockMerchantService) ListMerchants(ctx context.Context, req payload.PageRequest) (*payload.PageResponse[*payload.MerchantResponse], error) {
	return nil, nil
}
func (m *mockMerchantService) UpdateMerchant(ctx context.Context, id uuid.UUID, req *payload.UpdateMerchantRequest) (*payload.MerchantResponse, error) {
	return nil, nil
}
func (m *mockMerchantService) RotateAPIKey(ctx context.Context, id uuid.UUID) (*payload.MerchantResponse, error) {
	return nil, nil
}

func TestNewMasterAPIKey(t *testing.T) {
	app := fiber.New(fiber.Config{ErrorHandler: response.NewErrorHandler()})
	app.Use(NewMasterAPIKey("master-secret-123"))
	app.Get("/test", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	// Missing header
	req := httptest.NewRequest("GET", "/test", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != fiber.StatusUnauthorized {
		t.Errorf("expected 401 on missing key, got %d", resp.StatusCode)
	}

	// Wrong key
	req = httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-API-Key", "wrong-key")
	resp, _ = app.Test(req)
	if resp.StatusCode != fiber.StatusUnauthorized {
		t.Errorf("expected 401 on wrong key, got %d", resp.StatusCode)
	}

	// Correct master key
	req = httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-API-Key", "master-secret-123")
	resp, _ = app.Test(req)
	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("expected 200 on correct master key, got %d", resp.StatusCode)
	}
}

func TestNewMerchantAPIKey(t *testing.T) {
	mockMC := &mockMerchantService{
		validKey: "pg_localoka-v2_secret123",
		merchant: &entity.Merchant{
			ID:   uuid.New(),
			Code: "localoka-v2",
			Name: "Localoka V2",
		},
	}

	app := fiber.New(fiber.Config{ErrorHandler: response.NewErrorHandler()})
	app.Use(NewMerchantAPIKey(mockMC))

	var capturedCode string
	app.Get("/payment-test", func(c *fiber.Ctx) error {
		capturedCode, _ = c.Locals(constants.LocalsMerchantCode).(string)
		return c.SendStatus(fiber.StatusOK)
	})

	// Valid merchant key
	req := httptest.NewRequest("GET", "/payment-test", nil)
	req.Header.Set("X-API-Key", "pg_localoka-v2_secret123")
	resp, _ := app.Test(req)
	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("expected 200 for valid merchant key, got %d", resp.StatusCode)
	}
	if capturedCode != "localoka-v2" {
		t.Errorf("expected captured code 'localoka-v2', got %q", capturedCode)
	}

	// Master key must be REJECTED on payments endpoint (strict mode)
	req = httptest.NewRequest("GET", "/payment-test", nil)
	req.Header.Set("X-API-Key", "master-admin-key")
	resp, _ = app.Test(req)
	if resp.StatusCode != fiber.StatusUnauthorized {
		t.Errorf("expected 401 for master key on payment endpoint in strict mode, got %d", resp.StatusCode)
	}

	// Invalid merchant key
	req = httptest.NewRequest("GET", "/payment-test", nil)
	req.Header.Set("X-API-Key", "pg_unknown_badkey")
	resp, _ = app.Test(req)
	if resp.StatusCode != fiber.StatusUnauthorized {
		t.Errorf("expected 401 for invalid merchant key, got %d", resp.StatusCode)
	}
}
