package provider

import (
	_ "embed"
	"regexp"
)

const defaultChineseFontFamily = "CMFCaptchaChinese"

//go:embed assets/noto_sans_sc_subset.ttf
var defaultChineseFontTTF []byte

var validFontFamily = regexp.MustCompile(`^[A-Za-z0-9 _-]{1,64}$`)

type ChineseFont struct {
	Family string
	TTF    []byte
}

type ChineseOption func(*chineseOptions)

type chineseOptions struct {
	font ChineseFont
}

func WithChineseFont(font ChineseFont) ChineseOption {
	return func(options *chineseOptions) {
		options.font = normalizeChineseFont(font)
	}
}

func defaultChineseOptions(options ...ChineseOption) chineseOptions {
	value := chineseOptions{font: ChineseFont{
		Family: defaultChineseFontFamily,
		TTF:    defaultChineseFontTTF,
	}}
	for _, option := range options {
		if option != nil {
			option(&value)
		}
	}
	return value
}

func normalizeChineseFont(font ChineseFont) ChineseFont {
	if !validFontFamily.MatchString(font.Family) {
		font.Family = defaultChineseFontFamily
	}
	if len(font.TTF) == 0 {
		font.TTF = defaultChineseFontTTF
	}
	return font
}
