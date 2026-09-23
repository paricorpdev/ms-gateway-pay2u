package usecase

import (
	"context"
	"sync"
	"testing"
	"time"

	"paygate/internal/common/constants"
	"paygate/internal/entity"
	"paygate/internal/exception"
	"paygate/internal/logger"
	"paygate/internal/model/payload"
	"paygate/internal/provider"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type inMemoryTxRepo struct {
	mu           sync.RWMutex
	transactions map[uuid.UUID]*entity.Transaction
}

func newInMemoryTxRepo() *inMemoryTxRepo {
	return &inMemoryTxRepo{
		transactions: make(map[uuid.UUID]*entity.Transaction),
	}
}

func (r *inMemoryTxRepo) Create(ctx context.Context, db *gorm.DB, tx *entity.Transaction) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.transactions[tx.ID] = tx
	return nil
}

func (r *inMemoryTxRepo) Save(ctx context.Context, db *gorm.DB, tx *entity.Transaction) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.transactions[tx.ID] = tx
	return nil
}

func (r *inMemoryTxRepo) FindByID(ctx context.Context, db *gorm.DB, id uuid.UUID) (*entity.Transaction, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	tx, ok := r.transactions[id]
	if !ok {
		return nil, exception.NotFound("transaction not found")
	}
	return tx, nil
}

func (r *inMemoryTxRepo) FindByRequestID(ctx context.Context, db *gorm.DB, requestID string) (*entity.Transaction, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, tx := range r.transactions {
		if tx.RequestID == requestID {
			return tx, nil
		}
	}
	return nil, nil
}

func (r *inMemoryTxRepo) FindByMerchantReff(ctx context.Context, db *gorm.DB, reff string) (*entity.Transaction, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, tx := range r.transactions {
		if tx.MerchantReff == reff {
			return tx, nil
		}
	}
	return nil, nil
}

func (r *inMemoryTxRepo) FindByProviderToken(ctx context.Context, db *gorm.DB, token string) (*entity.Transaction, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, tx := range r.transactions {
		if tx.ProviderToken == token {
			return tx, nil
		}
	}
	return nil, nil
}

func (r *inMemoryTxRepo) UpdateStatus(ctx context.Context, db *gorm.DB, id uuid.UUID, status string, updates map[string]any) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	tx, ok := r.transactions[id]
	if !ok {
		return exception.NotFound("transaction not found")
	}
	tx.Status = status
	if paidAt, ok := updates["paid_at"].(*time.Time); ok {
		tx.PaidAt = paidAt
	}
	return nil
}

type mockProvider struct {
	lastBillReq *provider.BillRequest
	billResult  *provider.BillResult
}

func (m *mockProvider) Name() string { return "pay2u" }

func (m *mockProvider) CreateBill(ctx context.Context, req *provider.BillRequest) (*provider.BillResult, error) {
	m.lastBillReq = req
	return m.billResult, nil
}

func (m *mockProvider) GetBill(ctx context.Context, token, methodCode string) (*provider.BillResult, error) {
	return m.billResult, nil
}

func (m *mockProvider) ParseCallback(body []byte) (*provider.CallbackResult, error) {
	return &provider.CallbackResult{
		MerchantReff:  "INV-2026-0001",
		ProviderToken: "TOKEN-MOCK-123",
		Status:        1,
	}, nil
}

func TestPaymentUseCase_CreatePayment_Success(t *testing.T) {
	repo := newInMemoryTxRepo()
	mockProv := &mockProvider{
		billResult: &provider.BillResult{
			Token:       "TOKEN-MOCK-123",
			PaymentCode: "1392570141172029",
			Status:      0,
		},
	}
	val := validator.New()
	uc := NewPaymentUseCase(nil, val, repo, map[string]provider.PaymentProvider{"pay2u": mockProv})

	ctx := context.Background()
	req := &payload.CreatePaymentRequest{
		MerchantReff:    "INV-2026-0001",
		PaymentMethod:   "VA-MITRA",
		BillTitle:       "Order 1001",
		CustomerName:    "Budi Santoso",
		CustomerPhone:   "081234567890",
		Amount:          50000,
		AmountTotal:     50000,
		ExpiredMinutes:  1440,
	}

	res, err := uc.CreatePayment(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error creating payment: %v", err)
	}

	if res.MerchantReff != "INV-2026-0001" {
		t.Errorf("expected MerchantReff %q, got %q", "INV-2026-0001", res.MerchantReff)
	}
	if mockProv.lastBillReq.MerchantReff != "INV-2026-0001" {
		t.Errorf("expected provider BillRequest.MerchantReff %q, got %q", "INV-2026-0001", mockProv.lastBillReq.MerchantReff)
	}
	if res.PaymentCode != "1392570141172029" {
		t.Errorf("expected PaymentCode %q, got %q", "1392570141172029", res.PaymentCode)
	}
}

