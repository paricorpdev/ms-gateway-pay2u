// @title Paygate OpenAPI
// @version 1.0
// @description Internal Payment Gateway service.
// @BasePath /api/v1

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
	"paygate/internal/delivery/http/response"

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
	log.WithField("version", bootstrap.Version).Info("starting paygate api")

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

	app := config.NewFiber(cfg, response.NewErrorHandler())

	deps := &bootstrap.Dependencies{
		Config:   cfg,
		Log:      log,
		DB:       db,
		Redis:    redis,
		Validate: config.NewValidator(),
		App:      app,
	}
	container := bootstrap.NewContainer(deps)
	bootstrap.HTTP(deps, container)

	serverErrors := make(chan error, 1)
	go func() {
		log.WithField("address", cfg.Web.Address()).Info("http server listening")
		if err := app.Listen(cfg.Web.Address()); err != nil {
			serverErrors <- err
		}
	}()

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		return fmt.Errorf("http server: %w", err)
	case sig := <-shutdown:
		log.WithField("signal", sig.String()).Info("shutdown signal received")
	}

	if err := app.ShutdownWithTimeout(cfg.App.ShutdownTimeout); err != nil {
		log.WithError(err).Error("graceful shutdown did not complete cleanly")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.App.ShutdownTimeout)
	defer cancel()

	if container.Dispatcher != nil {
		if err := container.Dispatcher.Stop(shutdownCtx); err != nil {
			log.WithError(err).Warn("webhook dispatcher shutdown did not drain cleanly before timeout")
		}
	}

	if err := container.AuditWorker.Stop(shutdownCtx); err != nil {
		log.WithError(err).Warn("audit worker shutdown did not drain cleanly before timeout")
	}

	log.Info("shutdown complete")
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
