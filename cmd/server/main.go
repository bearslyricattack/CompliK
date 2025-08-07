package main

import (
	"flag"
	"fmt"
	"github.com/bearslyricattack/CompliK/pkg/config"
	"github.com/bearslyricattack/CompliK/pkg/eventbus"
	"github.com/bearslyricattack/CompliK/pkg/k8s"
	"github.com/bearslyricattack/CompliK/pkg/manager"
	_ "github.com/bearslyricattack/CompliK/pkg/plugins/collector"
	_ "github.com/bearslyricattack/CompliK/pkg/plugins/compliance"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	configPath := flag.String("config", "", "path to configuration file")
	flag.Parse()

	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// 初始化k8s client
	err = k8s.InitClient(cfg.Kubeconfig)
	if err != nil {
		log.Fatalf("Failed to initialize Kubernetes client: %v", err)
	}

	eventBus := eventbus.NewEventBus()

	// 创建插件管理器
	m := manager.NewManager(eventBus)

	// 读取配置，注册插件
	err = m.LoadPlugins(cfg.Plugins)
	if err != nil {
		log.Fatalf("Failed to load plugins: %v", err)
		return
	}
	// 启动所有插件
	if err := m.StartAll(); err != nil {
		log.Fatalf("Failed to start plugins: %v", err)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	log.Println("Plugin demo is running. Press Ctrl+C to stop.")
	<-sigChan
	fmt.Println("程序已优雅退出")
}
