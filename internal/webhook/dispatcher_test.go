package webhook

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"paygate/internal/entity"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// In-memory mock repository for WebhookDispatch
type inMemoryDispatchRepo struct {
	mu         sync.RWMutex
	dispatches map[uuid.UUID]*entity.WebhookDispatch
	logs       []*entity.WebhookDispatchLog
}

func newInMemoryDispatchRepo() *inMemoryDispatchRepo {
	return &inMemoryDispatchRepo{
		dispatches: make(map[uuid.UUID]*entity.WebhookDispatch),
		logs:       make([]*entity.WebhookDispatchLog, 0),
	}
}

func (r *inMemoryDispatchRepo) CreateDispatch(ctx context.Context, db *gorm.DB, d *entity.WebhookDispatch) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.dispatches[d.ID] = d
	return nil
}

func (r *inMemoryDispatchRepo) ClaimDispatch(ctx context.Context, db *gorm.DB, id uuid.UUID, workerID string, lockDuration time.Duration) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	d, ok := r.dispatches[id]
	if !ok {
		return false, nil
	}
	now := time.Now()
	if d.Status != entity.WebhookStatusPending {
		return false, nil
	}
	if d.LockedUntil != nil && d.LockedUntil.After(now) {
		return false, nil
	}

	expiry := now.Add(lockDuration)
	d.LockedUntil = &expiry
	d.LockedBy = &workerID
	d.UpdatedAt = now
	return true, nil
}

func (r *inMemoryDispatchRepo) ReleaseDispatch(ctx context.Context, db *gorm.DB, id uuid.UUID, status string, attempts int, nextRetry *time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	d, ok := r.dispatches[id]
	if !ok {
		return errors.New("dispatch not found")
	}
	d.Status = status
	d.Attempts = attempts
	d.NextRetryAt = nextRetry
	d.LockedUntil = nil
	d.LockedBy = nil
	d.UpdatedAt = time.Now()
	return nil
}

func (r *inMemoryDispatchRepo) CreateDispatchLog(ctx context.Context, db *gorm.DB, l *entity.WebhookDispatchLog) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.logs = append(r.logs, l)
	return nil
}

func (r *inMemoryDispatchRepo) FindDispatchByID(ctx context.Context, db *gorm.DB, id uuid.UUID) (*entity.WebhookDispatch, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	d, ok := r.dispatches[id]
	if !ok {
		return nil, nil
	}
	return d, nil
}

func (r *inMemoryDispatchRepo) FindRecoverableDispatches(ctx context.Context, db *gorm.DB, limit int) ([]*entity.WebhookDispatch, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	now := time.Now()
	var res []*entity.WebhookDispatch
	for _, d := range r.dispatches {
		if d.Status == entity.WebhookStatusPending {
			if (d.NextRetryAt == nil || !d.NextRetryAt.After(now)) && (d.LockedUntil == nil || !d.LockedUntil.After(now)) {
				res = append(res, d)
				if len(res) >= limit {
					break
				}
			}
		}
	}
	return res, nil
}

// 1. Policy Tests
func TestPolicy_IsSuccess(t *testing.T) {
	tests := []struct {
		code     int
		expected bool
	}{
		{200, true},
		{201, true},
		{204, true},
		{299, true},
		{301, false},
		{400, false},
		{500, false},
	}
	for _, tc := range tests {
		if got := IsSuccess(tc.code); got != tc.expected {
			t.Errorf("IsSuccess(%d) = %v, expected %v", tc.code, got, tc.expected)
		}
	}
}

func TestPolicy_IsRetryable(t *testing.T) {
	// Network errors are retryable
	if !IsRetryable(0, errors.New("connection timeout")) {
		t.Error("expected network error to be retryable")
	}

	// 5xx and specific 4xx (408, 429) are retryable
	retryableCodes := []int{408, 429, 500, 502, 503, 504}
	for _, code := range retryableCodes {
		if !IsRetryable(code, nil) {
			t.Errorf("expected status %d to be retryable", code)
		}
	}

	// General 4xx are non-retryable
	nonRetryableCodes := []int{400, 401, 403, 404, 405, 410, 422}
	for _, code := range nonRetryableCodes {
		if IsRetryable(code, nil) {
			t.Errorf("expected status %d to be non-retryable", code)
		}
	}
}

