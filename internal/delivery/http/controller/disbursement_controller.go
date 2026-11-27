package controller

import (
	"mime"
	"strconv"
	"strings"

	"paygate/internal/common/constants"
	"paygate/internal/delivery/http/response"
	"paygate/internal/exception"
	"paygate/internal/model/payload"
	"paygate/internal/usecase"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type DisbursementController struct {
	disbursementUC usecase.DisbursementService
}

func NewDisbursementController(disbursementUC usecase.DisbursementService) *DisbursementController {
	return &DisbursementController{disbursementUC: disbursementUC}
}

func (c *DisbursementController) InquiryAccount(ctx *fiber.Ctx) error {
	var req payload.DisbursementInquiryRequest
	if err := ctx.BodyParser(&req); err != nil {
		return exception.BadRequest("invalid request body").Wrap(err)
	}
	if err := req.Validate(); err != nil {
		return err
	}

	result, err := c.disbursementUC.InquiryAccount(ctx.UserContext(), &req)
	if err != nil {
		return err
	}
	ctx.Locals(constants.LocalsTransactionID, result.TransactionID)
	return response.Created(ctx, result)
}

func (c *DisbursementController) TransferBank(ctx *fiber.Ctx) error {
	merchantRef, err := c.merchantRef(ctx)
	if err != nil {
		return err
	}
	result, err := c.disbursementUC.TransferBank(ctx.UserContext(), merchantRef)
	if err != nil {
		return err
	}
	ctx.Locals(constants.LocalsTransactionID, result.TransactionID)
	return response.OK(ctx, result)
}

func (c *DisbursementController) GetDisbursement(ctx *fiber.Ctx) error {
	merchantRef, err := c.merchantRef(ctx)
	if err != nil {
		return err
	}
	result, err := c.disbursementUC.GetDisbursement(ctx.UserContext(), merchantRef)
	if err != nil {
		return err
	}
	ctx.Locals(constants.LocalsTransactionID, result.TransactionID)
	return response.OK(ctx, result)
}

func (c *DisbursementController) ListAdmin(ctx *fiber.Ctx) error {
	page, err := queryInt(ctx, "page", payload.DefaultPage)
	if err != nil {
		return exception.BadRequest("page must be a positive integer")
	}
	perPage, err := queryInt(ctx, "per_page", payload.DefaultPerPage)
	if err != nil {
		return exception.BadRequest("per_page must be a positive integer")
	}
	month, err := queryInt(ctx, "month", 0)
	if err != nil {
		return exception.BadRequest("month must be an integer")
	}
	year, err := queryInt(ctx, "year", 0)
	if err != nil {
		return exception.BadRequest("year must be an integer")
	}

	result, err := c.disbursementUC.ListAdminDisbursements(ctx.UserContext(), payload.AdminDisbursementListRequest{
		Page:       page,
		PerPage:    perPage,
		Search:     strings.TrimSpace(ctx.Query("search")),
		Status:     ctx.Query("status"),
		MerchantID: strings.TrimSpace(ctx.Query("merchant_id")),
		Month:      month,
		Year:       year,
		AdminID:    c.dashboardAdminScope(ctx),
	})
	if err != nil {
		return err
	}
	return response.OK(ctx, result)
}

func (c *DisbursementController) GetAdmin(ctx *fiber.Ctx) error {
	id, err := uuid.Parse(ctx.Params("id"))
	if err != nil {
		return exception.BadRequest("disbursement id must be a UUID")
	}
	result, err := c.disbursementUC.GetAdminDisbursement(ctx.UserContext(), id, c.dashboardAdminScope(ctx))
	if err != nil {
		return err
	}
	ctx.Locals(constants.LocalsTransactionID, result.TransactionID)
	return response.OK(ctx, result)
}

func (c *DisbursementController) HandleCallback(ctx *fiber.Ctx) error {
	contentType, _, err := mime.ParseMediaType(ctx.Get(fiber.HeaderContentType))
	if err != nil || !strings.EqualFold(contentType, "application/json") {
		return exception.UnsupportedMedia("content-type must be application/json")
	}

	if err := c.disbursementUC.HandleTransferCallback(ctx.UserContext(), ctx.Params("provider"), ctx.Body()); err != nil {
		return err
	}
	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"rc": "00",
		"rd": "Success",
	})
}

func (c *DisbursementController) merchantRef(ctx *fiber.Ctx) (string, error) {
	merchantRef := ctx.Params("merchant_reff")
	if merchantRef == "" {
		return "", exception.BadRequest("merchant_reff is required")
	}
	return merchantRef, nil
}

func (c *DisbursementController) dashboardAdminScope(ctx *fiber.Ctx) *uuid.UUID {
	roles, _ := ctx.Locals(constants.LocalsDashboardRoles).([]string)
	admin := false
	superAdmin := false
	for _, role := range roles {
		admin = admin || role == constants.DashboardRoleAdmin
		superAdmin = superAdmin || role == constants.DashboardRoleSuperAdmin
	}
	if !admin || superAdmin {
		return nil
	}
	adminID, ok := ctx.Locals(constants.LocalsDashboardUserID).(uuid.UUID)
	if !ok || adminID == uuid.Nil {
		return nil
	}
	return &adminID
}

func queryInt(ctx *fiber.Ctx, key string, fallback int) (int, error) {
	value := strings.TrimSpace(ctx.Query(key))
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, err
	}
	return parsed, nil
}
