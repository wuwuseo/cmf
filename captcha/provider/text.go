package provider

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/wuwuseo/cmf/captcha"
)

const (
	alphanumericName = "image_alphanumeric"
	mathName         = "image_math"
	chineseInputName = "image_chinese_input"
)

var (
	defaultAlphanumeric = []rune("23456789ABCDEFGHJKLMNPQRSTUVWXYZ")
	defaultChinese      = []rune("山水火木日月天地人中云风雨花石田川海春夏秋冬金土上下左右大小多少白黑红蓝绿黄")
)

type Alphanumeric struct{}

func NewAlphanumeric() *Alphanumeric { return &Alphanumeric{} }
func (*Alphanumeric) Name() string   { return alphanumericName }

func (*Alphanumeric) Descriptor() captcha.ProviderDescriptor {
	fields := []captcha.ProviderFieldDescriptor{
		{
			Key: "characters", Type: fieldTypeTextarea,
			Label:        localized("字符集合", "Character set"),
			Help:         localized("默认使用纯数字；至少提供 8 个不同字符。", "Defaults to digits; provide at least 8 distinct characters."),
			DefaultValue: "0123456789", Required: true, MinLength: 8,
		},
		{
			Key: "length", Type: fieldTypeNumber,
			Label:        localized("验证码长度", "Captcha length"),
			DefaultValue: 4, Required: true,
			Min: numberPointer(1), Max: numberPointer(12), Step: numberPointer(1),
		},
	}
	fields = append(fields, imageSizeFields(defaultWidth, defaultHeight, 100, 40)...)
	return captcha.ProviderDescriptor{
		DisplayName:   localized("数字/字符图片", "Numeric / text image"),
		ChallengeType: captcha.ChallengeTypeImageText,
		Secret:        imageSecretDescriptor(),
		Fields:        fields,
	}
}

func (*Alphanumeric) ValidateConfig(raw json.RawMessage) error {
	var extended struct {
		Characters string `json:"characters"`
	}
	if _, err := decodeTextConfig(raw, 5); err != nil {
		return err
	}
	if err := json.Unmarshal(raw, &extended); err != nil {
		return captcha.ErrProviderMisconfigured
	}
	if extended.Characters != "" && len([]rune(extended.Characters)) < 8 {
		return captcha.ErrProviderMisconfigured
	}
	return nil
}

func (p *Alphanumeric) Create(_ context.Context, raw json.RawMessage, _ captcha.ClientMeta) (captcha.Challenge, error) {
	config, err := decodeTextConfig(raw, 5)
	if err != nil {
		return captcha.Challenge{}, err
	}
	var extended struct {
		Characters string `json:"characters"`
	}
	_ = json.Unmarshal(raw, &extended)
	characters := defaultAlphanumeric
	if extended.Characters != "" {
		characters = []rune(extended.Characters)
	}
	answer, err := randomRunes(characters, config.Length)
	if err != nil {
		return captcha.Challenge{}, err
	}
	state, err := newAnswerState(normalizeUpper(answer), config.Secret)
	if err != nil {
		return captcha.Challenge{}, err
	}
	payload, err := renderTextPNG(answer, config.Width, config.Height)
	if err != nil {
		return captcha.Challenge{}, err
	}
	privateState, _ := json.Marshal(state)
	return captcha.Challenge{Payload: payload, PrivateState: privateState}, nil
}

func (*Alphanumeric) Verify(_ context.Context, raw, state, response json.RawMessage, _ captcha.ClientMeta) error {
	config, err := decodeTextConfig(raw, 5)
	if err != nil {
		return err
	}
	return verifyTextAnswer(config, state, response, normalizeUpper)
}

type Math struct{}

type mathConfig struct {
	textConfig
	Operators  []string `json:"operators"`
	MaxOperand int      `json:"max_operand"`
}

func NewMath() *Math       { return &Math{} }
func (*Math) Name() string { return mathName }

func (*Math) Descriptor() captcha.ProviderDescriptor {
	fields := []captcha.ProviderFieldDescriptor{
		{
			Key: "operators", Type: fieldTypeMultiSelect,
			Label:        localized("运算符", "Operators"),
			DefaultValue: []string{"+", "-"}, Required: true,
			Options: []captcha.ProviderFieldOption{
				{Value: "+", Label: localized("加法", "Addition")},
				{Value: "-", Label: localized("减法", "Subtraction")},
				{Value: "*", Label: localized("乘法", "Multiplication")},
			},
		},
		{
			Key: "max_operand", Type: fieldTypeNumber,
			Label:        localized("最大操作数", "Maximum operand"),
			DefaultValue: 20, Required: true,
			Min: numberPointer(2), Max: numberPointer(100), Step: numberPointer(1),
		},
	}
	fields = append(fields, imageSizeFields(defaultWidth, defaultHeight, 100, 40)...)
	return captcha.ProviderDescriptor{
		DisplayName:   localized("算术图片", "Math image"),
		ChallengeType: captcha.ChallengeTypeImageText,
		Secret:        imageSecretDescriptor(),
		Fields:        fields,
	}
}

