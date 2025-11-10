package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var Logger *zap.Logger

// InitLogger 初始化zap日志器
func InitLogger(debug bool) error {
	var config zap.Config
	if debug {
		config = zap.NewDevelopmentConfig()
		config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	} else {
		config = zap.NewProductionConfig()
		config.EncoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	}

	config.EncoderConfig.TimeKey = "time"
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	config.EncoderConfig.CallerKey = "caller"
	config.EncoderConfig.EncodeCaller = zapcore.ShortCallerEncoder

	var err error
	Logger, err = config.Build(zap.AddCaller(), zap.AddCallerSkip(1))
	if err != nil {
		return err
	}

	// 替换全局logger
	zap.ReplaceGlobals(Logger)

	return nil
}

// Sync 同步日志缓冲区
func Sync() {
	if Logger != nil {
		Logger.Sync()
	}
}

// 快捷方法
func Debug(msg string, fields ...zap.Field) {
	Logger.Debug(msg, fields...)
}

func Info(msg string, fields ...zap.Field) {
	Logger.Info(msg, fields...)
}

func Warn(msg string, fields ...zap.Field) {
	Logger.Warn(msg, fields...)
}

func Error(msg string, fields ...zap.Field) {
	Logger.Error(msg, fields...)
}

func Fatal(msg string, fields ...zap.Field) {
	Logger.Fatal(msg, fields...)
}

// 带上下文的快捷方法
func Debugf(msg string, args ...interface{}) {
	Logger.Sugar().Debugf(msg, args...)
}

func Infof(msg string, args ...interface{}) {
	Logger.Sugar().Infof(msg, args...)
}

func Warnf(msg string, args ...interface{}) {
	Logger.Sugar().Warnf(msg, args...)
}

func Errorf(msg string, args ...interface{}) {
	Logger.Sugar().Errorf(msg, args...)
}

func Fatalf(msg string, args ...interface{}) {
	Logger.Sugar().Fatalf(msg, args...)
}