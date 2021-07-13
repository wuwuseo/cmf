package http

import "github.com/gofiber/fiber/v2"

type Request struct {
	Ctx *fiber.Ctx
}

func NewRequest(ctx *fiber.Ctx) *Request {
	return &Request{Ctx: ctx}
}

func (r *Request) Hostname() string {
	return r.Ctx.Hostname()
}

func (r *Request) IP() string {
	return r.Ctx.IP()
}

func (r *Request) Method() string {
	return r.Ctx.Method()
}

func (r *Request) Route() *fiber.Route {
	return r.Ctx.Route()
}

func (r *Request) Path() string {
	return r.Ctx.Path()
}

func (r *Request) Protocol() string {
	return r.Ctx.Protocol()
}

func (r *Request) IsJson() bool {
	return r.Ctx.Is("json")
}

func (r *Request) Header(key string, defaultValue string) string {
	return r.Ctx.Get(key, defaultValue)
}

func (r *Request) Params(key string, defaultValue string) string {
	return r.Ctx.Params(key, defaultValue)
}

func (r *Request) ParamsInt(key string) (int, error) {
	return r.Ctx.ParamsInt(key)
}

func (r *Request) Query(key string, defaultValue string) string {
	return r.Ctx.Query(key, defaultValue)
}

func (r *Request) QueryParser(out interface{}) error {
	return r.Ctx.QueryParser(out)
}

func (r *Request) Redirect(location string, status ...int) error {
	if len(status) > 0 {
		r.Ctx.Redirect(location, status[0])
	} else {
		r.Ctx.Redirect(location)
	}
	return nil
}
