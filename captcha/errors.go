package captcha

import "errors"

var (
	ErrRequired              = errors.New("captcha required")
	ErrInvalid               = errors.New("captcha invalid")
	ErrSceneInvalid          = errors.New("captcha scene invalid")
	ErrProviderUnavailable   = errors.New("captcha provider unavailable")
	ErrProviderMisconfigured = errors.New("captcha provider misconfigured")
	ErrStoreNotFound         = errors.New("captcha record not found")
)

type ErrorCode string

const (
	CodeRequired      ErrorCode = "captcha_required"
	CodeInvalid       ErrorCode = "captcha_invalid"
	CodeSceneInvalid  ErrorCode = "captcha_scene_invalid"
	CodeUnavailable   ErrorCode = "captcha_provider_unavailable"
	CodeMisconfigured ErrorCode = "captcha_provider_misconfigured"
)

func CodeOf(err error) ErrorCode {
	switch {
	case errors.Is(err, ErrRequired):
		return CodeRequired
	case errors.Is(err, ErrSceneInvalid):
		return CodeSceneInvalid
	case errors.Is(err, ErrProviderUnavailable):
		return CodeUnavailable
	case errors.Is(err, ErrProviderMisconfigured):
		return CodeMisconfigured
	default:
		return CodeInvalid
	}
}
