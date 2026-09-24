package controller

import (
	"paygate/internal/common/constants"
	"paygate/internal/delivery/http/response"
	"paygate/internal/exception"
	"paygate/internal/model/payload"
	"paygate/internal/usecase"

	"github.com/gofiber/fiber/v2"
)

type PaymentController struct {
	paymentUC usecase.PaymentService
}

func NewPaymentController(paymentUC usecase.PaymentService) *PaymentController {
	return &PaymentController{
		paymentUC: paymentUC,
	}
}

func (c *PaymentController) CreatePayment(ctx *fiber.Ctx) error {
	var req payload.CreatePaymentRequest
	if err := ctx.BodyParser(&req); err != nil {
		return exception.BadRequest("invalid request body").Wrap(err)
	}

	res, err := c.paymentUC.CreatePayment(ctx.UserContext(), &req)
	if err != nil {
		return err
	}

	ctx.Locals(constants.LocalsTransactionID, res.ID)
	return response.Created(ctx, res)
}

func (c *PaymentController) getParamRef(ctx *fiber.Ctx) (string, error) {
	ref := ctx.Params("merchant_reff")
	if ref == "" {
		ref = ctx.Params("reff")
	}
	if ref == "" {
		ref = ctx.Params("uuid")
	}
	if ref == "" {
		return "", exception.BadRequest("merchant_reff is required")
	}
	return ref, nil
}

func (c *PaymentController) GetPayment(ctx *fiber.Ctx) error {
	ref, err := c.getParamRef(ctx)
	if err != nil {
		return err
	}

	res, err := c.paymentUC.GetPayment(ctx.UserContext(), ref)
	if err != nil {
		return err
	}

	ctx.Locals(constants.LocalsTransactionID, res.ID)
	return response.OK(ctx, res)
}

func (c *PaymentController) RefreshPayment(ctx *fiber.Ctx) error {
	ref, err := c.getParamRef(ctx)
	if err != nil {
		return err
	}

	res, err := c.paymentUC.RefreshPayment(ctx.UserContext(), ref)
	if err != nil {
		return err
	}

	ctx.Locals(constants.LocalsTransactionID, res.ID)
	return response.OK(ctx, res)
}

func (c *PaymentController) HandleCallback(ctx *fiber.Ctx) error {
	providerName := ctx.Params("provider")
	rawBody := ctx.Body()

	if err := c.paymentUC.HandleCallback(ctx.UserContext(), providerName, rawBody); err != nil {
		return err
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"rc": "00",
		"rd": "Sukses",
	})
}
