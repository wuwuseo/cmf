package logger

import (
	"github.com/natefinch/lumberjack"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"os"
)

type Log struct {
	Logger *zap.Logger
}

func (r *Log) Sync() {
	r.Logger.Sync()
}

func (r *Log) Info(msg string, fields ...zap.Field) {
	r.Logger.Info(msg, fields...)
}

func (r *Log) Error(msg string, fields ...zap.Field) {
	r.Logger.Error(msg, fields...)
}

func (r *Log) Warn(msg string, fields ...zap.Field) {
	r.Logger.Warn(msg, fields...)
}

func (r *Log) Debug(msg string, fields ...zap.Field) {
	r.Logger.Debug(msg, fields...)
}

func (r *Log) Fatal(msg string, fields ...zap.Field) {
	r.Logger.Fatal(msg, fields...)
}

func NewLog() *Log {
	debug := true
	loglevel := "debug"
	cfg := zap.Config{}
	if debug {
		cfg = zap.NewDevelopmentConfig()
		cfg.EncoderConfig = zap.NewDevelopmentEncoderConfig()
	} else {
		cfg = zap.NewProductionConfig()
		cfg.EncoderConfig = zap.NewProductionEncoderConfig()
	}

	w := zapcore.AddSync(&lumberjack.Logger{
		Filename:   "storage/log/stdout.log",
		MaxSize:    100, // megabytes
		MaxBackups: 3,
		MaxAge:     30, // days
	})
	var level zapcore.Level
	switch loglevel {
	case "debug":
		level = zap.DebugLevel
	case "info":
		level = zap.InfoLevel
	case "error":
		level = zap.ErrorLevel
	default:
		level = zap.InfoLevel
	}
	atomicLevel := zap.NewAtomicLevel()
	atomicLevel.SetLevel(level)
	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(cfg.EncoderConfig),
		zapcore.NewMultiWriteSyncer(zapcore.AddSync(os.Stdout), w), // 打印到控制台和文件
		atomicLevel,
	)
	logger := &Log{}
	if debug {
		// 开启开发模式，堆栈跟踪
		caller := zap.AddCaller()
		// 开启文件及行号
		development := zap.Development()
		logger.Logger = zap.New(core, caller, development)
	} else {
		logger.Logger = zap.New(core)
	}
	return logger
}
