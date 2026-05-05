package validate

import (
	"errors"
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v3"
)

// ParseAndValidate 解析请求体并验证
// 用法: data, err := validate.ParseAndValidate[CreateUserRequest](c)
func ParseAndValidate[T any](c fiber.Ctx) (*T, error) {
	var req T
	if err := c.Bind().Body(&req); err != nil {
		return nil, fmt.Errorf("请求体解析失败: %w", err)
	}

	if err := Validate.Struct(&req); err != nil {
		return nil, err
	}

	return &req, nil
}

// ParseQueryAndValidate 解析查询参数并验证
func ParseQueryAndValidate[T any](c fiber.Ctx) (*T, error) {
	var req T
	if err := c.Bind().Query(&req); err != nil {
		return nil, fmt.Errorf("查询参数解析失败: %w", err)
	}

	if err := Validate.Struct(&req); err != nil {
		return nil, err
	}

	return &req, nil
}

// ParseAndValidateWithCustom 使用自定义验证器解析请求体并验证
func ParseAndValidateWithCustom[T any](c fiber.Ctx, v *Validator) (*T, error) {
	var req T
	if err := c.Bind().Body(&req); err != nil {
		return nil, fmt.Errorf("请求体解析失败: %w", err)
	}

	if err := v.ValidateStruct(&req); err != nil {
		return nil, err
	}

	return &req, nil
}

// ParseQueryAndValidateWithCustom 使用自定义验证器解析查询参数并验证
func ParseQueryAndValidateWithCustom[T any](c fiber.Ctx, v *Validator) (*T, error) {
	var req T
	if err := c.Bind().Query(&req); err != nil {
		return nil, fmt.Errorf("查询参数解析失败: %w", err)
	}

	if err := v.ValidateStruct(&req); err != nil {
		return nil, err
	}

	return &req, nil
}

// IsValidationError 判断错误是否为验证错误
func IsValidationError(err error) bool {
	var validationErrors validator.ValidationErrors
	return errors.As(err, &validationErrors)
}

// FirstErrorMessage 获取验证错误中的第一条错误消息
func FirstErrorMessage(err error) string {
	errs := FormatValidationErrors(err)
	if len(errs) > 0 {
		return errs[0].Message
	}
	if err != nil {
		return err.Error()
	}
	return ""
}

// AllErrorMessages 获取验证错误中的所有错误消息，以分号分隔
func AllErrorMessages(err error) string {
	errs := FormatValidationErrors(err)
	if len(errs) > 0 {
		msgs := make([]string, len(errs))
		for i, e := range errs {
			msgs[i] = e.Message
		}
		return strings.Join(msgs, "; ")
	}
	if err != nil {
		return err.Error()
	}
	return ""
}
