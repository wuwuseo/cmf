package validate_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/wuwuseo/cmf/validate"
)

type FiberTestUser struct {
	Name  string `json:"name" validate:"required"`
	Email string `json:"email" validate:"required,email"`
	Age   int    `json:"age" validate:"gte=18"`
}

func TestParseAndValidate_Success(t *testing.T) {
	app := fiber.New()

	user := FiberTestUser{
		Name:  "张三",
		Email: "zhangsan@example.com",
		Age:   20,
	}
	body, _ := json.Marshal(user)

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	app.Post("/", func(c fiber.Ctx) error {
		result, err := validate.ParseAndValidate[FiberTestUser](c)
		if err != nil {
			return c.SendStatus(400)
		}
		if result.Name != user.Name || result.Email != user.Email || result.Age != user.Age {
			return c.SendStatus(422)
		}
		return c.SendStatus(200)
	})

	resp, _ := app.Test(req)
	if resp.StatusCode != 200 {
		t.Errorf("期望状态码 200，实际 %d", resp.StatusCode)
	}
}

func TestParseAndValidate_InvalidBody(t *testing.T) {
	app := fiber.New()

	invalidBody := []byte(`{"name": "李四"}`)
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(invalidBody))
	req.Header.Set("Content-Type", "application/json")

	app.Post("/", func(c fiber.Ctx) error {
		_, err := validate.ParseAndValidate[FiberTestUser](c)
		if err == nil {
			return c.SendStatus(200)
		}
		return c.SendStatus(400)
	})

	resp, _ := app.Test(req)
	if resp.StatusCode != 400 {
		t.Errorf("期望状态码 400，实际 %d", resp.StatusCode)
	}
}

func TestParseAndValidate_ValidationFail(t *testing.T) {
	app := fiber.New()

	user := FiberTestUser{
		Name:  "",
		Email: "not-email",
		Age:   10,
	}
	body, _ := json.Marshal(user)

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	app.Post("/", func(c fiber.Ctx) error {
		_, err := validate.ParseAndValidate[FiberTestUser](c)
		if err == nil {
			return c.SendStatus(200)
		}
		return c.SendStatus(422)
	})

	resp, _ := app.Test(req)
	if resp.StatusCode != 422 {
		t.Errorf("期望状态码 422，实际 %d", resp.StatusCode)
	}
}

func TestParseQueryAndValidate_Success(t *testing.T) {
	app := fiber.New()

	req := httptest.NewRequest(http.MethodGet, "/?name=王五&email=wangwu@example.com&age=25", nil)

	type FiberQueryUser struct {
		Name  string `query:"name" validate:"required"`
		Email string `query:"email" validate:"required,email"`
		Age   int    `query:"age" validate:"gte=18"`
	}

	app.Get("/", func(c fiber.Ctx) error {
		result, err := validate.ParseQueryAndValidate[FiberQueryUser](c)
		if err != nil {
			return c.SendStatus(400)
		}
		if result.Name != "王五" {
			return c.SendStatus(422)
		}
		return c.SendStatus(200)
	})

	resp, _ := app.Test(req)
	if resp.StatusCode != 200 {
		t.Errorf("期望状态码 200，实际 %d", resp.StatusCode)
	}
}

func TestParseAndValidateWithCustom_Success(t *testing.T) {
	app := fiber.New()

	customValidator := validate.NewValidator()

	user := FiberTestUser{
		Name:  "赵六",
		Email: "zhaoliu@example.com",
		Age:   30,
	}
	body, _ := json.Marshal(user)
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	app.Post("/", func(c fiber.Ctx) error {
		_, err := validate.ParseAndValidateWithCustom[FiberTestUser](c, customValidator)
		if err != nil {
			return c.SendStatus(400)
		}
		return c.SendStatus(200)
	})

	resp, _ := app.Test(req)
	if resp.StatusCode != 200 {
		t.Errorf("期望状态码 200，实际 %d", resp.StatusCode)
	}
}

func TestParseQueryAndValidateWithCustom_Success(t *testing.T) {
	app := fiber.New()

	customValidator := validate.NewValidator()
	req := httptest.NewRequest(http.MethodGet, "/?name=孙七&email=sunqi@example.com&age=35", nil)

	type FiberQueryUser struct {
		Name  string `query:"name" validate:"required"`
		Email string `query:"email" validate:"required,email"`
		Age   int    `query:"age" validate:"gte=18"`
	}

	app.Get("/", func(c fiber.Ctx) error {
		_, err := validate.ParseQueryAndValidateWithCustom[FiberQueryUser](c, customValidator)
		if err != nil {
			return c.SendStatus(400)
		}
		return c.SendStatus(200)
	})

	resp, _ := app.Test(req)
	if resp.StatusCode != 200 {
		t.Errorf("期望状态码 200，实际 %d", resp.StatusCode)
	}
}
