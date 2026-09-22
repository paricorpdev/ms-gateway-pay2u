package entity

import (
	"time"

	"github.com/google/uuid"
)

type InboundRequest struct {
	ID              uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	RequestID       string     `gorm:"size:64;not null;index" json:"request_id"`
	TransactionID   *uuid.UUID `gorm:"type:uuid;index" json:"transaction_id,omitempty"`
	SourceService   string     `gorm:"size:100" json:"source_service,omitempty"`
	Endpoint        string     `gorm:"type:text;not null" json:"endpoint"`
	Method          string     `gorm:"size:10;not null" json:"method"`
	Headers         JSONB      `gorm:"type:jsonb" json:"headers,omitempty"`
	RequestPayload  JSONB      `gorm:"type:jsonb" json:"request_payload,omitempty"`
	ResponseStatus  int        `gorm:"not null" json:"response_status"`
	ResponsePayload JSONB      `gorm:"type:jsonb" json:"response_payload,omitempty"`
	LatencyMs       int64      `gorm:"not null;default:0" json:"latency_ms"`
	IPAddress       string     `gorm:"size:50" json:"ip_address,omitempty"`
	CreatedAt       time.Time  `gorm:"not null;default:now();index" json:"created_at"`
}

func (InboundRequest) TableName() string { return "inbound_requests" }

type OutboundRequest struct {
	ID              uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	RequestID       string     `gorm:"size:64;not null;index" json:"request_id"`
	TransactionID   *uuid.UUID `gorm:"type:uuid;index" json:"transaction_id,omitempty"`
	Provider        string     `gorm:"size:50;not null" json:"provider"`
	RequestType     string     `gorm:"size:50;not null" json:"request_type"`
	Attempt         int        `gorm:"not null;default:1" json:"attempt"`
	Endpoint        string     `gorm:"type:text;not null" json:"endpoint"`
	Method          string     `gorm:"size:10;not null" json:"method"`
	Headers         JSONB      `gorm:"type:jsonb" json:"headers,omitempty"`
	RequestPayload  JSONB      `gorm:"type:jsonb" json:"request_payload,omitempty"`
	ResponseStatus  int        `gorm:"not null" json:"response_status"`
	ResponsePayload JSONB      `gorm:"type:jsonb" json:"response_payload,omitempty"`
	LatencyMs       int64      `gorm:"not null;default:0" json:"latency_ms"`
	CreatedAt       time.Time  `gorm:"not null;default:now();index" json:"created_at"`
}

func (OutboundRequest) TableName() string { return "outbound_requests" }
