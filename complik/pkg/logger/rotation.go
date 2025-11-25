package logger

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// RotatingFileWriter 支持日志轮转的文件写入器
type RotatingFileWriter struct {
	mu          sync.Mutex
	file        *os.File
	filename    string
	maxSize     int64 // 最大文件大小（字节）
	maxBackups  int   // 保留的备份文件数量
	maxAge      int   // 保留的最大天数
	currentSize int64
	lastRotate  time.Time
}

// NewRotatingFileWriter 创建轮转文件写入器
func NewRotatingFileWriter(
	filename string,
	maxSize int64,
	maxBackups, maxAge int,
) (*RotatingFileWriter, error) {
	w := &RotatingFileWriter{
		filename:   filename,
		maxSize:    maxSize,
		maxBackups: maxBackups,
		maxAge:     maxAge,
		lastRotate: time.Now(),
	}

	// 确保目录存在
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}

	// 打开文件
	if err := w.openFile(); err != nil {
		return nil, err
	}

	// 启动清理协程
	go w.cleanupOldFiles()

	return w, nil
}

// Write 实现 io.Writer 接口
func (w *RotatingFileWriter) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// 检查是否需要轮转
	if w.shouldRotate(int64(len(p))) {
		if err := w.rotate(); err != nil {
			return 0, err
		}
	}

	n, err = w.file.Write(p)
	w.currentSize += int64(n)
	return n, err
}

// Close 关闭文件
func (w *RotatingFileWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.file != nil {
		return w.file.Close()
	}
	return nil
}

// shouldRotate 检查是否需要轮转
func (w *RotatingFileWriter) shouldRotate(writeSize int64) bool {
	// 按大小轮转
	if w.maxSize > 0 && w.currentSize+writeSize > w.maxSize {
		return true
	}

	// 按日期轮转（每天）
	if time.Since(w.lastRotate) > 24*time.Hour {
		return true
	}

	return false
}

// rotate 执行日志轮转
func (w *RotatingFileWriter) rotate() error {
	// 关闭当前文件
	if w.file != nil {
		w.file.Close()
	}

	// 重命名当前文件
	backupName := w.backupName()
	if err := os.Rename(w.filename, backupName); err != nil && !os.IsNotExist(err) {
		return err
	}

	// 打开新文件
	if err := w.openFile(); err != nil {
		return err
	}

	w.lastRotate = time.Now()

	// 清理旧文件
	w.cleanupBackups()

	return nil
}

// openFile 打开日志文件
func (w *RotatingFileWriter) openFile() error {
	file, err := os.OpenFile(w.filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o666)
	if err != nil {
		return err
	}

	// 获取文件大小
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return err
	}

	w.file = file
	w.currentSize = info.Size()
	return nil
}

// backupName 生成备份文件名
func (w *RotatingFileWriter) backupName() string {
	dir := filepath.Dir(w.filename)
	base := filepath.Base(w.filename)
	ext := filepath.Ext(base)
	name := base[:len(base)-len(ext)]

	timestamp := time.Now().Format("20060102-150405")
	return filepath.Join(dir, fmt.Sprintf("%s-%s%s", name, timestamp, ext))
}

// cleanupBackups 清理旧的备份文件
func (w *RotatingFileWriter) cleanupBackups() {
	dir := filepath.Dir(w.filename)
	base := filepath.Base(w.filename)
	ext := filepath.Ext(base)
	name := base[:len(base)-len(ext)]

	pattern := filepath.Join(dir, fmt.Sprintf("%s-*%s", name, ext))
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return
	}

	// 按修改时间排序
	type fileInfo struct {
		path    string
		modTime time.Time
	}
	files := make([]fileInfo, 0, len(matches))
	for _, match := range matches {
		info, err := os.Stat(match)
		if err != nil {
			continue
		}
		files = append(files, fileInfo{
			path:    match,
			modTime: info.ModTime(),
		})
	}
	if w.maxBackups > 0 && len(files) > w.maxBackups {
		// 按时间排序，保留最新的
		for i := range len(files) - w.maxBackups {
			os.Remove(files[i].path)
		}
	}

	// 删除超过时间限制的文件
	if w.maxAge > 0 {
		cutoff := time.Now().AddDate(0, 0, -w.maxAge)
		for _, f := range files {
			if f.modTime.Before(cutoff) {
				os.Remove(f.path)
			}
		}
	}
}

// cleanupOldFiles 定期清理旧文件
func (w *RotatingFileWriter) cleanupOldFiles() {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		w.mu.Lock()
		w.cleanupBackups()
		w.mu.Unlock()
	}
}

// MultiWriter 多输出写入器
type MultiWriter struct {
	writers []io.Writer
}

// NewMultiWriter 创建多输出写入器
func NewMultiWriter(writers ...io.Writer) *MultiWriter {
	return &MultiWriter{writers: writers}
}

// Write 实现 io.Writer 接口
func (mw *MultiWriter) Write(p []byte) (n int, err error) {
	for _, w := range mw.writers {
		n, err = w.Write(p)
		if err != nil {
			return n, err
		}
	}
	return len(p), nil
}
