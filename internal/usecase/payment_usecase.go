package usecase

import (
	"context"
	"fmt"
	"time"

	"paygate/internal/common/constants"
	"paygate/internal/entity"
	"paygate/internal/exception"
	"paygate/internal/model/converter"
	"paygate/internal/model/payload"
	"paygate/internal/provider"
	"paygate/internal/repository"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type PaymentUseCase struct {
	db        *gorm.DB
	validate  *validator.Validate
	txRepo    repository.TransactionRepository
	providers map[string]provider.PaymentProvider
}

func NewPaymentUseCase(
	db *gorm.DB,
	validate *validator.Validate,
	txRepo repository.TransactionRepository,
	providers map[string]provider.PaymentProvider,
) *PaymentUseCase {
	return &PaymentUseCase{
		db:        db,
		validate:  validate,
		txRepo:    txRepo,
		providers: providers,
	}
}

func (u *PaymentUseCase) CreatePayment(ctx context.Context, req *payload.CreatePaymentRequest) (*payload.PaymentResponse, error) {
	if err := u.validate.Struct(req); err != nil {
		return nil, exception.FromValidation(err)
	}

	p, ok := u.providers["pay2u"]
	if !ok {
		return nil, exception.Internal(fmt.Errorf("default provider pay2u not configured"))
	}

	// Idempotency check
	existing, err := u.txRepo.FindByIdempotencyKey(ctx, u.db, req.IdempotencyKey)
	if err != nil {
		return nil, exception.Internal(fmt.Errorf("find idempotency key: %w", err))
	}
	if existing != nil {
		return converter.ToPaymentResponse(existing), nil
	}

	merchantReff := fmt.Sprintf("X%d", time.Now().UnixNano())

	currency := constants.CurrencyIDR

	var expiredAt *time.Time
	if req.ExpiredMinutes > 0 {
		exp := time.Now().Add(time.Duration(req.ExpiredMinutes) * time.Minute)
		expiredAt = &exp
	}

	billReq := &provider.BillRequest{
		MerchantReff:      merchantReff,
		PaymentMethodCode: req.PaymentMethod,
		CallbackURL:       req.CallbackURL,
		RedirectURL:       req.RedirectURL,
		BillTitle:         req.BillTitle,
		BillDescription:   req.BillDescription,
		CustomerName:      req.CustomerName,
		CustomerPhone:     req.CustomerPhone,
		CustomerEmail:     req.CustomerEmail,
		Currency:          currency,
		Amount:            req.Amount,
		AmountAdmin:       req.AmountAdmin,
		AmountDiscount:    req.AmountDiscount,
		AmountTotal:       req.AmountTotal,
		ExpiredMinutes:    req.ExpiredMinutes,
	}

	billResult, err := p.CreateBill(ctx, billReq)
	if err != nil {
		return nil, exception.Internal(fmt.Errorf("provider create bill: %w", err))
	}

	var merchantID *uuid.UUID
	if m, ok := ctx.Value(constants.LocalsMerchant).(*entity.Merchant); ok && m != nil {
		merchantID = &m.ID
	}

	tx := &entity.Transaction{
		ID:              uuid.New(),
		MerchantID:      merchantID,
		IdempotencyKey:  req.IdempotencyKey,
		Provider:        p.Name(),
		ProviderToken:   billResult.Token,
		MerchantReff:    merchantReff,
		PaymentMethod:   req.PaymentMethod,
		PaymentCode:     billResult.PaymentCode,
		Status:          constants.TransactionStatusPending,
		Amount:          req.Amount,
		AmountAdmin:     req.AmountAdmin,
		AmountDiscount:  req.AmountDiscount,
		AmountTotal:     req.AmountTotal,
		Currency:        currency,
		CustomerName:    req.CustomerName,
		CustomerPhone:   req.CustomerPhone,
		CustomerEmail:   req.CustomerEmail,
		BillTitle:       req.BillTitle,
		BillDescription: req.BillDescription,
		CallbackURL:     req.CallbackURL,
		RedirectURL:     req.RedirectURL,
		ExpiredAt:       expiredAt,
	}

	if err := u.txRepo.Create(ctx, u.db, tx); err != nil {
		return nil, exception.Internal(fmt.Errorf("save transaction: %w", err))
	}

	return converter.ToPaymentResponse(tx), nil
}

func (u *PaymentUseCase) GetPayment(ctx context.Context, id uuid.UUID) (*payload.PaymentResponse, error) {
	tx, err := u.txRepo.FindByID(ctx, u.db, id)
	if err != nil {
		return nil, err
	}
	return converter.ToPaymentResponse(tx), nil
}

func (u *PaymentUseCase) RefreshPayment(ctx context.Context, id uuid.UUID) (*payload.PaymentResponse, error) {
	tx, err := u.txRepo.FindByID(ctx, u.db, id)
	if err != nil {
		return nil, err
	}

	// if tx.Status != constants.TransactionStatusPending {
	// 	return converter.ToPaymentResponse(tx), nil
	// }

	p, ok := u.providers[tx.Provider]
	if !ok {
		return nil, exception.Internal(fmt.Errorf("provider %s not found", tx.Provider))
	}

	res, err := p.GetBill(ctx, tx.ProviderToken, tx.PaymentMethod)
	if err != nil {
		return nil, exception.Internal(fmt.Errorf("provider get bill: %w", err))
	}

	newStatus := mapProviderStatus(res.Status)
	if newStatus != tx.Status {
		updates := map[string]any{
			"status": newStatus,
		}
		if newStatus == constants.TransactionStatusSuccess {
			now := time.Now()
			updates["paid_at"] = &now
			tx.PaidAt = &now
		}
		if err := u.txRepo.UpdateStatus(ctx, u.db, tx.ID, newStatus, updates); err != nil {
			return nil, exception.Internal(fmt.Errorf("update transaction status: %w", err))
		}
		tx.Status = newStatus
	}

	return converter.ToPaymentResponse(tx), nil
}

func (u *PaymentUseCase) HandleCallback(ctx context.Context, providerName string, rawBody []byte) error {
	p, ok := u.providers[providerName]
	if !ok {
		return exception.BadRequest(fmt.Sprintf("unsupported provider: %s", providerName))
	}

	cb, err := p.ParseCallback(rawBody)
	if err != nil {
		return exception.BadRequest(fmt.Sprintf("parse callback: %v", err))
	}

	tx, err := u.txRepo.FindByMerchantReff(ctx, u.db, cb.MerchantReff)
	if err != nil {
		return exception.Internal(err)
	}
	if tx == nil && cb.ProviderToken != "" {
		tx, err = u.txRepo.FindByProviderToken(ctx, u.db, cb.ProviderToken)
		if err != nil {
			return exception.Internal(err)
		}
	}
	if tx == nil {
		return exception.NotFound(fmt.Sprintf("transaction with reff %s not found", cb.MerchantReff))
	}

	if tx.Status == constants.TransactionStatusSuccess {
		return nil
	}

	newStatus := mapProviderStatus(cb.Status)
	updates := map[string]any{
		"provider_callback": entity.JSONB(rawBody),
	}
	if cb.PaymentReff != "" {
		updates["payment_reff"] = cb.PaymentReff
	}
	if newStatus == constants.TransactionStatusSuccess {
		now := time.Now()
		updates["paid_at"] = &now
	}

	return u.txRepo.UpdateStatus(ctx, u.db, tx.ID, newStatus, updates)
}

func mapProviderStatus(status int) string {
	switch status {
	case 1:
		return constants.TransactionStatusSuccess
	case 2:
		return constants.TransactionStatusProcess
	case 3:
		return constants.TransactionStatusRefund
	default:
		return constants.TransactionStatusPending
	}
}
