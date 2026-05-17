package log_test

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"go.uber.org/zap"

	"github.com/wuwuseo/cmf/log"
)

var testMu sync.RWMutex

// ======================== 1. parseLevel（通过 NewLoggerFromConfig 间接测试） ========================

// TestParseLevel_Debug 测试 "debug" → DebugLevel
func TestParseLevel_Debug(t *testing.T) {
	cfg := log.LogConfig{
		Level:         "debug",
		Format:        "console",
		ConsoleOutput: true,
	}
	l := log.NewLoggerFromConfig(cfg)
	if l == nil {
		t.Fatal("debug 级别创建的 logger 不应为 nil")
	}
	_ = l.Sync()
}

// TestParseLevel_Info 测试 "info" → InfoLevel
func TestParseLevel_Info(t *testing.T) {
	cfg := log.LogConfig{
		Level:         "info",
		Format:        "console",
		ConsoleOutput: true,
	}
	l := log.NewLoggerFromConfig(cfg)
	if l == nil {
		t.Fatal("info 级别创建的 logger 不应为 nil")
	}
	_ = l.Sync()
}

// TestParseLevel_Warn 测试 "warn" → WarnLevel
func TestParseLevel_Warn(t *testing.T) {
	cfg := log.LogConfig{
		Level:         "warn",
		Format:        "console",
		ConsoleOutput: true,
	}
	l := log.NewLoggerFromConfig(cfg)
	if l == nil {
		t.Fatal("warn 级别创建的 logger 不应为 nil")
	}
	_ = l.Sync()
}

// TestParseLevel_Warning 测试 "warning" → WarnLevel（别名）
func TestParseLevel_Warning(t *testing.T) {
	cfg := log.LogConfig{
		Level:         "warning",
		Format:        "console",
		ConsoleOutput: true,
	}
	l := log.NewLoggerFromConfig(cfg)
	if l == nil {
		t.Fatal("warning 级别创建的 logger 不应为 nil")
	}
	_ = l.Sync()
}

// TestParseLevel_Error 测试 "error" → ErrorLevel
func TestParseLevel_Error(t *testing.T) {
	cfg := log.LogConfig{
		Level:         "error",
		Format:        "console",
		ConsoleOutput: true,
	}
	l := log.NewLoggerFromConfig(cfg)
	if l == nil {
		t.Fatal("error 级别创建的 logger 不应为 nil")
	}
	_ = l.Sync()
}

// TestParseLevel_Fatal 测试 "fatal" → FatalLevel
func TestParseLevel_Fatal(t *testing.T) {
	cfg := log.LogConfig{
		Level:         "fatal",
		Format:        "console",
		ConsoleOutput: true,
	}
	l := log.NewLoggerFromConfig(cfg)
	if l == nil {
		t.Fatal("fatal 级别创建的 logger 不应为 nil")
	}
	_ = l.Sync()
}

// TestParseLevel_Unknown 测试未知字符串 → InfoLevel（默认）
func TestParseLevel_Unknown(t *testing.T) {
	cfg := log.LogConfig{
		Level:         "unknown_level",
		Format:        "console",
		ConsoleOutput: true,
	}
	l := log.NewLoggerFromConfig(cfg)
	if l == nil {
		t.Fatal("未知级别字符串创建的 logger 不应为 nil")
	}
	_ = l.Sync()
}

// TestParseLevel_Empty 测试空字符串 → InfoLevel（默认）
func TestParseLevel_Empty(t *testing.T) {
	cfg := log.LogConfig{
		Level:         "",
		Format:        "console",
		ConsoleOutput: true,
	}
	l := log.NewLoggerFromConfig(cfg)
	if l == nil {
		t.Fatal("空字符串级别创建的 logger 不应为 nil")
	}
	_ = l.Sync()
}

// ======================== 2. NewLoggerFromConfig ========================

// TestNewLoggerFromConfig_ConsoleOutput 测试 Console 输出配置创建 logger
func TestNewLoggerFromConfig_ConsoleOutput(t *testing.T) {
	cfg := log.LogConfig{
		Level:         "info",
		Format:        "console",
		ConsoleOutput: true,
	}
	l := log.NewLoggerFromConfig(cfg)
	if l == nil {
		t.Fatal("Console 输出创建的 logger 不应为 nil")
	}
	_ = l.Sync()
}

// TestNewLoggerFromConfig_JSONFormat 测试 JSON 格式创建 logger
func TestNewLoggerFromConfig_JSONFormat(t *testing.T) {
	cfg := log.LogConfig{
		Level:         "info",
		Format:        "json",
		ConsoleOutput: true,
	}
	l := log.NewLoggerFromConfig(cfg)
	if l == nil {
		t.Fatal("JSON 格式创建的 logger 不应为 nil")
	}
	_ = l.Sync()
}

