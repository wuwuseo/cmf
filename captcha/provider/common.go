package provider

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math/big"
	"strings"

	"github.com/wuwuseo/cmf/captcha"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

const (
	defaultWidth  = 180
	defaultHeight = 64
)

type imagePayload struct {
	ImageBase64 string `json:"image_base64"`
	MIMEType    string `json:"mime_type"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
}

type textResponse struct {
	Answer string `json:"answer"`
}

type answerState struct {
	Salt   string `json:"salt"`
	Digest string `json:"digest"`
}

type textConfig struct {
	Secret string `json:"secret"`
	Length int    `json:"length"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

func decodeTextConfig(raw json.RawMessage, defaultLength int) (textConfig, error) {
	config := textConfig{Length: defaultLength, Width: defaultWidth, Height: defaultHeight}
	if err := json.Unmarshal(raw, &config); err != nil {
		return textConfig{}, captcha.ErrProviderMisconfigured
	}
	if len(config.Secret) < 16 || config.Length < 1 || config.Length > 12 {
		return textConfig{}, captcha.ErrProviderMisconfigured
	}
	if config.Width < 100 || config.Width > 800 || config.Height < 40 || config.Height > 300 {
		return textConfig{}, captcha.ErrProviderMisconfigured
	}
	return config, nil
}

func newAnswerState(answer, secret string) (answerState, error) {
	saltBytes := make([]byte, 16)
	if _, err := rand.Read(saltBytes); err != nil {
		return answerState{}, err
	}
	salt := base64.RawURLEncoding.EncodeToString(saltBytes)
	return answerState{Salt: salt, Digest: answerDigest(answer, secret, salt)}, nil
}

func verifyTextAnswer(config textConfig, stateRaw, responseRaw json.RawMessage, normalize func(string) string) error {
	var state answerState
	if err := json.Unmarshal(stateRaw, &state); err != nil || state.Salt == "" || state.Digest == "" {
		return captcha.ErrInvalid
	}
	var response textResponse
	if err := json.Unmarshal(responseRaw, &response); err != nil {
		return captcha.ErrInvalid
	}
	got := answerDigest(normalize(response.Answer), config.Secret, state.Salt)
	if subtle.ConstantTimeCompare([]byte(got), []byte(state.Digest)) != 1 {
		return captcha.ErrInvalid
	}
	return nil
}

func answerDigest(answer, secret, salt string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(salt))
	_, _ = mac.Write([]byte{0})
	_, _ = mac.Write([]byte(answer))
	return hex.EncodeToString(mac.Sum(nil))
}

func renderTextPNG(text string, width, height int) (json.RawMessage, error) {
	return renderTextPNGWithFont(text, width, height, ChineseFont{})
}

func renderTextPNGWithFont(
	text string,
	width, height int,
	fontConfig ChineseFont,
) (json.RawMessage, error) {
	face, err := newFontFace(fontConfig, textFontSize(text, width, height))
	if err != nil {
		return nil, fmt.Errorf("create captcha font face: %w", err)
	}
	defer face.Close()

	canvas := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(canvas, canvas.Bounds(), image.NewUniform(color.RGBA{245, 247, 250, 255}), image.Point{}, draw.Src)
	drawNoise(canvas, 5)
	drawCenteredText(canvas, face, text, width/2, height/2, color.RGBA{38, 56, 74, 255})
	return encodeImagePayload(canvas, width, height)
}

func renderClickPNG(
	characters []rune,
	width, height, columns, rows int,
	fontConfig ChineseFont,
) (imagePayload, error) {
	face, err := newFontFace(fontConfig, float64(height/rows)*0.38)
	if err != nil {
		return imagePayload{}, fmt.Errorf("create captcha font face: %w", err)
	}
	defer face.Close()

	canvas := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(canvas, canvas.Bounds(), image.NewUniform(color.RGBA{245, 247, 250, 255}), image.Point{}, draw.Src)
	gridColor := color.RGBA{168, 176, 186, 90}
	for column := 1; column < columns; column++ {
		x := column * width / columns
		drawLine(canvas, x, 0, x, height-1, gridColor)
	}
	for row := 1; row < rows; row++ {
		y := row * height / rows
		drawLine(canvas, 0, y, width-1, y, gridColor)
	}
	drawNoise(canvas, 8)
	for index, character := range characters {
		column, row := index%columns, index/columns
		centerX := (column*2 + 1) * width / (columns * 2)
		centerY := (row*2 + 1) * height / (rows * 2)
		drawCenteredText(canvas, face, string(character), centerX, centerY, color.RGBA{38, 56, 74, 255})
	}
	raw, err := encodePNG(canvas)
	if err != nil {
		return imagePayload{}, err
	}
	return imagePayload{
		ImageBase64: base64.StdEncoding.EncodeToString(raw),
		MIMEType:    "image/png",
		Width:       width,
		Height:      height,
	}, nil
}

