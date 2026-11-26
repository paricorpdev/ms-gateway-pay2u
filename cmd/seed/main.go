package main

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"paygate/internal/common/generator"
	"paygate/internal/config"
	"paygate/internal/entity"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	viperConfig, appConfig, err := config.Init()
	if err != nil {
		return fmt.Errorf("load config.yml: %w", err)
	}

	code := strings.TrimSpace(viperConfig.GetString("seed.merchant.code"))
	if code == "" {
		code = "demo"
	}
	code = generator.NormalizeCode(code)
	name := strings.TrimSpace(viperConfig.GetString("seed.merchant.name"))
	if name == "" {
		name = "Demo Merchant"
	}
	webhookURL := strings.TrimSpace(viperConfig.GetString("seed.merchant.webhook_url"))

	log, err := config.NewLogger(appConfig)
	if err != nil {
		return fmt.Errorf("create logger: %w", err)
	}
	db, err := config.NewDatabase(appConfig, log)
	if err != nil {
		return err
	}
	defer config.CloseDatabase(db)

	var merchant entity.Merchant
	created := false
	err = db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("code = ?", code).First(&merchant).Error; err == nil {
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("find merchant: %w", err)
		}

		apiKey, err := generator.GenerateAPIKey(code)
		if err != nil {
			return fmt.Errorf("generate merchant API key: %w", err)
		}
		webhookSecret, err := generator.GenerateWebhookSecret()
		if err != nil {
			return fmt.Errorf("generate webhook secret: %w", err)
		}

		now := time.Now().UTC()
		merchant = entity.Merchant{
			ID:            uuid.New(),
			Code:          code,
			Name:          name,
			APIKey:        apiKey,
			WebhookURL:    webhookURL,
			WebhookSecret: webhookSecret,
			IsActive:      true,
			CreatedAt:     now,
			UpdatedAt:     now,
		}
		if err := tx.Create(&merchant).Error; err != nil {
			return fmt.Errorf("create merchant: %w", err)
		}
		created = true
		return nil
	})
	if err != nil {
		return err
	}

	if created {
		fmt.Printf("Seeded merchant %q\nAPI key: %s\nWebhook secret: %s\n", merchant.Code, merchant.APIKey, merchant.WebhookSecret)
	} else {
		fmt.Printf("Merchant %q already exists; existing credentials were left unchanged.\n", merchant.Code)
	}
	return nil
}
