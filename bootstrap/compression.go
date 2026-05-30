package bootstrap

import (
	"strings"

	"github.com/gofiber/fiber/v3"
)

func shouldSkipCompression(c fiber.Ctx) bool {
	return strings.Contains(c.Get(fiber.HeaderAccept), "text/event-stream") ||
		strings.Contains(c.Path(), "/sse/")
}
