package worker

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"paygate/internal/common/constants"
	"paygate/internal/entity"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type mockTxRepoForExpiry struct {
	expiredTxs []*entity.Transaction
}

func (m *mockTxRepoForExpiry) Create(ctx context.Context, db *gorm.DB, tx *entity.Transaction) error {
	return nil
}
func (m *mockTxRepoForExpiry) Save(ctx context.Context, db *gorm.DB, tx *entity.Transaction) error {
	return nil
}
func (m *mockTxRepoForExpiry) FindByID(ctx context.Context, db *gorm.DB, id uuid.UUID) (*entity.Transaction, error) {
	return nil, nil
}
func (m *mockTxRepoForExpiry) FindByRequestID(ctx context.Context, db *gorm.DB, requestID string) (*entity.Transaction, error) {
	return nil, nil
}
func (m *mockTxRepoForExpiry) FindByMerchantReff(ctx context.Context, db *gorm.DB, reff string) (*entity.Transaction, error) {
	return nil, nil
}
func (m *mockTxRepoForExpiry) FindByProviderToken(ctx context.Context, db *gorm.DB, token string) (*entity.Transaction, error) {
	return nil, nil
}
func (m *mockTxRepoForExpiry) UpdateStatus(ctx context.Context, db *gorm.DB, id uuid.UUID, status string, updates map[string]any) error {
	return nil
}
func (m *mockTxRepoForExpiry) ExpirePendingTransactions(ctx context.Context, db *gorm.DB, limit int) ([]*entity.Transaction, error) {
	return m.expiredTxs, nil
}

type mockMerchantRepoForExpiry struct {
	merchant *entity.Merchant
}

func (m *mockMerchantRepoForExpiry) Create(ctx context.Context, db *gorm.DB, merchant *entity.Merchant) error {
	return nil
}
func (m *mockMerchantRepoForExpiry) FindByID(ctx context.Context, db *gorm.DB, id uuid.UUID) (*entity.Merchant, error) {
	return m.merchant, nil
}
func (m *mockMerchantRepoForExpiry) FindByAPIKey(ctx context.Context, db *gorm.DB, apiKey string) (*entity.Merchant, error) {
	return nil, nil
}
func (m *mockMerchantRepoForExpiry) FindByCode(ctx context.Context, db *gorm.DB, code string) (*entity.Merchant, error) {
	return nil, nil
}
func (m *mockMerchantRepoForExpiry) FindAll(ctx context.Context, db *gorm.DB, limit, offset int) ([]*entity.Merchant, int64, error) {
	return nil, 0, nil
}
func (m *mockMerchantRepoForExpiry) Update(ctx context.Context, db *gorm.DB, id uuid.UUID, updates map[string]any) error {
	return nil
}

type mockDispatchRepoForExpiry struct {
	mu         sync.Mutex
	dispatches []*entity.WebhookDispatch
}

func (m *mockDispatchRepoForExpiry) CreateDispatch(ctx context.Context, db *gorm.DB, d *entity.WebhookDispatch) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.dispatches = append(m.dispatches, d)
	return nil
}
func (m *mockDispatchRepoForExpiry) ClaimDispatch(ctx context.Context, db *gorm.DB, id uuid.UUID, workerID string, lockDuration time.Duration) (bool, error) {
	return true, nil
}
func (m *mockDispatchRepoForExpiry) ReleaseDispatch(ctx context.Context, db *gorm.DB, id uuid.UUID, status string, attempts int, nextRetry *time.Time) error {
	return nil
}
func (m *mockDispatchRepoForExpiry) CreateDispatchLog(ctx context.Context, db *gorm.DB, l *entity.WebhookDispatchLog) error {
	return nil
}
func (m *mockDispatchRepoForExpiry) FindDispatchByID(ctx context.Context, db *gorm.DB, id uuid.UUID) (*entity.WebhookDispatch, error) {
	return nil, nil
}
func (m *mockDispatchRepoForExpiry) FindRecoverableDispatches(ctx context.Context, db *gorm.DB, limit int) ([]*entity.WebhookDispatch, error) {
	return nil, nil
}
func (m *mockDispatchRepoForExpiry) DeleteDispatchesBefore(ctx context.Context, db *gorm.DB, cutoff time.Time, limit int) (int64, error) {
	return 0, nil
}
func (m *mockDispatchRepoForExpiry) DeleteDispatchLogsBefore(ctx context.Context, db *gorm.DB, cutoff time.Time, limit int) (int64, error) {
	return 0, nil
}

