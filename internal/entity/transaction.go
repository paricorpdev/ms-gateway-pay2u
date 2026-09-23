package entity

import (
	"bytes"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

// JSONB is a helper for GORM JSONB columns.
type JSONB json.RawMessage

func (j JSONB) Value() (driver.Value, error) {
	if len(j) == 0 {
		return nil, nil
	}
	b := bytes.ReplaceAll([]byte(j), []byte{0}, nil)
	cleanStr := strings.ToValidUTF8(string(b), "")
	cleanBytes := []byte(cleanStr)
	if len(cleanBytes) == 0 {
		return nil, nil
	}
	if json.Valid(cleanBytes) {
		return cleanBytes, nil
	}
	wrapped, err := json.Marshal(cleanStr)
	if err != nil {
		return nil, err
	}
	return wrapped, nil
}

func (j *JSONB) Scan(value any) error {
	if value == nil {
		*j = nil
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("JSONB: unsupported scan source")
	}
	*j = append((*j)[0:0], bytes...)
	return nil
}

type Transaction struct {
	ID               uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	MerchantID       *uuid.UUID `gorm:"type:uuid;index" json:"merchant_id,omitempty"`
	Merchant         *Merchant  `gorm:"foreignKey:MerchantID" json:"merchant,omitempty"`
	RequestID        string     `gorm:"column:request_id;uniqueIndex;size:255;not null" json:"request_id"`
	Provider         string     `gorm:"size:50;not null;default:'pay2u'" json:"provider"`
	ProviderToken    string     `gorm:"size:255" json:"provider_token,omitempty"`
	MerchantReff     string     `gorm:"uniqueIndex;size:255;not null" json:"merchant_reff"`
	PaymentMethod    string     `gorm:"size:50;not null" json:"payment_method"`
	PaymentCode      string     `gorm:"type:text" json:"payment_code,omitempty"`
	Status           string     `gorm:"size:20;not null;default:'PENDING';index" json:"status"`
	Amount           int64      `gorm:"not null" json:"amount"`
	AmountAdmin      int64      `gorm:"not null;default:0" json:"amount_admin"`
	AmountDiscount   int64      `gorm:"not null;default:0" json:"amount_discount"`
	AmountTotal      int64      `gorm:"not null" json:"amount_total"`
	Currency         string     `gorm:"size:10;not null;default:'IDR'" json:"currency"`
	CustomerName     string     `gorm:"size:255" json:"customer_name,omitempty"`
	CustomerPhone    string     `gorm:"size:50" json:"customer_phone,omitempty"`
	CustomerEmail    string     `gorm:"size:255" json:"customer_email,omitempty"`
	BillTitle        string     `gorm:"size:255" json:"bill_title,omitempty"`
	BillDescription  string     `gorm:"type:text" json:"bill_description,omitempty"`
	ProviderCallback JSONB      `gorm:"type:jsonb" json:"provider_callback,omitempty"`
	PaymentReff      string     `gorm:"size:255" json:"payment_reff,omitempty"`
	ExpiredAt        *time.Time `json:"expired_at,omitempty"`
	PaidAt           *time.Time `json:"paid_at,omitempty"`
	CreatedAt        time.Time  `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt        time.Time  `gorm:"not null;default:now()" json:"updated_at"`
}

func (Transaction) TableName() string { return "transactions" }
