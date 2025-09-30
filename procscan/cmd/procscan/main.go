package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	log "github.com/bearslyricattack/CompliK/procscan/pkg/log"
	"github.com/fsnotify/fsnotify"
	"github.com/sirupsen/logrus"

	"github.com/bearslyricattack/CompliK/procscan/internal/scanner"
	"github.com/bearslyricattack/CompliK/procscan/pkg/config"
)

func main() {
	configPath := flag.String("config", "", "path to configuration file")
	flag.Parse()

	log.L.Info("procscan 正在启动...")

	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.L.Fatalf("加载初始配置失败: %v", err)
	}
	// 从初始配置设置日志级别
	if cfg.Scanner.LogLevel != "" {
		log.SetLevel(cfg.Scanner.LogLevel)
	}
	log.L.Info("初始配置加载成功。")

	s := scanner.NewScanner(cfg)

	go watchConfig(*configPath, s)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go handleSignals(cancel)

	if err := s.Start(ctx); err != nil {
		log.L.Errorf("扫描器启动失败: %v", err)
		return
	}
}

func watchConfig(configPath string, s *scanner.Scanner) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.L.WithField("error", err).Error("创建文件观察者失败")
		return
	}
	defer watcher.Close()

	configDir := filepath.Dir(configPath)
	if err := watcher.Add(configDir); err != nil {
		log.L.WithField("error", err).Error("添加文件观察路径失败")
		return
	}

	log.L.WithField("path", configPath).Info("开始监控配置文件")

	var lastHash string
	hash, err := fileHash(configPath)
	if err == nil {
		lastHash = hash
	}

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}

			if event.Op&(fsnotify.Write|fsnotify.Create) != 0 {
				newHash, err := fileHash(configPath)
				if err != nil {
					continue
				}

				if newHash != lastHash {
					log.L.WithFields(logrus.Fields{
						"file":     configPath,
						"old_hash": lastHash,
						"new_hash": newHash,
					}).Info("检测到配置文件内容发生变更，准备热加载...")

					lastHash = newHash
					newCfg, err := config.LoadConfig(configPath)
					if err != nil {
						log.L.WithField("error", err).Error("热加载期间读取新配置失败，继续使用旧配置")
						continue
					}
					s.UpdateConfig(newCfg)
				}
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.L.WithField("error", err).Error("文件观察者出错")
		}
	}
}

func fileHash(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func handleSignals(cancel context.CancelFunc) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigChan
	log.L.WithFields(logrus.Fields{
		"signal": sig.String(),
	}).Info("收到关闭信号，准备优雅关闭...")
	cancel()
}