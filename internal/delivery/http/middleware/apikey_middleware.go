package middleware

import (
	"context"
	"crypto/subtle"

	"paygate/internal/common/constants"
	"paygate/internal/exception"
	"paygate/internal/usecase"

	"github.com/gofiber/fiber/v2"
)

// NewMasterAPIKey validates the X-API-Key header against a master admin key.
func NewMasterAPIKey(masterKey string) fiber.Handler {
	return func(ctx *fiber.Ctx) error {
		provided := ctx.Get(constants.HeaderAPIKey)
		if provided == "" {
			return exception.Unauthorized("missing API key")
		}
		if subtle.ConstantTimeCompare([]byte(provided), []byte(masterKey)) != 1 {
			return exception.Unauthorized("invalid master API key")
		}
		return ctx.Next()
	}
}

// NewMerchantAPIKey strictly validates the X-API-Key header against dynamic merchants via Redis/DB.
func NewMerchantAPIKey(merchantUC usecase.MerchantService) fiber.Handler {
	return func(ctx *fiber.Ctx) error {
		provided := ctx.Get(constants.HeaderAPIKey)
		if provided == "" {
			return exception.Unauthorized("missing API key")
		}

		if merchantUC == nil {
			return exception.Internal(nil)
		}

		merchant, err := merchantUC.ValidateAPIKey(ctx.UserContext(), provided)
		if err != nil {
			return err
		}

		ctx.Locals(constants.LocalsMerchant, merchant)
		ctx.Locals(constants.LocalsMerchantCode, merchant.Code)
		ctx.SetUserContext(context.WithValue(ctx.UserContext(), constants.LocalsMerchant, merchant))

		return ctx.Next()
	}
}
