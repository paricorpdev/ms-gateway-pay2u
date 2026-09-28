package bootstrap

import (
	"context"
	"sync"

	"paygate/internal/worker"
)

// WorkerJobs registers all scheduled background jobs configured for the service.
func WorkerJobs(deps *Dependencies, container *Container) []worker.Job {
	if !deps.Config.Worker.Enabled {
		return nil
	}

	var jobs []worker.Job

	// Auto-Expiry Job
	if container.TxRepo != nil && deps.DB != nil {
		expiryJob := worker.NewExpiryJob(
			deps.DB,
			container.TxRepo,
			container.MerchantRepo,
			container.DispatchRepo,
			container.Dispatcher,
			deps.Log,
			deps.Config.Worker.Expiry.Interval,
			deps.Config.Worker.Expiry.BatchSize,
		)
		jobs = append(jobs, expiryJob)
	}

	// Log Retention Job
	if container.AuditRepo != nil && deps.DB != nil {
		retentionJob := worker.NewRetentionJob(
			deps.DB,
			container.AuditRepo,
			container.DispatchRepo,
			deps.Log,
			deps.Config.Worker.Retention.RunHour,
			deps.Config.Worker.Retention.Days,
			deps.Config.Worker.Retention.BatchSize,
			deps.Config.App.Timezone,
		)
		jobs = append(jobs, retentionJob)
	}

	return jobs
}

// StartEmbeddedWorker starts background jobs inside the same process when worker.embedded is enabled.
// It returns a closer func to gracefully stop the runner during shutdown.
func StartEmbeddedWorker(ctx context.Context, deps *Dependencies, container *Container) func(context.Context) error {
	jobs := WorkerJobs(deps, container)
	if len(jobs) == 0 {
		return func(context.Context) error { return nil }
	}

	deps.Log.WithField("job_count", len(jobs)).Info("starting embedded background worker runner")
	runner := worker.NewRunner(deps.Log, jobs...)

	workerCtx, cancel := context.WithCancel(ctx)
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		runner.Start(workerCtx)
	}()

	return func(shutdownCtx context.Context) error {
		cancel()
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			return nil
		case <-shutdownCtx.Done():
			deps.Log.Warn("embedded worker shutdown did not finish within timeout")
			return shutdownCtx.Err()
		}
	}
}
