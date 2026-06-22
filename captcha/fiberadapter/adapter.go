package fiberadapter

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/gofiber/fiber/v3"
	"github.com/wuwuseo/cmf/captcha"
)

const (
	DefaultTokenHeader   = "X-Captcha-Token"
	DefaultSceneLocalKey = "captcha.scene"
)

type Service interface {
	CreateChallenge(context.Context, string, captcha.ClientMeta) (captcha.ChallengeResult, error)
	VerifyChallenge(context.Context, string, json.RawMessage, captcha.ClientMeta) (captcha.ProofResult, error)
	ConsumeProof(context.Context, string, string) error
}

type ErrorHandler func(c fiber.Ctx, code captcha.ErrorCode, err error) error
type SuccessHandler func(c fiber.Ctx, data any) error
type TokenLookup func(c fiber.Ctx) string
type ClientMetaLookup func(c fiber.Ctx) captcha.ClientMeta

type Option func(*Adapter)

type Adapter struct {
	service          Service
	errorHandler     ErrorHandler
	successHandler   SuccessHandler
	tokenLookup      TokenLookup
	clientMetaLookup ClientMetaLookup
	sceneLocalKey    string
}

func New(service Service, options ...Option) *Adapter {
	adapter := &Adapter{
		service: service,
		errorHandler: func(c fiber.Ctx, code captcha.ErrorCode, _ error) error {
			return c.Status(statusFor(code)).JSON(map[string]any{"code": code})
		},
		successHandler: func(c fiber.Ctx, data any) error {
			return c.JSON(data)
		},
		tokenLookup: func(c fiber.Ctx) string {
			return c.Get(DefaultTokenHeader)
		},
		clientMetaLookup: func(c fiber.Ctx) captcha.ClientMeta {
			return captcha.ClientMeta{IP: c.IP(), UserAgent: c.Get("User-Agent")}
		},
		sceneLocalKey: DefaultSceneLocalKey,
	}
	for _, option := range options {
		option(adapter)
	}
	return adapter
}

func WithErrorHandler(handler ErrorHandler) Option {
	return func(adapter *Adapter) {
		if handler != nil {
			adapter.errorHandler = handler
		}
	}
}

func WithSuccessHandler(handler SuccessHandler) Option {
	return func(adapter *Adapter) {
		if handler != nil {
			adapter.successHandler = handler
		}
	}
}

func WithTokenLookup(lookup TokenLookup) Option {
	return func(adapter *Adapter) {
		if lookup != nil {
			adapter.tokenLookup = lookup
		}
	}
}

func WithClientMetaLookup(lookup ClientMetaLookup) Option {
	return func(adapter *Adapter) {
		if lookup != nil {
			adapter.clientMetaLookup = lookup
		}
	}
}

func WithSceneLocalKey(key string) Option {
	return func(adapter *Adapter) {
		if key != "" {
			adapter.sceneLocalKey = key
		}
	}
}

func (a *Adapter) ChallengeHandler() fiber.Handler {
	type request struct {
		Scene string `json:"scene"`
	}
	return func(c fiber.Ctx) error {
		var input request
		if err := c.Bind().Body(&input); err != nil || input.Scene == "" {
			return a.errorHandler(c, captcha.CodeSceneInvalid, captcha.ErrSceneInvalid)
		}
		result, err := a.service.CreateChallenge(c.Context(), input.Scene, a.clientMetaLookup(c))
		if err != nil {
			return a.errorHandler(c, captcha.CodeOf(err), err)
		}
		return a.successHandler(c, result)
	}
}

func (a *Adapter) VerificationHandler() fiber.Handler {
	type request struct {
		ChallengeID string          `json:"challenge_id"`
		Response    json.RawMessage `json:"response"`
	}
	return func(c fiber.Ctx) error {
		var input request
		if err := c.Bind().Body(&input); err != nil || input.ChallengeID == "" {
			return a.errorHandler(c, captcha.CodeInvalid, captcha.ErrInvalid)
		}
		result, err := a.service.VerifyChallenge(
			c.Context(),
			input.ChallengeID,
			input.Response,
			a.clientMetaLookup(c),
		)
		if err != nil {
			return a.errorHandler(c, captcha.CodeOf(err), err)
		}
		return a.successHandler(c, result)
	}
}

func (a *Adapter) Require(scene string) fiber.Handler {
	return func(c fiber.Ctx) error {
		err := a.service.ConsumeProof(c.Context(), scene, a.tokenLookup(c))
		if err != nil {
			return a.errorHandler(c, captcha.CodeOf(err), err)
		}
		c.Locals(a.sceneLocalKey, scene)
		return c.Next()
	}
}

func statusFor(code captcha.ErrorCode) int {
	switch code {
	case captcha.CodeUnavailable:
		return http.StatusServiceUnavailable
	case captcha.CodeMisconfigured:
		return http.StatusInternalServerError
	default:
		return http.StatusBadRequest
	}
}
