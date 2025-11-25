package log

import (
	"github.com/sirupsen/logrus"
	"os"
)

// L 是一个全局的、标准化的 logrus 日志记录器实例。
var L = logrus.New()

func init() {
	// 设置日志输出为 JSON 格式。
	L.SetFormatter(&logrus.JSONFormatter{})
	// 设置日志输出到标准输出。
	L.SetOutput(os.Stdout)
	// 设置一个初始的默认级别。
	L.SetLevel(logrus.InfoLevel)
}

// SetLevel 从字符串解析并设置全局日志记录器的级别。
func SetLevel(levelStr string) {
	level, err := logrus.ParseLevel(levelStr)
	if err != nil {
		L.WithField("error", err).Warnf("无效的日志级别 '%s'，将继续使用当前级别", levelStr)
		return
	}
	L.SetLevel(level)
	L.WithField("new_level", level.String()).Info("日志级别已更新")
}
