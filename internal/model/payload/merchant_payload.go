package payload

import (
	"time"

	"github.com/google/uuid"
)

type CreateMerchantRequest struct {
	Name       string `json:"name" validate:"required,min=2,max=150"`
	Code       string `json:"code" validate:"omitempty,min=2,max=100"`
	WebhookURL string `json:"webhook_url" validate:"omitempty,url"`
}

type UpdateMerchantRequest struct {
	Name       *string `json:"name" validate:"omitempty,min=2,max=150"`
	WebhookURL *string `json:"webhook_url" validate:"omitempty,url"`
	IsActive   *bool   `json:"is_active"`
}

type MerchantResponse struct {
	ID            uuid.UUID `json:"id"`
	Code          string    `json:"code"`
	Name          string    `json:"name"`
	APIKey        string    `json:"api_key"`
	WebhookURL    string    `json:"webhook_url,omitempty"`
	WebhookSecret string    `json:"webhook_secret,omitempty"`
	IsActive      bool      `json:"is_active"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}
