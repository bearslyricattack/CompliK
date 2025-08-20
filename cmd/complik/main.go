package main

import (
	"flag"
	"github.com/bearslyricattack/CompliK/internal/app"
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
	"log"
	"os"
	"runtime/debug"
)

func main() {
	debug.SetTraceback("all")
	os.Setenv("GOTRACEBACK", "all")
	configPath := flag.String("config", "", "path to configuration file")
	flag.Parse()
	if err := app.Run(*configPath); err != nil {
		log.Fatalf("Application failed: %v", err)
	}
}
