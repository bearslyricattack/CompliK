package app

import (
	"fmt"
	"github.com/bearslyricattack/CompliK/pkg/eventbus"
	"github.com/bearslyricattack/CompliK/pkg/k8s"
	"github.com/bearslyricattack/CompliK/pkg/plugin"
	"github.com/bearslyricattack/CompliK/pkg/utils/config"
	"os"
	"os/signal"
	"syscall"
)

func Run(configPath string) error {
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}
	if err := k8s.InitClient(cfg.Kubeconfig); err != nil {
		return fmt.Errorf("failed to initialize Kubernetes client: %w", err)
	}
	eventBus := eventbus.NewEventBus(100)
	m := plugin.NewManager(eventBus)
	if err := m.LoadPlugins(cfg.Plugins); err != nil {
		return fmt.Errorf("failed to load plugins: %w", err)
	}

	if err := m.StartAll(); err != nil {
		return fmt.Errorf("failed to start plugins: %w", err)
	}
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan
	// TODO: Grace exit.
	return nil
}
