package main

import (
	"context"
	"github.com/bearslyricattack/CompliK/mining/internal/scanner"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	// 创建上下文
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 处理优雅关闭
	go handleSignals(cancel)

	// 创建并启动扫描器
	s := scanner.NewScanner()
	if err := s.Start(ctx); err != nil {
		log.Fatalf("扫描器启动失败: %v", err)
	}
}

// handleSignals 处理系统信号，实现优雅关闭
func handleSignals(cancel context.CancelFunc) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigChan
	log.Printf("收到信号 %v，准备关闭...", sig)
	cancel()
}
