package usecase

import (
	"context"

	"paygate/internal/entity"
	"paygate/internal/model/payload"

	"github.com/google/uuid"
)

type PaymentService interface {
	CreatePayment(ctx context.Context, req *payload.CreatePaymentRequest) (*payload.PaymentResponse, error)
	GetPayment(ctx context.Context, identifier string) (*payload.PaymentResponse, error)
	RefreshPayment(ctx context.Context, identifier string) (*payload.PaymentResponse, error)
	HandleCallback(ctx context.Context, providerName string, rawBody []byte) error
}

type MerchantService interface {
	CreateMerchant(ctx context.Context, adminID uuid.UUID, req *payload.CreateMerchantRequest) (*payload.MerchantResponse, error)
	GetMerchant(ctx context.Context, id uuid.UUID, adminID *uuid.UUID) (*payload.MerchantResponse, error)
	ListMerchants(ctx context.Context, req payload.MerchantListRequest) (*payload.PageResponse[*payload.MerchantResponse], error)
	UpdateMerchant(ctx context.Context, id uuid.UUID, adminID *uuid.UUID, req *payload.UpdateMerchantRequest) (*payload.MerchantResponse, error)
	RotateAPIKey(ctx context.Context, id uuid.UUID, adminID *uuid.UUID) (*payload.MerchantResponse, error)
	RotateWebhookSecret(ctx context.Context, id uuid.UUID, adminID *uuid.UUID) (*payload.MerchantResponse, error)
	ValidateAPIKey(ctx context.Context, apiKey string) (*entity.Merchant, error)
}

type DisbursementService interface {
	InquiryAccount(ctx context.Context, req *payload.DisbursementInquiryRequest) (*payload.DisbursementResponse, error)
	TransferBank(ctx context.Context, merchantRef string) (*payload.DisbursementResponse, error)
	GetDisbursement(ctx context.Context, merchantRef string) (*payload.DisbursementResponse, error)
	ListAdminDisbursements(ctx context.Context, request payload.AdminDisbursementListRequest) (*payload.PageResponse[payload.AdminDisbursementResponse], error)
	GetAdminDisbursement(ctx context.Context, id uuid.UUID, adminID *uuid.UUID) (*payload.AdminDisbursementDetailResponse, error)
	HandleTransferCallback(ctx context.Context, providerName string, rawBody []byte) error
}

var (
	_ PaymentService      = (*PaymentUseCase)(nil)
	_ MerchantService     = (*MerchantUseCase)(nil)
	_ DisbursementService = (*DisbursementUseCase)(nil)
)
