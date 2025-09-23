package main

import (
	"context"
	"flag"
	"github.com/bearslyricattack/CompliK/internal/app"
	"github.com/bearslyricattack/CompliK/pkg/logger"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/bearslyricattack/CompliK/procscan/internal/scanner"
)

func main() {

	configPath := flag.String("config", "", "path to configuration file")
	flag.Parse()

	log.Info("Starting CompliK", logger.Fields{
		"version": "1.0.0",
		"config":  *configPath,
	})

	if err := app.Run(*configPath); err != nil {
		log.Fatal("Application failed", logger.Fields{
			"error": err.Error(),
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go handleSignals(cancel)
	s := scanner.NewScanner()
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
