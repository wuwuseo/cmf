package provider

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/wuwuseo/cmf/captcha"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func TestGeeTestVerifyUsesOfficialEndpointAndSignature(t *testing.T) {
	var gotHost string
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		gotHost = request.URL.Host
		if request.URL.Path != "/validate" || request.URL.Query().Get("captcha_id") != "captcha-id" {
			t.Fatalf("request URL = %s", request.URL)
		}
		if err := request.ParseForm(); err != nil {
			t.Fatalf("ParseForm() error = %v", err)
		}
		if request.Form.Get("sign_token") != signGeeTest("private-key", "lot-number") {
			t.Fatalf("sign_token = %q", request.Form.Get("sign_token"))
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"result":"success","reason":""}`)),
			Header:     make(http.Header),
		}, nil
	})}
	provider := NewGeeTestGT4(client)
	config := json.RawMessage(`{"captcha_id":"captcha-id","captcha_key":"private-key","timeout_ms":1000}`)
	response := json.RawMessage(`{"lot_number":"lot-number","captcha_output":"output","pass_token":"pass","gen_time":"123"}`)

	if err := provider.Verify(t.Context(), config, nil, response, captcha.ClientMeta{}); err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if gotHost != "gcaptcha4.geetest.com" {
		t.Fatalf("request host = %q, want official GeeTest host", gotHost)
	}
}

func TestGeeTestVerifyFailsClosed(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusBadGateway,
			Body:       io.NopCloser(strings.NewReader("bad gateway")),
			Header:     make(http.Header),
		}, nil
	})}
	provider := NewGeeTestGT4(client)
	config := json.RawMessage(`{"captcha_id":"captcha-id","captcha_key":"private-key","timeout_ms":1000}`)
	response := json.RawMessage(`{"lot_number":"lot-number","captcha_output":"output","pass_token":"pass","gen_time":"123"}`)
	if err := provider.Verify(t.Context(), config, nil, response, captcha.ClientMeta{}); err == nil {
		t.Fatal("Verify() succeeded for non-200 provider response")
	}
}

func TestGeeTestCreateReturnsPublicCaptchaIDOnly(t *testing.T) {
	provider := NewGeeTestGT4(http.DefaultClient)
	config := json.RawMessage(`{"captcha_id":"captcha-id","captcha_key":"private-key"}`)
	challenge, err := provider.Create(t.Context(), config, captcha.ClientMeta{})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if strings.Contains(string(challenge.Payload), "private-key") {
		t.Fatal("Create() leaked captcha_key in public payload")
	}
	if !strings.Contains(string(challenge.Payload), "captcha-id") {
		t.Fatalf("Create() payload = %s", challenge.Payload)
	}
}
