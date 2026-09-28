package worker

import (
	"context"
	"encoding/json"
	"time"

	"paygate/internal/common/constants"
	"paygate/internal/entity"
	"paygate/internal/repository"
	"paygate/internal/webhook"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type ExpiryJob struct {
	db           *gorm.DB
	txRepo       repository.TransactionRepository
	merchantRepo repository.MerchantRepository
	dispatchRepo repository.WebhookDispatchRepository
	dispatcher   webhook.Dispatcher
	log          *logrus.Logger
	interval     time.Duration
	batchSize    int
}

func NewExpiryJob(
	db *gorm.DB,
	txRepo repository.TransactionRepository,
	merchantRepo repository.MerchantRepository,
	dispatchRepo repository.WebhookDispatchRepository,
	dispatcher webhook.Dispatcher,
	log *logrus.Logger,
	interval time.Duration,
	batchSize int,
) *ExpiryJob {
	if interval <= 0 {
		interval = 1 * time.Minute
	}
	if batchSize <= 0 {
		batchSize = 100
	}
	return &ExpiryJob{
		db:           db,
		txRepo:       txRepo,
		merchantRepo: merchantRepo,
		dispatchRepo: dispatchRepo,
		dispatcher:   dispatcher,
		log:          log,
		interval:     interval,
		batchSize:    batchSize,
	}
}

func (j *ExpiryJob) Name() string {
	return "auto-expiry"
}

func (j *ExpiryJob) Interval() time.Duration {
	return j.interval
}

func (j *ExpiryJob) Run(ctx context.Context) error {
	expiredTxs, err := j.txRepo.ExpirePendingTransactions(ctx, j.db, j.batchSize)
	if err != nil {
		return err
	}

	if len(expiredTxs) == 0 {
		return nil
	}

	j.log.WithField("count", len(expiredTxs)).Info("expired pending transactions detected and updated")

	for _, tx := range expiredTxs {
		j.processExpiredTransaction(ctx, tx)
	}

	return nil
}

func (j *ExpiryJob) processExpiredTransaction(ctx context.Context, tx *entity.Transaction) {
	if tx.MerchantID == nil || j.dispatchRepo == nil || j.dispatcher == nil {
		return
	}

	merchant, err := j.merchantRepo.FindByID(ctx, j.db, *tx.MerchantID)
	if err != nil {
		j.log.WithError(err).WithField("merchant_id", *tx.MerchantID).Warn("failed to load merchant for expired transaction webhook")
		return
	}

	if merchant == nil || merchant.WebhookURL == "" {
		return
	}

	expiredAtStr := ""
	if tx.ExpiredAt != nil {
		expiredAtStr = tx.ExpiredAt.Format(time.RFC3339)
	} else {
		expiredAtStr = time.Now().UTC().Format(time.RFC3339)
	}

	payloadMap := map[string]any{
		"event":           constants.WebhookEventPaymentExpired,
		"payment_id":      tx.ID.String(),
		"merchant_reff":   tx.MerchantReff,
		"payment_method":  tx.PaymentMethod,
		"payment_code":    tx.PaymentCode,
		"amount":          tx.Amount,
		"amount_admin":    tx.AmountAdmin,
		"amount_discount": tx.AmountDiscount,
		"amount_total":    tx.AmountTotal,
		"currency":        tx.Currency,
		"status":          constants.TransactionStatusExpired,
		"expired_at":      expiredAtStr,
	}

	payloadBytes, err := json.Marshal(payloadMap)
	if err != nil {
		j.log.WithError(err).WithField("merchant_reff", tx.MerchantReff).Error("failed to marshal expired payment webhook payload")
		return
	}

	dispatch := &entity.WebhookDispatch{
		ID:            uuid.New(),
		TransactionID: tx.ID,
		MerchantID:    merchant.ID,
		TargetURL:     merchant.WebhookURL,
		EventType:     constants.WebhookEventPaymentExpired,
		Payload:       entity.JSONB(payloadBytes),
		Status:        constants.WebhookStatusPending,
		Attempts:      0,
		MaxAttempts:   3,
	}

	if err := j.dispatchRepo.CreateDispatch(ctx, j.db, dispatch); err != nil {
		j.log.WithError(err).WithField("dispatch_id", dispatch.ID).Error("failed to create expired payment webhook dispatch")
		return
	}

	if err := j.dispatcher.Enqueue(ctx, dispatch.ID); err != nil {
		j.log.WithError(err).WithField("dispatch_id", dispatch.ID).Warn("failed to enqueue expired payment webhook dispatch; recovery scanner will retry")
	}
}
