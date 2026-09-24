package webhook

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"paygate/internal/entity"
	"paygate/internal/repository"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

const (
	StateRunning  int32 = 0
	StateStopping int32 = 1
	StateStopped  int32 = 2

	DefaultHTTPTimeout     = 10 * time.Second
	DefaultClaimDuration   = 30 * time.Second
	DefaultMaxResponseSize = 4096 // 4KB

	RedisRetryQueueKey = "paygate:webhook:retry_queue"
)

type Dispatcher interface {
	Start(ctx context.Context)
	Stop(ctx context.Context) error
	Enqueue(ctx context.Context, dispatchID uuid.UUID) error
}

type dispatcher struct {
	db          *gorm.DB
	redisClient redis.UniversalClient
	repo        repository.WebhookDispatchRepository
	httpClient  *http.Client
	log         *logrus.Logger
	workerID    string

	state         atomic.Int32
	immediateChan chan uuid.UUID
	mu            sync.RWMutex
	stopOnce      sync.Once
	wg            sync.WaitGroup
	stopCtx       context.Context
	cancelFunc    context.CancelFunc
}

func NewDispatcher(
	db *gorm.DB,
	redisClient redis.UniversalClient,
	repo repository.WebhookDispatchRepository,
	log *logrus.Logger,
) Dispatcher {
	hostname, _ := os.Hostname()
	workerID := fmt.Sprintf("%s-%d-%s", hostname, os.Getpid(), uuid.NewString()[:8])

	httpClient := &http.Client{
		Timeout: DefaultHTTPTimeout,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 20,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	stopCtx, cancel := context.WithCancel(context.Background())

	return &dispatcher{
		db:            db,
		redisClient:   redisClient,
		repo:          repo,
		httpClient:    httpClient,
		log:           log,
		workerID:      workerID,
		immediateChan: make(chan uuid.UUID, 500),
		stopCtx:       stopCtx,
		cancelFunc:    cancel,
	}
}

func (d *dispatcher) Start(ctx context.Context) {
	d.log.WithField("worker_id", d.workerID).Info("Webhook Dispatcher starting background routines")

	// Immediate dispatch consumer
	d.wg.Add(1)
	go d.runImmediateWorker()

	// Delayed retry scheduler via Redis ZSET
	d.wg.Add(1)
	go d.runDelayedRetryScheduler()

	// Durable recovery scanner (DB as single source of truth)
	d.wg.Add(1)
	go d.runRecoveryScanner()
}

func (d *dispatcher) Enqueue(ctx context.Context, dispatchID uuid.UUID) error {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if d.state.Load() != StateRunning {
		return errors.New("webhook dispatcher is stopped")
	}

	select {
	case d.immediateChan <- dispatchID:
		return nil
	default:
		d.log.WithField("dispatch_id", dispatchID).Warn("Webhook immediate queue is full, will be recovered by scanner")
		return nil
	}
}

func (d *dispatcher) Stop(ctx context.Context) error {
	var err error
	d.stopOnce.Do(func() {
		d.state.Store(StateStopping)
		d.cancelFunc()

		d.mu.Lock()
		close(d.immediateChan)
		d.mu.Unlock()

		done := make(chan struct{})
		go func() {
			d.wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			d.state.Store(StateStopped)
			d.log.Info("Webhook Dispatcher stopped gracefully")
		case <-ctx.Done():
			err = ctx.Err()
			d.log.Warn("Webhook Dispatcher shutdown timed out before all tasks drained")
		}
	})
	return err
}

func (d *dispatcher) runImmediateWorker() {
	defer d.wg.Done()

	for dispatchID := range d.immediateChan {
		d.safeExecute(dispatchID)
	}
}

func (d *dispatcher) runDelayedRetryScheduler() {
	defer d.wg.Done()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-d.stopCtx.Done():
			return
		case <-ticker.C:
			if d.redisClient == nil {
				continue
			}
			d.processRedisRetryQueue()
		}
	}
}

