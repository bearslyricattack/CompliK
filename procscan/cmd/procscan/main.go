package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/bearslyricattack/CompliK/procscan/internal/config"
	"github.com/bearslyricattack/CompliK/procscan/internal/core/scanner"
	legacy "github.com/bearslyricattack/CompliK/procscan/pkg/logger/legacy"
	"github.com/sirupsen/logrus"
)

func main() {
	configPath := flag.String("config", "", "path to configuration file")
	flag.Parse()

	legacy.L.Info("ProcScan is starting...")

	// Load initial configuration
	loader := config.NewLoader(*configPath)
	cfg, err := loader.Load()
	if err != nil {
		legacy.L.Fatalf("Failed to load initial configuration: %v", err)
	}

	// Set log level from initial configuration
	if cfg.Scanner.LogLevel != "" {
		legacy.SetLevel(cfg.Scanner.LogLevel)
	}
	legacy.L.Info("Initial configuration loaded successfully")

	// Create scanner
	s := scanner.NewScanner(cfg)

	// Setup configuration watcher
	configWatcher, err := config.NewWatcher(loader, s.UpdateConfig)
	if err != nil {
		legacy.L.WithError(err).Warn("Failed to create configuration watcher, hot-reload will be unavailable")
	} else {
		ctx := context.Background()
		if err := configWatcher.Start(ctx); err != nil {
			legacy.L.WithError(err).Warn("Failed to start configuration watcher, hot-reload will be unavailable")
		} else {
			defer configWatcher.Stop()
		}
	}

	// Setup context and signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go handleSignals(cancel)

	// Start scanner
	if err := s.Start(ctx); err != nil {
		legacy.L.Errorf("Failed to start scanner: %v", err)
		return
	}
}

// handleSignals handles OS signals for graceful shutdown
func handleSignals(cancel context.CancelFunc) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigChan
	legacy.L.WithFields(logrus.Fields{
		"signal": sig.String(),
	}).Info("Received shutdown signal, preparing graceful shutdown...")
	cancel()
}
