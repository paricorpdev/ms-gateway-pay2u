// Command worker runs scheduled background jobs, sharing configuration,
// persistence and use cases with the API.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"paygate/internal/bootstrap"
	"paygate/internal/config"
	"paygate/internal/worker"

	"github.com/sirupsen/logrus"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	_, cfg, err := config.Init()
	if err != nil {
		return err
	}

	if err := applyTimezone(cfg.App.Timezone); err != nil {
		return err
	}

	log, err := config.NewLogger(cfg)
	if err != nil {
		return err
	}
	log.WithField("version", bootstrap.Version).Info("starting paygate background worker")

	if !cfg.Worker.Enabled {
		log.Warn("worker is disabled by configuration; exiting")
		return nil
	}

	db, err := config.NewDatabase(cfg, log)
	if err != nil {
		return err
	}
	defer closeQuietly(log, "database", func() error { return config.CloseDatabase(db) })

	redis, err := config.NewRedis(cfg, log)
	if err != nil {
		return err
	}
	defer closeQuietly(log, "redis", redis.Close)

	deps := &bootstrap.Dependencies{
		Config:   cfg,
		Log:      log,
		DB:       db,
		Redis:    redis,
		Validate: config.NewValidator(),
	}
	container := bootstrap.NewContainer(deps)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	jobs := bootstrap.WorkerJobs(deps, container)
	if len(jobs) == 0 {
		log.Warn("no background jobs registered; exiting")
		return nil
	}

	runner := worker.NewRunner(log, jobs...)

	done := make(chan struct{})
	go func() {
		runner.Start(ctx)
		close(done)
	}()

	<-ctx.Done()
	log.Info("shutdown signal received; draining background jobs")

	select {
	case <-done:
	case <-time.After(cfg.Worker.ShutdownTimeout):
		log.Warn("jobs did not finish within shutdown timeout")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Worker.ShutdownTimeout)
	defer cancel()

	if container.Dispatcher != nil {
		if err := container.Dispatcher.Stop(shutdownCtx); err != nil {
			log.WithError(err).Warn("webhook dispatcher shutdown did not drain cleanly")
		}
	}

	if container.AuditWorker != nil {
		if err := container.AuditWorker.Stop(shutdownCtx); err != nil {
			log.WithError(err).Warn("audit worker shutdown did not drain cleanly")
		}
	}

	log.Info("worker process shutdown complete")
	return nil
}

func applyTimezone(name string) error {
	if name == "" {
		name = "UTC"
	}

	location, err := time.LoadLocation(name)
	if err != nil {
		return fmt.Errorf("app.timezone %q is not a known timezone: %w", name, err)
	}

	time.Local = location
	return os.Setenv("TZ", name)
}

func closeQuietly(log *logrus.Logger, name string, closer func() error) {
	if err := closer(); err != nil && !errors.Is(err, context.Canceled) {
		log.WithError(err).Errorf("failed to close %s", name)
		return
	}
	log.Debugf("%s closed", name)
}
