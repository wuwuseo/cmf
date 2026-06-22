package fiberadapter

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/wuwuseo/cmf/captcha"
)

type fakeService struct {
	challengeScene string
	verifyID       string
	consumeScene   string
	consumeToken   string
	consumeErr     error
}

func (s *fakeService) CreateChallenge(_ context.Context, scene string, _ captcha.ClientMeta) (captcha.ChallengeResult, error) {
	s.challengeScene = scene
	return captcha.ChallengeResult{Required: true, ChallengeID: "challenge", Provider: "test"}, nil
}

func (s *fakeService) VerifyChallenge(
	_ context.Context,
	id string,
	_ json.RawMessage,
	_ captcha.ClientMeta,
) (captcha.ProofResult, error) {
	s.verifyID = id
	return captcha.ProofResult{Token: "proof"}, nil
}

func (s *fakeService) ConsumeProof(_ context.Context, scene, token string) error {
	s.consumeScene = scene
	s.consumeToken = token
	return s.consumeErr
}

func TestChallengeAndVerificationHandlers(t *testing.T) {
	service := &fakeService{}
	adapter := New(service)
	app := fiber.New()
	app.Post("/challenges", adapter.ChallengeHandler())
	app.Post("/verifications", adapter.VerificationHandler())

	response := performJSON(t, app, "/challenges", `{"scene":"admin_login"}`, nil)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("challenge status = %d", response.StatusCode)
	}
	if service.challengeScene != "admin_login" {
		t.Fatalf("challenge scene = %q", service.challengeScene)
	}

	response = performJSON(t, app, "/verifications", `{"challenge_id":"challenge","response":{"answer":"ok"}}`, nil)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("verification status = %d", response.StatusCode)
	}
	if service.verifyID != "challenge" {
		t.Fatalf("verification id = %q", service.verifyID)
	}
}

func TestRequireReadsHeaderAndCallsNext(t *testing.T) {
	service := &fakeService{}
	adapter := New(service)
	app := fiber.New()
	app.Post("/login", adapter.Require("admin_login"), func(c fiber.Ctx) error {
		if c.Locals(DefaultSceneLocalKey) != "admin_login" {
			t.Fatalf("captcha scene local = %v", c.Locals(DefaultSceneLocalKey))
		}
		return c.SendStatus(http.StatusNoContent)
	})

	response := performJSON(t, app, "/login", `{}`, map[string]string{"X-Captcha-Token": "proof"})
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d", response.StatusCode)
	}
	if service.consumeScene != "admin_login" || service.consumeToken != "proof" {
		t.Fatalf("ConsumeProof(scene, token) = (%q, %q)", service.consumeScene, service.consumeToken)
	}
}

func TestRequireUsesInjectedErrorHandler(t *testing.T) {
	service := &fakeService{consumeErr: captcha.ErrRequired}
	adapter := New(service, WithErrorHandler(func(c fiber.Ctx, code captcha.ErrorCode, _ error) error {
		return c.Status(http.StatusTeapot).JSON(map[string]any{"code": code})
	}))
	app := fiber.New()
	app.Post("/login", adapter.Require("admin_login"), func(c fiber.Ctx) error {
		return c.SendStatus(http.StatusNoContent)
	})

	response := performJSON(t, app, "/login", `{}`, nil)
	if response.StatusCode != http.StatusTeapot {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusTeapot)
	}
	body, _ := io.ReadAll(response.Body)
	if !bytes.Contains(body, []byte(captcha.CodeRequired)) {
		t.Fatalf("body = %s", body)
	}
}

func performJSON(t *testing.T, app *fiber.App, path, body string, headers map[string]string) *http.Response {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	response, err := app.Test(request)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	return response
}
