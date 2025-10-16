package validate

import (
	"errors"
	"strings"

	"github.com/go-playground/validator/v10"
)

type Validator interface {
	GetMessagesMap() ValidatorMessages
}

type ValidatorMessages map[string]string

var Validate *validator.Validate

func init() {
	Validate = validator.New(validator.WithRequiredStructEnabled())
}

/**
 * @name: GetValidatorErrorMsg
 *
 * @description: 获取验证错误信息
 * @param {any} input 验证的结构体
 * @param {error} err 验证错误
 * @return {string} 错误信息
 */
func GetValidatorErrorMsg(input any, err error) string {
	var validationErrors validator.ValidationErrors
	if errors.As(err, &validationErrors) {
		_, isValidator := input.(Validator)

		// 收集所有错误信息
		var errorMessages []string

		for _, v := range err.(validator.ValidationErrors) {
			// 若 input 结构体实现 Validator 接口即可实现自定义错误信息
			if isValidator {
				if message, exist := input.(Validator).GetMessagesMap()[v.Field()+"."+v.Tag()]; exist {
					errorMessages = append(errorMessages, message)
					continue
				}
			}
			// 对于未自定义的错误信息，使用默认错误信息
			errorMessages = append(errorMessages, v.Error())
		}

		// 返回所有错误信息，用分号分隔
		if len(errorMessages) > 0 {
			return strings.Join(errorMessages, ",")
		}
	}

	return "Parameter error"
}
