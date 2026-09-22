package audit

import (
	"context"
	"sync"
	"sync/atomic"

	"paygate/internal/entity"
	"paygate/internal/repository"

	"github.com/sirupsen/logrus"
)

const (
	StateRunning  int32 = 0
	StateStopping int32 = 1
	StateStopped  int32 = 2
)

const defaultQueueBuffer = 5000

type AuditWorker struct {
	repo         repository.AuditLogRepository
	log          *logrus.Logger
	inboundChan  chan *entity.InboundRequest
	outboundChan chan *entity.OutboundRequest
	state        atomic.Int32
	mu           sync.RWMutex
	stopOnce     sync.Once
	wg           sync.WaitGroup
	drainedChan  chan struct{}
}

func NewWorker(repo repository.AuditLogRepository, log *logrus.Logger, bufferSize int) *AuditWorker {
	if bufferSize <= 0 {
		bufferSize = defaultQueueBuffer
	}

	w := &AuditWorker{
		repo:         repo,
		log:          log,
		inboundChan:  make(chan *entity.InboundRequest, bufferSize),
		outboundChan: make(chan *entity.OutboundRequest, bufferSize),
		drainedChan:  make(chan struct{}),
	}
	w.state.Store(StateRunning)

	w.wg.Add(2)
	go w.inboundWorker()
	go w.outboundWorker()

	return w
}

func (w *AuditWorker) State() int32 {
	return w.state.Load()
}

func (w *AuditWorker) RecordInbound(entry *entity.InboundRequest) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	if w.state.Load() != StateRunning {
		w.log.Warn("audit worker stopping or stopped, inbound event dropped")
		return
	}

	select {
	case w.inboundChan <- entry:
	default:
		w.log.Warn("audit inbound queue full, event dropped")
	}
}

func (w *AuditWorker) RecordOutbound(entry *entity.OutboundRequest) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	if w.state.Load() != StateRunning {
		w.log.Warn("audit worker stopping or stopped, outbound event dropped")
		return
	}

	select {
	case w.outboundChan <- entry:
	default:
		w.log.Warn("audit outbound queue full, event dropped")
	}
}

func (w *AuditWorker) Stop(ctx context.Context) error {
	w.stopOnce.Do(func() {
		w.state.Store(StateStopping)

		// Acquire write lock to ensure in-flight RLock senders have finished enqueuing.
		w.mu.Lock()
		close(w.inboundChan)
		close(w.outboundChan)
		w.mu.Unlock()

		go func() {
			w.wg.Wait()
			w.state.Store(StateStopped)
			close(w.drainedChan)
		}()
	})

	select {
	case <-w.drainedChan:
		w.log.Info("audit worker drained and stopped cleanly")
		return nil
	case <-ctx.Done():
		w.log.Warn("audit worker stop timed out, drain continues in background")
		return ctx.Err()
	}
}

func (w *AuditWorker) inboundWorker() {
	defer w.wg.Done()
	for entry := range w.inboundChan {
		w.processInbound(entry)
	}
}

func (w *AuditWorker) processInbound(entry *entity.InboundRequest) {
	defer func() {
		if r := recover(); r != nil {
			w.log.Errorf("recovered from panic in audit inbound worker: %v", r)
		}
	}()

	if err := w.repo.SaveInbound(context.Background(), entry); err != nil {
		w.log.WithError(err).Error("failed to save inbound audit log")
	}
}

func (w *AuditWorker) outboundWorker() {
	defer w.wg.Done()
	for entry := range w.outboundChan {
		w.processOutbound(entry)
	}
}

func (w *AuditWorker) processOutbound(entry *entity.OutboundRequest) {
	defer func() {
		if r := recover(); r != nil {
			w.log.Errorf("recovered from panic in audit outbound worker: %v", r)
		}
	}()

	if err := w.repo.SaveOutbound(context.Background(), entry); err != nil {
		w.log.WithError(err).Error("failed to save outbound audit log")
	}
}
