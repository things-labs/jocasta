package izap

import (
	"go.uber.org/zap"
)

type logger struct {
	*zap.Logger
}

var Logger = logger{zap.NewNop()}

// 调试
func Debug(msg string, fields ...zap.Field) {
	Logger.Debug(msg, fields...)
}

// Info 消息
func Info(msg string, fields ...zap.Field) {
	Logger.Info(msg, fields...)
}

// Warn 警告
func Warn(msg string, fields ...zap.Field) {
	Logger.Warn(msg, fields...)
}

// Error 错误
func Error(msg string, fields ...zap.Field) {
	Logger.Error(msg, fields...)
}

// DPanic 开发模式下会panic,生产模型为recover
func DPanic(msg string, fields ...zap.Field) {
	Logger.DPanic(msg, fields...)
}

// Panic 调用会panic
func Panic(msg string, fields ...zap.Field) {
	Logger.Panic(msg, fields...)
}

// Fetal 致命错误
func Fetal(msg string, fields ...zap.Field) {
	Logger.Fatal(msg, fields...)
}
