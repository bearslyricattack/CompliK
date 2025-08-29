package main

import (
	"flag"
	"github.com/bearslyricattack/CompliK/internal/app"
	"github.com/bearslyricattack/CompliK/pkg/logger"
	_ "github.com/bearslyricattack/CompliK/plugins/compliance/collector/browser"
	_ "github.com/bearslyricattack/CompliK/plugins/compliance/detector/custom"
	_ "github.com/bearslyricattack/CompliK/plugins/compliance/detector/safety"
	_ "github.com/bearslyricattack/CompliK/plugins/discovery/cronjob/complete"
	_ "github.com/bearslyricattack/CompliK/plugins/discovery/cronjob/devbox"
	_ "github.com/bearslyricattack/CompliK/plugins/discovery/informer/deployment"
	_ "github.com/bearslyricattack/CompliK/plugins/discovery/informer/endPointSlice"
	_ "github.com/bearslyricattack/CompliK/plugins/discovery/informer/statefulset"
	_ "github.com/bearslyricattack/CompliK/plugins/handle/database/postages"
	_ "github.com/bearslyricattack/CompliK/plugins/handle/lark"
	"os"
	"runtime/debug"
)

func main() {
	debug.SetTraceback("all")
	os.Setenv("GOTRACEBACK", "all")

	// 初始化日志系统
	logger.Init()
	log := logger.GetLogger()

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
}
