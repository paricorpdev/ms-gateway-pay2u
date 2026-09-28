package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"paygate/internal/common/constants"
	"paygate/internal/entity"
	"paygate/internal/exception"
	"paygate/internal/logger"
	"paygate/internal/model/converter"
	"paygate/internal/model/payload"
	"paygate/internal/provider"
	"paygate/internal/repository"
	"paygate/internal/utils"
	"paygate/internal/webhook"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type PaymentUseCase struct {
	db           *gorm.DB
	validate     *validator.Validate
	txRepo       repository.TransactionRepository
	merchantRepo repository.MerchantRepository
	dispatchRepo repository.WebhookDispatchRepository
	dispatcher   webhook.Dispatcher
	providers    map[string]provider.PaymentProvider
}

func NewPaymentUseCase(
	db *gorm.DB,
	validate *validator.Validate,
	txRepo repository.TransactionRepository,
	merchantRepo repository.MerchantRepository,
	dispatchRepo repository.WebhookDispatchRepository,
	dispatcher webhook.Dispatcher,
	providers map[string]provider.PaymentProvider,
) *PaymentUseCase {
	return &PaymentUseCase{
		db:           db,
		validate:     validate,
		txRepo:       txRepo,
		merchantRepo: merchantRepo,
		dispatchRepo: dispatchRepo,
		dispatcher:   dispatcher,
		providers:    providers,
	}
}

func (u *PaymentUseCase) CreatePayment(ctx context.Context, req *payload.CreatePaymentRequest) (*payload.PaymentResponse, error) {
	p, ok := u.providers["pay2u"]
	if !ok {
		return nil, exception.Internal(fmt.Errorf("default provider pay2u not configured"))
	}

	req.PaymentMethod = utils.NormalizePaymentMethod(req.PaymentMethod)
	if !utils.IsValidPaymentMethod(req.PaymentMethod) {
		return nil, exception.Validation("payment_method is invalid or unsupported").WithDetails([]exception.FieldError{
			{
				Field:   "payment_method",
				Rule:    "supported_method",
				Message: fmt.Sprintf("payment_method %q is invalid or unsupported", req.PaymentMethod),
			},
		})
	}

	// Check existing transaction by request_id (replay protection)
	requestID := logger.RequestIDFromContext(ctx)
	if requestID == "" {
		requestID = uuid.NewString()
	}

	existing, err := u.txRepo.FindByRequestID(ctx, u.db, requestID)
	if err != nil {
		return nil, exception.Internal(fmt.Errorf("find transaction by request_id: %w", err))
	}
	if existing != nil {
		return converter.ToPaymentResponse(existing), nil
	}

	merchantReff := req.MerchantReff
	if merchantReff == "" {
		merchantReff = fmt.Sprintf("X%d", time.Now().UnixNano())
	}

	existingReff, err := u.txRepo.FindByMerchantReff(ctx, u.db, merchantReff)
	if err != nil {
		return nil, exception.Internal(fmt.Errorf("check merchant reff: %w", err))
	}
	if existingReff != nil {
		return nil, exception.Conflict(fmt.Sprintf("transaction with merchant_reff %q already exists", merchantReff))
	}

	// Calculate admin fee and amount_total from paygate rules
	adminFee := utils.CalculateAdminFee(req.PaymentMethod, req.Amount)
	req.AmountAdmin = adminFee
	req.AmountTotal = req.Amount + adminFee - req.AmountDiscount
	if req.AmountTotal < 0 {
		req.AmountTotal = 0
	}

	currency := constants.CurrencyIDR

	var expiredAt *time.Time
	if req.ExpiredMinutes > 0 {
		exp := time.Now().Add(time.Duration(req.ExpiredMinutes) * time.Minute)
		expiredAt = &exp
	}

	billReq := &provider.BillRequest{
		MerchantReff:      merchantReff,
		PaymentMethodCode: req.PaymentMethod,
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
		RequestID:       requestID,
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
		ExpiredAt:       expiredAt,
	}

	if err := u.txRepo.Create(ctx, u.db, tx); err != nil {
		return nil, exception.Internal(fmt.Errorf("save transaction: %w", err))
	}

	return converter.ToPaymentResponse(tx), nil
}

