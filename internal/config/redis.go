package config

import (
	"context"
	"crypto/tls"
	"fmt"

	"github.com/gofiber/storage/redis/v3"
	"github.com/sirupsen/logrus"
)

// NewRedis opens the Redis connection and verifies it.
func NewRedis(cfg *Config, log *logrus.Logger) (*redis.Storage, error) {
	rc := cfg.Database.Redis

	options := redis.Config{
		Host:     rc.Host,
		Port:     rc.Port,
		Password: rc.Password,
		Database: rc.Database,
		PoolSize: rc.PoolSize,
	}
	if rc.TLS {
		options.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	}

	storage := redis.New(options)

	if err := PingRedis(context.Background(), storage); err != nil {
		return nil, fmt.Errorf("ping redis: %w", err)
	}

	log.WithField("target", rc.Redacted()).Info("redis connected")
	return storage, nil
}

// PingRedis backs the readiness probe.
func PingRedis(ctx context.Context, storage *redis.Storage) error {
	if storage == nil {
		return fmt.Errorf("redis storage is not initialised")
	}
	return storage.Conn().Ping(ctx).Err()
}
