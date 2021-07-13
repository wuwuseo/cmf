package controller

import (
	"github.com/gofiber/fiber/v2"
	"github.com/wuwuseo/cmf/pkg/app/http"
)

type Demo struct {
}

func NewDemo() Demo {
	return Demo{}
}

// Handler
func (r Demo) Hello(c *fiber.Ctx) error {
	var data interface{}
	return http.NewResponse(c).Success(data, "hello world!")
}
