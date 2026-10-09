package usecase

import (
	"context"
	"fmt"
	"strings"
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

func (r *inMemoryTxRepo) FindByMerchantAndReff(ctx context.Context, db *gorm.DB, merchantID uuid.UUID, reff string) (*entity.Transaction, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, tx := range r.transactions {
		if tx.MerchantID != nil && *tx.MerchantID == merchantID && tx.MerchantReff == reff {
			return tx, nil
		}
	}
	return nil, nil
}

func (r *inMemoryTxRepo) FindByMerchantAndID(ctx context.Context, db *gorm.DB, merchantID uuid.UUID, id uuid.UUID) (*entity.Transaction, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	tx, ok := r.transactions[id]
	if !ok || tx.MerchantID == nil || *tx.MerchantID != merchantID {
		return nil, nil
	}
	return tx, nil
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

func (r *inMemoryTxRepo) FindByProviderTokenAndMerchantReff(ctx context.Context, db *gorm.DB, token, reff string) (*entity.Transaction, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, tx := range r.transactions {
		if tx.ProviderToken == token && tx.MerchantReff == reff {
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

func (r *inMemoryTxRepo) ExpirePendingTransactions(ctx context.Context, db *gorm.DB, limit int) ([]*entity.Transaction, error) {
	return nil, nil
}

type inMemoryCacheRepo struct {
	mu    sync.RWMutex
	store map[string][]byte
	locks map[string]bool
}

func newInMemoryCacheRepo() *inMemoryCacheRepo {
	return &inMemoryCacheRepo{
		store: make(map[string][]byte),
		locks: make(map[string]bool),
	}
}

func (c *inMemoryCacheRepo) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.store[key] = value
	return nil
}

func (c *inMemoryCacheRepo) Get(ctx context.Context, key string) ([]byte, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	val, ok := c.store[key]
	if !ok {
		return nil, nil
	}
	return val, nil
}

func (c *inMemoryCacheRepo) Delete(ctx context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.store, key)
	return nil
}

func (c *inMemoryCacheRepo) AcquireLock(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.locks[key] {
		return false, nil
	}
	c.locks[key] = true
	return true, nil
}

func (c *inMemoryCacheRepo) ReleaseLock(ctx context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.locks, key)
	return nil
}

var defaultTestMerchant = &entity.Merchant{
	ID:         uuid.MustParse("00000000-0000-0000-0000-000000000001"),
	Code:       "TEST_MERCHANT",
	Name:       "Test Merchant",
	APIKey:     "test-key",
	WebhookURL: "https://example.com/webhook",
	IsActive:   true,
}

func testContext() context.Context {
	return context.WithValue(context.Background(), constants.LocalsMerchant, defaultTestMerchant)
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

func (r *inMemoryDispatchRepoForPaymentTest) DeleteDispatchesBefore(ctx context.Context, db *gorm.DB, cutoff time.Time, limit int) (int64, error) {
	return 0, nil
}

func (r *inMemoryDispatchRepoForPaymentTest) DeleteDispatchLogsBefore(ctx context.Context, db *gorm.DB, cutoff time.Time, limit int) (int64, error) {
	return 0, nil
}

type mockProvider struct {
	lastBillReq *provider.BillRequest
	billResult  *provider.BillResult
	callbackRes *provider.CallbackResult
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
	if m.callbackRes != nil {
		return m.callbackRes, nil
	}
	return &provider.CallbackResult{
		MerchantReff:  "INV-2026-0001",
		ProviderToken: "TOKEN-MOCK-123",
		Status:        1,
	}, nil
}

func (m *mockProvider) NormalizePaymentMethod(method string) string {
	return strings.ToUpper(strings.TrimSpace(method))
}

func (m *mockProvider) IsValidPaymentMethod(method string) bool {
	switch m.NormalizePaymentMethod(method) {
	case constants.PaymentMethodQRIS, constants.PaymentMethodCC, constants.PaymentMethodVA,
		constants.PaymentMethodVABRI, constants.PaymentMethodVABNI, constants.PaymentMethodVAMandiri,
		constants.PaymentMethodVAPermata, constants.PaymentMethodVACIMB,
		constants.PaymentMethodVABSI, constants.PaymentMethodVABTN, constants.PaymentMethodVADanamon:
		return true
	default:
		return false
	}
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
	uc := NewPaymentUseCase(nil, val, repo, newInMemoryMerchantRepo(), newInMemoryDispatchRepoForPaymentTest(), &mockDispatcher{}, map[string]provider.PaymentProvider{"pay2u": mockProv}, newInMemoryCacheRepo())

	ctx := testContext()
	req := &payload.CreatePaymentRequest{
		MerchantReff:   "INV-2026-0001",
		PaymentMethod:  constants.PaymentMethodVA,
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

func TestPaymentUseCase_CreatePayment_AutoGeneratesMerchantReffWhenEmpty(t *testing.T) {
	repo := newInMemoryTxRepo()
	mockProv := &mockProvider{billResult: &provider.BillResult{Token: "TOKEN-MOCK"}}
	val := validator.New()
	uc := NewPaymentUseCase(nil, val, repo, newInMemoryMerchantRepo(), newInMemoryDispatchRepoForPaymentTest(), &mockDispatcher{}, map[string]provider.PaymentProvider{"pay2u": mockProv}, newInMemoryCacheRepo())

	ctx := testContext()
	req := &payload.CreatePaymentRequest{
		// MerchantReff omitted -> usecase auto-generates
		PaymentMethod: constants.PaymentMethodVA,
		BillTitle:     "Order 1002",
		CustomerName:  "Budi",
		CustomerPhone: "081234567890",
		Amount:        50000,
		AmountTotal:   50000,
	}

	res, err := uc.CreatePayment(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error creating payment: %v", err)
	}
	if res.MerchantReff == "" || res.MerchantReff[0] != 'X' {
		t.Errorf("expected auto-generated MerchantReff starting with X, got %q", res.MerchantReff)
	}
}

func TestPaymentUseCase_CreatePayment_DuplicateMerchantReff(t *testing.T) {
	repo := newInMemoryTxRepo()
	mockProv := &mockProvider{billResult: &provider.BillResult{Token: "TOKEN-MOCK"}}
	val := validator.New()
	uc := NewPaymentUseCase(nil, val, repo, newInMemoryMerchantRepo(), newInMemoryDispatchRepoForPaymentTest(), &mockDispatcher{}, map[string]provider.PaymentProvider{"pay2u": mockProv}, newInMemoryCacheRepo())

	ctx1 := logger.WithRequestID(testContext(), "req-001")
	req1 := &payload.CreatePaymentRequest{
		MerchantReff:  "INV-DUP-TEST",
		PaymentMethod: constants.PaymentMethodVA,
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
	ctx2 := logger.WithRequestID(testContext(), "req-002")
	req2 := &payload.CreatePaymentRequest{
		MerchantReff:  "INV-DUP-TEST",
		PaymentMethod: constants.PaymentMethodVA,
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
	_ = calls
	mockProv := &mockProvider{
		billResult: &provider.BillResult{Token: "TOKEN-MOCK", PaymentCode: "VA-123"},
	}
	val := validator.New()
	uc := NewPaymentUseCase(nil, val, repo, newInMemoryMerchantRepo(), newInMemoryDispatchRepoForPaymentTest(), &mockDispatcher{}, map[string]provider.PaymentProvider{"pay2u": mockProv}, newInMemoryCacheRepo())

	// Same request_id in context simulates HTTP retry with same X-Request-Id
	ctx := logger.WithRequestID(testContext(), "req-replay-same-123")
	req := &payload.CreatePaymentRequest{
		MerchantReff:  "INV-REPLAY-1",
		PaymentMethod: constants.PaymentMethodVA,
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
	uc := NewPaymentUseCase(nil, val, repo, newInMemoryMerchantRepo(), newInMemoryDispatchRepoForPaymentTest(), &mockDispatcher{}, map[string]provider.PaymentProvider{"pay2u": mockProv}, newInMemoryCacheRepo())

	ctx := testContext()
	req := &payload.CreatePaymentRequest{
		MerchantReff:  "INV-2026-0001",
		PaymentMethod: constants.PaymentMethodVA,
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

	uc := NewPaymentUseCase(nil, val, repo, merchantRepo, dispatchRepo, dispatcher, map[string]provider.PaymentProvider{"pay2u": mockProv}, newInMemoryCacheRepo())

	ctx := context.WithValue(context.Background(), constants.LocalsMerchant, merchant)
	req := &payload.CreatePaymentRequest{
		MerchantReff:  "INV-WH-001",
		PaymentMethod: constants.PaymentMethodVA,
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

	uc := NewPaymentUseCase(nil, val, repo, merchantRepo, dispatchRepo, dispatcher, map[string]provider.PaymentProvider{"pay2u": mockProv}, newInMemoryCacheRepo())

	ctx := context.WithValue(context.Background(), constants.LocalsMerchant, merchant)
	req := &payload.CreatePaymentRequest{
		MerchantReff:  "ref-1790222645904-5585",
		PaymentMethod: constants.PaymentMethodQRIS,
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
	uc := NewPaymentUseCase(nil, val, repo, newInMemoryMerchantRepo(), newInMemoryDispatchRepoForPaymentTest(), &mockDispatcher{}, map[string]provider.PaymentProvider{"pay2u": mockProv}, newInMemoryCacheRepo())

	ctx := testContext()
	// Merchant only provides amount, no amount_admin and no amount_total
	req := &payload.CreatePaymentRequest{
		MerchantReff:   "INV-ONLY-AMOUNT-1",
		PaymentMethod:  constants.PaymentMethodVAMandiri,
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
	uc := NewPaymentUseCase(nil, val, repo, newInMemoryMerchantRepo(), newInMemoryDispatchRepoForPaymentTest(), &mockDispatcher{}, map[string]provider.PaymentProvider{"pay2u": mockProv}, newInMemoryCacheRepo())

	ctx := testContext()
	req := &payload.CreatePaymentRequest{
		MerchantReff:  "INV-QRIS-001",
		PaymentMethod: constants.PaymentMethodQRIS,
		BillTitle:     "Order QRIS",
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
	uc := NewPaymentUseCase(nil, val, repo, newInMemoryMerchantRepo(), newInMemoryDispatchRepoForPaymentTest(), &mockDispatcher{}, map[string]provider.PaymentProvider{"pay2u": mockProv}, newInMemoryCacheRepo())

	ctx := testContext()
	// Merchant attempts to tamper admin fee to 0 and total to 50000
	req := &payload.CreatePaymentRequest{
		MerchantReff:   "INV-TAMPER-1",
		PaymentMethod:  constants.PaymentMethodVA,
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

func TestPaymentUseCase_HandleCallback_RefundFromSuccess(t *testing.T) {
	repo := newInMemoryTxRepo()
	mockProv := &mockProvider{
		billResult: &provider.BillResult{Token: "TOKEN-MOCK-REFUND"},
	}
	val := validator.New()
	merchantRepo := newInMemoryMerchantRepo()
	dispatchRepo := newInMemoryDispatchRepoForPaymentTest()
	dispatcher := &mockDispatcher{}

	merchantID := uuid.New()
	merchant := &entity.Merchant{
		ID:         merchantID,
		Code:       "M-REFUND",
		Name:       "Refund Merchant",
		IsActive:   true,
		WebhookURL: "https://merchant.example.com/webhook",
	}
	_ = merchantRepo.Create(context.Background(), nil, merchant)

	uc := NewPaymentUseCase(nil, val, repo, merchantRepo, dispatchRepo, dispatcher, map[string]provider.PaymentProvider{"pay2u": mockProv}, newInMemoryCacheRepo())

	ctx := context.WithValue(context.Background(), constants.LocalsMerchant, merchant)
	req := &payload.CreatePaymentRequest{
		MerchantReff:  "INV-REFUND-001",
		PaymentMethod: constants.PaymentMethodVA,
		BillTitle:     "Refund Test Order",
		CustomerName:  "Eldiva",
		CustomerPhone: "081234567890",
		Amount:        100000,
		AmountTotal:   100000,
	}

	res, err := uc.CreatePayment(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error creating payment: %v", err)
	}

	// 1. Initial Callback: Status 1 (SUCCESS)
	mockProv.callbackRes = &provider.CallbackResult{
		MerchantReff:  "INV-REFUND-001",
		ProviderToken: "TOKEN-MOCK-REFUND",
		PaymentReff:   "PAYREFF-SUCCESS-1",
		PaymentDate:   "2026-09-28 10:00:00",
		Status:        1,
	}
	if err := uc.HandleCallback(ctx, "pay2u", []byte(`{"status":1}`)); err != nil {
		t.Fatalf("unexpected error handling success callback: %v", err)
	}

	txID, _ := uuid.Parse(res.ID)
	tx, _ := repo.FindByID(ctx, nil, txID)
	if tx.Status != constants.TransactionStatusSuccess {
		t.Fatalf("expected status SUCCESS, got %s", tx.Status)
	}
	if len(dispatcher.enqueued) != 1 {
		t.Fatalf("expected 1 enqueued dispatch for success, got %d", len(dispatcher.enqueued))
	}

	// 2. Subsequent Callback: Status 3 (REFUND)
	mockProv.callbackRes = &provider.CallbackResult{
		MerchantReff:  "INV-REFUND-001",
		ProviderToken: "TOKEN-MOCK-REFUND",
		PaymentReff:   "PAYREFF-REFUND-99",
		PaymentDate:   "2026-09-28 11:00:00",
		Status:        3,
	}
	if err := uc.HandleCallback(ctx, "pay2u", []byte(`{"status":3}`)); err != nil {
		t.Fatalf("unexpected error handling refund callback: %v", err)
	}

	tx, _ = repo.FindByID(ctx, nil, txID)
	if tx.Status != constants.TransactionStatusRefund {
		t.Fatalf("expected status REFUND, got %s", tx.Status)
	}
	if tx.PaymentReff != "PAYREFF-REFUND-99" {
		t.Errorf("expected updated payment_reff PAYREFF-REFUND-99, got %s", tx.PaymentReff)
	}

	// Webhook should now have 2 enqueued dispatches (1 success, 1 refund)
	if len(dispatcher.enqueued) != 2 {
		t.Fatalf("expected 2 enqueued dispatches, got %d", len(dispatcher.enqueued))
	}

	refundDispatchID := dispatcher.enqueued[1]
	refundDispatch, err := dispatchRepo.FindDispatchByID(ctx, nil, refundDispatchID)
	if err != nil || refundDispatch == nil {
		t.Fatalf("failed to find refund dispatch: %v", err)
	}
	if refundDispatch.EventType != constants.WebhookEventPaymentRefund {
		t.Errorf("expected EventType %q, got %q", constants.WebhookEventPaymentRefund, refundDispatch.EventType)
	}
}

func TestPaymentUseCase_HandleCallback_Idempotency(t *testing.T) {
	repo := newInMemoryTxRepo()
	mockProv := &mockProvider{
		billResult: &provider.BillResult{Token: "TOKEN-IDEM"},
	}
	val := validator.New()
	merchantRepo := newInMemoryMerchantRepo()
	dispatchRepo := newInMemoryDispatchRepoForPaymentTest()
	dispatcher := &mockDispatcher{}

	merchantID := uuid.New()
	merchant := &entity.Merchant{
		ID:         merchantID,
		Code:       "M-IDEM",
		Name:       "Idempotent Merchant",
		IsActive:   true,
		WebhookURL: "https://merchant.example.com/webhook",
	}
	_ = merchantRepo.Create(context.Background(), nil, merchant)

	uc := NewPaymentUseCase(nil, val, repo, merchantRepo, dispatchRepo, dispatcher, map[string]provider.PaymentProvider{"pay2u": mockProv}, newInMemoryCacheRepo())

	ctx := context.WithValue(context.Background(), constants.LocalsMerchant, merchant)
	req := &payload.CreatePaymentRequest{
		MerchantReff:  "INV-IDEM-001",
		PaymentMethod: constants.PaymentMethodVA,
		BillTitle:     "Idempotent Test",
		CustomerName:  "Eldiva",
		CustomerPhone: "081234567890",
		Amount:        50000,
		AmountTotal:   50000,
	}

	_, err := uc.CreatePayment(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error creating payment: %v", err)
	}

	mockProv.callbackRes = &provider.CallbackResult{
		MerchantReff:  "INV-IDEM-001",
		ProviderToken: "TOKEN-IDEM",
		Status:        1,
	}

	// 1st callback -> triggers 1 dispatch
	if err := uc.HandleCallback(ctx, "pay2u", []byte(`{}`)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(dispatcher.enqueued) != 1 {
		t.Fatalf("expected 1 dispatch, got %d", len(dispatcher.enqueued))
	}

	// 2nd duplicate callback -> must be ignored (idempotent)
	if err := uc.HandleCallback(ctx, "pay2u", []byte(`{}`)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(dispatcher.enqueued) != 1 {
		t.Fatalf("expected still 1 dispatch after duplicate callback, got %d", len(dispatcher.enqueued))
	}
}

func TestPaymentUseCase_HandleCallback_LatePaymentFromExpired(t *testing.T) {
	repo := newInMemoryTxRepo()
	mockProv := &mockProvider{
		billResult: &provider.BillResult{Token: "TOKEN-LATE"},
	}
	val := validator.New()
	merchantRepo := newInMemoryMerchantRepo()
	dispatchRepo := newInMemoryDispatchRepoForPaymentTest()
	dispatcher := &mockDispatcher{}

	merchantID := uuid.New()
	merchant := &entity.Merchant{
		ID:         merchantID,
		Code:       "M-LATE",
		Name:       "Late Merchant",
		IsActive:   true,
		WebhookURL: "https://merchant.example.com/webhook",
	}
	_ = merchantRepo.Create(context.Background(), nil, merchant)

	uc := NewPaymentUseCase(nil, val, repo, merchantRepo, dispatchRepo, dispatcher, map[string]provider.PaymentProvider{"pay2u": mockProv}, newInMemoryCacheRepo())

	ctx := context.WithValue(context.Background(), constants.LocalsMerchant, merchant)
	req := &payload.CreatePaymentRequest{
		MerchantReff:  "INV-LATE-001",
		PaymentMethod: constants.PaymentMethodVA,
		BillTitle:     "Late Payment Test",
		CustomerName:  "Eldiva",
		CustomerPhone: "081234567890",
		Amount:        60000,
		AmountTotal:   60000,
	}

	res, err := uc.CreatePayment(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error creating payment: %v", err)
	}

	txID, _ := uuid.Parse(res.ID)
	// Manually simulate transaction marked as EXPIRED by background worker
	tx, _ := repo.FindByID(ctx, nil, txID)
	tx.Status = constants.TransactionStatusExpired

	// Callback status 1 arrives from Pay2U
	mockProv.callbackRes = &provider.CallbackResult{
		MerchantReff:  "INV-LATE-001",
		ProviderToken: "TOKEN-LATE",
		PaymentReff:   "PAYREFF-LATE-99",
		Status:        1,
	}

	if err := uc.HandleCallback(ctx, "pay2u", []byte(`{}`)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tx, _ = repo.FindByID(ctx, nil, txID)
	if tx.Status != constants.TransactionStatusSuccess {
		t.Fatalf("expected EXPIRED transaction to transition to SUCCESS on payment callback, got %s", tx.Status)
	}
	if len(dispatcher.enqueued) != 1 {
		t.Fatalf("expected 1 webhook dispatch enqueued, got %d", len(dispatcher.enqueued))
	}
}

func TestPaymentUseCase_RefreshPayment_RefundDetection(t *testing.T) {
	repo := newInMemoryTxRepo()
	mockProv := &mockProvider{
		billResult: &provider.BillResult{
			Token:        "TOKEN-REFRESH-REFUND",
			MerchantReff: "INV-REFRESH-001",
			Status:       3, // Pay2U reports refund
			PaymentReff:  "PAYREFF-REFUND-INQUIRY",
		},
	}
	val := validator.New()
	merchantRepo := newInMemoryMerchantRepo()
	dispatchRepo := newInMemoryDispatchRepoForPaymentTest()
	dispatcher := &mockDispatcher{}

	merchantID := uuid.New()
	merchant := &entity.Merchant{
		ID:         merchantID,
		Code:       "M-REFRESH-REFUND",
		Name:       "Refresh Refund Merchant",
		IsActive:   true,
		WebhookURL: "https://merchant.example.com/webhook",
	}
	_ = merchantRepo.Create(context.Background(), nil, merchant)

	uc := NewPaymentUseCase(nil, val, repo, merchantRepo, dispatchRepo, dispatcher, map[string]provider.PaymentProvider{"pay2u": mockProv}, newInMemoryCacheRepo())

	ctx := context.WithValue(context.Background(), constants.LocalsMerchant, merchant)
	req := &payload.CreatePaymentRequest{
		MerchantReff:  "INV-REFRESH-001",
		PaymentMethod: constants.PaymentMethodVA,
		BillTitle:     "Refresh Refund Test",
		CustomerName:  "Eldiva",
		CustomerPhone: "081234567890",
		Amount:        45000,
		AmountTotal:   45000,
	}

	res, err := uc.CreatePayment(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error creating payment: %v", err)
	}

	// Transaction initially SUCCESS
	txID, _ := uuid.Parse(res.ID)
	tx, _ := repo.FindByID(ctx, nil, txID)
	tx.Status = constants.TransactionStatusSuccess

	// Refresh payment calls GetBill -> reports status 3 -> triggers refund update
	resp, err := uc.RefreshPayment(ctx, "INV-REFRESH-001")
	if err != nil {
		t.Fatalf("unexpected error on refresh payment: %v", err)
	}

	if resp.Status != constants.TransactionStatusRefund {
		t.Fatalf("expected refreshed status REFUND, got %s", resp.Status)
	}

	tx, _ = repo.FindByID(ctx, nil, txID)
	if tx.Status != constants.TransactionStatusRefund {
		t.Fatalf("expected db status REFUND, got %s", tx.Status)
	}

	if len(dispatcher.enqueued) != 1 {
		t.Fatalf("expected 1 webhook dispatch for refund, got %d", len(dispatcher.enqueued))
	}
	dispatch, _ := dispatchRepo.FindDispatchByID(ctx, nil, dispatcher.enqueued[0])
	if dispatch.EventType != constants.WebhookEventPaymentRefund {
		t.Errorf("expected EventType %q, got %q", constants.WebhookEventPaymentRefund, dispatch.EventType)
	}
}

func TestPaymentUseCase_CreatePayment_StandardPaymentMethods(t *testing.T) {
	repo := newInMemoryTxRepo()
	mockProv := &mockProvider{
		billResult: &provider.BillResult{Token: "TOKEN-MOCK-STD"},
	}
	val := validator.New()
	uc := NewPaymentUseCase(nil, val, repo, newInMemoryMerchantRepo(), newInMemoryDispatchRepoForPaymentTest(), &mockDispatcher{}, map[string]provider.PaymentProvider{"pay2u": mockProv}, newInMemoryCacheRepo())

	ctx := testContext()

	methods := []string{
		constants.PaymentMethodQRIS,
		constants.PaymentMethodCC,
		constants.PaymentMethodVA,
		constants.PaymentMethodVABRI,
		constants.PaymentMethodVABNI,
		constants.PaymentMethodVAMandiri,
	}

	for i, method := range methods {
		req := &payload.CreatePaymentRequest{
			MerchantReff:  fmt.Sprintf("INV-STD-%d", i),
			PaymentMethod: method,
			BillTitle:     "Standard Method Test",
			CustomerName:  "Budi",
			CustomerPhone: "081234567890",
			Amount:        50000,
			AmountTotal:   50000,
		}

		res, err := uc.CreatePayment(ctx, req)
		if err != nil {
			t.Fatalf("unexpected error creating payment with method %s: %v", method, err)
		}
		if res.PaymentMethod != method {
			t.Errorf("expected PaymentMethod %q, got %q", method, res.PaymentMethod)
		}
	}
}

func TestPaymentUseCase_CreatePayment_CaseInsensitive(t *testing.T) {
	repo := newInMemoryTxRepo()
	mockProv := &mockProvider{
		billResult: &provider.BillResult{Token: "TOKEN-CASE"},
	}
	val := validator.New()
	uc := NewPaymentUseCase(nil, val, repo, newInMemoryMerchantRepo(), newInMemoryDispatchRepoForPaymentTest(), &mockDispatcher{}, map[string]provider.PaymentProvider{"pay2u": mockProv}, newInMemoryCacheRepo())

	ctx := testContext()

	req := &payload.CreatePaymentRequest{
		MerchantReff:  "INV-CASE-001",
		PaymentMethod: "qris", // lowercase
		BillTitle:     "Case Insensitive Test",
		CustomerName:  "Budi",
		CustomerPhone: "081234567890",
		Amount:        50000,
		AmountTotal:   50000,
	}

	res, err := uc.CreatePayment(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.PaymentMethod != "QRIS" {
		t.Errorf("expected normalized PaymentMethod 'QRIS', got %q", res.PaymentMethod)
	}
}

func TestPaymentUseCase_CreatePayment_InvalidPaymentMethod_Rejected(t *testing.T) {
	repo := newInMemoryTxRepo()
	mockProv := &mockProvider{
		billResult: &provider.BillResult{Token: "TOKEN-MOCK-INVALID"},
	}
	val := validator.New()
	uc := NewPaymentUseCase(nil, val, repo, newInMemoryMerchantRepo(), newInMemoryDispatchRepoForPaymentTest(), &mockDispatcher{}, map[string]provider.PaymentProvider{"pay2u": mockProv}, newInMemoryCacheRepo())

	ctx := testContext()

	invalidMethods := []string{
		"QRIS-MITRA",
		"VA-MITRA",
		"CC-MITRA",
		"BRIVA-MITRA",
		"BNIVA-MITRA",
		"BRI",
		"BNI",
		"MANDIRI",
		"BCA",
		"BITCOIN",
		"PAYPAL",
		"UNKNOWN_METHOD",
	}

	for _, method := range invalidMethods {
		req := &payload.CreatePaymentRequest{
			MerchantReff:  fmt.Sprintf("INV-INVALID-%s", method),
			PaymentMethod: method,
			BillTitle:     "Invalid Method Test",
			CustomerName:  "Budi",
			CustomerPhone: "081234567890",
			Amount:        50000,
			AmountTotal:   50000,
		}

		_, err := uc.CreatePayment(ctx, req)
		if err == nil {
			t.Errorf("expected error for invalid payment method %q, got nil", method)
			continue
		}

		appErr, ok := exception.As(err)
		if !ok || appErr.Code != exception.CodeValidation {
			t.Errorf("expected CodeValidation for method %q, got: %v", method, err)
		}
	}
}

func TestPaymentUseCase_GetPayment_IDOR_Isolation(t *testing.T) {
	repo := newInMemoryTxRepo()
	val := validator.New()
	uc := NewPaymentUseCase(nil, val, repo, newInMemoryMerchantRepo(), newInMemoryDispatchRepoForPaymentTest(), &mockDispatcher{}, map[string]provider.PaymentProvider{}, newInMemoryCacheRepo())

	merchantA := &entity.Merchant{
		ID:   uuid.New(),
		Code: "MERCHANT_A",
		Name: "Merchant A",
	}
	merchantB := &entity.Merchant{
		ID:   uuid.New(),
		Code: "MERCHANT_B",
		Name: "Merchant B",
	}

	txA := &entity.Transaction{
		ID:           uuid.New(),
		MerchantID:   &merchantA.ID,
		MerchantReff: "INV-TENANT-A",
		AmountTotal:  50000,
		Status:       constants.TransactionStatusPending,
	}
	_ = repo.Create(context.Background(), nil, txA)

	ctxA := context.WithValue(context.Background(), constants.LocalsMerchant, merchantA)
	ctxB := context.WithValue(context.Background(), constants.LocalsMerchant, merchantB)

	// Merchant A queries own transaction by Reff -> Success
	resA, err := uc.GetPayment(ctxA, "INV-TENANT-A")
	if err != nil {
		t.Fatalf("expected merchant A to find own transaction, got err: %v", err)
	}
	if resA.MerchantReff != "INV-TENANT-A" {
		t.Errorf("expected MerchantReff %q, got %q", "INV-TENANT-A", resA.MerchantReff)
	}

	// Merchant A queries own transaction by UUID -> Success
	resAUUID, err := uc.GetPayment(ctxA, txA.ID.String())
	if err != nil {
		t.Fatalf("expected merchant A to find own transaction by UUID, got err: %v", err)
	}
	if resAUUID.ID != txA.ID.String() {
		t.Errorf("expected transaction ID %q, got %q", txA.ID.String(), resAUUID.ID)
	}

	// Merchant B queries Merchant A's transaction by Reff -> 404 Not Found (IDOR Blocked)
	_, err = uc.GetPayment(ctxB, "INV-TENANT-A")
	if err == nil {
		t.Fatal("expected IDOR protection to return 404 for Merchant B querying Merchant A's transaction by Reff")
	}
	appErr, ok := exception.As(err)
	if !ok || appErr.Code != exception.CodeNotFound {
		t.Errorf("expected CodeNotFound, got %v", err)
	}

	// Merchant B queries Merchant A's transaction by UUID -> 404 Not Found (IDOR Blocked)
	_, err = uc.GetPayment(ctxB, txA.ID.String())
	if err == nil {
		t.Fatal("expected IDOR protection to return 404 for Merchant B querying Merchant A's transaction by UUID")
	}
	appErr, ok = exception.As(err)
	if !ok || appErr.Code != exception.CodeNotFound {
		t.Errorf("expected CodeNotFound, got %v", err)
	}
}

func TestPaymentUseCase_RefreshPayment_IDOR_Isolation(t *testing.T) {
	repo := newInMemoryTxRepo()
	val := validator.New()
	mockProv := &mockProvider{
		billResult: &provider.BillResult{Status: 1},
	}
	uc := NewPaymentUseCase(nil, val, repo, newInMemoryMerchantRepo(), newInMemoryDispatchRepoForPaymentTest(), &mockDispatcher{}, map[string]provider.PaymentProvider{"pay2u": mockProv}, newInMemoryCacheRepo())

	merchantA := &entity.Merchant{
		ID:   uuid.New(),
		Code: "MERCHANT_A",
		Name: "Merchant A",
	}
	merchantB := &entity.Merchant{
		ID:   uuid.New(),
		Code: "MERCHANT_B",
		Name: "Merchant B",
	}

	txA := &entity.Transaction{
		ID:            uuid.New(),
		MerchantID:    &merchantA.ID,
		MerchantReff:  "INV-REFRESH-A",
		Provider:      "pay2u",
		ProviderToken: "TOKEN-A",
		Status:        constants.TransactionStatusPending,
	}
	_ = repo.Create(context.Background(), nil, txA)

	ctxB := context.WithValue(context.Background(), constants.LocalsMerchant, merchantB)

	// Merchant B attempts to refresh Merchant A's transaction -> 404 Not Found
	_, err := uc.RefreshPayment(ctxB, "INV-REFRESH-A")
	if err == nil {
		t.Fatal("expected IDOR protection to block RefreshPayment across tenants")
	}
	appErr, ok := exception.As(err)
	if !ok || appErr.Code != exception.CodeNotFound {
		t.Errorf("expected CodeNotFound, got %v", err)
	}
}

func TestPaymentUseCase_CreatePayment_Concurrency_Lock(t *testing.T) {
	repo := newInMemoryTxRepo()
	mockProv := &mockProvider{
		billResult: &provider.BillResult{Token: "TOKEN-CONCURRENCY", PaymentCode: "VA-CONC"},
	}
	val := validator.New()
	cache := newInMemoryCacheRepo()
	uc := NewPaymentUseCase(nil, val, repo, newInMemoryMerchantRepo(), newInMemoryDispatchRepoForPaymentTest(), &mockDispatcher{}, map[string]provider.PaymentProvider{"pay2u": mockProv}, cache)

	merchant := &entity.Merchant{
		ID:   uuid.New(),
		Code: "MERCHANT_CONC",
		Name: "Merchant Concurrency",
	}
	ctx := context.WithValue(context.Background(), constants.LocalsMerchant, merchant)

	// Simulate lock already acquired by an in-flight request
	lockKey := fmt.Sprintf("paygate:lock:payment:%s:%s", merchant.ID.String(), "INV-RACE-001")
	_, _ = cache.AcquireLock(ctx, lockKey, 30*time.Second)

	req := &payload.CreatePaymentRequest{
		MerchantReff:  "INV-RACE-001",
		PaymentMethod: constants.PaymentMethodVA,
		BillTitle:     "Order Race",
		CustomerName:  "Budi",
		CustomerPhone: "081234567890",
		Amount:        50000,
	}

	// Request should hit concurrency detection and return Conflict after poll timeout
	start := time.Now()
	_, err := uc.CreatePayment(ctx, req)
	duration := time.Since(start)

	if err == nil {
		t.Fatal("expected conflict error due to active lock, got nil")
	}
	appErr, ok := exception.As(err)
	if !ok || appErr.Code != exception.CodeConflict {
		t.Errorf("expected CodeConflict, got %v", err)
	}
	if duration < 1800*time.Millisecond {
		t.Errorf("expected polling wait of around 2s, but took %v", duration)
	}
}
