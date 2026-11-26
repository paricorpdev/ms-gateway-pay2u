package controller

import (
	"strconv"

	"paygate/internal/common/constants"
	"paygate/internal/delivery/http/response"
	"paygate/internal/exception"
	"paygate/internal/model/payload"
	"paygate/internal/usecase"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type MerchantController struct {
	merchantUC usecase.MerchantService
}

func NewMerchantController(merchantUC usecase.MerchantService) *MerchantController {
	return &MerchantController{
		merchantUC: merchantUC,
	}
}

func (c *MerchantController) CreateMerchant(ctx *fiber.Ctx) error {
	var req payload.CreateMerchantRequest
	if err := ctx.BodyParser(&req); err != nil {
		return exception.BadRequest("invalid request body").Wrap(err)
	}

	adminID, ok := ctx.Locals(constants.LocalsDashboardUserID).(uuid.UUID)
	if !ok || adminID == uuid.Nil {
		return exception.Unauthorized("dashboard admin identity is required")
	}
	res, err := c.merchantUC.CreateMerchant(ctx.UserContext(), adminID, &req)
	if err != nil {
		return err
	}

	return response.Created(ctx, res)
}

func (c *MerchantController) GetMerchant(ctx *fiber.Ctx) error {
	id, err := PathUUID(ctx)
	if err != nil {
		return err
	}

	res, err := c.merchantUC.GetMerchant(ctx.UserContext(), id, c.dashboardAdminScope(ctx))
	if err != nil {
		return err
	}

	return response.OK(ctx, res)
}

func (c *MerchantController) ListMerchants(ctx *fiber.Ctx) error {
	pageReq := payload.MerchantListRequest{
		Page:    ctx.QueryInt("page", payload.DefaultPage),
		PerPage: ctx.QueryInt("per_page", payload.DefaultPerPage),
		Search:  ctx.Query("search"),
		SortBy:  ctx.Query("sort_by"),
		SortDir: ctx.Query("sort_dir"),
		AdminID: c.dashboardAdminScope(ctx),
	}
	if value := ctx.Query("is_active"); value != "" {
		isActive, err := strconv.ParseBool(value)
		if err != nil {
			return exception.BadRequest("is_active must be true or false")
		}
		pageReq.IsActive = &isActive
	}

	res, err := c.merchantUC.ListMerchants(ctx.UserContext(), pageReq)
	if err != nil {
		return err
	}

	return response.OK(ctx, res)
}

func (c *MerchantController) UpdateMerchant(ctx *fiber.Ctx) error {
	id, err := PathUUID(ctx)
	if err != nil {
		return err
	}

	var req payload.UpdateMerchantRequest
	if err := ctx.BodyParser(&req); err != nil {
		return exception.BadRequest("invalid request body").Wrap(err)
	}

	res, err := c.merchantUC.UpdateMerchant(ctx.UserContext(), id, c.dashboardAdminScope(ctx), &req)
	if err != nil {
		return err
	}

	return response.OK(ctx, res)
}

func (c *MerchantController) RotateAPIKey(ctx *fiber.Ctx) error {
	id, err := PathUUID(ctx)
	if err != nil {
		return err
	}

	res, err := c.merchantUC.RotateAPIKey(ctx.UserContext(), id, c.dashboardAdminScope(ctx))
	if err != nil {
		return err
	}

	return response.OK(ctx, res)
}

func (c *MerchantController) RotateWebhookSecret(ctx *fiber.Ctx) error {
	id, err := PathUUID(ctx)
	if err != nil {
		return err
	}

	res, err := c.merchantUC.RotateWebhookSecret(ctx.UserContext(), id, c.dashboardAdminScope(ctx))
	if err != nil {
		return err
	}

	return response.OK(ctx, res)
}

func (c *MerchantController) dashboardAdminScope(ctx *fiber.Ctx) *uuid.UUID {
	roles, _ := ctx.Locals(constants.LocalsDashboardRoles).([]string)
	isAdmin := false
	isSuperAdmin := false
	for _, role := range roles {
		isAdmin = isAdmin || role == constants.DashboardRoleAdmin
		isSuperAdmin = isSuperAdmin || role == constants.DashboardRoleSuperAdmin
	}
	if !isAdmin || isSuperAdmin {
		return nil
	}
	adminID, ok := ctx.Locals(constants.LocalsDashboardUserID).(uuid.UUID)
	if !ok || adminID == uuid.Nil {
		return nil
	}
	return &adminID
}
