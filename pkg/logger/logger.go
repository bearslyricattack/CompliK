package logger

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// LogLevel 日志级别
type LogLevel int

const (
	DebugLevel LogLevel = iota
	InfoLevel
	WarnLevel
	ErrorLevel
	FatalLevel
)

var (
	logLevelNames = map[LogLevel]string{
		DebugLevel: "DEBUG",
		InfoLevel:  "INFO",
		WarnLevel:  "WARN",
		ErrorLevel: "ERROR",
		FatalLevel: "FATAL",
	}

	logLevelColors = map[LogLevel]string{
		DebugLevel: "\033[36m", // Cyan
		InfoLevel:  "\033[32m", // Green
		WarnLevel:  "\033[33m", // Yellow
		ErrorLevel: "\033[31m", // Red
		FatalLevel: "\033[35m", // Magenta
	}

	resetColor = "\033[0m"
)

// Fields 日志字段类型
type Fields map[string]any

// Logger 日志接口
type Logger interface {
	Debug(msg string, fields ...Fields)
	Info(msg string, fields ...Fields)
	Warn(msg string, fields ...Fields)
	Error(msg string, fields ...Fields)
	Fatal(msg string, fields ...Fields)

	WithField(key string, value any) Logger
	WithFields(fields Fields) Logger
	WithContext(ctx context.Context) Logger
	WithError(err error) Logger

	SetLevel(level LogLevel)
	SetOutput(w io.Writer)
}

// StandardLogger 标准日志实现
type StandardLogger struct {
	mu         sync.RWMutex
	level      LogLevel
	output     io.Writer
	fields     Fields
	ctx        context.Context
	colored    bool
	jsonFormat bool
	showCaller bool
	timeFormat string
}

// globalLogger 全局日志实例
var (
	globalLogger *StandardLogger
	once         sync.Once
)

// Init 初始化全局日志
func Init() {
	once.Do(func() {
		globalLogger = &StandardLogger{
			level:      InfoLevel,
			output:     os.Stdout,
			fields:     make(Fields),
			colored:    true,
			jsonFormat: false,
			showCaller: true,
			timeFormat: "2006-01-02 15:04:05.000",
		}

		// 从环境变量读取配置
		configureFromEnv()
	})
}

// configureFromEnv 从环境变量配置日志
func configureFromEnv() {
	// 日志级别
	if level := os.Getenv("COMPLIK_LOG_LEVEL"); level != "" {
		switch strings.ToUpper(level) {
		case "DEBUG":
			globalLogger.SetLevel(DebugLevel)
		case "INFO":
			globalLogger.SetLevel(InfoLevel)
		case "WARN":
			globalLogger.SetLevel(WarnLevel)
		case "ERROR":
			globalLogger.SetLevel(ErrorLevel)
		case "FATAL":
			globalLogger.SetLevel(FatalLevel)
		}
	}

	// 日志格式
	if format := os.Getenv("COMPLIK_LOG_FORMAT"); format == "json" {
		globalLogger.jsonFormat = true
		globalLogger.colored = false
	}

	// 是否显示颜色
	if colored := os.Getenv("COMPLIK_LOG_COLORED"); colored == "false" {
		globalLogger.colored = false
	}

	// 是否显示调用位置
	if caller := os.Getenv("COMPLIK_LOG_CALLER"); caller == "false" {
		globalLogger.showCaller = false
	}

	// 日志文件
	if logFile := os.Getenv("COMPLIK_LOG_FILE"); logFile != "" {
		file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o666)
		if err == nil {
			globalLogger.SetOutput(file)
			globalLogger.colored = false // 文件输出不使用颜色
		}
	}
}

// GetLogger 获取全局日志实例
func GetLogger() Logger {
	if globalLogger == nil {
		Init()
	}
	return globalLogger
}

// New 创建新的日志实例
func New() Logger {
	return &StandardLogger{
		level:      InfoLevel,
		output:     os.Stdout,
		fields:     make(Fields),
		colored:    true,
		jsonFormat: false,
		showCaller: true,
		timeFormat: "2006-01-02 15:04:05.000",
	}
}

// SetLevel 设置日志级别
func (l *StandardLogger) SetLevel(level LogLevel) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

// SetOutput 设置输出
func (l *StandardLogger) SetOutput(w io.Writer) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.output = w
}

// WithField 添加单个字段
func (l *StandardLogger) WithField(key string, value any) Logger {
	return l.WithFields(Fields{key: value})
}

// WithFields 添加多个字段
func (l *StandardLogger) WithFields(fields Fields) Logger {
	l.mu.RLock()
	defer l.mu.RUnlock()

	newFields := make(Fields, len(l.fields)+len(fields))
	for k, v := range l.fields {
		newFields[k] = v
	}
	for k, v := range fields {
		newFields[k] = v
	}

	return &StandardLogger{
		level:      l.level,
		output:     l.output,
		fields:     newFields,
		ctx:        l.ctx,
		colored:    l.colored,
		jsonFormat: l.jsonFormat,
		showCaller: l.showCaller,
		timeFormat: l.timeFormat,
	}
}