func newFontFace(fontConfig ChineseFont, size float64) (*opentype.Face, error) {
	fontConfig = normalizeChineseFont(fontConfig)
	parsed, err := opentype.Parse(fontConfig.TTF)
	if err != nil {
		return nil, err
	}
	if size < 14 {
		size = 14
	}
	face, err := opentype.NewFace(parsed, &opentype.FaceOptions{
		Size:    size,
		DPI:     72,
		Hinting: font.HintingFull,
	})
	if err != nil {
		return nil, err
	}
	return face.(*opentype.Face), nil
}

func textFontSize(text string, width, height int) float64 {
	size := float64(height) * 0.52
	runes := len([]rune(text))
	if runes > 0 {
		maxSize := float64(width-20) / float64(runes) * 0.85
		if maxSize < size {
			size = maxSize
		}
	}
	return size
}

func drawCenteredText(
	dst draw.Image,
	face font.Face,
	text string,
	centerX, centerY int,
	textColor color.Color,
) {
	width := font.MeasureString(face, text).Ceil()
	metrics := face.Metrics()
	height := (metrics.Ascent + metrics.Descent).Ceil()
	drawer := font.Drawer{
		Dst:  dst,
		Src:  image.NewUniform(textColor),
		Face: face,
		Dot: fixed.P(
			centerX-width/2,
			centerY-height/2+metrics.Ascent.Ceil(),
		),
	}
	drawer.DrawString(text)
}

func drawNoise(dst *image.RGBA, lineCount int) {
	palette := []color.RGBA{
		{128, 144, 160, 100},
		{90, 125, 160, 90},
		{170, 120, 120, 80},
	}
	for range lineCount {
		x0, err0 := randomIndex(dst.Bounds().Dx())
		y0, err1 := randomIndex(dst.Bounds().Dy())
		x1, err2 := randomIndex(dst.Bounds().Dx())
		y1, err3 := randomIndex(dst.Bounds().Dy())
		colorIndex, err4 := randomIndex(len(palette))
		if err0 != nil || err1 != nil || err2 != nil || err3 != nil || err4 != nil {
			return
		}
		drawLine(dst, x0, y0, x1, y1, palette[colorIndex])
	}
}

func drawLine(dst *image.RGBA, x0, y0, x1, y1 int, lineColor color.Color) {
	dx, dy := absInt(x1-x0), -absInt(y1-y0)
	stepX, stepY := -1, -1
	if x0 < x1 {
		stepX = 1
	}
	if y0 < y1 {
		stepY = 1
	}
	err := dx + dy
	for {
		if image.Pt(x0, y0).In(dst.Bounds()) {
			dst.Set(x0, y0, lineColor)
		}
		if x0 == x1 && y0 == y1 {
			return
		}
		doubleErr := 2 * err
		if doubleErr >= dy {
			err += dy
			x0 += stepX
		}
		if doubleErr <= dx {
			err += dx
			y0 += stepY
		}
	}
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func encodeImagePayload(canvas image.Image, width, height int) (json.RawMessage, error) {
	raw, err := encodePNG(canvas)
	if err != nil {
		return nil, err
	}
	return json.Marshal(imagePayload{
		ImageBase64: base64.StdEncoding.EncodeToString(raw),
		MIMEType:    "image/png",
		Width:       width,
		Height:      height,
	})
}

func encodePNG(canvas image.Image) ([]byte, error) {
	var output bytes.Buffer
	if err := png.Encode(&output, canvas); err != nil {
		return nil, fmt.Errorf("encode captcha PNG: %w", err)
	}
	return output.Bytes(), nil
}

func randomIndex(limit int) (int, error) {
	if limit <= 0 {
		return 0, fmt.Errorf("random limit must be positive")
	}
	value, err := rand.Int(rand.Reader, big.NewInt(int64(limit)))
	if err != nil {
		return 0, err
	}
	return int(value.Int64()), nil
}

func randomRunes(characters []rune, length int) (string, error) {
	output := make([]rune, length)
	for i := range output {
		index, err := randomIndex(len(characters))
		if err != nil {
			return "", err
		}
		output[i] = characters[index]
	}
	return string(output), nil
}

func normalizeUpper(value string) string {
	return strings.ToUpper(strings.TrimSpace(value))
}

func normalizeTrim(value string) string {
	return strings.TrimSpace(value)
}