func decodeMathConfig(raw json.RawMessage) (mathConfig, error) {
	base, err := decodeTextConfig(raw, 1)
	if err != nil {
		return mathConfig{}, err
	}
	config := mathConfig{textConfig: base, Operators: []string{"+", "-"}, MaxOperand: 20}
	if err := json.Unmarshal(raw, &config); err != nil {
		return mathConfig{}, captcha.ErrProviderMisconfigured
	}
	if config.MaxOperand < 2 || config.MaxOperand > 100 || len(config.Operators) == 0 {
		return mathConfig{}, captcha.ErrProviderMisconfigured
	}
	for _, operator := range config.Operators {
		if operator != "+" && operator != "-" && operator != "*" {
			return mathConfig{}, captcha.ErrProviderMisconfigured
		}
	}
	return config, nil
}

func (*Math) ValidateConfig(raw json.RawMessage) error {
	_, err := decodeMathConfig(raw)
	return err
}

func (p *Math) Create(_ context.Context, raw json.RawMessage, _ captcha.ClientMeta) (captcha.Challenge, error) {
	config, err := decodeMathConfig(raw)
	if err != nil {
		return captcha.Challenge{}, err
	}
	left, err := randomIndex(config.MaxOperand)
	if err != nil {
		return captcha.Challenge{}, err
	}
	right, err := randomIndex(config.MaxOperand)
	if err != nil {
		return captcha.Challenge{}, err
	}
	operatorIndex, err := randomIndex(len(config.Operators))
	if err != nil {
		return captcha.Challenge{}, err
	}
	operator := config.Operators[operatorIndex]
	if operator == "-" && left < right {
		left, right = right, left
	}
	result := left + right
	if operator == "-" {
		result = left - right
	} else if operator == "*" {
		result = left * right
	}
	answer := strconv.Itoa(result)
	state, err := newAnswerState(answer, config.Secret)
	if err != nil {
		return captcha.Challenge{}, err
	}
	payload, err := renderTextPNG(strconv.Itoa(left)+" "+operator+" "+strconv.Itoa(right)+" = ?", config.Width, config.Height)
	if err != nil {
		return captcha.Challenge{}, err
	}
	privateState, _ := json.Marshal(state)
	return captcha.Challenge{Payload: payload, PrivateState: privateState}, nil
}

func (*Math) Verify(_ context.Context, raw, state, response json.RawMessage, _ captcha.ClientMeta) error {
	config, err := decodeMathConfig(raw)
	if err != nil {
		return err
	}
	return verifyTextAnswer(config.textConfig, state, response, normalizeTrim)
}

type ChineseInput struct {
	font ChineseFont
}

func NewChineseInput(options ...ChineseOption) *ChineseInput {
	return &ChineseInput{font: defaultChineseOptions(options...).font}
}
func (*ChineseInput) Name() string { return chineseInputName }

func (*ChineseInput) Descriptor() captcha.ProviderDescriptor {
	fields := []captcha.ProviderFieldDescriptor{
		{
			Key: "characters", Type: fieldTypeTextarea,
			Label:        localized("中文字符集合", "Chinese character set"),
			DefaultValue: string(defaultChinese), Required: true,
		},
		{
			Key: "length", Type: fieldTypeNumber,
			Label:        localized("验证码长度", "Captcha length"),
			DefaultValue: 4, Required: true,
			Min: numberPointer(1), Max: numberPointer(12), Step: numberPointer(1),
		},
	}
	fields = append(fields, imageSizeFields(defaultWidth, defaultHeight, 100, 40)...)
	return captcha.ProviderDescriptor{
		DisplayName:   localized("中文输入图片", "Chinese input image"),
		ChallengeType: captcha.ChallengeTypeImageText,
		Secret:        imageSecretDescriptor(),
		Fields:        fields,
	}
}

func decodeChineseConfig(raw json.RawMessage) (textConfig, []rune, error) {
	config, err := decodeTextConfig(raw, 4)
	if err != nil {
		return textConfig{}, nil, err
	}
	var extended struct {
		Characters string `json:"characters"`
	}
	if err := json.Unmarshal(raw, &extended); err != nil {
		return textConfig{}, nil, captcha.ErrProviderMisconfigured
	}
	characters := defaultChinese
	if strings.TrimSpace(extended.Characters) != "" {
		characters = []rune(extended.Characters)
	}
	if len(characters) < config.Length {
		return textConfig{}, nil, captcha.ErrProviderMisconfigured
	}
	return config, characters, nil
}

func (*ChineseInput) ValidateConfig(raw json.RawMessage) error {
	_, _, err := decodeChineseConfig(raw)
	return err
}

func (p *ChineseInput) Create(_ context.Context, raw json.RawMessage, _ captcha.ClientMeta) (captcha.Challenge, error) {
	config, characters, err := decodeChineseConfig(raw)
	if err != nil {
		return captcha.Challenge{}, err
	}
	answer, err := randomRunes(characters, config.Length)
	if err != nil {
		return captcha.Challenge{}, err
	}
	state, err := newAnswerState(normalizeTrim(answer), config.Secret)
	if err != nil {
		return captcha.Challenge{}, err
	}
	payload, err := renderTextPNGWithFont(answer, config.Width, config.Height, p.font)
	if err != nil {
		return captcha.Challenge{}, err
	}
	privateState, _ := json.Marshal(state)
	return captcha.Challenge{Payload: payload, PrivateState: privateState}, nil
}

func (*ChineseInput) Verify(_ context.Context, raw, state, response json.RawMessage, _ captcha.ClientMeta) error {
	config, _, err := decodeChineseConfig(raw)
	if err != nil {
		return err
	}
	return verifyTextAnswer(config, state, response, normalizeTrim)
}