func TestPaymentUseCase_CreatePayment_MissingMerchantReff(t *testing.T) {
	repo := newInMemoryTxRepo()
	mockProv := &mockProvider{billResult: &provider.BillResult{Token: "TOKEN-MOCK"}}
	val := validator.New()
	uc := NewPaymentUseCase(nil, val, repo, map[string]provider.PaymentProvider{"pay2u": mockProv})

	ctx := context.Background()
	req := &payload.CreatePaymentRequest{
		// MerchantReff omitted
		PaymentMethod:  "VA-MITRA",
		BillTitle:      "Order 1002",
		CustomerName:   "Budi",
		CustomerPhone:  "081234567890",
		Amount:         50000,
		AmountTotal:    50000,
	}

	_, err := uc.CreatePayment(ctx, req)
	if err == nil {
		t.Fatal("expected validation error when merchant_reff is missing, got nil")
	}
}

func TestPaymentUseCase_CreatePayment_DuplicateMerchantReff(t *testing.T) {
	repo := newInMemoryTxRepo()
	mockProv := &mockProvider{billResult: &provider.BillResult{Token: "TOKEN-MOCK"}}
	val := validator.New()
	uc := NewPaymentUseCase(nil, val, repo, map[string]provider.PaymentProvider{"pay2u": mockProv})

	ctx1 := logger.WithRequestID(context.Background(), "req-001")
	req1 := &payload.CreatePaymentRequest{
		MerchantReff:   "INV-DUP-TEST",
		PaymentMethod:  "VA-MITRA",
		BillTitle:      "Order 1",
		CustomerName:   "Budi",
		CustomerPhone:  "081234567890",
		Amount:         50000,
		AmountTotal:    50000,
	}

	_, err := uc.CreatePayment(ctx1, req1)
	if err != nil {
		t.Fatalf("unexpected error creating first payment: %v", err)
	}

	// Different request_id, but same merchant_reff -> should conflict
	ctx2 := logger.WithRequestID(context.Background(), "req-002")
	req2 := &payload.CreatePaymentRequest{
		MerchantReff:   "INV-DUP-TEST",
		PaymentMethod:  "VA-MITRA",
		BillTitle:      "Order 2",
		CustomerName:   "Siti",
		CustomerPhone:  "081234567891",
		Amount:         60000,
		AmountTotal:    60000,
	}

	_, err = uc.CreatePayment(ctx2, req2)
	if err == nil {
		t.Fatal("expected conflict error on duplicate merchant_reff, got nil")
	}
	appErr, ok := exception.As(err)
	if !ok || appErr.Code != exception.CodeConflict {
		t.Errorf("expected CodeConflict error, got %v", err)
	}
}

func TestPaymentUseCase_CreatePayment_ReplayByRequestID(t *testing.T) {
	repo := newInMemoryTxRepo()
	calls := 0
	mockProv := &mockProvider{
		billResult: &provider.BillResult{Token: "TOKEN-MOCK", PaymentCode: "VA-123"},
	}
	val := validator.New()
	uc := NewPaymentUseCase(nil, val, repo, map[string]provider.PaymentProvider{"pay2u": mockProv})

	// Same request_id in context simulates HTTP retry with same X-Request-Id
	ctx := logger.WithRequestID(context.Background(), "req-replay-same-123")
	req := &payload.CreatePaymentRequest{
		MerchantReff:   "INV-REPLAY-1",
		PaymentMethod:  "VA-MITRA",
		BillTitle:      "Order Same",
		CustomerName:   "Budi",
		CustomerPhone:  "081234567890",
		Amount:         50000,
		AmountTotal:    50000,
	}

	res1, err := uc.CreatePayment(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error on first call: %v", err)
	}
	calls++

	// Resend exact same request with same request_id
	res2, err := uc.CreatePayment(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error on second call: %v", err)
	}

	if res1.ID != res2.ID {
		t.Errorf("expected same transaction ID on replay, got %s and %s", res1.ID, res2.ID)
	}
}

func TestPaymentUseCase_HandleCallback(t *testing.T) {
	repo := newInMemoryTxRepo()
	mockProv := &mockProvider{
		billResult: &provider.BillResult{Token: "TOKEN-MOCK-123"},
	}
	val := validator.New()
	uc := NewPaymentUseCase(nil, val, repo, map[string]provider.PaymentProvider{"pay2u": mockProv})

	ctx := context.Background()
	req := &payload.CreatePaymentRequest{
		MerchantReff:   "INV-2026-0001",
		PaymentMethod:  "VA-MITRA",
		BillTitle:      "Order Callback",
		CustomerName:   "Budi",
		CustomerPhone:  "081234567890",
		Amount:         50000,
		AmountTotal:    50000,
	}

	res, err := uc.CreatePayment(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error creating payment: %v", err)
	}
	if res.Status != constants.TransactionStatusPending {
		t.Fatalf("expected initial status PENDING, got %s", res.Status)
	}

	// Trigger callback with matching MerchantReff
	err = uc.HandleCallback(ctx, "pay2u", []byte(`{}`))
	if err != nil {
		t.Fatalf("unexpected error handling callback: %v", err)
	}

	txID, _ := uuid.Parse(res.ID)
	tx, _ := repo.FindByID(ctx, nil, txID)
	if tx.Status != constants.TransactionStatusSuccess {
		t.Errorf("expected status SUCCESS after callback, got %s", tx.Status)
	}
	if tx.PaidAt == nil {
		t.Error("expected PaidAt to be set after successful callback")
	}
}
