package bootstrap

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
)

func TestShouldSkipCompressionForEventStreamAccept(t *testing.T) {
	app := fiber.New()
	app.Get("/api/v1/admin/notifications", func(c fiber.Ctx) error {
		if !shouldSkipCompression(c) {
			t.Fatal("expected compression to be skipped for EventSource requests")
		}
		return c.SendStatus(fiber.StatusNoContent)
	})

	req := httptest.NewRequest(fiber.MethodGet, "/api/v1/admin/notifications", nil)
	req.Header.Set(fiber.HeaderAccept, "text/event-stream")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusNoContent {
		t.Fatalf("unexpected status %d", resp.StatusCode)
	}
}

func TestShouldSkipCompressionForSSEPath(t *testing.T) {
	app := fiber.New()
	app.Get("/api/v1/admin/sse/connect", func(c fiber.Ctx) error {
		if !shouldSkipCompression(c) {
			t.Fatal("expected compression to be skipped for SSE paths")
		}
		return c.SendStatus(fiber.StatusNoContent)
	})

	req := httptest.NewRequest(fiber.MethodGet, "/api/v1/admin/sse/connect", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusNoContent {
		t.Fatalf("unexpected status %d", resp.StatusCode)
	}
}

func TestShouldNotSkipCompressionByDefault(t *testing.T) {
	app := fiber.New()
	app.Get("/api/v1/admin/users", func(c fiber.Ctx) error {
		if shouldSkipCompression(c) {
			t.Fatal("did not expect compression to be skipped for normal requests")
		}
		return c.SendStatus(fiber.StatusNoContent)
	})

	req := httptest.NewRequest(fiber.MethodGet, "/api/v1/admin/users", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusNoContent {
		t.Fatalf("unexpected status %d", resp.StatusCode)
	}
}
