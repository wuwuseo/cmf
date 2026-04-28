package validate

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"

	"github.com/go-playground/validator/v10"
)

// MessageProvider 自定义错误消息提供者
// 若结构体实现此接口，可在验证失败时返回自定义错误消息
type MessageProvider interface {
	GetMessagesMap() ValidatorMessages
}

// ValidatorMessages 验证消息映射
type ValidatorMessages map[string]string

// Validate 全局验证器实例（向后兼容）
var Validate *validator.Validate

func init() {
	Validate = validator.New(validator.WithRequiredStructEnabled())
}

// Validator 验证器
type Validator struct {
	validate *validator.Validate
	mu       sync.RWMutex
	messages map[string]string
}

// NewValidator 创建验证器实例
func NewValidator() *Validator {
	v := &Validator{
		validate: validator.New(validator.WithRequiredStructEnabled()),
		messages: make(map[string]string),
	}
	return v
}

// Instance 获取底层 validator.Validate 实例
func (v *Validator) Instance() *validator.Validate {
	return v.validate
}

// ValidateStruct 验证结构体
func (v *Validator) ValidateStruct(s interface{}) error {
	return v.validate.Struct(s)
}

// RegisterValidation 注册自定义验证规则
func (v *Validator) RegisterValidation(tag string, fn validator.Func) error {
	return v.validate.RegisterValidation(tag, fn)
}

// SetDefaultMessage 为指定 tag 设置默认错误消息
func (v *Validator) SetDefaultMessage(tag, message string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.messages[tag] = message
}

// ValidationError 验证错误
type ValidationError struct {
	Field   string `json:"field"`
	Tag     string `json:"tag"`
	Message string `json:"message"`
	Value   any    `json:"value,omitempty"`
}

// Error 实现 error 接口
func (e ValidationError) Error() string {
	return e.Message
}

// ValidationErrors 多个验证错误
type ValidationErrors []ValidationError

// Error 实现 error 接口
func (e ValidationErrors) Error() string {
	msgs := make([]string, len(e))
	for i, v := range e {
		msgs[i] = v.Message
	}
	return strings.Join(msgs, "; ")
}

// FormatValidationErrors 格式化验证错误为友好的响应格式
func FormatValidationErrors(err error) []ValidationError {
	var validationErrors validator.ValidationErrors
	if !errors.As(err, &validationErrors) {
		return nil
	}

	result := make([]ValidationError, 0, len(validationErrors))
	for _, ve := range validationErrors {
		result = append(result, ValidationError{
			Field:   ve.Field(),
			Tag:     ve.Tag(),
			Message: getDefaultErrorMessage(ve),
			Value:   ve.Value(),
		})
	}
	return result
}

// FormatValidationErrorsWithInput 根据输入结构体的自定义消息格式化验证错误
func FormatValidationErrorsWithInput(input any, err error) []ValidationError {
	var validationErrors validator.ValidationErrors
	if !errors.As(err, &validationErrors) {
		return nil
	}

	var customMsgs ValidatorMessages
	if mp, ok := input.(MessageProvider); ok {
		customMsgs = mp.GetMessagesMap()
	}

	result := make([]ValidationError, 0, len(validationErrors))
	for _, ve := range validationErrors {
		msg := getErrorMessage(ve, customMsgs)
		result = append(result, ValidationError{
			Field:   ve.Field(),
			Tag:     ve.Tag(),
			Message: msg,
			Value:   ve.Value(),
		})
	}
	return result
}

// GetValidatorErrorMsg 获取验证错误信息（字符串形式，向后兼容）
func GetValidatorErrorMsg(input any, err error) string {
	errs := FormatValidationErrorsWithInput(input, err)
	if len(errs) == 0 {
		return "Parameter error"
	}

	msgs := make([]string, len(errs))
	for i, e := range errs {
		msgs[i] = e.Message
	}
	return strings.Join(msgs, ",")
}