// WithContext 添加上下文
func (l *StandardLogger) WithContext(ctx context.Context) Logger {
	l.mu.RLock()
	defer l.mu.RUnlock()

	newLogger := &StandardLogger{
		level:      l.level,
		output:     l.output,
		fields:     make(Fields, len(l.fields)),
		ctx:        ctx,
		colored:    l.colored,
		jsonFormat: l.jsonFormat,
		showCaller: l.showCaller,
		timeFormat: l.timeFormat,
	}

	for k, v := range l.fields {
		newLogger.fields[k] = v
	}

	// 从上下文提取请求ID等信息
	if ctx != nil {
		if requestID := ctx.Value("request_id"); requestID != nil {
			newLogger.fields["request_id"] = requestID
		}
		if traceID := ctx.Value("trace_id"); traceID != nil {
			newLogger.fields["trace_id"] = traceID
		}
	}

	return newLogger
}

// WithError 添加错误
func (l *StandardLogger) WithError(err error) Logger {
	if err == nil {
		return l
	}
	return l.WithField("error", err.Error())
}

// Debug 输出调试日志
func (l *StandardLogger) Debug(msg string, fields ...Fields) {
	l.log(DebugLevel, msg, fields...)
}

// Info 输出信息日志
func (l *StandardLogger) Info(msg string, fields ...Fields) {
	l.log(InfoLevel, msg, fields...)
}

// Warn 输出警告日志
func (l *StandardLogger) Warn(msg string, fields ...Fields) {
	l.log(WarnLevel, msg, fields...)
}

// Error 输出错误日志
func (l *StandardLogger) Error(msg string, fields ...Fields) {
	l.log(ErrorLevel, msg, fields...)
}

// Fatal 输出致命错误日志并退出
func (l *StandardLogger) Fatal(msg string, fields ...Fields) {
	l.log(FatalLevel, msg, fields...)
	os.Exit(1)
}

// log 核心日志方法
func (l *StandardLogger) log(level LogLevel, msg string, extraFields ...Fields) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if level < l.level {
		return
	}

	// 合并字段
	fields := make(Fields, len(l.fields))
	for k, v := range l.fields {
		fields[k] = v
	}
	for _, f := range extraFields {
		for k, v := range f {
			fields[k] = v
		}
	}

	// 添加基础字段
	fields["time"] = time.Now().Format(l.timeFormat)
	fields["level"] = logLevelNames[level]
	fields["msg"] = msg

	// 添加调用位置
	if l.showCaller {
		if pc, file, line, ok := runtime.Caller(2); ok {
			funcName := runtime.FuncForPC(pc).Name()
			fields["caller"] = fmt.Sprintf("%s:%d", filepath.Base(file), line)
			fields["func"] = filepath.Base(funcName)
		}
	}

	// 格式化输出
	var output string
	if l.jsonFormat {
		output = l.formatJSON(fields)
	} else {
		output = l.formatText(level, msg, fields)
	}

	// 写入输出
	fmt.Fprint(l.output, output)
}

// formatJSON JSON格式化
func (l *StandardLogger) formatJSON(fields Fields) string {
	data, err := json.Marshal(fields)
	if err != nil {
		return fmt.Sprintf(`{"error":"failed to marshal log: %v"}\n`, err)
	}
	return string(data) + "\n"
}

// formatText 文本格式化
func (l *StandardLogger) formatText(level LogLevel, msg string, fields Fields) string {
	var builder strings.Builder

	// 时间
	if t, ok := fields["time"].(string); ok {
		builder.WriteString(t)
		builder.WriteString(" ")
	}

	// 级别（带颜色）
	levelStr := logLevelNames[level]
	if l.colored {
		builder.WriteString(logLevelColors[level])
		builder.WriteString(fmt.Sprintf("[%-5s]", levelStr))
		builder.WriteString(resetColor)
	} else {
		builder.WriteString(fmt.Sprintf("[%-5s]", levelStr))
	}
	builder.WriteString(" ")

	// 调用位置
	if caller, ok := fields["caller"].(string); ok {
		builder.WriteString("[")
		builder.WriteString(caller)
		builder.WriteString("] ")
		delete(fields, "caller")
	}

	// 消息
	builder.WriteString(msg)

	// 其他字段
	delete(fields, "time")
	delete(fields, "level")
	delete(fields, "msg")
	delete(fields, "func")

	if len(fields) > 0 {
		builder.WriteString(" | ")
		first := true
		for k, v := range fields {
			if !first {
				builder.WriteString(", ")
			}
			builder.WriteString(fmt.Sprintf("%s=%v", k, v))
			first = false
		}
	}

	builder.WriteString("\n")
	return builder.String()
}

// 全局便捷方法
func Debug(msg string, fields ...Fields) {
	GetLogger().Debug(msg, fields...)
}

func Info(msg string, fields ...Fields) {
	GetLogger().Info(msg, fields...)
}

func Warn(msg string, fields ...Fields) {
	GetLogger().Warn(msg, fields...)
}

func Error(msg string, fields ...Fields) {
	GetLogger().Error(msg, fields...)
}

func Fatal(msg string, fields ...Fields) {
	GetLogger().Fatal(msg, fields...)
}

func WithField(key string, value any) Logger {
	return GetLogger().WithField(key, value)
}

func WithFields(fields Fields) Logger {
	return GetLogger().WithFields(fields)
}

func WithContext(ctx context.Context) Logger {
	return GetLogger().WithContext(ctx)
}

func WithError(err error) Logger {
	return GetLogger().WithError(err)
}