// TestNewLoggerFromConfig_ConsoleFormat 测试 Console 格式创建 logger
func TestNewLoggerFromConfig_ConsoleFormat(t *testing.T) {
	cfg := log.LogConfig{
		Level:         "debug",
		Format:        "console",
		ConsoleOutput: true,
	}
	l := log.NewLoggerFromConfig(cfg)
	if l == nil {
		t.Fatal("Console 格式创建的 logger 不应为 nil")
	}
	_ = l.Sync()
}

// TestNewLoggerFromConfig_OnlyConsole 测试只有 Console Output
func TestNewLoggerFromConfig_OnlyConsole(t *testing.T) {
	cfg := log.LogConfig{
		Level:         "info",
		Format:        "console",
		ConsoleOutput: true,
		FileOutput:    false,
	}
	l := log.NewLoggerFromConfig(cfg)
	if l == nil {
		t.Fatal("仅 Console 输出创建的 logger 不应为 nil")
	}
	_ = l.Sync()
}

// TestNewLoggerFromConfig_OnlyFile 测试只有 File Output（使用临时文件路径）
func TestNewLoggerFromConfig_OnlyFile(t *testing.T) {
	tmpFile := filepath.Join(os.TempDir(), "cmf_test_only_file.log")
	defer os.Remove(tmpFile)

	cfg := log.LogConfig{
		Level:         "info",
		Format:        "json",
		FilePath:      tmpFile,
		ConsoleOutput: false,
		FileOutput:    true,
		MaxSize:       10,
		MaxBackups:    3,
		MaxAge:        7,
	}
	l := log.NewLoggerFromConfig(cfg)
	if l == nil {
		t.Fatal("仅 File 输出创建的 logger 不应为 nil")
	}
	_ = l.Sync()
}

// TestNewLoggerFromConfig_ConsoleAndFile 测试同时 Console + File Output
func TestNewLoggerFromConfig_ConsoleAndFile(t *testing.T) {
	tmpFile := filepath.Join(os.TempDir(), "cmf_test_console_and_file.log")
	defer os.Remove(tmpFile)

	cfg := log.LogConfig{
		Level:         "debug",
		Format:        "console",
		FilePath:      tmpFile,
		ConsoleOutput: true,
		FileOutput:    true,
		MaxSize:       10,
		MaxBackups:    3,
		MaxAge:        7,
	}
	l := log.NewLoggerFromConfig(cfg)
	if l == nil {
		t.Fatal("Console + File 输出创建的 logger 不应为 nil")
	}
	_ = l.Sync()
}

// TestNewLoggerFromConfig_VariousLevels 测试各种日志级别
func TestNewLoggerFromConfig_VariousLevels(t *testing.T) {
	levels := []string{"debug", "info", "warn", "warning", "error", "fatal"}

	for _, lvl := range levels {
		t.Run(lvl, func(t *testing.T) {
			cfg := log.LogConfig{
				Level:         lvl,
				Format:        "console",
				ConsoleOutput: true,
			}
			l := log.NewLoggerFromConfig(cfg)
			if l == nil {
				t.Fatalf("%s 级别创建的 logger 不应为 nil", lvl)
			}
			_ = l.Sync()
		})
	}
}

// ======================== 3. Logger 接口方法 ========================

// newTestLogger 创建一个用于测试的 logger 实例
func newTestLogger() log.Logger {
	cfg := log.LogConfig{
		Level:         "debug",
		Format:        "console",
		ConsoleOutput: true,
	}
	return log.NewLoggerFromConfig(cfg)
}

// TestLogger_Debug 测试 Debug 方法不 panic
func TestLogger_Debug(t *testing.T) {
	l := newTestLogger()
	defer l.Sync()

	l.Debug("测试 Debug 消息")
	l.Debug("测试带字段的 Debug 消息", zap.String("key", "value"))
}

// TestLogger_Info 测试 Info 方法不 panic
func TestLogger_Info(t *testing.T) {
	l := newTestLogger()
	defer l.Sync()

	l.Info("测试 Info 消息")
	l.Info("测试带字段的 Info 消息", zap.Int("count", 42))
}

// TestLogger_Warn 测试 Warn 方法不 panic
func TestLogger_Warn(t *testing.T) {
	l := newTestLogger()
	defer l.Sync()

	l.Warn("测试 Warn 消息")
	l.Warn("测试带字段的 Warn 消息", zap.Bool("flag", true))
}

// TestLogger_Error 测试 Error 方法不 panic
func TestLogger_Error(t *testing.T) {
	l := newTestLogger()
	defer l.Sync()

	l.Error("测试 Error 消息")
	l.Error("测试带字段的 Error 消息", zap.Error(os.ErrNotExist))
}

