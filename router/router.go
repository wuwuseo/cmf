package router

import (
	"github.com/wuwuseo/cmf/internal/http/controller"

	"github.com/gofiber/fiber/v2"
)

func NewRouter(app *fiber.App) {
	demo := controller.NewDemo()
	user := controller.NewUser()
	app.Get("/", demo.Hello)
	app.Get("/user/count", user.Count)
}
