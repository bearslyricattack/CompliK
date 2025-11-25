package logger

import (
	"sync"

	"github.com/bearslyricattack/CompliK/procscan/pkg/logger/logrus"
)

var (
	defaultLogger *logrus.Logger
	once          sync.Once
)

// Init 初始化默认日志器
func Init() {
	once.Do(func() {
		defaultLogger = logrus.NewLogger()
	})
}

// GetLogger 获取默认日志器
func GetLogger() *logrus.Logger {
	if defaultLogger == nil {
		Init()
	}
	return defaultLogger
}

// SetLevel 设置日志级别
func SetLevel(level string) {
	GetLogger().SetLevel(level)
}

// Debug 输出调试日志
func Debug(msg string) {
	GetLogger().Debug(msg)
}

// Info 输出信息日志
func Info(msg string) {
	GetLogger().Info(msg)
}

// Warn 输出警告日志
func Warn(msg string) {
	GetLogger().Warn(msg)
}

// Error 输出错误日志
func Error(msg string) {
	GetLogger().Error(msg)
}

// Fatal 输出致命错误日志并退出
func Fatal(msg string) {
	GetLogger().Fatal(msg)
}

// WithField 添加单个字段
func WithField(key string, value interface{}) *logrus.Logger {
	return GetLogger().WithField(key, value)
}

// WithFields 添加多个字段
func WithFields(fields map[string]interface{}) *logrus.Logger {
	return GetLogger().WithFields(fields)
}

// WithError 添加错误字段
func WithError(err error) *logrus.Logger {
	return GetLogger().WithError(err)
}
