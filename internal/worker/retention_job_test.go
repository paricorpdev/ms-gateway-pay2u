package worker

import (
	"context"
	"testing"
	"time"

	"paygate/internal/entity"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type mockAuditRepoForRetention struct {
	inboundCalls     int
	outboundCalls    int
	inboundRemaining int64
	outboundRemaining int64
}

func (m *mockAuditRepoForRetention) SaveInbound(ctx context.Context, log *entity.InboundRequest) error {
	return nil
}
func (m *mockAuditRepoForRetention) SaveOutbound(ctx context.Context, log *entity.OutboundRequest) error {
	return nil
}
func (m *mockAuditRepoForRetention) DeleteInboundBefore(ctx context.Context, cutoff time.Time, limit int) (int64, error) {
	m.inboundCalls++
	toDelete := int64(limit)
	if m.inboundRemaining < toDelete {
		toDelete = m.inboundRemaining
	}
	m.inboundRemaining -= toDelete
	return toDelete, nil
}
func (m *mockAuditRepoForRetention) DeleteOutboundBefore(ctx context.Context, cutoff time.Time, limit int) (int64, error) {
	m.outboundCalls++
	toDelete := int64(limit)
	if m.outboundRemaining < toDelete {
		toDelete = m.outboundRemaining
	}
	m.outboundRemaining -= toDelete
	return toDelete, nil
}

type mockDispatchRepoForRetention struct {
	logsRemaining       int64
	dispatchesRemaining int64
	logsCalls           int
	dispatchesCalls     int
}

func (m *mockDispatchRepoForRetention) CreateDispatch(ctx context.Context, db *gorm.DB, d *entity.WebhookDispatch) error {
	return nil
}
func (m *mockDispatchRepoForRetention) ClaimDispatch(ctx context.Context, db *gorm.DB, id uuid.UUID, workerID string, lockDuration time.Duration) (bool, error) {
	return true, nil
}
func (m *mockDispatchRepoForRetention) ReleaseDispatch(ctx context.Context, db *gorm.DB, id uuid.UUID, status string, attempts int, nextRetry *time.Time) error {
	return nil
}
func (m *mockDispatchRepoForRetention) CreateDispatchLog(ctx context.Context, db *gorm.DB, l *entity.WebhookDispatchLog) error {
	return nil
}
func (m *mockDispatchRepoForRetention) FindDispatchByID(ctx context.Context, db *gorm.DB, id uuid.UUID) (*entity.WebhookDispatch, error) {
	return nil, nil
}
func (m *mockDispatchRepoForRetention) FindRecoverableDispatches(ctx context.Context, db *gorm.DB, limit int) ([]*entity.WebhookDispatch, error) {
	return nil, nil
}
func (m *mockDispatchRepoForRetention) DeleteDispatchLogsBefore(ctx context.Context, db *gorm.DB, cutoff time.Time, limit int) (int64, error) {
	m.logsCalls++
	toDelete := int64(limit)
	if m.logsRemaining < toDelete {
		toDelete = m.logsRemaining
	}
	m.logsRemaining -= toDelete
	return toDelete, nil
}
func (m *mockDispatchRepoForRetention) DeleteDispatchesBefore(ctx context.Context, db *gorm.DB, cutoff time.Time, limit int) (int64, error) {
	m.dispatchesCalls++
	toDelete := int64(limit)
	if m.dispatchesRemaining < toDelete {
		toDelete = m.dispatchesRemaining
	}
	m.dispatchesRemaining -= toDelete
	return toDelete, nil
}

func TestRetentionJob_PurgeBatches(t *testing.T) {
	log := logrus.New()
	log.SetOutput(logrus.StandardLogger().Out)

	auditRepo := &mockAuditRepoForRetention{
		inboundRemaining:  2500, // should take 3 batches of size 1000 (1000, 1000, 500)
		outboundRemaining: 1200, // should take 2 batches of size 1000 (1000, 200)
	}

	dispatchRepo := &mockDispatchRepoForRetention{
		logsRemaining:       1500, // 2 batches
		dispatchesRemaining: 800,  // 1 batch
	}

	// Fake non-nil gorm.DB by passing a dummy pointer or struct
	fakeDB := &gorm.DB{}

	job := NewRetentionJob(fakeDB, auditRepo, dispatchRepo, log, 2, 30, 1000, "UTC")
	job.SetForceRun(true)

	if job.Name() != "log-retention" {
		t.Errorf("expected name 'log-retention', got %s", job.Name())
	}

	err := job.Run(context.Background())
	if err != nil {
		t.Fatalf("job.Run failed: %v", err)
	}

	if auditRepo.inboundCalls != 3 {
		t.Errorf("expected 3 inbound delete batches, got %d", auditRepo.inboundCalls)
	}
	if auditRepo.outboundCalls != 2 {
		t.Errorf("expected 2 outbound delete batches, got %d", auditRepo.outboundCalls)
	}
	if dispatchRepo.logsCalls != 2 {
		t.Errorf("expected 2 dispatch log delete batches, got %d", dispatchRepo.logsCalls)
	}
	if dispatchRepo.dispatchesCalls != 1 {
		t.Errorf("expected 1 dispatch delete batch, got %d", dispatchRepo.dispatchesCalls)
	}
}

func TestRetentionJob_ScheduledHour(t *testing.T) {
	log := logrus.New()
	log.SetOutput(logrus.StandardLogger().Out)

	auditRepo := &mockAuditRepoForRetention{
		inboundRemaining: 100,
	}

	fakeDB := &gorm.DB{}

	// Choose a runHour that is NOT the current hour
	nowHour := time.Now().Hour()
	targetHour := (nowHour + 5) % 24

	job := NewRetentionJob(fakeDB, auditRepo, nil, log, targetHour, 30, 1000, "")

	// Without forceRun, it should skip execution because nowHour != targetHour
	err := job.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if auditRepo.inboundCalls != 0 {
		t.Errorf("expected 0 calls when not scheduled hour, got %d", auditRepo.inboundCalls)
	}
}