type mockDispatcherForExpiry struct {
	mu       sync.Mutex
	enqueued []uuid.UUID
}

func (m *mockDispatcherForExpiry) Start(ctx context.Context) {}
func (m *mockDispatcherForExpiry) Stop(ctx context.Context) error {
	return nil
}
func (m *mockDispatcherForExpiry) Enqueue(ctx context.Context, dispatchID uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.enqueued = append(m.enqueued, dispatchID)
	return nil
}

func TestExpiryJob_Run(t *testing.T) {
	log := logrus.New()
	log.SetOutput(logrus.StandardLogger().Out)

	merchantID := uuid.New()
	merchant := &entity.Merchant{
		ID:         merchantID,
		Code:       "MERCHANT-1",
		WebhookURL: "https://merchant.example.com/webhook",
	}

	now := time.Now()
	expiredAt := now.Add(-10 * time.Minute)

	txID := uuid.New()
	tx := &entity.Transaction{
		ID:            txID,
		MerchantID:    &merchantID,
		MerchantReff:  "ORDER-EXP-123",
		PaymentMethod: constants.PaymentMethodQRIS,
		Amount:        50000,
		AmountAdmin:   1000,
		AmountTotal:   51000,
		Currency:      "IDR",
		Status:        constants.TransactionStatusExpired,
		ExpiredAt:     &expiredAt,
	}

	txRepo := &mockTxRepoForExpiry{
		expiredTxs: []*entity.Transaction{tx},
	}
	merchantRepo := &mockMerchantRepoForExpiry{
		merchant: merchant,
	}
	dispatchRepo := &mockDispatchRepoForExpiry{}
	dispatcher := &mockDispatcherForExpiry{}

	job := NewExpiryJob(nil, txRepo, merchantRepo, dispatchRepo, dispatcher, log, 1*time.Minute, 100)

	if job.Name() != "auto-expiry" {
		t.Errorf("expected name 'auto-expiry', got %s", job.Name())
	}
	if job.Interval() != 1*time.Minute {
		t.Errorf("expected interval 1m, got %v", job.Interval())
	}

	err := job.Run(context.Background())
	if err != nil {
		t.Fatalf("job.Run failed: %v", err)
	}

	dispatchRepo.mu.Lock()
	defer dispatchRepo.mu.Unlock()

	if len(dispatchRepo.dispatches) != 1 {
		t.Fatalf("expected 1 dispatch record created, got %d", len(dispatchRepo.dispatches))
	}

	d := dispatchRepo.dispatches[0]
	if d.EventType != constants.WebhookEventPaymentExpired {
		t.Errorf("expected event_type %s, got %s", constants.WebhookEventPaymentExpired, d.EventType)
	}
	if d.TargetURL != merchant.WebhookURL {
		t.Errorf("expected target_url %s, got %s", merchant.WebhookURL, d.TargetURL)
	}

	var payloadMap map[string]any
	if err := json.Unmarshal([]byte(d.Payload), &payloadMap); err != nil {
		t.Fatalf("failed to unmarshal payload JSON: %v", err)
	}

	if payloadMap["status"] != constants.TransactionStatusExpired {
		t.Errorf("expected status 'EXPIRED', got %v", payloadMap["status"])
	}
	if payloadMap["merchant_reff"] != "ORDER-EXP-123" {
		t.Errorf("expected merchant_reff 'ORDER-EXP-123', got %v", payloadMap["merchant_reff"])
	}

	dispatcher.mu.Lock()
	defer dispatcher.mu.Unlock()
	if len(dispatcher.enqueued) != 1 || dispatcher.enqueued[0] != d.ID {
		t.Errorf("expected dispatch ID %s enqueued, got %v", d.ID, dispatcher.enqueued)
	}
}
