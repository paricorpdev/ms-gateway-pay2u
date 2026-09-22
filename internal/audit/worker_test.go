package audit

import (
	"context"
	"errors"
	"io"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"paygate/internal/entity"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

type mockAuditRepo struct {
	mu           sync.Mutex
	inboundLogs  []*entity.InboundRequest
	outboundLogs []*entity.OutboundRequest
	delay        time.Duration
	panicOnNth   int
	callCount    atomic.Int32
}

func newMockRepo(delay time.Duration) *mockAuditRepo {
	return &mockAuditRepo{
		delay: delay,
	}
}

func (m *mockAuditRepo) SaveInbound(ctx context.Context, log *entity.InboundRequest) error {
	count := m.callCount.Add(1)
	if m.panicOnNth > 0 && int(count) == m.panicOnNth {
		panic("simulated repository panic")
	}

	if m.delay > 0 {
		time.Sleep(m.delay)
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.inboundLogs = append(m.inboundLogs, log)
	return nil
}

func (m *mockAuditRepo) SaveOutbound(ctx context.Context, log *entity.OutboundRequest) error {
	count := m.callCount.Add(1)
	if m.panicOnNth > 0 && int(count) == m.panicOnNth {
		panic("simulated repository panic")
	}

	if m.delay > 0 {
		time.Sleep(m.delay)
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.outboundLogs = append(m.outboundLogs, log)
	return nil
}

func (m *mockAuditRepo) InboundCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.inboundLogs)
}

func (m *mockAuditRepo) OutboundCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.outboundLogs)
}

func newDiscardLogger() *logrus.Logger {
	l := logrus.New()
	l.SetOutput(io.Discard)
	return l
}

func TestWorker_TimeoutWhileDraining(t *testing.T) {
	repo := newMockRepo(30 * time.Millisecond)
	w := NewWorker(repo, newDiscardLogger(), 100)

	for i := 0; i < 10; i++ {
		w.RecordInbound(&entity.InboundRequest{
			ID:        uuid.New(),
			RequestID: "req-timeout",
		})
	}

	shortCtx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	err := w.Stop(shortCtx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context.DeadlineExceeded, got %v", err)
	}

	if state := w.State(); state != StateStopping {
		t.Fatalf("expected state to remain StateStopping (%d) after timeout, got %d", StateStopping, state)
	}

	longCtx, cancelLong := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancelLong()

	err2 := w.Stop(longCtx)
	if err2 != nil {
		t.Fatalf("expected second Stop with longer context to return nil, got %v", err2)
	}

	if state := w.State(); state != StateStopped {
		t.Fatalf("expected state to be StateStopped (%d) after drain complete, got %d", StateStopped, state)
	}

	if count := repo.InboundCount(); count != 10 {
		t.Errorf("expected 10 inbound logs saved, got %d", count)
	}
}

func TestWorker_NoSendOnClosedChannel(t *testing.T) {
	repo := newMockRepo(0)
	w := NewWorker(repo, newDiscardLogger(), 1000)

	var wg sync.WaitGroup
	workers := 50
	itemsPerWorker := 100

	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < itemsPerWorker; j++ {
				w.RecordInbound(&entity.InboundRequest{
					ID:        uuid.New(),
					RequestID: "req-concurrent",
				})
				w.RecordOutbound(&entity.OutboundRequest{
					ID:        uuid.New(),
					RequestID: "req-concurrent",
				})
			}
		}()
	}

	time.Sleep(5 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := w.Stop(ctx); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	wg.Wait()

	if state := w.State(); state != StateStopped {
		t.Errorf("expected state StateStopped, got %d", state)
	}
}

func TestWorker_MultipleConcurrentStop(t *testing.T) {
	repo := newMockRepo(0)
	w := NewWorker(repo, newDiscardLogger(), 100)

	for i := 0; i < 5; i++ {
		w.RecordInbound(&entity.InboundRequest{
			ID:        uuid.New(),
			RequestID: "req-multi-stop",
		})
	}

	var wg sync.WaitGroup
	stopCallers := 10
	wg.Add(stopCallers)

	for i := 0; i < stopCallers; i++ {
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			_ = w.Stop(ctx)
		}()
	}

	wg.Wait()

	if state := w.State(); state != StateStopped {
		t.Errorf("expected state StateStopped, got %d", state)
	}

	if count := repo.InboundCount(); count != 5 {
		t.Errorf("expected 5 inbound logs, got %d", count)
	}
}

func TestWorker_RecordAfterStopped(t *testing.T) {
	repo := newMockRepo(0)
	w := NewWorker(repo, newDiscardLogger(), 10)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	if err := w.Stop(ctx); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	if state := w.State(); state != StateStopped {
		t.Fatalf("expected StateStopped, got %d", state)
	}

	w.RecordInbound(&entity.InboundRequest{ID: uuid.New()})
	w.RecordOutbound(&entity.OutboundRequest{ID: uuid.New()})

	if count := repo.InboundCount(); count != 0 {
		t.Errorf("expected 0 inbound logs after stop, got %d", count)
	}
	if count := repo.OutboundCount(); count != 0 {
		t.Errorf("expected 0 outbound logs after stop, got %d", count)
	}
}

func TestWorker_PerEventPanicIsolation(t *testing.T) {
	repo := newMockRepo(0)
	repo.panicOnNth = 3

	w := NewWorker(repo, newDiscardLogger(), 10)

	for i := 0; i < 5; i++ {
		w.RecordInbound(&entity.InboundRequest{
			ID:        uuid.New(),
			RequestID: "req-panic-test",
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := w.Stop(ctx); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	if count := repo.InboundCount(); count != 4 {
		t.Errorf("expected 4 inbound logs saved (1 panicked), got %d", count)
	}
}
