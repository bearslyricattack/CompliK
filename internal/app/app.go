package app

import (
	"fmt"
	"github.com/bearslyricattack/CompliK/pkg/eventbus"
	"github.com/bearslyricattack/CompliK/pkg/k8s"
	"github.com/bearslyricattack/CompliK/pkg/logger"
	"github.com/bearslyricattack/CompliK/pkg/plugin"
	"github.com/bearslyricattack/CompliK/pkg/utils/config"
	"os"
	"os/signal"
	"syscall"
)

func Run(configPath string) error {
	log := logger.GetLogger()

	log.Info("Loading configuration", logger.Fields{"path": configPath})
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		log.Error("Failed to load configuration", logger.Fields{"error": err.Error()})
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	log.Info("Initializing Kubernetes client", logger.Fields{"kubeconfig": cfg.Kubeconfig})
	if err := k8s.InitClient(cfg.Kubeconfig); err != nil {
		log.Error("Failed to initialize Kubernetes client", logger.Fields{"error": err.Error()})
		return fmt.Errorf("failed to initialize Kubernetes client: %w", err)
	}

	log.Info("Creating event bus")
	eventBus := eventbus.NewEventBus(100)

	log.Info("Initializing plugin manager")
	m := plugin.NewManager(eventBus)

	log.Info("Loading plugins", logger.Fields{"count": len(cfg.Plugins)})
	if err := m.LoadPlugins(cfg.Plugins); err != nil {
		log.Error("Failed to load plugins", logger.Fields{"error": err.Error()})
		return fmt.Errorf("failed to load plugins: %w", err)
	}

	log.Info("Starting all plugins")
	if err := m.StartAll(); err != nil {
		log.Error("Failed to start plugins", logger.Fields{"error": err.Error()})
		return fmt.Errorf("failed to start plugins: %w", err)
	}

	log.Info("Application started successfully, waiting for shutdown signal")
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigChan

	// 优雅关闭
	log.Info("Received shutdown signal", logger.Fields{"signal": sig.String()})
	log.Info("Shutting down gracefully...")
	if err := m.StopAll(); err != nil {
		log.Error("Failed to stop plugins", logger.Fields{"error": err.Error()})
		return fmt.Errorf("failed to stop plugins: %w", err)
	}

	log.Info("Application shutdown completed")
	return nil
}