func (d *dispatcher) processRedisRetryQueue() {
	now := time.Now().Unix()
	opt := &redis.ZRangeBy{
		Min:    "-inf",
		Max:    strconv.FormatInt(now, 10),
		Offset: 0,
		Count:  20,
	}

	items, err := d.redisClient.ZRangeByScore(d.stopCtx, RedisRetryQueueKey, opt).Result()
	if err != nil {
		if !errors.Is(err, context.Canceled) {
			d.log.WithError(err).Warn("Failed to poll redis webhook retry queue")
		}
		return
	}

	for _, item := range items {
		// Remove from ZSET first
		d.redisClient.ZRem(d.stopCtx, RedisRetryQueueKey, item)

		dispatchID, err := uuid.Parse(item)
		if err != nil {
			continue
		}

		go d.safeExecute(dispatchID)
	}
}

func (d *dispatcher) runRecoveryScanner() {
	defer d.wg.Done()

	// Initial scan on startup (cold-start recovery)
	d.scanAndRecover()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-d.stopCtx.Done():
			return
		case <-ticker.C:
			d.scanAndRecover()
		}
	}
}

func (d *dispatcher) scanAndRecover() {
	ctx, cancel := context.WithTimeout(d.stopCtx, 10*time.Second)
	defer cancel()

	list, err := d.repo.FindRecoverableDispatches(ctx, d.db, 25)
	if err != nil {
		if !errors.Is(err, context.Canceled) {
			d.log.WithError(err).Warn("Durable recovery scanner query failed")
		}
		return
	}

	for _, dispatch := range list {
		if d.state.Load() != StateRunning {
			return
		}
		go d.safeExecute(dispatch.ID)
	}
}

func (d *dispatcher) safeExecute(dispatchID uuid.UUID) {
	defer func() {
		if r := recover(); r != nil {
			d.log.WithFields(logrus.Fields{
				"dispatch_id": dispatchID,
				"panic":       r,
			}).Error("Recovered from panic during webhook delivery execution")
		}
	}()

	d.execute(dispatchID)
}

