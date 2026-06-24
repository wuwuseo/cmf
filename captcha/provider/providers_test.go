package provider

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/wuwuseo/cmf/captcha"
)

const testSecret = "0123456789abcdef0123456789abcdef"

func TestAlphanumericCreateAndVerify(t *testing.T) {
	provider := NewAlphanumeric()
	config := json.RawMessage(`{"secret":"` + testSecret + `","length":2,"characters":"ABCDEFGH"}`)
	challenge, err := provider.Create(t.Context(), config, captcha.ClientMeta{})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	var payload imagePayload
	if err := json.Unmarshal(challenge.Payload, &payload); err != nil {
		t.Fatalf("payload decode error = %v", err)
	}
	if payload.ImageBase64 == "" || payload.MIMEType != "image/png" {
		t.Fatalf("payload = %+v", payload)
	}
	answer := findTextAnswer(t, provider, config, challenge.PrivateState, alphanumericCandidates("ABCDEFGH", 2))
	assertNoPNGTextChunks(t, payloadImage(t, challenge.Payload))
	response, _ := json.Marshal(textResponse{Answer: answer})
	if err := provider.Verify(t.Context(), config, challenge.PrivateState, response, captcha.ClientMeta{}); err != nil {
		t.Fatalf("Verify(correct) error = %v", err)
	}
	if err := provider.Verify(t.Context(), config, challenge.PrivateState, json.RawMessage(`{"answer":"wrong"}`), captcha.ClientMeta{}); !errors.Is(err, captcha.ErrInvalid) {
		t.Fatalf("Verify(wrong) error = %v, want ErrInvalid", err)
	}
}

func alphanumericCandidates(characters string, length int) []string {
	candidates := []string{""}
	for range length {
		next := make([]string, 0, len(candidates)*len(characters))
		for _, prefix := range candidates {
			for _, character := range characters {
				next = append(next, prefix+string(character))
			}
		}
		candidates = next
	}
	return candidates
}

func TestMathCreateProducesVerifiableExpression(t *testing.T) {
	provider := NewMath()
	config := json.RawMessage(`{"secret":"` + testSecret + `","operators":["+"],"max_operand":2}`)
	challenge, err := provider.Create(t.Context(), config, captcha.ClientMeta{})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	answer := findTextAnswer(t, provider, config, challenge.PrivateState, []string{"0", "1", "2"})
	response, _ := json.Marshal(textResponse{Answer: answer})
	if err := provider.Verify(t.Context(), config, challenge.PrivateState, response, captcha.ClientMeta{}); err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
}

func TestChineseInputNormalizesWhitespace(t *testing.T) {
	provider := NewChineseInput()
	config := json.RawMessage(`{"secret":"` + testSecret + `","length":1,"characters":"山水火木日月天地"}`)
	challenge, err := provider.Create(t.Context(), config, captcha.ClientMeta{})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	answer := findTextAnswer(t, provider, config, challenge.PrivateState, []string{
		"山", "水", "火", "木", "日", "月", "天", "地",
	})
	response, _ := json.Marshal(textResponse{Answer: "  " + answer + "  "})
	if err := provider.Verify(t.Context(), config, challenge.PrivateState, response, captcha.ClientMeta{}); err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
}

func TestChineseClickChecksOrderAndNormalizedCoordinates(t *testing.T) {
	provider := NewChineseClick()
	config := json.RawMessage(`{"secret":"` + testSecret + `","characters":"山水火木日月天地人中","display_count":6,"point_count":3,"tolerance":0.08}`)
	challenge, err := provider.Create(t.Context(), config, captcha.ClientMeta{})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	var state clickState
	if err := json.Unmarshal(challenge.PrivateState, &state); err != nil {
		t.Fatalf("state decode error = %v", err)
	}
	points := make([]point, 0, len(state.Targets))
	for _, target := range state.Targets {
		points = append(points, point{X: target.X, Y: target.Y})
	}
	response, _ := json.Marshal(clickResponse{Points: points})
	if err := provider.Verify(t.Context(), config, challenge.PrivateState, response, captcha.ClientMeta{}); err != nil {
		t.Fatalf("Verify(correct) error = %v", err)
	}
	if len(points) >= 2 {
		points[0], points[1] = points[1], points[0]
	}
	response, _ = json.Marshal(clickResponse{Points: points})
	if err := provider.Verify(t.Context(), config, challenge.PrivateState, response, captcha.ClientMeta{}); !errors.Is(err, captcha.ErrInvalid) {
		t.Fatalf("Verify(wrong order) error = %v, want ErrInvalid", err)
	}
}

func TestChineseProvidersRenderPNGWithDefaultFont(t *testing.T) {
	config := json.RawMessage(`{"secret":"` + testSecret + `","length":3,"characters":"山水火木日月天地人中云风雨花石田川海春夏秋冬"}`)
	challenge, err := NewChineseInput().Create(t.Context(), config, captcha.ClientMeta{})
	if err != nil {
		t.Fatalf("ChineseInput.Create() error = %v", err)
	}
	assertPNG(t, challenge.Payload)

	clickConfig := json.RawMessage(`{"secret":"` + testSecret + `","characters":"山水火木日月天地人中云风雨花石田川海春夏秋冬","display_count":6,"point_count":3}`)
	click, err := NewChineseClick().Create(t.Context(), clickConfig, captcha.ClientMeta{})
	if err != nil {
		t.Fatalf("ChineseClick.Create() error = %v", err)
	}
	assertPNG(t, click.Payload)
}

