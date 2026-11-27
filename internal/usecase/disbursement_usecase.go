package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"paygate/internal/common/constants"
	"paygate/internal/entity"
	"paygate/internal/exception"
	"paygate/internal/logger"
	"paygate/internal/model/payload"
	"paygate/internal/provider"
	"paygate/internal/repository"
	"paygate/internal/webhook"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type DisbursementUseCase struct {
	db            *gorm.DB
	disbursements repository.DisbursementRepository
	transactions  repository.TransactionRepository
	merchants     repository.MerchantRepository
	provider      provider.DisbursementProvider
	dispatches    repository.WebhookDispatchRepository
	dispatcher    webhook.Dispatcher
	timeLocation  *time.Location
}

func NewDisbursementUseCase(
	db *gorm.DB,
	disbursements repository.DisbursementRepository,
	transactions repository.TransactionRepository,
	merchants repository.MerchantRepository,
	disbursementProvider provider.DisbursementProvider,
	dispatches repository.WebhookDispatchRepository,
	dispatcher webhook.Dispatcher,
	timeZone string,
) *DisbursementUseCase {
	timeLocation, err := time.LoadLocation(timeZone)
	if err != nil {
		timeLocation = time.UTC
	}
	return &DisbursementUseCase{
		db:            db,
		disbursements: disbursements,
		transactions:  transactions,
		merchants:     merchants,
		provider:      disbursementProvider,
		dispatches:    dispatches,
		dispatcher:    dispatcher,
		timeLocation:  timeLocation,
	}
}

