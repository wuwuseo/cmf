package config_test

import (
	"testing"

	"github.com/wuwuseo/cmf/config"
)

func TestNewConfigCaptchaDefaults(t *testing.T) {
	cfg := config.NewConfig()
	if cfg.Captcha.Store != "memory" {
		t.Fatalf("Captcha.Store = %q, want memory", cfg.Captcha.Store)
	}
	if cfg.Captcha.RedisConnection != "redis" {
		t.Fatalf("Captcha.RedisConnection = %q, want redis", cfg.Captcha.RedisConnection)
	}
	if cfg.Captcha.KeyPrefix != "captcha" {
		t.Fatalf("Captcha.KeyPrefix = %q, want captcha", cfg.Captcha.KeyPrefix)
	}
}
