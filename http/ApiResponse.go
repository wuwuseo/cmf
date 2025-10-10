package http

import (
	"github.com/gofiber/fiber/v2"
)

type ApiResponse struct {
	Ctx *fiber.Ctx
}

func NewApiResponse(ctx *fiber.Ctx) *ApiResponse {
	return &ApiResponse{Ctx: ctx}
}

func (r *ApiResponse) ToResponse(data fiber.Map, status int) error {
	if data == nil {
		data = fiber.Map{}
	}

	r.Ctx.Status(status)
	r.Ctx.JSON(data)
	return nil
}

func (r *ApiResponse) Result(msg string, data fiber.Map, code int) error {
	response := fiber.Map{
		"code": code,
		"msg":  msg,
		"data": data,
	}
	r.ToResponse(response, 200)
	return nil
}

func (r *ApiResponse) Success(msg string, data fiber.Map) error {
	r.Result(msg, data, 1)
	return nil
}

func (r *ApiResponse) Error(msg string, data fiber.Map) error {
	r.Result(msg, data, 0)
	return nil
}