func (u *DisbursementUseCase) InquiryAccount(ctx context.Context, req *payload.DisbursementInquiryRequest) (*payload.DisbursementResponse, error) {
	merchant, err := merchantFromContext(ctx)
	if err != nil {
		return nil, err
	}
	if u.provider == nil {
		return nil, exception.Unavailable("disbursement provider is not configured")
	}

	transactionID, err := uuid.Parse(req.TransactionID)
	if err != nil {
		return nil, exception.BadRequest("transaction_id must be a UUID")
	}
	transaction, err := u.transactions.FindByMerchantAndID(ctx, u.db, merchant.ID, transactionID)
	if err != nil {
		return nil, exception.Internal(fmt.Errorf("find source transaction: %w", err))
	}
	if transaction == nil {
		return nil, exception.NotFound("source transaction not found")
	}
	providerCtx := logger.WithTransactionID(ctx, transaction.ID)
	if transaction.Status != constants.TransactionStatusSuccess {
		return nil, exception.Conflict("disbursement inquiry requires a successful payment transaction")
	}
	merchantWebhookURL := strings.TrimSpace(merchant.WebhookURL)

	amountTotal, err := calculateDisbursementTotal(req.Amount, req.AmountAdmin, req.AmountDiscount)
	if err != nil {
		return nil, err
	}
	if req.AmountTotal != amountTotal {
		return nil, exception.BadRequest("amount_total must equal amount + amount_admin - amount_discount")
	}
	if req.Amount != transaction.Amount {
		return nil, exception.BadRequest("amount must equal the source transaction amount")
	}

	merchantRef := strings.TrimSpace(req.MerchantReff)
	existingRef, err := u.disbursements.FindByMerchantAndRef(ctx, u.db, merchant.ID, merchantRef)
	if err != nil {
		return nil, exception.Internal(fmt.Errorf("find existing disbursement reference: %w", err))
	}
	if existingRef != nil && existingRef.TransactionID != transaction.ID {
		return nil, exception.Conflict("merchant_reff has already been used for another disbursement")
	}
	existing, err := u.disbursements.FindByMerchantAndTransactionID(ctx, u.db, merchant.ID, transaction.ID)
	if err != nil {
		return nil, exception.Internal(fmt.Errorf("find existing disbursement: %w", err))
	}
	if existing != nil {
		if existing.MerchantRef == merchantRef && sameDisbursementRequest(existing, req, amountTotal, merchantWebhookURL) {
			return disbursementResponse(existing), nil
		}
		return nil, exception.Conflict("a disbursement already exists for this transaction")
	}

	channelCode := strings.TrimSpace(req.ChannelCode)
	if channelCode == "" {
		channelCode = "TF-MITRA"
	}
	expiredMinutes := req.ExpiredMinutes
	if expiredMinutes == 0 {
		expiredMinutes = 30
	}
	providerRequest := &provider.BankTransferRequest{
		MerchantReff:          merchantRef,
		ChannelCode:           channelCode,
		URLListenerMerchant:   "",
		URLRedirectMerchant:   strings.TrimSpace(req.URLRedirectMerchant),
		ReceiverAccountNumber: strings.TrimSpace(req.ReceiverAccount),
		ReceiverName:          strings.TrimSpace(req.ReceiverName),
		ReceiverBank:          strings.TrimSpace(req.ReceiverBank),
		Amount:                req.Amount,
		AmountAdmin:           req.AmountAdmin,
		AmountDiscount:        req.AmountDiscount,
		AmountTotal:           amountTotal,
		Note:                  strings.TrimSpace(req.Note),
		ExpiredMinutes:        expiredMinutes,
	}
	inquiry, err := u.provider.AccountInquiry(providerCtx, providerRequest)
	if err != nil {
		var providerErr *provider.APIError
		if errors.As(err, &providerErr) {
			return nil, exception.BadRequest(fmt.Sprintf("Pay2U account inquiry rejected (%s): %s", providerErr.Code, providerErr.Description))
		}
		return nil, exception.Unavailable(fmt.Sprintf("Pay2U account inquiry failed: %v", err))
	}
	if inquiry.Token == "" {
		return nil, exception.Unavailable("Pay2U account inquiry did not return a transaction token")
	}
	if inquiry.AmountTotal != 0 && inquiry.AmountTotal != amountTotal {
		return nil, exception.Conflict("Pay2U account inquiry amount_total does not match the requested amount_total")
	}

	now := time.Now().UTC()
	eligibleAt := now
	if transaction.PaidAt != nil {
		eligibleAt = transaction.PaidAt.UTC()
	}
	requestID := logger.RequestIDFromContext(ctx)
	if requestID == "" {
		requestID = uuid.NewString()
	}
	bankName := inquiry.ReceiverBankName
	if bankName == "" {
		bankName = ""
	}
	disbursement := &entity.Disbursement{
		ID:                    uuid.New(),
		TransactionID:         transaction.ID,
		MerchantID:            merchant.ID,
		RequestID:             requestID,
		MerchantRef:           merchantRef,
		Provider:              "pay2u",
		ProviderToken:         inquiry.Token,
		ChannelCode:           channelCode,
		URLListenerMerchant:   merchantWebhookURL,
		URLRedirectMerchant:   strings.TrimSpace(req.URLRedirectMerchant),
		ExpiredMinutes:        expiredMinutes,
		Amount:                req.Amount,
		AmountAdmin:           req.AmountAdmin,
		AmountDiscount:        req.AmountDiscount,
		AmountTotal:           amountTotal,
		Currency:              constants.CurrencyIDR,
		BeneficiaryName:       inquiry.ReceiverName,
		BeneficiaryBankCode:   inquiry.ReceiverBank,
		BeneficiaryBankName:   bankName,
		BeneficiaryAccountRef: inquiry.ReceiverAccountNumber,
		Note:                  strings.TrimSpace(req.Note),
		Status:                constants.DisbursementStatusWaitingSettlement,
		EligibleAt:            &eligibleAt,
		InquiryAt:             &now,
		MaxAttempts:           3,
		ProviderResponse:      entity.JSONB(inquiry.RawResponse),
		CreatedAt:             now,
		UpdatedAt:             now,
	}
	if err := u.disbursements.Create(ctx, u.db, disbursement); err != nil {
		return nil, exception.Internal(fmt.Errorf("save disbursement inquiry: %w", err))
	}
	return disbursementResponse(disbursement), nil
}