// TestLogger_With 测试 With 方法返回新的 Logger
func TestLogger_With(t *testing.T) {
	l := newTestLogger()
	defer l.Sync()

	l2 := l.With(zap.String("module", "test"))
	if l2 == nil {
		t.Fatal("With 方法应返回非 nil 的 Logger")
	}

	// 确保 l 和 l2 是不同的实例
	if l == l2 {
		t.Fatal("With 方法应返回新的 Logger 实例")
	}

	// 带字段的新 logger 应能正常使用
	l2.Info("带 module 字段的消息")
	l2.Debug("带 module 字段的 Debug 消息")
}

// TestLogger_WithMultipleFields 测试 With 多个字段
func TestLogger_WithMultipleFields(t *testing.T) {
	l := newTestLogger()
	defer l.Sync()

	l2 := l.With(
		zap.String("method", "GET"),
		zap.String("path", "/api/test"),
		zap.Int("status", 200),
	)
	if l2 == nil {
		t.Fatal("With 多个字段应返回非 nil 的 Logger")
	}
	l2.Info("带多个字段的消息")
}

// TestLogger_Sync 测试 Sync 方法
func TestLogger_Sync(t *testing.T) {
	l := newTestLogger()

	l.Info("Sync 测试消息")

	err := l.Sync()
	if err != nil {
		t.Logf("Sync 返回了错误（该行为取决于底层实现）: %v", err)
	}
}

// TestLogger_AllMethods 测试所有接口方法不 panic
func TestLogger_AllMethods(t *testing.T) {
	l := newTestLogger()
	defer l.Sync()

	l.Debug("debug 消息", zap.String("k", "v"))
	l.Info("info 消息", zap.String("k", "v"))
	l.Warn("warn 消息", zap.String("k", "v"))
	l.Error("error 消息", zap.String("k", "v"))

	withLogger := l.With(zap.String("component", "test"))
	withLogger.Info("with 后的消息")

	err := l.Sync()
	_ = err
}

// ======================== 4. GetDefault / SetDefault ========================

// TestGetDefault_Initial 测试初始 GetDefault 返回 nop logger
func TestGetDefault_Initial(t *testing.T) {
	l := log.GetDefault()
	if l == nil {
		t.Fatal("初始 GetDefault 不应返回 nil")
	}

	// 初始默认 logger 的方法调用不应 panic
	l.Debug("默认 logger debug")
	l.Info("默认 logger info")
	l.Warn("默认 logger warn")
	l.Error("默认 logger error")
}

// TestSetDefault 测试 SetDefault 后 GetDefault 返回设置的值
func TestSetDefault(t *testing.T) {
	// 保存原始默认 logger
	testMu.Lock()
	original := log.GetDefault()
	testMu.Unlock()

	// 创建一个新的 logger
	cfg := log.LogConfig{
		Level:         "debug",
		Format:        "console",
		ConsoleOutput: true,
	}
	newLogger := log.NewLoggerFromConfig(cfg)

	// 设置为默认
	log.SetDefault(newLogger)

	// 验证 GetDefault 返回设置的值
	current := log.GetDefault()
	if current != newLogger {
		t.Fatal("SetDefault 后 GetDefault 应返回设置的值")
	}

	// 恢复原始默认 logger
	log.SetDefault(original)
}

// ======================== 5. 包级别便捷函数 ========================

// TestPackageLevel_Debug 测试包级别 Debug 不 panic
func TestPackageLevel_Debug(t *testing.T) {
	log.Debug("包级别 Debug 消息")
	log.Debug("包级别 Debug 带字段", zap.String("key", "value"))
}

// TestPackageLevel_Info 测试包级别 Info 不 panic
func TestPackageLevel_Info(t *testing.T) {
	log.Info("包级别 Info 消息")
	log.Info("包级别 Info 带字段", zap.Int("count", 10))
}

// TestPackageLevel_Warn 测试包级别 Warn 不 panic
func TestPackageLevel_Warn(t *testing.T) {
	log.Warn("包级别 Warn 消息")
	log.Warn("包级别 Warn 带字段", zap.Bool("warning", true))
}

// TestPackageLevel_Error 测试包级别 Error 不 panic
func TestPackageLevel_Error(t *testing.T) {
	log.Error("包级别 Error 消息")
	log.Error("包级别 Error 带字段", zap.String("detail", "测试错误"))
}

// ======================== 6. RequestLoggerMiddleware ========================

// TestRequestLoggerMiddleware 测试创建中间件后不为 nil
func TestRequestLoggerMiddleware(t *testing.T) {
	cfg := log.LogConfig{
		Level:         "info",
		Format:        "console",
		ConsoleOutput: true,
	}
	l := log.NewLoggerFromConfig(cfg)
	defer l.Sync()

	handler := log.RequestLoggerMiddleware(l)
	if handler == nil {
		t.Fatal("RequestLoggerMiddleware 返回的 Fiber 中间件不应为 nil")
	}
}
