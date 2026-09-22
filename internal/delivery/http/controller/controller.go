package controller

import (
	"paygate/internal/exception"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// PathParamUUID is the name of the identifier route parameter.
const PathParamUUID = "uuid"

// PathUUID parses the :uuid route parameter.
func PathUUID(ctx *fiber.Ctx) (uuid.UUID, error) {
	return NamedPathUUID(ctx, PathParamUUID)
}

// NamedPathUUID parses an arbitrary UUID route parameter.
func NamedPathUUID(ctx *fiber.Ctx, name string) (uuid.UUID, error) {
	raw := ctx.Params(name)
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, exception.BadRequest(name + " must be a valid UUID").Wrap(err)
	}
	if id == uuid.Nil {
		return uuid.Nil, exception.BadRequest(name + " must not be the nil UUID")
	}
	return id, nil
}
