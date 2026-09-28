// Package worker runs background jobs. A Job receives only a context.Context,
// so the same use cases or repositories the HTTP API calls can be scheduled here.
package worker

import (
	"context"
	"runtime/debug"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

const defaultSlowJobRun = 10 * time.Second

// Job is one unit of background work.
type Job interface {
	// Name identifies the job in logs and metrics.
	Name() string
	// Interval is how often the job runs. Zero means "run once at startup".
	Interval() time.Duration
	// Run performs the work. It must respect ctx cancellation.
	Run(ctx context.Context) error
}

// JobFunc adapts a plain function into a Job.
type JobFunc struct {
	JobName string
	Every   time.Duration
	Do      func(ctx context.Context) error
}

func (j JobFunc) Name() string            { return j.JobName }
func (j JobFunc) Interval() time.Duration { return j.Every }
func (j JobFunc) Run(ctx context.Context) error {
	return j.Do(ctx)
}

// Runner supervises a set of jobs.
type Runner struct {
	log     *logrus.Logger
	jobs    []Job
	slowRun time.Duration
}

func NewRunner(log *logrus.Logger, jobs ...Job) *Runner {
	return &Runner{log: log, jobs: jobs, slowRun: defaultSlowJobRun}
}

// Register adds a job before Start is called.
func (r *Runner) Register(jobs ...Job) {
	r.jobs = append(r.jobs, jobs...)
}

// Start runs every job until ctx is cancelled, then waits for in-flight executions to finish.
func (r *Runner) Start(ctx context.Context) {
	var waitGroup sync.WaitGroup

	for _, job := range r.jobs {
		waitGroup.Add(1)
		go func(job Job) {
			defer waitGroup.Done()
			r.runJob(ctx, job)
		}(job)
	}

	waitGroup.Wait()
	r.log.Info("all background jobs stopped cleanly")
}

func (r *Runner) runJob(ctx context.Context, job Job) {
	entry := r.log.WithField("job", job.Name())

	if job.Interval() <= 0 {
		r.execute(ctx, job, entry)
		return
	}

	ticker := time.NewTicker(job.Interval())
	defer ticker.Stop()

	// Initial run immediately on start
	r.execute(ctx, job, entry)

	for {
		select {
		case <-ctx.Done():
			entry.Debug("job stopped by context cancellation")
			return
		case <-ticker.C:
			r.execute(ctx, job, entry)
		}
	}
}

func (r *Runner) execute(ctx context.Context, job Job, entry *logrus.Entry) {
	defer func() {
		if rec := recover(); rec != nil {
			entry.WithFields(logrus.Fields{
				"panic": rec,
				"stack": string(debug.Stack()),
			}).Error("background job panicked")
		}
	}()

	start := time.Now()
	err := job.Run(ctx)
	duration := time.Since(start)

	if err != nil && ctx.Err() == nil {
		entry.WithError(err).WithField("duration_ms", duration.Milliseconds()).Error("background job returned error")
		return
	}

	if duration > r.slowRun {
		entry.WithField("duration_ms", duration.Milliseconds()).Warn("background job took longer than slow threshold")
	} else {
		entry.WithField("duration_ms", duration.Milliseconds()).Debug("background job completed")
	}
}
