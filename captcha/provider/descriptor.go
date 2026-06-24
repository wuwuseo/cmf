package provider

import "github.com/wuwuseo/cmf/captcha"

const (
	fieldTypeText        = "text"
	fieldTypeTextarea    = "textarea"
	fieldTypeNumber      = "number"
	fieldTypeMultiSelect = "multi_select"
)

func localized(zhCN, enUS string) captcha.LocalizedText {
	return captcha.LocalizedText{"zh-CN": zhCN, "en-US": enUS}
}

func numberPointer(value float64) *float64 {
	return &value
}

func imageSecretDescriptor() captcha.ProviderSecretDescriptor {
	return captcha.ProviderSecretDescriptor{
		Label:     localized("签名密钥", "Signing secret"),
		Help:      localized("用于保护验证码答案，至少 16 个字符。", "Protects captcha answers; use at least 16 characters."),
		Required:  true,
		MinLength: 16,
	}
}

func imageSizeFields(defaultWidth, defaultHeight, minWidth, minHeight int) []captcha.ProviderFieldDescriptor {
	return []captcha.ProviderFieldDescriptor{
		{
			Key: "width", Type: fieldTypeNumber, Label: localized("图片宽度", "Image width"),
			DefaultValue: defaultWidth, Required: true,
			Min: numberPointer(float64(minWidth)), Max: numberPointer(800), Step: numberPointer(1),
		},
		{
			Key: "height", Type: fieldTypeNumber, Label: localized("图片高度", "Image height"),
			DefaultValue: defaultHeight, Required: true,
			Min: numberPointer(float64(minHeight)), Max: numberPointer(300), Step: numberPointer(1),
		},
	}
}