func TestPolicy_ParseRetryAfter(t *testing.T) {
	// Seconds parsing
	dur, ok := ParseRetryAfter("30")
	if !ok || dur != 30*time.Second {
		t.Errorf("expected 30s, got %v (ok=%v)", dur, ok)
	}

	// Minimum clamp (0 or negative seconds)
	dur, ok = ParseRetryAfter("0")
	if !ok || dur != MinRetryAfterDuration {
		t.Errorf("expected min duration %v, got %v", MinRetryAfterDuration, dur)
	}

	// Maximum clamp (> 5m)
	dur, ok = ParseRetryAfter("99999")
	if !ok || dur != MaxRetryAfterDuration {
		t.Errorf("expected max duration %v, got %v", MaxRetryAfterDuration, dur)
	}

	// Invalid input
	_, ok = ParseRetryAfter("invalid")
	if ok {
		t.Error("expected invalid Retry-After to return ok=false")
	}
}

// 2. Signer Test
func TestSigner_ComputeSignature(t *testing.T) {
	payload := []byte(`{"event":"payment.success","amount":50000}`)
	secret := "whsec_test_secret_123"

	sig := ComputeSignature(payload, secret)
	if sig == "" {
		t.Fatal("expected non-empty HMAC signature")
	}

	// Same payload + secret must produce exact same signature (deterministic)
	sig2 := ComputeSignature(payload, secret)
	if sig != sig2 {
		t.Errorf("expected signature to be deterministic, got %s vs %s", sig, sig2)
	}

	// Empty secret returns empty signature
	if sigEmpty := ComputeSignature(payload, ""); sigEmpty != "" {
		t.Errorf("expected empty signature for empty secret, got %s", sigEmpty)
	}
}

