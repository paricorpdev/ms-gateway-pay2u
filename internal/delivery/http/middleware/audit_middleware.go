package middleware

import (
	"bytes"
	"encoding/json"
	"strings"
	"time"

	"paygate/internal/audit"
	"paygate/internal/common/constants"
	"paygate/internal/entity"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

func NewAuditMiddleware(worker *audit.AuditWorker) fiber.Handler {
	return func(ctx *fiber.Ctx) error {
		if worker == nil {
			return ctx.Next()
		}

		path := strings.Clone(ctx.Path())
		if !strings.HasPrefix(path, "/api/") {
			return ctx.Next()
		}

		start := time.Now()

		reqBody := bytes.Clone(ctx.Body())

		headers := make(map[string]string)
		ctx.Request().Header.VisitAll(func(key, val []byte) {
			headers[string(key)] = string(val)
		})
		headersJSON, _ := json.Marshal(headers)

		err := ctx.Next()

		respBody := bytes.Clone(ctx.Response().Body())

		reqID, _ := ctx.Locals(constants.LocalsRequestID).(string)
		if reqID == "" {
			reqID = ctx.Get(constants.HeaderRequestID)
		}

		var txID *uuid.UUID
		if rawTxID := ctx.Locals(constants.LocalsTransactionID); rawTxID != nil {
			if id, ok := rawTxID.(uuid.UUID); ok {
				txID = &id
			} else if idStr, ok := rawTxID.(string); ok {
				if parsed, parseErr := uuid.Parse(idStr); parseErr == nil {
					txID = &parsed
				}
			}
		}

		sourceService, _ := ctx.Locals(constants.LocalsMerchantCode).(string)
		if sourceService == "" {
			sourceService = ctx.Get("X-Service-Name")
		}
		if sourceService == "" {
			sourceService = ctx.Get("User-Agent")
		}

		inbound := &entity.InboundRequest{
			ID:              uuid.New(),
			RequestID:       reqID,
			TransactionID:   txID,
			SourceService:   sourceService,
			Endpoint:        path,
			Method:          strings.Clone(ctx.Method()),
			Headers:         entity.JSONB(headersJSON),
			RequestPayload:  entity.JSONB(reqBody),
			ResponseStatus:  ctx.Response().StatusCode(),
			ResponsePayload: entity.JSONB(respBody),
			LatencyMs:       time.Since(start).Milliseconds(),
			IPAddress:       strings.Clone(ctx.IP()),
			CreatedAt:       time.Now().UTC(),
		}

		worker.RecordInbound(inbound)

		return err
	}
}