func (u *DisbursementUseCase) TransferBank(ctx context.Context, merchantRef string) (*payload.DisbursementResponse, error) {
	merchant, err := merchantFromContext(ctx)
	if err != nil {
		return nil, err
	}
	if u.provider == nil {
		return nil, exception.Unavailable("disbursement provider is not configured")
	}

	var disbursement *entity.Disbursement
	var alreadySubmitted bool
	err = u.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		disbursement, err = u.disbursements.FindByMerchantAndRefForUpdate(ctx, tx, merchant.ID, merchantRef)
		if err != nil {
			return exception.Internal(fmt.Errorf("load disbursement for transfer: %w", err))
		}
		if disbursement == nil {
			return exception.NotFound("disbursement inquiry not found")
		}
		if disbursement.Status != constants.DisbursementStatusWaitingSettlement {
			alreadySubmitted = disbursement.Status == constants.DisbursementStatusProcessing ||
				disbursement.Status == constants.DisbursementStatusSuccess ||
				disbursement.Status == constants.DisbursementStatusCancelled ||
				disbursement.Status == constants.DisbursementStatusFailed
			if alreadySubmitted {
				return nil
			}
			return exception.Conflict("disbursement is not ready for transfer")
		}
		if disbursement.MaxAttempts > 0 && disbursement.Attempts >= disbursement.MaxAttempts {
			return exception.Conflict("disbursement has reached its maximum transfer attempts")
		}
		if disbursement.EligibleAt != nil && time.Now().Before(*disbursement.EligibleAt) {
			return exception.Conflict("disbursement has not reached its settlement time")
		}

		sourceTransaction, err := u.transactions.FindByMerchantAndID(ctx, tx, merchant.ID, disbursement.TransactionID)
		if err != nil {
			return exception.Internal(fmt.Errorf("find source transaction: %w", err))
		}
		if sourceTransaction == nil || sourceTransaction.Status != constants.TransactionStatusSuccess {
			return exception.Conflict("source transaction is not settled")
		}

		now := time.Now().UTC()
		disbursement.Status = constants.DisbursementStatusProcessing
		disbursement.Attempts++
		disbursement.UpdatedAt = now
		return u.disbursements.Update(ctx, tx, disbursement.ID, map[string]any{
			"status":     disbursement.Status,
			"attempts":   disbursement.Attempts,
			"updated_at": now,
		})
	})
	if err != nil {
		return nil, err
	}
	if alreadySubmitted {
		return disbursementResponse(disbursement), nil
	}

	providerRequest := provider.BankTransferRequest{
		Token:                 disbursement.ProviderToken,
		MerchantReff:          disbursement.MerchantRef,
		ChannelCode:           disbursement.ChannelCode,
		URLListenerMerchant:   "",
		URLRedirectMerchant:   disbursement.URLRedirectMerchant,
		ReceiverAccountNumber: disbursement.BeneficiaryAccountRef,
		ReceiverName:          disbursement.BeneficiaryName,
		ReceiverBank:          disbursement.BeneficiaryBankCode,
		Amount:                disbursement.Amount,
		AmountAdmin:           disbursement.AmountAdmin,
		AmountDiscount:        disbursement.AmountDiscount,
		AmountTotal:           disbursement.AmountTotal,
		Note:                  disbursement.Note,
		ExpiredMinutes:        disbursement.ExpiredMinutes,
	}
	providerCtx := logger.WithTransactionID(ctx, disbursement.TransactionID)
	result, err := u.provider.AccountTransfer(providerCtx, &providerRequest)
	if err != nil {
		var providerErr *provider.APIError
		if errors.As(err, &providerErr) {
			failedAt := time.Now().UTC()
			responseBody, _ := json.Marshal(map[string]string{"code": providerErr.Code, "description": providerErr.Description})
			if updateErr := u.disbursements.Update(ctx, u.db, disbursement.ID, map[string]any{
				"status":            constants.DisbursementStatusFailed,
				"provider_response": entity.JSONB(responseBody),
				"updated_at":        failedAt,
			}); updateErr != nil {
				return nil, exception.Internal(fmt.Errorf("save rejected disbursement status: %w", updateErr))
			}
			disbursement.Status = constants.DisbursementStatusFailed
			disbursement.ProviderResponse = entity.JSONB(responseBody)
			return disbursementResponse(disbursement), nil
		}

		var unknownOutcome *provider.OutcomeUnknownError
		if errors.As(err, &unknownOutcome) {
			// A timeout can happen after Pay2U accepted the request. Keep PROCESSING
			// and wait for the provider notification instead of risking a duplicate transfer.
			return nil, exception.Unavailable("transfer status is pending provider confirmation; check the disbursement status before retrying")
		}

		status := constants.DisbursementStatusWaitingSettlement
		if disbursement.MaxAttempts > 0 && disbursement.Attempts >= disbursement.MaxAttempts {
			status = constants.DisbursementStatusFailed
		}
		updatedAt := time.Now().UTC()
		if updateErr := u.disbursements.Update(ctx, u.db, disbursement.ID, map[string]any{
			"status":         status,
			"transferred_at": nil,
			"updated_at":     updatedAt,
		}); updateErr != nil {
			return nil, exception.Internal(fmt.Errorf("save disbursement transfer error: %w", updateErr))
		}
		disbursement.Status = status
		disbursement.TransferredAt = nil
		if status == constants.DisbursementStatusFailed {
			return disbursementResponse(disbursement), nil
		}
		return nil, exception.Unavailable("Pay2U transfer was not submitted; retry the transfer after resolving the provider error")
	}

	updates := map[string]any{
		"status":            constants.DisbursementStatusProcessing,
		"provider_response": entity.JSONB(result.RawResponse),
		"updated_at":        time.Now().UTC(),
	}
	if result.ReferenceNo != "" {
		updates["provider_disbursement_ref"] = result.ReferenceNo
		disbursement.ProviderDisbursementRef = &result.ReferenceNo
	}
	if result.ReceiverBankName != "" {
		updates["beneficiary_bank_name"] = result.ReceiverBankName
		disbursement.BeneficiaryBankName = result.ReceiverBankName
	}
	if result.TransactionDate != "" {
		transferredAt := parseProviderTime(result.TransactionDate)
		updates["transferred_at"] = transferredAt
		disbursement.TransferredAt = &transferredAt
	}
	if err := u.disbursements.Update(ctx, u.db, disbursement.ID, updates); err != nil {
		return nil, exception.Internal(fmt.Errorf("save provider transfer response: %w", err))
	}
	disbursement.Status = constants.DisbursementStatusProcessing
	disbursement.ProviderResponse = entity.JSONB(result.RawResponse)
	return disbursementResponse(disbursement), nil
}

