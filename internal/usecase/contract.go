package usecase

import (
	"context"

	"paygate/internal/entity"
	"paygate/internal/model/payload"

	"github.com/google/uuid"
)

type PaymentService interface {
	CreatePayment(ctx context.Context, req *payload.CreatePaymentRequest) (*payload.PaymentResponse, error)
	GetPayment(ctx context.Context, id uuid.UUID) (*payload.PaymentResponse, error)
	RefreshPayment(ctx context.Context, id uuid.UUID) (*payload.PaymentResponse, error)
	HandleCallback(ctx context.Context, providerName string, rawBody []byte) error
}

type MerchantService interface {
	CreateMerchant(ctx context.Context, req *payload.CreateMerchantRequest) (*payload.MerchantResponse, error)
	GetMerchant(ctx context.Context, id uuid.UUID) (*payload.MerchantResponse, error)
	ListMerchants(ctx context.Context, req payload.PageRequest) (*payload.PageResponse[*payload.MerchantResponse], error)
	UpdateMerchant(ctx context.Context, id uuid.UUID, req *payload.UpdateMerchantRequest) (*payload.MerchantResponse, error)
	RotateAPIKey(ctx context.Context, id uuid.UUID) (*payload.MerchantResponse, error)
	ValidateAPIKey(ctx context.Context, apiKey string) (*entity.Merchant, error)
}

var (
	_ PaymentService  = (*PaymentUseCase)(nil)
	_ MerchantService = (*MerchantUseCase)(nil)
)
