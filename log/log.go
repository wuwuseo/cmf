package log

import (
	"os"

	"github.com/gofiber/contrib/fiberzap/v2"
	"github.com/gofiber/fiber/v2/log"
	"github.com/wuwuseo/cmf/config"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// InitLogger 初始化日志系统，配置 Zap 作为 Fiber 的日志记录器
// debug: 是否启用调试模式
// consoleOutput: 是否输出到控制台
// fileOutput: 是否输出到文件
// logFilePath: 日志文件路径
// maxSize: 日志文件最大大小（MB）
// maxBackups: 保留的旧日志文件数量
// maxAge: 保留的旧日志文件最大天数
func InitLogger(debug bool, consoleOutput bool, fileOutput bool, logFilePath string, maxSize int, maxBackups int, maxAge int) {
	// 创建 Zap 配置
	config := zap.NewProductionConfig()

	// 如果是调试模式，使用开发配置
	if debug {
		config = zap.NewDevelopmentConfig()
	}

	// 配置时间格式
	config.EncoderConfig.TimeKey = "time"
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	// 配置输出目标
	var cores []zapcore.Core

	// 创建编码器
	encoder := zapcore.NewJSONEncoder(config.EncoderConfig)

	// 控制台输出
	if consoleOutput {
		consoleCore := zapcore.NewCore(encoder, zapcore.AddSync(os.Stdout), zapcore.DebugLevel)
		cores = append(cores, consoleCore)
	}

	// 文件输出
	if fileOutput {
		// 使用 lumberjack 实现日志分割
		lumberjackLogger := &lumberjack.Logger{
			Filename:   logFilePath,
			MaxSize:    maxSize,    // 日志文件最大大小（MB）
			MaxBackups: maxBackups, // 保留的旧日志文件数量
			MaxAge:     maxAge,     // 保留的旧日志文件最大天数
			Compress:   true,       // 是否压缩旧日志文件
		}
		fileCore := zapcore.NewCore(encoder, zapcore.AddSync(lumberjackLogger), zapcore.DebugLevel)
		cores = append(cores, fileCore)
	}

	// 如果没有配置输出目标，默认使用标准输出
	if len(cores) == 0 {
		cores = append(cores, zapcore.NewCore(encoder, zapcore.AddSync(os.Stdout), zapcore.DebugLevel))
	}

	// 创建 logger
	logger := zap.New(zapcore.NewTee(cores...))
	defer logger.Sync()

	// 创建 Fiberzap 中间件配置
	zapLogger := fiberzap.NewLogger(fiberzap.LoggerConfig{
		SetLogger: logger,
	})

	// 设置 Fiber 的日志记录器
	log.SetLogger(zapLogger)
}

// 提供默认的初始化方法，使用合理的默认值
func InitDefaultLogger(cfg *config.Config) {
	// 默认值设置：文件大小10MB，保留5个备份，保存30天
	InitLogger(cfg.App.Debug, cfg.Log.ConsoleOutput, cfg.Log.FileOutput, cfg.Log.FilePath, cfg.Log.MaxSize, cfg.Log.MaxBackups, cfg.Log.MaxAge)
}
