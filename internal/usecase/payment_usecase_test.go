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
	"paygate/internal/utils"

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
	if paymentReff, ok := updates["payment_reff"].(string); ok {
		tx.PaymentReff = paymentReff
	}
	return nil
}

type mockDispatcher struct {
	mu       sync.Mutex
	enqueued []uuid.UUID
}

func (m *mockDispatcher) Start(ctx context.Context)      {}
func (m *mockDispatcher) Stop(ctx context.Context) error { return nil }
func (m *mockDispatcher) Enqueue(ctx context.Context, id uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.enqueued = append(m.enqueued, id)
	return nil
}

type inMemoryDispatchRepoForPaymentTest struct {
	mu         sync.Mutex
	dispatches map[uuid.UUID]*entity.WebhookDispatch
}

func newInMemoryDispatchRepoForPaymentTest() *inMemoryDispatchRepoForPaymentTest {
	return &inMemoryDispatchRepoForPaymentTest{
		dispatches: make(map[uuid.UUID]*entity.WebhookDispatch),
	}
}

func (r *inMemoryDispatchRepoForPaymentTest) CreateDispatch(ctx context.Context, db *gorm.DB, d *entity.WebhookDispatch) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.dispatches[d.ID] = d
	return nil
}

func (r *inMemoryDispatchRepoForPaymentTest) ClaimDispatch(ctx context.Context, db *gorm.DB, id uuid.UUID, workerID string, lockDuration time.Duration) (bool, error) {
	return true, nil
}

func (r *inMemoryDispatchRepoForPaymentTest) ReleaseDispatch(ctx context.Context, db *gorm.DB, id uuid.UUID, status string, attempts int, nextRetry *time.Time) error {
	return nil
}

func (r *inMemoryDispatchRepoForPaymentTest) CreateDispatchLog(ctx context.Context, db *gorm.DB, l *entity.WebhookDispatchLog) error {
	return nil
}

func (r *inMemoryDispatchRepoForPaymentTest) FindDispatchByID(ctx context.Context, db *gorm.DB, id uuid.UUID) (*entity.WebhookDispatch, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.dispatches[id], nil
}

