package controller

import (
	"fmt"
	"github.com/gofiber/fiber/v2"
	"github.com/wuwuseo/cmf/internal/service"
	"github.com/wuwuseo/cmf/pkg/app/http"
)

type User struct {
}

func NewUser() User {
	return User{}
}

// Handler
func (r User) Count(c *fiber.Ctx) error {
	svc := service.New(c.Context())

	data, err := svc.CountUser()
	fmt.Println(err)
	return http.NewResponse(c).Success(data, "user count!")
}