func TestChineseProviderAllowsFontReplacement(t *testing.T) {
	custom := ChineseFont{Family: "CallerCaptchaFont", TTF: []byte("invalid-custom-font")}
	provider := NewChineseInput(WithChineseFont(custom))
	config := json.RawMessage(`{"secret":"` + testSecret + `","length":3,"characters":"山水火木日月天地人中云风雨花石田川海春夏秋冬"}`)
	if _, err := provider.Create(t.Context(), config, captcha.ClientMeta{}); err == nil {
		t.Fatal("Create() ignored the caller-provided font")
	}
}

func TestBuiltInProvidersRejectMissingSecret(t *testing.T) {
	providers := []captcha.Provider{NewAlphanumeric(), NewMath(), NewChineseInput(), NewChineseClick()}
	for _, provider := range providers {
		t.Run(provider.Name(), func(t *testing.T) {
			if err := provider.ValidateConfig(json.RawMessage(`{}`)); !errors.Is(err, captcha.ErrProviderMisconfigured) {
				t.Fatalf("ValidateConfig() error = %v, want ErrProviderMisconfigured", err)
			}
		})
	}
}

func TestBuiltInProvidersExposeConfigurationDescriptors(t *testing.T) {
	tests := []struct {
		provider      captcha.Provider
		challengeType captcha.ChallengeType
	}{
		{NewAlphanumeric(), captcha.ChallengeTypeImageText},
		{NewMath(), captcha.ChallengeTypeImageText},
		{NewChineseInput(), captcha.ChallengeTypeImageText},
		{NewChineseClick(), captcha.ChallengeTypeImageClick},
		{NewGeeTestGT4(nil), captcha.ChallengeTypeGeeTestGT4},
	}

	for _, test := range tests {
		t.Run(test.provider.Name(), func(t *testing.T) {
			described, ok := test.provider.(captcha.DescribedProvider)
			if !ok {
				t.Fatal("provider does not implement DescribedProvider")
			}
			descriptor := described.Descriptor()
			if descriptor.ChallengeType != test.challengeType {
				t.Fatalf("ChallengeType = %q, want %q", descriptor.ChallengeType, test.challengeType)
			}
			if descriptor.DisplayName["zh-CN"] == "" || descriptor.DisplayName["en-US"] == "" {
				t.Fatalf("DisplayName = %#v", descriptor.DisplayName)
			}
			if descriptor.Secret.Label["zh-CN"] == "" || descriptor.Secret.Label["en-US"] == "" {
				t.Fatalf("Secret.Label = %#v", descriptor.Secret.Label)
			}
			if len(descriptor.Fields) == 0 {
				t.Fatal("descriptor has no configuration fields")
			}
			for _, field := range descriptor.Fields {
				if field.Key == "" || field.Type == "" ||
					field.Label["zh-CN"] == "" || field.Label["en-US"] == "" {
					t.Fatalf("invalid field descriptor: %#v", field)
				}
			}
		})
	}
}

func payloadImage(t *testing.T, raw json.RawMessage) []byte {
	t.Helper()
	var payload imagePayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("payload decode error = %v", err)
	}
	image, err := base64.StdEncoding.DecodeString(payload.ImageBase64)
	if err != nil {
		t.Fatalf("image base64 decode error = %v", err)
	}
	return image
}

func assertPNG(t *testing.T, raw json.RawMessage) {
	t.Helper()
	image := payloadImage(t, raw)
	signature := []byte("\x89PNG\r\n\x1a\n")
	if !bytes.HasPrefix(image, signature) {
		limit := len(image)
		if limit > 16 {
			limit = 16
		}
		t.Fatalf("image is not PNG: %x", image[:limit])
	}
}

func assertNoPNGTextChunks(t *testing.T, image []byte) {
	t.Helper()
	if len(image) < 8 {
		t.Fatal("PNG is too short")
	}
	for offset := 8; offset+12 <= len(image); {
		length := int(binary.BigEndian.Uint32(image[offset : offset+4]))
		end := offset + 12 + length
		if length < 0 || end > len(image) {
			t.Fatal("PNG contains an invalid chunk")
		}
		chunkType := string(image[offset+4 : offset+8])
		if chunkType == "tEXt" || chunkType == "iTXt" || chunkType == "zTXt" {
			t.Fatalf("PNG contains plaintext metadata chunk %s", chunkType)
		}
		offset = end
	}
}

func findTextAnswer(
	t *testing.T,
	provider captcha.Provider,
	config, state json.RawMessage,
	candidates []string,
) string {
	t.Helper()
	for _, candidate := range candidates {
		response, _ := json.Marshal(textResponse{Answer: candidate})
		if err := provider.Verify(
			t.Context(),
			config,
			state,
			response,
			captcha.ClientMeta{},
		); err == nil {
			return candidate
		}
	}
	t.Fatalf("no candidate verified: %s", strings.Join(candidates, ","))
	return ""
}
