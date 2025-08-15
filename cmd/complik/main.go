package main

import (
	"flag"
	"github.com/bearslyricattack/CompliK/internal/app"
	_ "github.com/bearslyricattack/CompliK/plugins/compliance/collector"
	_ "github.com/bearslyricattack/CompliK/plugins/compliance/website"
	_ "github.com/bearslyricattack/CompliK/plugins/discovery/cronjob/complete"
	_ "github.com/bearslyricattack/CompliK/plugins/discovery/cronjob/devbox"
	_ "github.com/bearslyricattack/CompliK/plugins/discovery/informer/deployment"
	_ "github.com/bearslyricattack/CompliK/plugins/discovery/informer/endPointSlice"
	_ "github.com/bearslyricattack/CompliK/plugins/handle/database"
	_ "github.com/bearslyricattack/CompliK/plugins/handle/lark"
	"log"
)

func main() {
	configPath := flag.String("config", "", "path to configuration file")
	flag.Parse()
	if err := app.Run(*configPath); err != nil {
		log.Fatalf("Application failed: %v", err)
	}
}