// 3. Dispatcher Execution Tests
func TestDispatcher_Execute_Success2xx(t *testing.T) {
	var receivedDeliveryID, receivedSignature string
	var receivedBody string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedDeliveryID = r.Header.Get("X-Paygate-Delivery-Id")
		receivedSignature = r.Header.Get("X-Paygate-Signature")
		body, _ := io.ReadAll(r.Body)
		receivedBody = string(body)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"received"}`))
	}))
	defer server.Close()

	repo := newInMemoryDispatchRepo()
	logger := logrus.New()
	logger.SetOutput(io.Discard)

	d := NewDispatcher(nil, nil, repo, logger).(*dispatcher)

	dispatchID := uuid.New()
	secret := "whsec_super_secret"
	payload := []byte(`{"order_id":"123","status":"SUCCESS"}`)

	dispatch := &entity.WebhookDispatch{
		ID:            dispatchID,
		TransactionID: uuid.New(),
		MerchantID:    uuid.New(),
		Merchant: &entity.Merchant{
			WebhookSecret: secret,
		},
		TargetURL:   server.URL,
		EventType:   "payment.success",
		Payload:     entity.JSONB(payload),
		Status:      entity.WebhookStatusPending,
		Attempts:    0,
		MaxAttempts: 3,
	}
	repo.dispatches[dispatchID] = dispatch

	d.safeExecute(dispatchID)

	repo.mu.RLock()
	defer repo.mu.RUnlock()

	// 1. Verify status changed to SUCCESS
	if dispatch.Status != entity.WebhookStatusSuccess {
		t.Errorf("expected status %s, got %s", entity.WebhookStatusSuccess, dispatch.Status)
	}
	if dispatch.Attempts != 1 {
		t.Errorf("expected attempts 1, got %d", dispatch.Attempts)
	}

	// 2. Verify Delivery ID and HMAC headers
	if receivedDeliveryID != dispatchID.String() {
		t.Errorf("expected delivery ID %s, got %s", dispatchID.String(), receivedDeliveryID)
	}
	expectedSig := "sha256=" + ComputeSignature(payload, secret)
	if receivedSignature != expectedSig {
		t.Errorf("expected signature %s, got %s", expectedSig, receivedSignature)
	}
	if receivedBody != string(payload) {
		t.Errorf("expected body %s, got %s", string(payload), receivedBody)
	}

	// 3. Verify attempt log recorded
	if len(repo.logs) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(repo.logs))
	}
	if *repo.logs[0].HTTPStatus != 200 {
		t.Errorf("expected log status 200, got %d", *repo.logs[0].HTTPStatus)
	}
}

func TestDispatcher_Execute_NonRetryable4xx(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest) // 400 Bad Request
		w.Write([]byte(`{"error":"bad request"}`))
	}))
	defer server.Close()

	repo := newInMemoryDispatchRepo()
	logger := logrus.New()
	logger.SetOutput(io.Discard)

	d := NewDispatcher(nil, nil, repo, logger).(*dispatcher)

	dispatchID := uuid.New()
	dispatch := &entity.WebhookDispatch{
		ID:            dispatchID,
		TransactionID: uuid.New(),
		MerchantID:    uuid.New(),
		TargetURL:     server.URL,
		EventType:     "payment.success",
		Payload:       entity.JSONB(`{}`),
		Status:        entity.WebhookStatusPending,
		Attempts:      0,
		MaxAttempts:   3,
	}
	repo.dispatches[dispatchID] = dispatch

	d.safeExecute(dispatchID)

	repo.mu.RLock()
	defer repo.mu.RUnlock()

	// Non-retryable 4xx should immediately mark as FAILED without next_retry_at
	if dispatch.Status != entity.WebhookStatusFailed {
		t.Errorf("expected status %s, got %s", entity.WebhookStatusFailed, dispatch.Status)
	}
	if dispatch.Attempts != 1 {
		t.Errorf("expected attempts 1, got %d", dispatch.Attempts)
	}
	if dispatch.NextRetryAt != nil {
		t.Errorf("expected NextRetryAt to be nil, got %v", dispatch.NextRetryAt)
	}
}

func TestDispatcher_Execute_RetryableBackoffAndStop3x(t *testing.T) {
	attemptsReceived := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attemptsReceived++
		w.WriteHeader(http.StatusInternalServerError) // 500
		w.Write([]byte(`{"error":"internal server error"}`))
	}))
	defer server.Close()

	repo := newInMemoryDispatchRepo()
	logger := logrus.New()
	logger.SetOutput(io.Discard)

	d := NewDispatcher(nil, nil, repo, logger).(*dispatcher)

	dispatchID := uuid.New()
	dispatch := &entity.WebhookDispatch{
		ID:            dispatchID,
		TransactionID: uuid.New(),
		MerchantID:    uuid.New(),
		TargetURL:     server.URL,
		EventType:     "payment.success",
		Payload:       entity.JSONB(`{}`),
		Status:        entity.WebhookStatusPending,
		Attempts:      0,
		MaxAttempts:   3,
	}
	repo.dispatches[dispatchID] = dispatch

	// Attempt 1: Fails -> remains PENDING, NextRetryAt set
	d.safeExecute(dispatchID)
	if dispatch.Status != entity.WebhookStatusPending {
		t.Errorf("attempt 1: expected status %s, got %s", entity.WebhookStatusPending, dispatch.Status)
	}
	if dispatch.Attempts != 1 {
		t.Errorf("attempt 1: expected attempts 1, got %d", dispatch.Attempts)
	}
	if dispatch.NextRetryAt == nil {
		t.Error("attempt 1: expected NextRetryAt to be set")
	}

	// Attempt 2: Fails -> remains PENDING, NextRetryAt set
	d.safeExecute(dispatchID)
	if dispatch.Status != entity.WebhookStatusPending {
		t.Errorf("attempt 2: expected status %s, got %s", entity.WebhookStatusPending, dispatch.Status)
	}
	if dispatch.Attempts != 2 {
		t.Errorf("attempt 2: expected attempts 2, got %d", dispatch.Attempts)
	}

	// Attempt 3: Fails -> status changes to FAILED, STOP!
	d.safeExecute(dispatchID)
	if dispatch.Status != entity.WebhookStatusFailed {
		t.Errorf("attempt 3: expected status %s, got %s", entity.WebhookStatusFailed, dispatch.Status)
	}
	if dispatch.Attempts != 3 {
		t.Errorf("attempt 3: expected attempts 3, got %d", dispatch.Attempts)
	}

	// Total HTTP requests should be 3
	if attemptsReceived != 3 {
		t.Errorf("expected 3 server hits, got %d", attemptsReceived)
	}
}

func TestDispatcher_AtomicClaim_PreventsDoubleExecution(t *testing.T) {
	repo := newInMemoryDispatchRepo()
	dispatchID := uuid.New()
	dispatch := &entity.WebhookDispatch{
		ID:            dispatchID,
		TransactionID: uuid.New(),
		MerchantID:    uuid.New(),
		Status:        entity.WebhookStatusPending,
	}
	repo.dispatches[dispatchID] = dispatch

	// Worker 1 claims
	claimed1, err := repo.ClaimDispatch(context.Background(), nil, dispatchID, "worker-1", 10*time.Second)
	if err != nil || !claimed1 {
		t.Fatalf("worker 1 claim failed: %v", err)
	}

	// Worker 2 tries to claim same dispatch -> must return false
	claimed2, err := repo.ClaimDispatch(context.Background(), nil, dispatchID, "worker-2", 10*time.Second)
	if err != nil {
		t.Fatalf("worker 2 claim error: %v", err)
	}
	if claimed2 {
		t.Error("worker 2 should NOT be able to claim an already locked dispatch")
	}

	// Worker 1 releases
	repo.ReleaseDispatch(context.Background(), nil, dispatchID, entity.WebhookStatusSuccess, 1, nil)

	// Worker 2 tries to claim after SUCCESS -> must return false
	claimed3, _ := repo.ClaimDispatch(context.Background(), nil, dispatchID, "worker-2", 10*time.Second)
	if claimed3 {
		t.Error("worker 2 should NOT claim a non-pending dispatch")
	}
}
