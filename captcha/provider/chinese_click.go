package provider

import (
	"context"
	"encoding/json"
	"math"
	"strings"

	"github.com/wuwuseo/cmf/captcha"
)

const chineseClickName = "image_chinese_click"

type ChineseClick struct {
	font ChineseFont
}

type clickConfig struct {
	Secret       string  `json:"secret"`
	Characters   string  `json:"characters"`
	DisplayCount int     `json:"display_count"`
	PointCount   int     `json:"point_count"`
	Tolerance    float64 `json:"tolerance"`
	Width        int     `json:"width"`
	Height       int     `json:"height"`
}

type point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type clickState struct {
	Targets   []point `json:"targets"`
	Tolerance float64 `json:"tolerance"`
}

type clickResponse struct {
	Points []point `json:"points"`
}

type clickPayload struct {
	imagePayload
	Prompt     string `json:"prompt"`
	PointCount int    `json:"point_count"`
}

func NewChineseClick(options ...ChineseOption) *ChineseClick {
	return &ChineseClick{font: defaultChineseOptions(options...).font}
}
func (*ChineseClick) Name() string { return chineseClickName }

func decodeClickConfig(raw json.RawMessage) (clickConfig, []rune, error) {
	config := clickConfig{
		Characters:   string(defaultChinese),
		DisplayCount: 6,
		PointCount:   3,
		Tolerance:    0.08,
		Width:        300,
		Height:       180,
	}
	if err := json.Unmarshal(raw, &config); err != nil {
		return clickConfig{}, nil, captcha.ErrProviderMisconfigured
	}
	characters := []rune(strings.TrimSpace(config.Characters))
	if len(config.Secret) < 16 || config.DisplayCount < 4 || config.DisplayCount > 12 ||
		config.PointCount < 1 || config.PointCount > config.DisplayCount ||
		len(characters) < config.DisplayCount || config.Tolerance <= 0 || config.Tolerance > 0.2 ||
		config.Width < 180 || config.Height < 100 {
		return clickConfig{}, nil, captcha.ErrProviderMisconfigured
	}
	return config, characters, nil
}

func (*ChineseClick) ValidateConfig(raw json.RawMessage) error {
	_, _, err := decodeClickConfig(raw)
	return err
}

func (p *ChineseClick) Create(_ context.Context, raw json.RawMessage, _ captcha.ClientMeta) (captcha.Challenge, error) {
	config, characters, err := decodeClickConfig(raw)
	if err != nil {
		return captcha.Challenge{}, err
	}
	selected, err := uniqueRunes(characters, config.DisplayCount)
	if err != nil {
		return captcha.Challenge{}, err
	}
	order, err := uniqueIndices(config.DisplayCount, config.PointCount)
	if err != nil {
		return captcha.Challenge{}, err
	}
	columns := 3
	rows := int(math.Ceil(float64(config.DisplayCount) / float64(columns)))
	targets := make([]point, 0, config.PointCount)
	promptRunes := make([]rune, 0, config.PointCount)
	for _, index := range order {
		column, row := index%columns, index/columns
		targets = append(targets, point{
			X: (float64(column) + 0.5) / float64(columns),
			Y: (float64(row) + 0.5) / float64(rows),
		})
		promptRunes = append(promptRunes, selected[index])
	}
	image, err := renderClickPNG(selected, config.Width, config.Height, columns, rows, p.font)
	if err != nil {
		return captcha.Challenge{}, err
	}
	payload, err := json.Marshal(clickPayload{
		imagePayload: image,
		Prompt:       string(promptRunes),
		PointCount:   config.PointCount,
	})
	if err != nil {
		return captcha.Challenge{}, err
	}
	privateState, _ := json.Marshal(clickState{Targets: targets, Tolerance: config.Tolerance})
	return captcha.Challenge{Payload: payload, PrivateState: privateState}, nil
}

func (*ChineseClick) Verify(_ context.Context, raw, stateRaw, responseRaw json.RawMessage, _ captcha.ClientMeta) error {
	if _, _, err := decodeClickConfig(raw); err != nil {
		return err
	}
	var state clickState
	var response clickResponse
	if err := json.Unmarshal(stateRaw, &state); err != nil {
		return captcha.ErrInvalid
	}
	if err := json.Unmarshal(responseRaw, &response); err != nil || len(response.Points) != len(state.Targets) {
		return captcha.ErrInvalid
	}
	for index, target := range state.Targets {
		got := response.Points[index]
		if got.X < 0 || got.X > 1 || got.Y < 0 || got.Y > 1 ||
			math.Abs(got.X-target.X) > state.Tolerance ||
			math.Abs(got.Y-target.Y) > state.Tolerance {
			return captcha.ErrInvalid
		}
	}
	return nil
}

func uniqueRunes(characters []rune, count int) ([]rune, error) {
	indices, err := uniqueIndices(len(characters), count)
	if err != nil {
		return nil, err
	}
	output := make([]rune, count)
	for i, index := range indices {
		output[i] = characters[index]
	}
	return output, nil
}

func uniqueIndices(limit, count int) ([]int, error) {
	available := make([]int, limit)
	for i := range available {
		available[i] = i
	}
	for i := 0; i < count; i++ {
		offset, err := randomIndex(len(available) - i)
		if err != nil {
			return nil, err
		}
		selected := i + offset
		available[i], available[selected] = available[selected], available[i]
	}
	return available[:count], nil
}
