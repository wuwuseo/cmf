package http

import "github.com/gofiber/fiber/v2"

type Response struct {
	Ctx *fiber.Ctx
}

func NewResponse(ctx *fiber.Ctx) *Response {
	return &Response{Ctx: ctx}
}

func (r *Response) Status(status int) *Response {
	r.Ctx.Status(status)
	return r
}

func (r *Response) ToResponseJson(data interface{}) error {
	if data == nil {
		data = fiber.Map{}
	}
	return r.Ctx.JSON(data)
}

func (r *Response) Response(code int, msg string, data interface{}) error {
	responseData := fiber.Map{
		"code": code,
		"msg":  msg,
		"data": data,
	}
	return r.ToResponseJson(responseData)
}

func (r *Response) Success(data interface{}, msg ...string) error {
	if len(msg) > 0 {
		return r.Response(1, msg[0], data)
	}
	return r.Response(1, "ok", data)
}

func (r *Response) Error(msg string, data ...interface{}) error {
	if len(data) > 0 {
		return r.Response(0, msg, data[0])
	} else {
		var tmp interface{}
		return r.Response(0, msg, tmp)
	}
}
