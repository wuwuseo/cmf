package provider

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/wuwuseo/cmf/captcha"
)

const (
	geeTestName     = "geetest_gt4"
	geeTestEndpoint = "https://gcaptcha4.geetest.com/validate"
)

type GeeTestGT4 struct {
	client *http.Client
}

type geeTestConfig struct {
	CaptchaID  string `json:"captcha_id"`
	CaptchaKey string `json:"captcha_key"`
	TimeoutMS  int    `json:"timeout_ms"`
}

type geeTestResponse struct {
	LotNumber     string `json:"lot_number"`
	CaptchaOutput string `json:"captcha_output"`
	PassToken     string `json:"pass_token"`
	GenTime       string `json:"gen_time"`
}

func NewGeeTestGT4(client *http.Client) *GeeTestGT4 {
	if client == nil {
		client = http.DefaultClient
	}
	return &GeeTestGT4{client: client}
}

func (*GeeTestGT4) Name() string { return geeTestName }

func decodeGeeTestConfig(raw json.RawMessage) (geeTestConfig, error) {
	config := geeTestConfig{TimeoutMS: 3000}
	if err := json.Unmarshal(raw, &config); err != nil {
		return geeTestConfig{}, captcha.ErrProviderMisconfigured
	}
	if strings.TrimSpace(config.CaptchaID) == "" || strings.TrimSpace(config.CaptchaKey) == "" ||
		config.TimeoutMS < 100 || config.TimeoutMS > 10000 {
		return geeTestConfig{}, captcha.ErrProviderMisconfigured
	}
	return config, nil
}

func (*GeeTestGT4) ValidateConfig(raw json.RawMessage) error {
	_, err := decodeGeeTestConfig(raw)
	return err
}

func (p *GeeTestGT4) Create(_ context.Context, raw json.RawMessage, _ captcha.ClientMeta) (captcha.Challenge, error) {
	config, err := decodeGeeTestConfig(raw)
	if err != nil {
		return captcha.Challenge{}, err
	}
	payload, _ := json.Marshal(map[string]string{"captcha_id": config.CaptchaID})
	return captcha.Challenge{Payload: payload, PrivateState: json.RawMessage(`{}`)}, nil
}

func (p *GeeTestGT4) Verify(ctx context.Context, raw, _, responseRaw json.RawMessage, _ captcha.ClientMeta) error {
	config, err := decodeGeeTestConfig(raw)
	if err != nil {
		return err
	}
	var response geeTestResponse
	if err := json.Unmarshal(responseRaw, &response); err != nil ||
		response.LotNumber == "" || response.CaptchaOutput == "" || response.PassToken == "" || response.GenTime == "" {
		return captcha.ErrInvalid
	}
	form := url.Values{
		"lot_number":     {response.LotNumber},
		"captcha_output": {response.CaptchaOutput},
		"pass_token":     {response.PassToken},
		"gen_time":       {response.GenTime},
		"sign_token":     {signGeeTest(config.CaptchaKey, response.LotNumber)},
	}
	requestContext, cancel := context.WithTimeout(ctx, time.Duration(config.TimeoutMS)*time.Millisecond)
	defer cancel()
	request, err := http.NewRequestWithContext(
		requestContext,
		http.MethodPost,
		geeTestEndpoint+"?captcha_id="+url.QueryEscape(config.CaptchaID),
		strings.NewReader(form.Encode()),
	)
	if err != nil {
		return fmt.Errorf("%w: create geetest request", captcha.ErrProviderUnavailable)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	httpResponse, err := p.client.Do(request)
	if err != nil {
		return fmt.Errorf("%w: geetest request failed", captcha.ErrProviderUnavailable)
	}
	defer httpResponse.Body.Close()
	if httpResponse.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, httpResponse.Body)
		return fmt.Errorf("%w: geetest status %s", captcha.ErrProviderUnavailable, httpResponse.Status)
	}
	body, err := io.ReadAll(io.LimitReader(httpResponse.Body, 64*1024))
	if err != nil {
		return fmt.Errorf("%w: read geetest response", captcha.ErrProviderUnavailable)
	}
	var result struct {
		Result string `json:"result"`
		Reason string `json:"reason"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("%w: invalid geetest response", captcha.ErrProviderUnavailable)
	}
	if result.Result != "success" {
		return captcha.ErrInvalid
	}
	return nil
}

func signGeeTest(secret, lotNumber string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(lotNumber))
	return hex.EncodeToString(mac.Sum(nil))
}

func (c geeTestConfig) String() string {
	return c.CaptchaID + ":" + strconv.Itoa(c.TimeoutMS)
}
