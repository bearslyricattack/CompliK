package main

import (
	"flag"
	"fmt"
	"github.com/bearslyricattack/CompliK/pkg/config"
	"github.com/bearslyricattack/CompliK/pkg/eventbus"
	"github.com/bearslyricattack/CompliK/pkg/manager"
	_ "github.com/bearslyricattack/CompliK/pkg/plugins"
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

	// 创建事件总线
	eventBus := eventbus.NewEventBus()

	// 创建插件管理器
	manager := manager.NewManager(eventBus)

	// 读取配置，注册插件
	manager.LoadPlugins(cfg.Plugins)

	fmt.Printf("Plugins loaded: %v\n", cfg.Plugins)
	// 启动所有插件，传入事件总线
	if err := manager.StartAll(); err != nil {
		log.Fatalf("Failed to start plugins: %v", err)
	}

	// 等待信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	log.Println("Plugin demo is running. Press Ctrl+C to stop.")
	<-sigChan
	fmt.Println("程序已优雅退出")
}