func (r *inMemoryDispatchRepoForPaymentTest) FindRecoverableDispatches(ctx context.Context, db *gorm.DB, limit int) ([]*entity.WebhookDispatch, error) {
	return nil, nil
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
	uc := NewPaymentUseCase(nil, val, repo, newInMemoryMerchantRepo(), newInMemoryDispatchRepoForPaymentTest(), &mockDispatcher{}, map[string]provider.PaymentProvider{"pay2u": mockProv})

	ctx := context.Background()
	req := &payload.CreatePaymentRequest{
		MerchantReff:   "INV-2026-0001",
		PaymentMethod:  "VA-MITRA",
		BillTitle:      "Order 1001",
		CustomerName:   "Budi Santoso",
		CustomerPhone:  "081234567890",
		Amount:         50000,
		AmountTotal:    50000,
		ExpiredMinutes: 1440,
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
	uc := NewPaymentUseCase(nil, val, repo, newInMemoryMerchantRepo(), newInMemoryDispatchRepoForPaymentTest(), &mockDispatcher{}, map[string]provider.PaymentProvider{"pay2u": mockProv})

	ctx := context.Background()
	req := &payload.CreatePaymentRequest{
		// MerchantReff omitted
		PaymentMethod: "VA-MITRA",
		BillTitle:     "Order 1002",
		CustomerName:  "Budi",
		CustomerPhone: "081234567890",
		Amount:        50000,
		AmountTotal:   50000,
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
	uc := NewPaymentUseCase(nil, val, repo, newInMemoryMerchantRepo(), newInMemoryDispatchRepoForPaymentTest(), &mockDispatcher{}, map[string]provider.PaymentProvider{"pay2u": mockProv})

	ctx1 := logger.WithRequestID(context.Background(), "req-001")
	req1 := &payload.CreatePaymentRequest{
		MerchantReff:  "INV-DUP-TEST",
		PaymentMethod: "VA-MITRA",
		BillTitle:     "Order 1",
		CustomerName:  "Budi",
		CustomerPhone: "081234567890",
		Amount:        50000,
		AmountTotal:   50000,
	}

	_, err := uc.CreatePayment(ctx1, req1)
	if err != nil {
		t.Fatalf("unexpected error creating first payment: %v", err)
	}

	// Different request_id, but same merchant_reff -> should conflict
	ctx2 := logger.WithRequestID(context.Background(), "req-002")
	req2 := &payload.CreatePaymentRequest{
		MerchantReff:  "INV-DUP-TEST",
		PaymentMethod: "VA-MITRA",
		BillTitle:     "Order 2",
		CustomerName:  "Siti",
		CustomerPhone: "081234567891",
		Amount:        60000,
		AmountTotal:   60000,
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
	uc := NewPaymentUseCase(nil, val, repo, newInMemoryMerchantRepo(), newInMemoryDispatchRepoForPaymentTest(), &mockDispatcher{}, map[string]provider.PaymentProvider{"pay2u": mockProv})

	// Same request_id in context simulates HTTP retry with same X-Request-Id
	ctx := logger.WithRequestID(context.Background(), "req-replay-same-123")
	req := &payload.CreatePaymentRequest{
		MerchantReff:  "INV-REPLAY-1",
		PaymentMethod: "VA-MITRA",
		BillTitle:     "Order Same",
		CustomerName:  "Budi",
		CustomerPhone: "081234567890",
		Amount:        50000,
		AmountTotal:   50000,
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
	uc := NewPaymentUseCase(nil, val, repo, newInMemoryMerchantRepo(), newInMemoryDispatchRepoForPaymentTest(), &mockDispatcher{}, map[string]provider.PaymentProvider{"pay2u": mockProv})

	ctx := context.Background()
	req := &payload.CreatePaymentRequest{
		MerchantReff:  "INV-2026-0001",
		PaymentMethod: "VA-MITRA",
		BillTitle:     "Order Callback",
		CustomerName:  "Budi",
		CustomerPhone: "081234567890",
		Amount:        50000,
		AmountTotal:   50000,
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

func TestPaymentUseCase_HandleCallback_TriggersMerchantWebhook(t *testing.T) {
	repo := newInMemoryTxRepo()
	mockProv := &mockProvider{
		billResult: &provider.BillResult{Token: "TOKEN-MOCK-123"},
	}
	val := validator.New()
	merchantRepo := newInMemoryMerchantRepo()
	dispatchRepo := newInMemoryDispatchRepoForPaymentTest()
	dispatcher := &mockDispatcher{}

	merchantID := uuid.New()
	merchant := &entity.Merchant{
		ID:            merchantID,
		Code:          "M-001",
		Name:          "Test Merchant",
		IsActive:      true,
		WebhookURL:    "https://merchant.example.com/webhook",
		WebhookSecret: "secret-123",
	}
	_ = merchantRepo.Create(context.Background(), nil, merchant)

	uc := NewPaymentUseCase(nil, val, repo, merchantRepo, dispatchRepo, dispatcher, map[string]provider.PaymentProvider{"pay2u": mockProv})

	ctx := context.WithValue(context.Background(), constants.LocalsMerchant, merchant)
	req := &payload.CreatePaymentRequest{
		MerchantReff:  "INV-WH-001",
		PaymentMethod: "VA-MITRA",
		BillTitle:     "Order Webhook",
		CustomerName:  "Budi",
		CustomerPhone: "081234567890",
		Amount:        75000,
		AmountTotal:   75000,
	}

	res, err := uc.CreatePayment(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error creating payment: %v", err)
	}

	// Handle callback -> status goes to SUCCESS -> triggers webhook
	err = uc.HandleCallback(ctx, "pay2u", []byte(`{}`))
	if err != nil {
		t.Fatalf("unexpected error handling callback: %v", err)
	}

	if len(dispatcher.enqueued) != 1 {
		t.Fatalf("expected 1 enqueued dispatch, got %d", len(dispatcher.enqueued))
	}

	enqueuedID := dispatcher.enqueued[0]
	dispatch, err := dispatchRepo.FindDispatchByID(ctx, nil, enqueuedID)
	if err != nil || dispatch == nil {
		t.Fatalf("failed to find enqueued dispatch: %v", err)
	}

	if dispatch.TargetURL != "https://merchant.example.com/webhook" {
		t.Errorf("expected TargetURL %q, got %q", "https://merchant.example.com/webhook", dispatch.TargetURL)
	}
	if dispatch.EventType != entity.WebhookEventPaymentSuccess {
		t.Errorf("expected EventType %q, got %q", entity.WebhookEventPaymentSuccess, dispatch.EventType)
	}
	if dispatch.TransactionID.String() != res.ID {
		t.Errorf("expected TransactionID %s, got %s", res.ID, dispatch.TransactionID)
	}
}

func TestPaymentUseCase_RefreshPayment_Success(t *testing.T) {
	repo := newInMemoryTxRepo()
	mockProv := &mockProvider{
		billResult: &provider.BillResult{
			Token:        "AILRWULCE1TGLRFYLGPX",
			MerchantReff: "ref-1790222645904-5585",
			PaymentCode:  "MDAwMjAx...",
			Status:       1,
			PaymentReff:  "TW2026092487",
			PaymentDate:  "2026-09-24 11:25:01",
		},
	}
	val := validator.New()
	merchantRepo := newInMemoryMerchantRepo()
	dispatchRepo := newInMemoryDispatchRepoForPaymentTest()
	dispatcher := &mockDispatcher{}

	merchantID := uuid.New()
	merchant := &entity.Merchant{
		ID:            merchantID,
		Code:          "M-001",
		Name:          "Test Merchant",
		IsActive:      true,
		WebhookURL:    "https://merchant.example.com/webhook",
		WebhookSecret: "secret-123",
	}
	_ = merchantRepo.Create(context.Background(), nil, merchant)

	uc := NewPaymentUseCase(nil, val, repo, merchantRepo, dispatchRepo, dispatcher, map[string]provider.PaymentProvider{"pay2u": mockProv})

	ctx := context.WithValue(context.Background(), constants.LocalsMerchant, merchant)
	req := &payload.CreatePaymentRequest{
		MerchantReff:  "ref-1790222645904-5585",
		PaymentMethod: "QRIS-MITRA",
		BillTitle:     "QRIS Order #1",
		CustomerName:  "Siti Rahma",
		CustomerPhone: "081298765432",
		Amount:        75000,
		AmountTotal:   78500,
	}

	createRes, err := uc.CreatePayment(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error creating payment: %v", err)
	}
	if createRes.Status != constants.TransactionStatusPending {
		t.Fatalf("expected initial status PENDING, got %s", createRes.Status)
	}

	txID, _ := uuid.Parse(createRes.ID)
	// Refresh using merchant_reff (new behavior requested by user)
	refreshRes, err := uc.RefreshPayment(ctx, "ref-1790222645904-5585")
	if err != nil {
		t.Fatalf("unexpected error on RefreshPayment: %v", err)
	}

	if refreshRes.Status != constants.TransactionStatusSuccess {
		t.Errorf("expected status SUCCESS on refresh, got %s", refreshRes.Status)
	}
	if refreshRes.PaymentReff != "TW2026092487" {
		t.Errorf("expected PaymentReff TW2026092487, got %q", refreshRes.PaymentReff)
	}
	if refreshRes.PaidAt == nil {
		t.Error("expected PaidAt to be set after refresh")
	} else if refreshRes.PaidAt.Format("2006-01-02 15:04:05") != "2026-09-24 11:25:01" {
		t.Errorf("expected PaidAt 2026-09-24 11:25:01, got %v", refreshRes.PaidAt.Format("2006-01-02 15:04:05"))
	}

	// Verify GetPayment by merchant_reff
	getResByReff, err := uc.GetPayment(ctx, "ref-1790222645904-5585")
	if err != nil {
		t.Fatalf("unexpected error on GetPayment by merchant_reff: %v", err)
	}
	if getResByReff.ID != createRes.ID {
		t.Errorf("expected ID %s, got %s", createRes.ID, getResByReff.ID)
	}

	// Verify GetPayment by UUID (fallback)
	getResByUUID, err := uc.GetPayment(ctx, createRes.ID)
	if err != nil {
		t.Fatalf("unexpected error on GetPayment by UUID: %v", err)
	}
	if getResByUUID.MerchantReff != "ref-1790222645904-5585" {
		t.Errorf("expected MerchantReff %q, got %q", "ref-1790222645904-5585", getResByUUID.MerchantReff)
	}

	// Verify DB state
	savedTx, _ := repo.FindByID(ctx, nil, txID)
	if savedTx.Status != constants.TransactionStatusSuccess {
		t.Errorf("expected saved tx status SUCCESS, got %s", savedTx.Status)
	}
	if savedTx.PaymentReff != "TW2026092487" {
		t.Errorf("expected saved tx PaymentReff TW2026092487, got %q", savedTx.PaymentReff)
	}

	// Verify webhook triggered once on transition
	if len(dispatcher.enqueued) != 1 {
		t.Fatalf("expected 1 webhook dispatch enqueued on refresh, got %d", len(dispatcher.enqueued))
	}

	// Second refresh call on already successful payment using UUID fallback should NOT trigger a second webhook dispatch
	_, err = uc.RefreshPayment(ctx, createRes.ID)
	if err != nil {
		t.Fatalf("unexpected error on second RefreshPayment: %v", err)
	}
	if len(dispatcher.enqueued) != 1 {
		t.Errorf("expected still 1 webhook dispatch after second refresh, got %d", len(dispatcher.enqueued))
	}
}

func TestCalculateAdminFee(t *testing.T) {
	tests := []struct {
		name          string
		paymentMethod string
		amount        int64
		expectedFee   int64
	}{
		{"VA BNI", "BNI", 50000, 3500},
		{"VA BRI", "BRI", 50000, 3500},
		{"VA BSI", "BSI", 50000, 3500},
		{"VA BTN", "BTN", 50000, 3500},
		{"VA CIMB", "CIMB", 50000, 3500},
		{"VA Danamon", "Danamon", 50000, 3500},
		{"VA Mandiri", "Mandiri", 50000, 3500},
		{"VA Permata", "Permata", 50000, 3500},
		{"VA-MITRA", "VA-MITRA", 50000, 3500},
		{"BRIVA-MITRA", "BRIVA-MITRA", 50000, 3500},
		{"BNIVA-MITRA", "BNIVA-MITRA", 50000, 3500},
		{"VA prefix", "VA-MANDIRI", 50000, 3500},
		{"QRIS 100k", "QRIS", 100000, 700},
		{"QRIS MPM 50k", "QRIS MPM", 50000, 350},
		{"QRIS-MITRA 75k", "QRIS-MITRA", 75000, 525},
		{"QRIS-MPM 200k", "QRIS-MPM", 200000, 1400},
		{"CC MITRA no admin fee", "CC-MITRA", 100000, 0},
		{"Unknown method", "UNKNOWN", 100000, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := utils.CalculateAdminFee(tt.paymentMethod, tt.amount)
			if actual != tt.expectedFee {
				t.Errorf("CalculateAdminFee(%q, %d) = %d; want %d", tt.paymentMethod, tt.amount, actual, tt.expectedFee)
			}
		})
	}
}

func TestPaymentUseCase_CreatePayment_OnlyAmountGiven(t *testing.T) {
	repo := newInMemoryTxRepo()
	mockProv := &mockProvider{
		billResult: &provider.BillResult{
			Token:       "TOKEN-ONLY-AMOUNT",
			PaymentCode: "988123456789",
		},
	}
	val := validator.New()
	uc := NewPaymentUseCase(nil, val, repo, newInMemoryMerchantRepo(), newInMemoryDispatchRepoForPaymentTest(), &mockDispatcher{}, map[string]provider.PaymentProvider{"pay2u": mockProv})

	ctx := context.Background()
	// Merchant only provides amount, no amount_admin and no amount_total
	req := &payload.CreatePaymentRequest{
		MerchantReff:   "INV-ONLY-AMOUNT-1",
		PaymentMethod:  "MANDIRI",
		BillTitle:      "Order Mandiri",
		CustomerName:   "Ahmad",
		CustomerPhone:  "081234567890",
		Amount:         100000,
		ExpiredMinutes: 60,
	}

	res, err := uc.CreatePayment(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error creating payment: %v", err)
	}

	if res.Amount != 100000 {
		t.Errorf("expected Amount 100000, got %d", res.Amount)
	}
	if res.AmountAdmin != 3500 {
		t.Errorf("expected AmountAdmin 3500, got %d", res.AmountAdmin)
	}
	if res.AmountTotal != 103500 {
		t.Errorf("expected AmountTotal 103500, got %d", res.AmountTotal)
	}

	// Verify what was sent to provider
	if mockProv.lastBillReq.Amount != 100000 {
		t.Errorf("expected provider Amount 100000, got %d", mockProv.lastBillReq.Amount)
	}
	if mockProv.lastBillReq.AmountAdmin != 3500 {
		t.Errorf("expected provider AmountAdmin 3500, got %d", mockProv.lastBillReq.AmountAdmin)
	}
	if mockProv.lastBillReq.AmountTotal != 103500 {
		t.Errorf("expected provider AmountTotal 103500, got %d", mockProv.lastBillReq.AmountTotal)
	}

	// Verify DB state
	savedTx, _ := repo.FindByMerchantReff(ctx, nil, "INV-ONLY-AMOUNT-1")
	if savedTx.AmountAdmin != 3500 {
		t.Errorf("expected DB AmountAdmin 3500, got %d", savedTx.AmountAdmin)
	}
	if savedTx.AmountTotal != 103500 {
		t.Errorf("expected DB AmountTotal 103500, got %d", savedTx.AmountTotal)
	}
}

func TestPaymentUseCase_CreatePayment_QRISFeeCalculation(t *testing.T) {
	repo := newInMemoryTxRepo()
	mockProv := &mockProvider{
		billResult: &provider.BillResult{
			Token:       "TOKEN-QRIS",
			PaymentCode: "QRIS-BASE64-DATA",
		},
	}
	val := validator.New()
	uc := NewPaymentUseCase(nil, val, repo, newInMemoryMerchantRepo(), newInMemoryDispatchRepoForPaymentTest(), &mockDispatcher{}, map[string]provider.PaymentProvider{"pay2u": mockProv})

	ctx := context.Background()
	req := &payload.CreatePaymentRequest{
		MerchantReff:  "INV-QRIS-001",
		PaymentMethod: "QRIS-MPM",
		BillTitle:     "Order QRIS MPM",
		CustomerName:  "Siti",
		CustomerPhone: "081298765432",
		Amount:        75000,
	}

	res, err := uc.CreatePayment(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error creating QRIS payment: %v", err)
	}

	// 75000 * 0.70% = 525
	if res.AmountAdmin != 525 {
		t.Errorf("expected AmountAdmin 525, got %d", res.AmountAdmin)
	}
	// 75000 + 525 = 75525
	if res.AmountTotal != 75525 {
		t.Errorf("expected AmountTotal 75525, got %d", res.AmountTotal)
	}
	if mockProv.lastBillReq.AmountTotal != 75525 {
		t.Errorf("expected provider AmountTotal 75525, got %d", mockProv.lastBillReq.AmountTotal)
	}
}

func TestPaymentUseCase_CreatePayment_OverwritesMerchantAdminFee(t *testing.T) {
	repo := newInMemoryTxRepo()
	mockProv := &mockProvider{
		billResult: &provider.BillResult{
			Token:       "TOKEN-OVERWRITE",
			PaymentCode: "12345678",
		},
	}
	val := validator.New()
	uc := NewPaymentUseCase(nil, val, repo, newInMemoryMerchantRepo(), newInMemoryDispatchRepoForPaymentTest(), &mockDispatcher{}, map[string]provider.PaymentProvider{"pay2u": mockProv})

	ctx := context.Background()
	// Merchant attempts to tamper admin fee to 0 and total to 50000
	req := &payload.CreatePaymentRequest{
		MerchantReff:   "INV-TAMPER-1",
		PaymentMethod:  "VA-MITRA",
		BillTitle:      "Order Tamper",
		CustomerName:   "Budi",
		CustomerPhone:  "081234567890",
		Amount:         50000,
		AmountAdmin:    0,
		AmountTotal:    50000,
		AmountDiscount: 1000,
	}

	res, err := uc.CreatePayment(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Paygate overrides AmountAdmin with 3500
	if res.AmountAdmin != 3500 {
		t.Errorf("expected AmountAdmin 3500, got %d", res.AmountAdmin)
	}
	// Total = 50000 + 3500 - 1000 = 52500
	if res.AmountTotal != 52500 {
		t.Errorf("expected AmountTotal 52500, got %d", res.AmountTotal)
	}
	if mockProv.lastBillReq.AmountTotal != 52500 {
		t.Errorf("expected provider AmountTotal 52500, got %d", mockProv.lastBillReq.AmountTotal)
	}
}

