package middleware

import (
	"context"

	"paygate/internal/common/constants"
	"paygate/internal/exception"
	"paygate/internal/usecase"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type dashboardRoleRow struct {
	UserID uuid.UUID `gorm:"column:user_id"`
	Code   string    `gorm:"column:code"`
}

// NewDashboardUserKey authenticates dashboard administration requests against
// the per-user key stored in the shared Dashboard users table.
func NewDashboardUserKey(db *gorm.DB) fiber.Handler {
	return func(ctx *fiber.Ctx) error {
		provided := ctx.Get(constants.HeaderAPIKey)
		if provided == "" {
			return exception.Unauthorized("missing API key")
		}
		if db == nil {
			return exception.Internal(nil)
		}

		var roles []dashboardRoleRow
		result := db.WithContext(ctx.UserContext()).
			Table("users").
			Select("users.id AS user_id, roles.code").
			Joins("JOIN user_roles ON user_roles.user_id = users.id").
			Joins("JOIN roles ON roles.id = user_roles.role_id").
			Where("users.ms_master_key = ? AND users.status = ?", provided, "ACTIVE").
			Find(&roles)
		if result.Error != nil {
			return exception.Internal(result.Error)
		}

		roleCodes := make([]string, 0, len(roles))
		var dashboardUserID uuid.UUID
		for _, role := range roles {
			if dashboardUserID == uuid.Nil {
				dashboardUserID = role.UserID
			}
			if role.Code == constants.DashboardRoleAdmin || role.Code == constants.DashboardRoleSuperAdmin {
				roleCodes = append(roleCodes, role.Code)
			}
		}
		if len(roleCodes) == 0 || dashboardUserID == uuid.Nil {
			return exception.Unauthorized("invalid dashboard user key")
		}
		ctx.Locals(constants.LocalsDashboardRoles, roleCodes)
		ctx.Locals(constants.LocalsDashboardUserID, dashboardUserID)
		return ctx.Next()
	}
}

func RequireDashboardAdmin() fiber.Handler {
	return func(ctx *fiber.Ctx) error {
		roles, _ := ctx.Locals(constants.LocalsDashboardRoles).([]string)
		isAdmin := false
		isSuperAdmin := false
		for _, role := range roles {
			isAdmin = isAdmin || role == constants.DashboardRoleAdmin
			isSuperAdmin = isSuperAdmin || role == constants.DashboardRoleSuperAdmin
		}
		if isAdmin && !isSuperAdmin {
			userID, ok := ctx.Locals(constants.LocalsDashboardUserID).(uuid.UUID)
			if !ok || userID == uuid.Nil {
				return exception.Unauthorized("dashboard admin identity is required")
			}
			return ctx.Next()
		}
		return exception.Forbidden("only dashboard admins can manage merchants")
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
