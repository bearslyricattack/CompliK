package main

import (
	"context"
	"flag"
	"github.com/bearslyricattack/CompliK/procscan/pkg/config"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/bearslyricattack/CompliK/procscan/internal/scanner"
)

func main() {
	configPath := flag.String("config", "", "path to configuration file")
	flag.Parse()
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go handleSignals(cancel)
	s := scanner.NewScanner(cfg)
	if err := s.Start(ctx); err != nil {
		log.Printf("扫描器启动失败: %v", err)
		return
	}
}

func handleSignals(cancel context.CancelFunc) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigChan
	log.Printf("收到信号 %v，准备关闭...", sig)
	cancel()
}
