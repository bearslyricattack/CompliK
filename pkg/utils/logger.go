package utils

import (
	"log"
	"os"
)

// Logger 简单的日志记录器
type Logger struct {
	infoLogger  *log.Logger
	warnLogger  *log.Logger
	errorLogger *log.Logger
}

// NewLogger 创建一个新的日志记录器
func NewLogger() *Logger {
	return &Logger{
		infoLogger:  log.New(os.Stdout, "INFO: ", log.Ldate|log.Ltime),
		warnLogger:  log.New(os.Stdout, "WARN: ", log.Ldate|log.Ltime),
		errorLogger: log.New(os.Stderr, "ERROR: ", log.Ldate|log.Ltime),
	}
}

// Info 记录信息日志
func (l *Logger) Info(message string) {
	l.infoLogger.Println(message)
}

// Warning 记录警告日志
func (l *Logger) Warning(message string) {
	l.warnLogger.Println(message)
}

// Error 记录错误日志
func (l *Logger) Error(message string) {
	l.errorLogger.Println(message)
}
