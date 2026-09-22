package controller

import (
	"paygate/internal/delivery/http/response"
	"paygate/internal/exception"
	"paygate/internal/model/payload"
	"paygate/internal/usecase"

	"github.com/gofiber/fiber/v2"
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

	res, err := c.merchantUC.CreateMerchant(ctx.UserContext(), &req)
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

	res, err := c.merchantUC.GetMerchant(ctx.UserContext(), id)
	if err != nil {
		return err
	}

	return response.OK(ctx, res)
}

func (c *MerchantController) ListMerchants(ctx *fiber.Ctx) error {
	pageReq := payload.PageRequest{
		Page:    ctx.QueryInt("page", payload.DefaultPage),
		PerPage: ctx.QueryInt("per_page", payload.DefaultPerPage),
	}
	pageReq.Normalize()

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

	res, err := c.merchantUC.UpdateMerchant(ctx.UserContext(), id, &req)
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

	res, err := c.merchantUC.RotateAPIKey(ctx.UserContext(), id)
	if err != nil {
		return err
	}

	return response.OK(ctx, res)
}
