package logrus

import (
	"github.com/sirupsen/logrus"
)

// Logger 简化的日志器
type Logger struct {
	logger *logrus.Logger
	entry  *logrus.Entry
}

// NewLogger 创建新的 logrus 日志器
func NewLogger() *Logger {
	l := logrus.New()
	l.SetFormatter(&logrus.JSONFormatter{})
	l.SetLevel(logrus.InfoLevel)

	return &Logger{
		logger: l,
		entry:  l.WithFields(logrus.Fields{}),
	}
}

// Debug 输出调试日志
func (l *Logger) Debug(msg string) {
	l.entry.Debug(msg)
}

// Info 输出信息日志
func (l *Logger) Info(msg string) {
	l.entry.Info(msg)
}

// Warn 输出警告日志
func (l *Logger) Warn(msg string) {
	l.entry.Warn(msg)
}

// Error 输出错误日志
func (l *Logger) Error(msg string) {
	l.entry.Error(msg)
}

// Fatal 输出致命错误日志并退出
func (l *Logger) Fatal(msg string) {
	l.entry.Fatal(msg)
}

// WithField 添加单个字段
func (l *Logger) WithField(key string, value interface{}) *Logger {
	return &Logger{
		logger: l.logger,
		entry:  l.entry.WithField(key, value),
	}
}

// WithFields 添加多个字段
func (l *Logger) WithFields(fields map[string]interface{}) *Logger {
	return &Logger{
		logger: l.logger,
		entry:  l.entry.WithFields(fields),
	}
}

// WithError 添加错误字段
func (l *Logger) WithError(err error) *Logger {
	return &Logger{
		logger: l.logger,
		entry:  l.entry.WithError(err),
	}
}

// SetLevel 设置日志级别
func (l *Logger) SetLevel(level string) {
	logLevel, err := logrus.ParseLevel(level)
	if err != nil {
		logLevel = logrus.InfoLevel
	}
	l.logger.SetLevel(logLevel)
}

// GetLevel 获取当前日志级别
func (l *Logger) GetLevel() string {
	return l.logger.GetLevel().String()
}