// getErrorMessage 获取单条错误消息（优先自定义消息）
func getErrorMessage(fe validator.FieldError, customMsgs ValidatorMessages) string {
	if customMsgs != nil {
		if msg, ok := customMsgs[fe.Field()+"."+fe.Tag()]; ok {
			return msg
		}
	}
	return getDefaultErrorMessage(fe)
}

// getDefaultErrorMessage 获取默认中文错误消息
func getDefaultErrorMessage(fe validator.FieldError) string {
	field := fe.Field()
	tag := fe.Tag()
	param := fe.Param()

	switch tag {
	case "required":
		return fmt.Sprintf("%s为必填字段", field)
	case "required_if":
		return fmt.Sprintf("%s为必填字段", field)
	case "required_unless":
		return fmt.Sprintf("%s为必填字段", field)
	case "required_with":
		return fmt.Sprintf("%s为必填字段", field)
	case "required_without":
		return fmt.Sprintf("%s为必填字段", field)
	case "email":
		return fmt.Sprintf("%s必须是有效的邮箱地址", field)
	case "url":
		return fmt.Sprintf("%s必须是有效的URL地址", field)
	case "min":
		if isStringOrSlice(fe.Kind()) {
			return fmt.Sprintf("%s长度不能小于%s", field, param)
		}
		return fmt.Sprintf("%s不能小于%s", field, param)
	case "max":
		if isStringOrSlice(fe.Kind()) {
			return fmt.Sprintf("%s长度不能大于%s", field, param)
		}
		return fmt.Sprintf("%s不能大于%s", field, param)
	case "len":
		return fmt.Sprintf("%s长度必须为%s", field, param)
	case "eq":
		return fmt.Sprintf("%s必须等于%s", field, param)
	case "ne":
		return fmt.Sprintf("%s不能等于%s", field, param)
	case "gt":
		return fmt.Sprintf("%s必须大于%s", field, param)
	case "gte":
		return fmt.Sprintf("%s必须大于或等于%s", field, param)
	case "lt":
		return fmt.Sprintf("%s必须小于%s", field, param)
	case "lte":
		return fmt.Sprintf("%s必须小于或等于%s", field, param)
	case "alpha":
		return fmt.Sprintf("%s只能包含字母", field)
	case "alphanum":
		return fmt.Sprintf("%s只能包含字母和数字", field)
	case "numeric":
		return fmt.Sprintf("%s必须是数字", field)
	case "number":
		return fmt.Sprintf("%s必须是有效的数字", field)
	case "uuid":
		return fmt.Sprintf("%s必须是有效的UUID", field)
	case "uuid3":
		return fmt.Sprintf("%s必须是有效的UUID v3", field)
	case "uuid4":
		return fmt.Sprintf("%s必须是有效的UUID v4", field)
	case "uuid5":
		return fmt.Sprintf("%s必须是有效的UUID v5", field)
	case "ip":
		return fmt.Sprintf("%s必须是有效的IP地址", field)
	case "ipv4":
		return fmt.Sprintf("%s必须是有效的IPv4地址", field)
	case "ipv6":
		return fmt.Sprintf("%s必须是有效的IPv6地址", field)
	case "tcp_addr":
		return fmt.Sprintf("%s必须是有效的TCP地址", field)
	case "mac":
		return fmt.Sprintf("%s必须是有效的MAC地址", field)
	case "datetime":
		return fmt.Sprintf("%s必须是有效的日期时间格式(%s)", field, param)
	case "date":
		return fmt.Sprintf("%s必须是有效的日期格式", field)
	case "time":
		return fmt.Sprintf("%s必须是有效的时间格式", field)
	case "json":
		return fmt.Sprintf("%s必须是有效的JSON格式", field)
	case "ascii":
		return fmt.Sprintf("%s只能包含ASCII字符", field)
	case "printascii":
		return fmt.Sprintf("%s只能包含可打印的ASCII字符", field)
	case "contains":
		return fmt.Sprintf("%s必须包含%s", field, param)
	case "containsany":
		return fmt.Sprintf("%s必须包含以下任意字符%s", field, param)
	case "excludes":
		return fmt.Sprintf("%s不能包含%s", field, param)
	case "excludesall":
		return fmt.Sprintf("%s不能包含以下任意字符%s", field, param)
	case "startswith":
		return fmt.Sprintf("%s必须以%s开头", field, param)
	case "endswith":
		return fmt.Sprintf("%s必须以%s结尾", field, param)
	case "lowercase":
		return fmt.Sprintf("%s必须为小写", field)
	case "uppercase":
		return fmt.Sprintf("%s必须为大写", field)
	case "oneof":
		return fmt.Sprintf("%s必须是以下之一：%s", field, param)
	case "gtefield":
		return fmt.Sprintf("%s必须大于或等于%s", field, param)
	case "ltefield":
		return fmt.Sprintf("%s必须小于或等于%s", field, param)
	case "gtfield":
		return fmt.Sprintf("%s必须大于%s", field, param)
	case "ltfield":
		return fmt.Sprintf("%s必须小于%s", field, param)
	case "eqfield":
		return fmt.Sprintf("%s必须等于%s", field, param)
	case "nefield":
		return fmt.Sprintf("%s不能等于%s", field, param)
	case "base64":
		return fmt.Sprintf("%s必须是有效的Base64编码", field)
	case "base64url":
		return fmt.Sprintf("%s必须是有效的Base64URL编码", field)
	case "btc_addr":
		return fmt.Sprintf("%s必须是有效的比特币地址", field)
	case "btc_addr_bech32":
		return fmt.Sprintf("%s必须是有效的Bech32比特币地址", field)
	case "eth_addr":
		return fmt.Sprintf("%s必须是有效的以太坊地址", field)
	case "latitude":
		return fmt.Sprintf("%s必须是有效的纬度", field)
	case "longitude":
		return fmt.Sprintf("%s必须是有效的经度", field)
	case "semver":
		return fmt.Sprintf("%s必须是有效的语义化版本", field)
	case "ulid":
		return fmt.Sprintf("%s必须是有效的ULID", field)
	case "md5":
		return fmt.Sprintf("%s必须是有效的MD5哈希", field)
	case "sha256":
		return fmt.Sprintf("%s必须是有效的SHA256哈希", field)
	case "sha384":
		return fmt.Sprintf("%s必须是有效的SHA384哈希", field)
	case "sha512":
		return fmt.Sprintf("%s必须是有效的SHA512哈希", field)
	case "ripemd128":
		return fmt.Sprintf("%s必须是有效的RIPEMD128哈希", field)
	case "ripemd160":
		return fmt.Sprintf("%s必须是有效的RIPEMD160哈希", field)
	case "hsla":
		return fmt.Sprintf("%s必须是有效的HSLA颜色值", field)
	case "hsl":
		return fmt.Sprintf("%s必须是有效的HSL颜色值", field)
	case "rgba":
		return fmt.Sprintf("%s必须是有效的RGBA颜色值", field)
	case "rgb":
		return fmt.Sprintf("%s必须是有效的RGB颜色值", field)
	case "hexcolor":
		return fmt.Sprintf("%s必须是有效的十六进制颜色值", field)
	case "isbn":
		return fmt.Sprintf("%s必须是有效的ISBN", field)
	case "isbn10":
		return fmt.Sprintf("%s必须是有效的ISBN-10", field)
	case "isbn13":
		return fmt.Sprintf("%s必须是有效的ISBN-13", field)
	case "ean":
		return fmt.Sprintf("%s必须是有效的EAN", field)
	case "ean13":
		return fmt.Sprintf("%s必须是有效的EAN-13", field)
	case "ean8":
		return fmt.Sprintf("%s必须是有效的EAN-8", field)
	case "isin":
		return fmt.Sprintf("%s必须是有效的ISIN", field)
	case "dive", "omitempty":
		return ""
	default:
		// 对于未知 tag，返回英文原始错误
		return fe.Error()
	}
}

func isStringOrSlice(k reflect.Kind) bool {
	return k == reflect.String || k == reflect.Slice || k == reflect.Array || k == reflect.Map
}
