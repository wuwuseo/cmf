package log_test

import (
	"testing"

	"github.com/wuwuseo/cmf/config"
	"github.com/wuwuseo/cmf/log"
)

func TestNewLogger(t *testing.T) {
	cfg := &config.Config{}
	cfg.Log.Level = "info"
	cfg.Log.Format = "json"
	cfg.Log.ConsoleOutput = true
	cfg.Log.FileOutput = false

	logger, err := log.NewLogger(cfg)
	if err != nil {
		t.Errorf("NewLogger 应该返回成功，错误 %v", err)
	}
	if logger == nil {
		t.Error("NewLogger 返回 nil")
	}

	logger.Debug("测试 debug")
	logger.Info("测试 info")
	logger.Warn("测试 warn")
	logger.Error("测试 error")
	// logger.Sync()
}

func TestNewLogger_Levels(t *testing.T) {
	cases := []struct {
		name  string
		level string
	}{
		{"debug", "debug"},
		{"info", "info"},
		{"warn", "warn"},
		{"warning", "warning"},
		{"error", "error"},
		{"fatal", "fatal"},
		{"unknown", "unknown"},
		{"empty", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &config.Config{}
			cfg.Log.Level = tc.level
			cfg.Log.ConsoleOutput = true
			cfg.Log.FileOutput = false

			logger, err := log.NewLogger(cfg)
			if err != nil {
				t.Errorf("level %s 应该返回成功，错误 %v", tc.level, err)
			}
			if logger == nil {
				t.Error("logger 应该非 nil")
			}
		})
	}
}

func TestNewLogger_Formats(t *testing.T) {
	cases := []struct {
		name   string
		format string
	}{
		{"json", "json"},
		{"console", "console"},
		{"empty", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &config.Config{}
			cfg.Log.Format = tc.format
			cfg.Log.ConsoleOutput = true
			cfg.Log.FileOutput = false
			cfg.App.Debug = tc.format == "console"

			logger, err := log.NewLogger(cfg)
			if err != nil {
				t.Errorf("format %s 应该返回成功，错误 %v", tc.format, err)
			}
			if logger == nil {
				t.Error("logger 应该非 nil")
			}
		})
	}
}

func TestGetSetDefault(t *testing.T) {
	original := log.GetDefault()
	defer log.SetDefault(original)

	newLogger := log.NewLoggerFromConfig(log.LogConfig{
		Level:         "debug",
		ConsoleOutput: true,
	})
	log.SetDefault(newLogger)

	if log.GetDefault() != newLogger {
		t.Error("GetDefault 应该返回已设置的 logger")
	}
}

func TestInitLogger(t *testing.T) {
	originalConf := config.Conf
	defer func() { config.Conf = originalConf }()

	cfg := &config.Config{}
	cfg.App.Debug = true
	cfg.App.Name = "test-app"
	cfg.Log.Level = "debug"
	cfg.Log.Format = "console"
	cfg.Log.ConsoleOutput = true
	cfg.Log.FileOutput = false

	log.InitDefaultLogger(cfg)

	if log.GetDefault() == nil {
		t.Error("InitDefaultLogger 后 GetDefault 应该非 nil")
	}
}
