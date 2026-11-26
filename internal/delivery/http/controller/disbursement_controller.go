package controller

import (
	"paygate/internal/common/constants"
	"paygate/internal/delivery/http/response"
	"paygate/internal/exception"
	"paygate/internal/model/payload"
	"paygate/internal/usecase"

	"github.com/gofiber/fiber/v2"
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

func (c *DisbursementController) HandleCallback(ctx *fiber.Ctx) error {
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
