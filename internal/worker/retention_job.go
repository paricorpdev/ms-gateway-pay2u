package worker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"paygate/internal/repository"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type RetentionStats struct {
	InboundDeleted    int64
	OutboundDeleted   int64
	DispatchesDeleted int64
	LogsDeleted       int64
}

type RetentionJob struct {
	db            *gorm.DB
	auditRepo     repository.AuditLogRepository
	dispatchRepo  repository.WebhookDispatchRepository
	log           *logrus.Logger
	runHour       int
	retentionDays int
	batchSize     int
	location      *time.Location
	lastRunDate   string
	forceRun      bool
	mu            sync.Mutex
}

func NewRetentionJob(
	db *gorm.DB,
	auditRepo repository.AuditLogRepository,
	dispatchRepo repository.WebhookDispatchRepository,
	log *logrus.Logger,
	runHour int,
	retentionDays int,
	batchSize int,
	timezone string,
) *RetentionJob {
	if runHour < 0 || runHour > 23 {
		runHour = 2
	}
	if retentionDays <= 0 {
		retentionDays = 30
	}
	if batchSize <= 0 {
		batchSize = 1000
	}

	loc := time.Local
	if timezone != "" {
		if l, err := time.LoadLocation(timezone); err == nil {
			loc = l
		}
	}

	return &RetentionJob{
		db:            db,
		auditRepo:     auditRepo,
		dispatchRepo:  dispatchRepo,
		log:           log,
		runHour:       runHour,
		retentionDays: retentionDays,
		batchSize:     batchSize,
		location:      loc,
	}
}

func (j *RetentionJob) Name() string {
	return "log-retention"
}

// Interval returns 15 minutes so the runner checks periodically if the scheduled hour has arrived.
func (j *RetentionJob) Interval() time.Duration {
	return 15 * time.Minute
}

// SetForceRun allows test suites or manual triggers to bypass the scheduled hour check.
func (j *RetentionJob) SetForceRun(force bool) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.forceRun = force
}

func (j *RetentionJob) Run(ctx context.Context) error {
	j.mu.Lock()
	defer j.mu.Unlock()

	now := time.Now().In(j.location)
	today := now.Format("2006-01-02")

	if !j.forceRun {
		if now.Hour() != j.runHour {
			return nil
		}
		if j.lastRunDate == today {
			return nil
		}
	}

	cutoff := now.AddDate(0, 0, -j.retentionDays)
	j.log.WithFields(logrus.Fields{
		"retention_days": j.retentionDays,
		"cutoff":         cutoff.Format(time.RFC3339),
		"batch_size":     j.batchSize,
	}).Info("starting log retention cleanup batch")

	stats, err := j.Purge(ctx, cutoff)
	if err != nil {
		return err
	}

	j.lastRunDate = today
	j.log.WithFields(logrus.Fields{
		"inbound_deleted":    stats.InboundDeleted,
		"outbound_deleted":   stats.OutboundDeleted,
		"dispatches_deleted": stats.DispatchesDeleted,
		"logs_deleted":       stats.LogsDeleted,
		"cutoff":             cutoff.Format(time.RFC3339),
	}).Info("log retention cleanup completed")

	return nil
}

// Purge executes the batch deletion process for historical audit and dispatch records.
func (j *RetentionJob) Purge(ctx context.Context, cutoff time.Time) (RetentionStats, error) {
	var stats RetentionStats

	// Purge inbound_requests
	if j.auditRepo != nil {
		for {
			if ctx.Err() != nil {
				return stats, ctx.Err()
			}
			deleted, err := j.auditRepo.DeleteInboundBefore(ctx, cutoff, j.batchSize)
			if err != nil {
				return stats, fmt.Errorf("purge inbound_requests: %w", err)
			}
			stats.InboundDeleted += deleted
			if deleted < int64(j.batchSize) {
				break
			}
			time.Sleep(50 * time.Millisecond)
		}

		// Purge outbound_requests
		for {
			if ctx.Err() != nil {
				return stats, ctx.Err()
			}
			deleted, err := j.auditRepo.DeleteOutboundBefore(ctx, cutoff, j.batchSize)
			if err != nil {
				return stats, fmt.Errorf("purge outbound_requests: %w", err)
			}
			stats.OutboundDeleted += deleted
			if deleted < int64(j.batchSize) {
				break
			}
			time.Sleep(50 * time.Millisecond)
		}
	}

	// Purge webhook_dispatch_logs and terminal webhook_dispatches
	if j.dispatchRepo != nil && j.db != nil {
		for {
			if ctx.Err() != nil {
				return stats, ctx.Err()
			}
			deleted, err := j.dispatchRepo.DeleteDispatchLogsBefore(ctx, j.db, cutoff, j.batchSize)
			if err != nil {
				return stats, fmt.Errorf("purge webhook_dispatch_logs: %w", err)
			}
			stats.LogsDeleted += deleted
			if deleted < int64(j.batchSize) {
				break
			}
			time.Sleep(50 * time.Millisecond)
		}

		for {
			if ctx.Err() != nil {
				return stats, ctx.Err()
			}
			deleted, err := j.dispatchRepo.DeleteDispatchesBefore(ctx, j.db, cutoff, j.batchSize)
			if err != nil {
				return stats, fmt.Errorf("purge webhook_dispatches: %w", err)
			}
			stats.DispatchesDeleted += deleted
			if deleted < int64(j.batchSize) {
				break
			}
			time.Sleep(50 * time.Millisecond)
		}
	}

	return stats, nil
}
