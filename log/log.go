package log

import (
	"os"
	"sync"
	"time"

	zapmiddleware "github.com/gofiber/contrib/v3/zap"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/log"
	"github.com/google/wire"
	"github.com/wuwuseo/cmf/config"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Logger 标准日志接口
type Logger interface {
	Debug(msg string, fields ...zap.Field)
	Info(msg string, fields ...zap.Field)
	Warn(msg string, fields ...zap.Field)
	Error(msg string, fields ...zap.Field)
	Fatal(msg string, fields ...zap.Field)
	With(fields ...zap.Field) Logger
	Sync() error
}

// LogConfig 日志配置
type LogConfig struct {
	Level         string
	Format        string // json 或 console
	FilePath      string
	ConsoleOutput bool
	FileOutput    bool
	MaxSize       int // 单个日志文件最大大小（MB）
	MaxBackups    int // 保留的旧日志文件数量
	MaxAge        int // 保留的旧日志文件最大天数
}

// zapLogger 是 Logger 接口的 Zap 实现
type zapLogger struct {
	logger *zap.Logger
}

func (l *zapLogger) Debug(msg string, fields ...zap.Field) {
	l.logger.Debug(msg, fields...)
}

func (l *zapLogger) Info(msg string, fields ...zap.Field) {
	l.logger.Info(msg, fields...)
}

func (l *zapLogger) Warn(msg string, fields ...zap.Field) {
	l.logger.Warn(msg, fields...)
}

func (l *zapLogger) Error(msg string, fields ...zap.Field) {
	l.logger.Error(msg, fields...)
}

func (l *zapLogger) Fatal(msg string, fields ...zap.Field) {
	l.logger.Fatal(msg, fields...)
}

func (l *zapLogger) With(fields ...zap.Field) Logger {
	return &zapLogger{logger: l.logger.With(fields...)}
}

func (l *zapLogger) Sync() error {
	return l.logger.Sync()
}

// Zap 返回底层的 zap.Logger 实例（供内部兼容使用）
func (l *zapLogger) Zap() *zap.Logger {
	return l.logger
}

var (
	defaultLogger Logger
	mu            sync.RWMutex
)

// SetDefault 设置全局默认 logger
func SetDefault(logger Logger) {
	mu.Lock()
	defer mu.Unlock()
	defaultLogger = logger
}

// GetDefault 返回当前全局默认 logger 实例
func GetDefault() Logger {
	mu.RLock()
	defer mu.RUnlock()
	if defaultLogger == nil {
		return &zapLogger{logger: zap.NewNop()}
	}
	return defaultLogger
}

// 包级别便捷函数

func Debug(msg string, fields ...zap.Field) {
	GetDefault().Debug(msg, fields...)
}

func Info(msg string, fields ...zap.Field) {
	GetDefault().Info(msg, fields...)
}

func Warn(msg string, fields ...zap.Field) {
	GetDefault().Warn(msg, fields...)
}

func Error(msg string, fields ...zap.Field) {
	GetDefault().Error(msg, fields...)
}

func Fatal(msg string, fields ...zap.Field) {
	GetDefault().Fatal(msg, fields...)
}

func Sync() error {
	return GetDefault().Sync()
}

func parseLevel(level string) zapcore.Level {
	switch level {
	case "debug":
		return zapcore.DebugLevel
	case "info":
		return zapcore.InfoLevel
	case "warn", "warning":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	case "fatal":
		return zapcore.FatalLevel
	default:
		return zapcore.InfoLevel
	}
}

// NewLogger 基于应用配置创建日志实例（Wire-friendly）
func NewLogger(cfg *config.Config) (Logger, error) {
	logCfg := LogConfig{
		Level:         cfg.Log.Level,
		Format:        cfg.Log.Format,
		FilePath:      cfg.Log.FilePath,
		ConsoleOutput: cfg.Log.ConsoleOutput,
		FileOutput:    cfg.Log.FileOutput,
		MaxSize:       cfg.Log.MaxSize,
		MaxBackups:    cfg.Log.MaxBackups,
		MaxAge:        cfg.Log.MaxAge,
	}
	if logCfg.Level == "" {
		if cfg.App.Debug {
			logCfg.Level = "debug"
		} else {
			logCfg.Level = "info"
		}
	}
	if logCfg.Format == "" {
		if cfg.App.Debug {
			logCfg.Format = "console"
		} else {
			logCfg.Format = "json"
		}
	}
	logger := NewLoggerFromConfig(logCfg)
	return logger, nil
}

// ProviderSet 提供 Wire 依赖注入所需的 provider 集合
var ProviderSet = wire.NewSet(NewLogger)

// NewLoggerFromConfig 基于 LogConfig 创建日志实例
func NewLoggerFromConfig(cfg LogConfig) Logger {
	lvl := parseLevel(cfg.Level)

	var encoder zapcore.Encoder
	if cfg.Format == "console" {
		encoderConfig := zap.NewDevelopmentEncoderConfig()
		encoderConfig.TimeKey = "time"
		encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	} else {
		encoderConfig := zap.NewProductionEncoderConfig()
		encoderConfig.TimeKey = "time"
		encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	}

	var cores []zapcore.Core

	if cfg.ConsoleOutput {
		cores = append(cores, zapcore.NewCore(encoder, zapcore.AddSync(os.Stdout), lvl))
	}

	if cfg.FileOutput {
		lumberjackLogger := &lumberjack.Logger{
			Filename:   cfg.FilePath,
			MaxSize:    cfg.MaxSize,
			MaxBackups: cfg.MaxBackups,
			MaxAge:     cfg.MaxAge,
			Compress:   true,
		}
		cores = append(cores, zapcore.NewCore(encoder, zapcore.AddSync(lumberjackLogger), lvl))
	}

	if len(cores) == 0 {
		cores = append(cores, zapcore.NewCore(encoder, zapcore.AddSync(os.Stdout), lvl))
	}

	core := zapcore.NewTee(cores...)
	zapInst := zap.New(core, zap.AddCallerSkip(1))
	return &zapLogger{logger: zapInst}
}

// RequestLoggerMiddleware 返回一个 Fiber 中间件，记录每个请求的信息
func RequestLoggerMiddleware(logger Logger) fiber.Handler {
	return func(c fiber.Ctx) error {
		start := time.Now()
		defer func() {
			duration := time.Since(start)
			status := c.Response().StatusCode()
			method := c.Method()
			path := c.Path()
			ip := c.IP()

			logger.Info("HTTP请求",
				zap.String("method", method),
				zap.String("path", path),
				zap.Int("status", status),
				zap.Duration("duration", duration),
				zap.String("ip", ip),
			)
		}()
		return c.Next()
	}
}

// InitLogger 初始化日志系统，配置 Zap 作为 Fiber 的日志记录器
// 保持向后兼容
func InitLogger(debug bool, consoleOutput bool, fileOutput bool, logFilePath string, maxSize int, maxBackups int, maxAge int) {
	cfg := LogConfig{
		Level:         "info",
		Format:        "json",
		FilePath:      logFilePath,
		ConsoleOutput: consoleOutput,
		FileOutput:    fileOutput,
		MaxSize:       maxSize,
		MaxBackups:    maxBackups,
		MaxAge:        maxAge,
	}
	if debug {
		cfg.Level = "debug"
		cfg.Format = "console"
	}
	logger := NewLoggerFromConfig(cfg)
	SetDefault(logger)

	// 同时设置 Fiber 的日志记录器以保持兼容
	zapL := logger.(*zapLogger).Zap()
	fiberZap := zapmiddleware.NewLogger(zapmiddleware.LoggerConfig{
		SetLogger: zapL,
	})
	log.SetLogger(fiberZap)
}

// InitDefaultLogger 提供默认的初始化方法，使用合理的默认值
// 保持向后兼容
func InitDefaultLogger(cfg *config.Config) {
	logger, _ := NewLogger(cfg)
	SetDefault(logger)

	// 同时设置 Fiber 的日志记录器以保持兼容
	zapL := logger.(*zapLogger).Zap()
	fiberZap := zapmiddleware.NewLogger(zapmiddleware.LoggerConfig{
		SetLogger: zapL,
	})
	log.SetLogger(fiberZap)
}