func (u *DisbursementUseCase) GetDisbursement(ctx context.Context, merchantRef string) (*payload.DisbursementResponse, error) {
	merchant, err := merchantFromContext(ctx)
	if err != nil {
		return nil, err
	}
	disbursement, err := u.disbursements.FindByMerchantAndRef(ctx, u.db, merchant.ID, merchantRef)
	if err != nil {
		return nil, exception.Internal(fmt.Errorf("find disbursement: %w", err))
	}
	if disbursement == nil {
		return nil, exception.NotFound("disbursement not found")
	}
	return disbursementResponse(disbursement), nil
}

func (u *DisbursementUseCase) ListAdminDisbursements(ctx context.Context, request payload.AdminDisbursementListRequest) (*payload.PageResponse[payload.AdminDisbursementResponse], error) {
	if request.Page < 1 {
		request.Page = payload.DefaultPage
	}
	if request.PerPage < 1 {
		request.PerPage = payload.DefaultPerPage
	}
	if request.PerPage > payload.MaxPerPage {
		request.PerPage = payload.MaxPerPage
	}
	request.Status = strings.ToUpper(strings.TrimSpace(request.Status))
	switch request.Status {
	case "", constants.DisbursementStatusWaitingSettlement, constants.DisbursementStatusProcessing,
		constants.DisbursementStatusSuccess, constants.DisbursementStatusFailed, constants.DisbursementStatusCancelled:
	default:
		return nil, exception.BadRequest("invalid disbursement status")
	}

	if request.Month != 0 && (request.Month < 1 || request.Month > 12 || request.Year == 0) {
		return nil, exception.BadRequest("month requires a valid year and must be between 1 and 12")
	}
	if request.Year != 0 && (request.Year < 2000 || request.Year > 2100) {
		return nil, exception.BadRequest("year must be between 2000 and 2100")
	}
	var merchantID *uuid.UUID
	if request.MerchantID != "" {
		parsedMerchantID, err := uuid.Parse(request.MerchantID)
		if err != nil {
			return nil, exception.BadRequest("merchant_id must be a UUID")
		}
		merchantID = &parsedMerchantID
	}

	var start, end *time.Time
	if request.Year != 0 {
		month := request.Month
		endMonth := month
		if month == 0 {
			month = 1
			endMonth = 12
		}
		startDate := time.Date(request.Year, time.Month(month), 1, 0, 0, 0, 0, u.timeLocation)
		endDate := time.Date(request.Year, time.Month(endMonth), 1, 0, 0, 0, 0, u.timeLocation).AddDate(0, 1, 0)
		start, end = &startDate, &endDate
	}

	rows, total, err := u.disbursements.ListAdmin(ctx, u.db, request, merchantID, start, end)
	if err != nil {
		return nil, exception.Internal(fmt.Errorf("list admin disbursements: %w", err))
	}
	data := make([]payload.AdminDisbursementResponse, 0, len(rows))
	for i := range rows {
		data = append(data, adminDisbursementResponse(&rows[i]))
	}
	return payload.NewPageResponse(data, request.Page, request.PerPage, total), nil
}