func (u *PaymentUseCase) findTransaction(ctx context.Context, identifier string) (*entity.Transaction, error) {
	tx, err := u.txRepo.FindByMerchantReff(ctx, u.db, identifier)
	if err != nil {
		return nil, exception.Internal(fmt.Errorf("find transaction by merchant_reff: %w", err))
	}
	if tx == nil {
		if uid, parseErr := uuid.Parse(identifier); parseErr == nil {
			tx, err = u.txRepo.FindByID(ctx, u.db, uid)
			if err != nil && !exception.IsNotFound(err) {
				return nil, exception.Internal(fmt.Errorf("find transaction by id: %w", err))
			}
		}
	}
	return tx, nil
}

func (u *PaymentUseCase) GetPayment(ctx context.Context, identifier string) (*payload.PaymentResponse, error) {
	tx, err := u.findTransaction(ctx, identifier)
	if err != nil {
		return nil, err
	}

	if tx == nil {
		return nil, exception.NotFound(fmt.Sprintf("transaction with merchant_reff %q not found", identifier))
	}

	return converter.ToPaymentResponse(tx), nil
}

func (u *PaymentUseCase) RefreshPayment(ctx context.Context, identifier string) (*payload.PaymentResponse, error) {
	tx, err := u.findTransaction(ctx, identifier)
	if err != nil {
		return nil, err
	}

	if tx == nil {
		return nil, exception.NotFound(fmt.Sprintf("transaction with merchant_reff %q not found", identifier))
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
		// Valid state transitions for RefreshPayment:
		// Do not downgrade terminal states unless transitioning to REFUND
		if tx.Status == constants.TransactionStatusRefund {
			return converter.ToPaymentResponse(tx), nil
		}
		if tx.Status == constants.TransactionStatusSuccess && newStatus != constants.TransactionStatusRefund {
			return converter.ToPaymentResponse(tx), nil
		}

		updates := map[string]any{
			"status": newStatus,
		}
		var paidAt time.Time
		if newStatus == constants.TransactionStatusSuccess {
			paidAt = time.Now()
			if res.PaymentDate != "" {
				if t, err := time.ParseInLocation("2006-01-02 15:04:05", res.PaymentDate, time.Local); err == nil {
					paidAt = t
				}
			}
			updates["paid_at"] = &paidAt
			tx.PaidAt = &paidAt
		}
		if res.PaymentReff != "" {
			updates["payment_reff"] = res.PaymentReff
			tx.PaymentReff = res.PaymentReff
		}
		if err := u.txRepo.UpdateStatus(ctx, u.db, tx.ID, newStatus, updates); err != nil {
			return nil, exception.Internal(fmt.Errorf("update transaction status: %w", err))
		}
		tx.Status = newStatus

		if newStatus == constants.TransactionStatusSuccess {
			u.triggerMerchantWebhook(ctx, tx, constants.WebhookEventPaymentSuccess, tx.PaymentReff)
		} else if newStatus == constants.TransactionStatusRefund {
			u.triggerMerchantWebhook(ctx, tx, constants.WebhookEventPaymentRefund, tx.PaymentReff)
		}
	} else if newStatus == constants.TransactionStatusSuccess {
		updates := make(map[string]any)
		if res.PaymentReff != "" && tx.PaymentReff == "" {
			updates["payment_reff"] = res.PaymentReff
			tx.PaymentReff = res.PaymentReff
		}
		if tx.PaidAt == nil && res.PaymentDate != "" {
			if t, err := time.ParseInLocation("2006-01-02 15:04:05", res.PaymentDate, time.Local); err == nil {
				updates["paid_at"] = &t
				tx.PaidAt = &t
			}
		}
		if len(updates) > 0 {
			_ = u.txRepo.UpdateStatus(ctx, u.db, tx.ID, tx.Status, updates)
		}
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

	newStatus := mapProviderStatus(cb.Status)

	if tx.Status == newStatus {
		return nil
	}

	// Transaction already REFUND -> terminal, ignore further transitions
	if tx.Status == constants.TransactionStatusRefund {
		return nil
	}

	// Transaction already SUCCESS -> only allow transition to REFUND
	if tx.Status == constants.TransactionStatusSuccess && newStatus != constants.TransactionStatusRefund {
		return nil
	}

	updates := map[string]any{
		"provider_callback": entity.JSONB(rawBody),
	}
	if cb.PaymentReff != "" {
		updates["payment_reff"] = cb.PaymentReff
		tx.PaymentReff = cb.PaymentReff
	}

	var eventTime time.Time
	if cb.PaymentDate != "" {
		if t, err := time.ParseInLocation("2006-01-02 15:04:05", cb.PaymentDate, time.Local); err == nil {
			eventTime = t
		}
	}
	if eventTime.IsZero() {
		eventTime = time.Now()
	}

	if newStatus == constants.TransactionStatusSuccess {
		updates["paid_at"] = &eventTime
		tx.PaidAt = &eventTime
	}

	if err := u.txRepo.UpdateStatus(ctx, u.db, tx.ID, newStatus, updates); err != nil {
		return err
	}
	tx.Status = newStatus

	if newStatus == constants.TransactionStatusSuccess {
		u.triggerMerchantWebhook(ctx, tx, constants.WebhookEventPaymentSuccess, cb.PaymentReff)
	} else if newStatus == constants.TransactionStatusRefund {
		u.triggerMerchantWebhook(ctx, tx, constants.WebhookEventPaymentRefund, cb.PaymentReff)
	}

	return nil
}

func (u *PaymentUseCase) triggerMerchantWebhook(ctx context.Context, tx *entity.Transaction, eventType, paymentReff string) {
	if u.dispatcher == nil || u.dispatchRepo == nil || u.merchantRepo == nil || tx.MerchantID == nil {
		return
	}

	merchant, err := u.merchantRepo.FindByID(ctx, u.db, *tx.MerchantID)
	if err != nil || merchant == nil || !merchant.IsActive || merchant.WebhookURL == "" {
		return
	}

	payloadMap := map[string]any{
		"event":           eventType,
		"payment_id":      tx.ID.String(),
		"merchant_reff":   tx.MerchantReff,
		"payment_method":  tx.PaymentMethod,
		"payment_code":    tx.PaymentCode,
		"amount":          tx.Amount,
		"amount_admin":    tx.AmountAdmin,
		"amount_discount": tx.AmountDiscount,
		"amount_total":    tx.AmountTotal,
		"currency":        tx.Currency,
		"status":          tx.Status,
		"payment_reff":    paymentReff,
	}

	if tx.PaidAt != nil {
		payloadMap["paid_at"] = tx.PaidAt.Format(time.RFC3339)
	}

	payloadBytes, err := json.Marshal(payloadMap)
	if err != nil {
		logger.FromContext(ctx).WithError(err).Error("Failed to marshal webhook payload")
		return
	}

	dispatch := &entity.WebhookDispatch{
		ID:            uuid.New(),
		TransactionID: tx.ID,
		MerchantID:    merchant.ID,
		TargetURL:     merchant.WebhookURL,
		EventType:     eventType,
		Payload:       entity.JSONB(payloadBytes),
		Status:        constants.WebhookStatusPending,
		Attempts:      0,
		MaxAttempts:   3,
	}

	if err := u.dispatchRepo.CreateDispatch(ctx, u.db, dispatch); err != nil {
		logger.FromContext(ctx).WithError(err).Error("Failed to persist webhook dispatch record")
		return
	}

	if err := u.dispatcher.Enqueue(ctx, dispatch.ID); err != nil {
		logger.FromContext(ctx).WithError(err).Warn("Failed to enqueue webhook dispatch; will be recovered by scanner")
	}
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
