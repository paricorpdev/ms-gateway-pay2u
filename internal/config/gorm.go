package config

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

// NewDatabase opens the PostgreSQL connection pool and verifies it.
func NewDatabase(cfg *Config, log *logrus.Logger) (*gorm.DB, error) {
	pg := cfg.Database.Postgres

	db, err := gorm.Open(postgres.Open(pg.DSN()), &gorm.Config{
		Logger: gormlogger.New(&logrusWriter{log: log}, gormlogger.Config{
			SlowThreshold:             pg.SlowThreshold,
			Colorful:                  false,
			IgnoreRecordNotFoundError: true,
			ParameterizedQueries:      true,
			LogLevel:                  gormLogLevel(pg.LogLevel),
		}),
		NamingStrategy:                           schema.NamingStrategy{SingularTable: false},
		SkipDefaultTransaction:                   true,
		DisableForeignKeyConstraintWhenMigrating: true,
		TranslateError:                           true,
	})
	if err != nil {
		return nil, fmt.Errorf("connect database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("resolve database handle: %w", err)
	}

	sqlDB.SetMaxIdleConns(pg.MaxIdleConns)
	sqlDB.SetMaxOpenConns(pg.MaxOpenConns)
	sqlDB.SetConnMaxLifetime(pg.ConnMaxLifetime)
	sqlDB.SetConnMaxIdleTime(pg.ConnMaxIdleTime)

	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("ping database: %w", err)
	}

	log.WithField("target", pg.Redacted()).Info("database connected")
	return db, nil
}

// PingDatabase backs the readiness probe.
func PingDatabase(ctx context.Context, db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.PingContext(ctx)
}

// CloseDatabase drains the pool on shutdown.
func CloseDatabase(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

func gormLogLevel(level string) gormlogger.LogLevel {
	switch level {
	case "silent":
		return gormlogger.Silent
	case "error":
		return gormlogger.Error
	case "info":
		return gormlogger.Info
	default:
		return gormlogger.Warn
	}
}

type logrusWriter struct {
	log *logrus.Logger
}

func (l *logrusWriter) Printf(message string, args ...any) {
	l.log.Debugf(message, args...)
}