func (u *DisbursementUseCase) GetAdminDisbursement(ctx context.Context, id uuid.UUID, adminID *uuid.UUID) (*payload.AdminDisbursementDetailResponse, error) {
	row, err := u.disbursements.FindAdminByID(ctx, u.db, id, adminID)
	if err != nil {
		return nil, exception.Internal(fmt.Errorf("get admin disbursement: %w", err))
	}
	if row == nil {
		return nil, exception.NotFound("disbursement not found")
	}
	return &payload.AdminDisbursementDetailResponse{
		AdminDisbursementResponse: adminDisbursementResponse(row),
		BeneficiaryAccountNumber:  row.BeneficiaryAccountRef,
	}, nil
}

func adminDisbursementResponse(row *entity.Disbursement) payload.AdminDisbursementResponse {
	response := payload.AdminDisbursementResponse{
		ID:                      row.ID,
		TransactionID:           row.TransactionID,
		RequestID:               row.RequestID,
		MerchantRef:             row.MerchantRef,
		Provider:                row.Provider,
		ProviderDisbursementRef: row.ProviderDisbursementRef,
		ChannelCode:             row.ChannelCode,
		Amount:                  row.Amount,
		AmountAdmin:             row.AmountAdmin,
		AmountDiscount:          row.AmountDiscount,
		AmountTotal:             row.AmountTotal,
		Currency:                row.Currency,
		BeneficiaryName:         row.BeneficiaryName,
		BeneficiaryBankCode:     row.BeneficiaryBankCode,
		BeneficiaryBankName:     row.BeneficiaryBankName,
		Note:                    row.Note,
		Status:                  row.Status,
		EligibleAt:              row.EligibleAt,
		InquiryAt:               row.InquiryAt,
		TransferredAt:           row.TransferredAt,
		Attempts:                row.Attempts,
		MaxAttempts:             row.MaxAttempts,
		PaidAt:                  row.PaidAt,
		CreatedAt:               row.CreatedAt,
		UpdatedAt:               row.UpdatedAt,
	}
	if row.Merchant != nil {
		response.Merchant = &payload.AdminMerchantSummary{
			ID:   row.Merchant.ID,
			Code: row.Merchant.Code,
			Name: row.Merchant.Name,
		}
	}
	return response
}

