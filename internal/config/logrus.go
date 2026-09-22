package config

import (
	"os"

	applogger "paygate/internal/logger"

	"github.com/sirupsen/logrus"
)

// NewLogger builds the root logger.
func NewLogger(cfg *Config) (*logrus.Logger, error) {
	log := logrus.New()
	log.SetOutput(os.Stdout)

	level, err := logrus.ParseLevel(cfg.Log.Level)
	if err != nil {
		return nil, err
	}
	log.SetLevel(level)

	switch cfg.Log.Format {
	case "text":
		log.SetFormatter(&logrus.TextFormatter{FullTimestamp: true})
	default:
		log.SetFormatter(&logrus.JSONFormatter{TimestampFormat: "2006-01-02T15:04:05.000Z07:00"})
	}

	log = log.WithFields(logrus.Fields{
		"service": cfg.App.Name,
		"env":     cfg.App.Env,
	}).Logger

	applogger.SetFallback(log)
	return log, nil
}
