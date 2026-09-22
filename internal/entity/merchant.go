package entity

import (
	"time"

	"github.com/google/uuid"
)

type Merchant struct {
	ID            uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Code          string    `gorm:"size:100;uniqueIndex;not null" json:"code"`
	Name          string    `gorm:"size:150;not null" json:"name"`
	APIKey        string    `gorm:"size:255;uniqueIndex;not null" json:"api_key"`
	WebhookURL    string    `gorm:"type:text" json:"webhook_url,omitempty"`
	WebhookSecret string    `gorm:"size:255" json:"webhook_secret,omitempty"`
	IsActive      bool      `gorm:"not null;default:true;index" json:"is_active"`
	CreatedAt     time.Time `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt     time.Time `gorm:"not null;default:now()" json:"updated_at"`
}

func (Merchant) TableName() string { return "merchants" }