func (d *dispatcher) execute(dispatchID uuid.UUID) {
	ctx, cancel := context.WithTimeout(context.Background(), DefaultHTTPTimeout+5*time.Second)
	defer cancel()

	// Atomic claim to guarantee single execution in distributed environment
	claimed, err := d.repo.ClaimDispatch(ctx, d.db, dispatchID, d.workerID, DefaultClaimDuration)
	if err != nil {
		d.log.WithError(err).WithField("dispatch_id", dispatchID).Error("Failed to claim webhook dispatch")
		return
	}
	if !claimed {
		// Already claimed by another worker or not in PENDING state
		return
	}

	// Fetch dispatch record
	dispatch, err := d.repo.FindDispatchByID(ctx, d.db, dispatchID)
	if err != nil || dispatch == nil {
		d.log.WithError(err).WithField("dispatch_id", dispatchID).Error("Failed to find claimed webhook dispatch")
		return
	}
	if dispatch.Status != entity.WebhookStatusPending {
		return
	}

	attemptNumber := dispatch.Attempts + 1

	// Prepare HTTP Request
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, dispatch.TargetURL, bytes.NewReader(dispatch.Payload))
	if err != nil {
		d.log.WithError(err).WithField("dispatch_id", dispatchID).Error("Failed to create webhook HTTP request")
		d.repo.ReleaseDispatch(ctx, d.db, dispatchID, entity.WebhookStatusFailed, attemptNumber, nil)
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Paygate-Delivery-Id", dispatch.ID.String())
	req.Header.Set("X-Paygate-Event", dispatch.EventType)
	req.Header.Set("X-Request-Id", uuid.NewString())

	// Compute HMAC-SHA256 signature if merchant secret is present
	if dispatch.Merchant != nil && dispatch.Merchant.WebhookSecret != "" {
		sig := ComputeSignature(dispatch.Payload, dispatch.Merchant.WebhookSecret)
		req.Header.Set("X-Paygate-Signature", "sha256="+sig)
	}

	// Execute HTTP Call & measure latency
	start := time.Now()
	resp, httpErr := d.httpClient.Do(req)
	latencyMs := time.Since(start).Milliseconds()

	var statusCode *int
	var respBodyStr *string
	var errMsg *string
	var retryAfterHeader string

	if httpErr != nil {
		errStr := httpErr.Error()
		errMsg = &errStr
	} else if resp != nil {
		code := resp.StatusCode
		statusCode = &code
		retryAfterHeader = resp.Header.Get("Retry-After")

		// Read response body (capped at 4KB)
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, DefaultMaxResponseSize))
		resp.Body.Close()

		bodyStr := string(bodyBytes)
		respBodyStr = &bodyStr
	}

	// Create attempt log
	logEntry := &entity.WebhookDispatchLog{
		DispatchID:      dispatch.ID,
		AttemptNumber:   attemptNumber,
		HTTPStatus:      statusCode,
		ResponsePayload: respBodyStr,
		ErrorMessage:    errMsg,
		LatencyMs:       latencyMs,
		DispatchedAt:    start,
	}
	if logErr := d.repo.CreateDispatchLog(ctx, d.db, logEntry); logErr != nil {
		d.log.WithError(logErr).WithField("dispatch_id", dispatchID).Warn("Failed to save webhook dispatch log")
	}

	// Evaluate Result via Policy
	currentCode := 0
	if statusCode != nil {
		currentCode = *statusCode
	}

	// Case A: 2xx Success
	if IsSuccess(currentCode) {
		d.repo.ReleaseDispatch(ctx, d.db, dispatch.ID, entity.WebhookStatusSuccess, attemptNumber, nil)
		d.log.WithFields(logrus.Fields{
			"dispatch_id": dispatch.ID,
			"attempt":     attemptNumber,
			"latency_ms":  latencyMs,
		}).Info("Webhook delivered successfully")
		return
	}

	// Case B: Non-retryable error (e.g. 400, 401, 403, 404, 422)
	if !IsRetryable(currentCode, httpErr) {
		d.repo.ReleaseDispatch(ctx, d.db, dispatch.ID, entity.WebhookStatusFailed, attemptNumber, nil)
		d.log.WithFields(logrus.Fields{
			"dispatch_id": dispatch.ID,
			"status_code": currentCode,
			"attempt":     attemptNumber,
		}).Warn("Webhook delivery failed with non-retryable status, stopping")
		return
	}

	// Case C: Retryable error
	if attemptNumber >= dispatch.MaxAttempts {
		// Exhausted all attempts (3x default) -> Mark FAILED
		d.repo.ReleaseDispatch(ctx, d.db, dispatch.ID, entity.WebhookStatusFailed, attemptNumber, nil)
		d.log.WithFields(logrus.Fields{
			"dispatch_id": dispatch.ID,
			"attempts":    attemptNumber,
		}).Warn("Webhook delivery exhausted maximum attempts, marked as FAILED")
		return
	}

	// Schedule next retry
	backoff := CalculateBackoff(attemptNumber, retryAfterHeader)
	nextRetry := time.Now().Add(backoff)

	if err := d.repo.ReleaseDispatch(ctx, d.db, dispatch.ID, entity.WebhookStatusPending, attemptNumber, &nextRetry); err != nil {
		d.log.WithError(err).WithField("dispatch_id", dispatch.ID).Error("Failed to update next retry in database")
	}

	// Enqueue to Redis ZSET
	if d.redisClient != nil {
		zErr := d.redisClient.ZAdd(ctx, RedisRetryQueueKey, redis.Z{
			Score:  float64(nextRetry.Unix()),
			Member: dispatch.ID.String(),
		}).Err()
		if zErr != nil {
			d.log.WithError(zErr).WithField("dispatch_id", dispatch.ID).Warn("Failed to enqueue retry to redis; will be handled by recovery scanner")
		}
	}

	d.log.WithFields(logrus.Fields{
		"dispatch_id": dispatch.ID,
		"attempt":     attemptNumber,
		"backoff":     backoff,
		"next_retry":  nextRetry,
	}).Info("Webhook delivery failed (retryable), scheduled next retry")
}