func (u *DisbursementUseCase) HandleTransferCallback(ctx context.Context, providerName string, rawBody []byte) error {
	if providerName != "pay2u" || u.provider == nil {
		return exception.BadRequest(fmt.Sprintf("unsupported disbursement provider: %s", providerName))
	}
	var identity struct {
		Token        string `json:"token"`
		MerchantReff string `json:"merchant_reff"`
	}
	if err := json.Unmarshal(rawBody, &identity); err != nil {
		return exception.BadRequest(fmt.Sprintf("parse account transfer callback identity: %v", err))
	}
	if identity.Token == "" || identity.MerchantReff == "" {
		return exception.BadRequest("account transfer callback is missing token or merchant_reff")
	}

	var callback *provider.BankTransferCallbackResult
	var dispatch *entity.WebhookDispatch
	err := u.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		disbursement, err := u.disbursements.FindByProviderTokenAndRefForUpdate(ctx, tx, identity.Token, identity.MerchantReff)
		if err != nil {
			return exception.Internal(fmt.Errorf("find callback disbursement: %w", err))
		}
		if disbursement == nil {
			return exception.NotFound("disbursement for provider callback not found")
		}
		if disbursement.Status == constants.DisbursementStatusSuccess ||
			disbursement.Status == constants.DisbursementStatusFailed ||
			disbursement.Status == constants.DisbursementStatusCancelled {
			return nil
		}

		callback, err = u.provider.ParseTransferCallback(rawBody)
		if err != nil {
			return exception.BadRequest(fmt.Sprintf("parse account transfer callback: %v", err))
		}

		newStatus, err := mapDisbursementCallbackStatus(callback.Status)
		if err != nil {
			return err
		}
		if disbursement.Status == newStatus {
			return nil
		}
		if callback.ReceiverAccountNumber != "" && callback.ReceiverAccountNumber != disbursement.BeneficiaryAccountRef {
			return exception.Conflict("provider callback receiver account does not match the inquiry")
		}
		if callback.ReceiverBank != "" && callback.ReceiverBank != disbursement.BeneficiaryBankCode {
			return exception.Conflict("provider callback receiver bank does not match the inquiry")
		}
		if callback.ReceiverName != "" && callback.ReceiverName != disbursement.BeneficiaryName {
			return exception.Conflict("provider callback receiver name does not match the inquiry")
		}
		if callback.ChannelCode != "" && callback.ChannelCode != disbursement.ChannelCode {
			return exception.Conflict("provider callback channel_code does not match the inquiry")
		}
		if callback.ReceiverBankName != "" && callback.ReceiverBankName != disbursement.BeneficiaryBankName {
			return exception.Conflict("provider callback receiver bank name does not match the inquiry")
		}
		if callback.Amount != nil && *callback.Amount != disbursement.Amount {
			return exception.Conflict("provider callback amount does not match the inquiry")
		}
		if callback.AmountAdmin != nil && *callback.AmountAdmin != disbursement.AmountAdmin {
			return exception.Conflict("provider callback amount_admin does not match the inquiry")
		}
		if callback.AmountDiscount != nil && *callback.AmountDiscount != disbursement.AmountDiscount {
			return exception.Conflict("provider callback amount_discount does not match the inquiry")
		}
		if callback.AmountTotal != nil && *callback.AmountTotal != disbursement.AmountTotal {
			return exception.Conflict("provider callback amount_total does not match the inquiry")
		}
		if callback.Note != "" && callback.Note != disbursement.Note {
			return exception.Conflict("provider callback note does not match the inquiry")
		}
		if callback.ExpiredMinutes != nil && *callback.ExpiredMinutes != disbursement.ExpiredMinutes {
			return exception.Conflict("provider callback expired_minutes does not match the inquiry")
		}
		if newStatus == constants.DisbursementStatusSuccess && callback.AmountPaid != disbursement.AmountTotal {
			return exception.Conflict("provider callback amount_paid does not match amount_total")
		}

		updates := map[string]any{
			"status":            newStatus,
			"provider_callback": entity.JSONB(rawBody),
			"updated_at":        time.Now().UTC(),
		}
		if callback.ReferenceNo != "" {
			updates["provider_disbursement_ref"] = callback.ReferenceNo
		}
		if newStatus == constants.DisbursementStatusSuccess {
			paidAt := parseProviderTime(callback.TransferDate)
			updates["paid_at"] = paidAt
		}
		if callback.TransferDate != "" {
			updates["transferred_at"] = parseProviderTime(callback.TransferDate)
		}
		if err := u.disbursements.Update(ctx, tx, disbursement.ID, updates); err != nil {
			return exception.Internal(fmt.Errorf("update disbursement callback: %w", err))
		}
		disbursement.Status = newStatus
		disbursement.ProviderCallback = entity.JSONB(rawBody)
		if callback.ReferenceNo != "" {
			disbursement.ProviderDisbursementRef = &callback.ReferenceNo
		}
		if newStatus == constants.DisbursementStatusSuccess {
			paidAt := parseProviderTime(callback.TransferDate)
			disbursement.PaidAt = &paidAt
		}
		if callback.TransferDate != "" {
			transferredAt := parseProviderTime(callback.TransferDate)
			disbursement.TransferredAt = &transferredAt
		}

		if newStatus == constants.DisbursementStatusSuccess || newStatus == constants.DisbursementStatusCancelled {
			event := constants.WebhookEventDisbursementSuccess
			if newStatus == constants.DisbursementStatusCancelled {
				event = constants.WebhookEventDisbursementReversed
			}
			dispatch, err = u.newMerchantWebhook(ctx, tx, disbursement, callback, event)
			if err != nil {
				return exception.Internal(fmt.Errorf("create disbursement webhook: %w", err))
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	if dispatch != nil && u.dispatcher != nil {
		if err := u.dispatcher.Enqueue(ctx, dispatch.ID); err != nil {
			logger.FromContext(ctx).WithError(err).Warn("failed to enqueue disbursement webhook; recovery scanner will retry")
		}
	}
	return nil
}

func (u *DisbursementUseCase) newMerchantWebhook(ctx context.Context, db *gorm.DB, disbursement *entity.Disbursement, callback *provider.BankTransferCallbackResult, event string) (*entity.WebhookDispatch, error) {
	if u.dispatches == nil || u.merchants == nil || disbursement.MerchantID == uuid.Nil {
		return nil, nil
	}
	merchant, err := u.merchants.FindByID(ctx, db, disbursement.MerchantID)
	if err != nil || merchant == nil || disbursement.URLListenerMerchant == "" {
		return nil, err
	}
	receiverAccount := callback.ReceiverAccountNumber
	if receiverAccount == "" {
		receiverAccount = disbursement.BeneficiaryAccountRef
	}
	receiverName := callback.ReceiverName
	if receiverName == "" {
		receiverName = disbursement.BeneficiaryName
	}
	receiverBank := callback.ReceiverBank
	if receiverBank == "" {
		receiverBank = disbursement.BeneficiaryBankCode
	}
	channelCode := callback.ChannelCode
	if channelCode == "" {
		channelCode = disbursement.ChannelCode
	}
	receiverBankName := callback.ReceiverBankName
	if receiverBankName == "" {
		receiverBankName = disbursement.BeneficiaryBankName
	}
	payloadBytes, err := json.Marshal(map[string]any{
		"event":                   event,
		"token":                   disbursement.ProviderToken,
		"merchant_reff":           disbursement.MerchantRef,
		"channel_code":            channelCode,
		"receiver_account_number": receiverAccount,
		"receiver_name":           receiverName,
		"receiver_bank":           receiverBank,
		"receiver_bank_name":      receiverBankName,
		"amount":                  disbursement.Amount,
		"amount_admin":            disbursement.AmountAdmin,
		"amount_discount":         disbursement.AmountDiscount,
		"amount_total":            disbursement.AmountTotal,
		"amount_paid":             callback.AmountPaid,
		"note":                    disbursement.Note,
		"status":                  callback.Status,
		"transfer_date":           callback.TransferDate,
		"reference_no":            callback.ReferenceNo,
		"expired_minutes":         disbursement.ExpiredMinutes,
		"disbursement_id":         disbursement.ID.String(),
	})
	if err != nil {
		return nil, err
	}
	dispatch := &entity.WebhookDispatch{
		ID:            uuid.New(),
		TransactionID: disbursement.TransactionID,
		MerchantID:    merchant.ID,
		TargetURL:     disbursement.URLListenerMerchant,
		EventType:     event,
		Payload:       entity.JSONB(payloadBytes),
		Status:        constants.WebhookStatusPending,
		Attempts:      0,
		MaxAttempts:   3,
	}
	if err := u.dispatches.CreateDispatch(ctx, db, dispatch); err != nil {
		return nil, err
	}
	return dispatch, nil
}

func merchantFromContext(ctx context.Context) (*entity.Merchant, error) {
	merchant, ok := ctx.Value(constants.LocalsMerchant).(*entity.Merchant)
	if !ok || merchant == nil {
		return nil, exception.Unauthorized("merchant authentication required")
	}
	return merchant, nil
}

func calculateDisbursementTotal(amount, amountAdmin, amountDiscount int64) (int64, error) {
	if amount <= 0 || amountAdmin < 0 || amountDiscount < 0 {
		return 0, exception.BadRequest("amount must be positive and fees/discounts cannot be negative")
	}
	if amount > math.MaxInt64-amountAdmin {
		return 0, exception.BadRequest("amount_total overflows")
	}
	total := amount + amountAdmin - amountDiscount
	if total <= 0 {
		return 0, exception.BadRequest("amount_total must be positive")
	}
	return total, nil
}

func sameDisbursementRequest(existing *entity.Disbursement, req *payload.DisbursementInquiryRequest, amountTotal int64, merchantWebhookURL string) bool {
	return existing.BeneficiaryAccountRef == strings.TrimSpace(req.ReceiverAccount) &&
		existing.BeneficiaryName == strings.TrimSpace(req.ReceiverName) &&
		existing.BeneficiaryBankCode == strings.TrimSpace(req.ReceiverBank) &&
		existing.Amount == req.Amount &&
		existing.AmountAdmin == req.AmountAdmin &&
		existing.AmountDiscount == req.AmountDiscount &&
		existing.AmountTotal == amountTotal && req.AmountTotal == amountTotal &&
		existing.URLListenerMerchant == merchantWebhookURL &&
		existing.URLRedirectMerchant == strings.TrimSpace(req.URLRedirectMerchant) &&
		existing.Note == strings.TrimSpace(req.Note)
}

func disbursementResponse(disbursement *entity.Disbursement) *payload.DisbursementResponse {
	response := &payload.DisbursementResponse{
		ID:                  disbursement.ID.String(),
		TransactionID:       disbursement.TransactionID.String(),
		MerchantReff:        disbursement.MerchantRef,
		ChannelCode:         disbursement.ChannelCode,
		URLListenerMerchant: disbursement.URLListenerMerchant,
		ExpiredMinutes:      disbursement.ExpiredMinutes,
		Amount:              disbursement.Amount,
		AmountAdmin:         disbursement.AmountAdmin,
		AmountDiscount:      disbursement.AmountDiscount,
		AmountTotal:         disbursement.AmountTotal,
		Currency:            disbursement.Currency,
		BeneficiaryAccount:  disbursement.BeneficiaryAccountRef,
		BeneficiaryName:     disbursement.BeneficiaryName,
		BeneficiaryBankCode: disbursement.BeneficiaryBankCode,
		BeneficiaryBankName: disbursement.BeneficiaryBankName,
		Status:              disbursement.Status,
		Note:                disbursement.Note,
	}
	if disbursement.ProviderDisbursementRef != nil {
		response.ProviderDisbursementRef = *disbursement.ProviderDisbursementRef
	}
	if disbursement.PaidAt != nil {
		response.TransferDate = disbursement.PaidAt.Format(time.RFC3339)
	} else if disbursement.TransferredAt != nil {
		response.TransferDate = disbursement.TransferredAt.Format(time.RFC3339)
	}
	return response
}

func mapDisbursementCallbackStatus(status int) (string, error) {
	switch status {
	case 1:
		return constants.DisbursementStatusSuccess, nil
	case 2:
		return constants.DisbursementStatusProcessing, nil
	case 3:
		return constants.DisbursementStatusCancelled, nil
	default:
		return "", exception.BadRequest(fmt.Sprintf("unsupported account transfer callback status: %d", status))
	}
}

func parseProviderTime(value string) time.Time {
	location, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		location = time.UTC
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05"} {
		if parsed, err := time.ParseInLocation(layout, value, location); err == nil {
			return parsed.UTC()
		}
	}
	return time.Now().UTC()
}
