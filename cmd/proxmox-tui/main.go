package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/devnullvoid/proxmox-tui/internal/app"
	"github.com/devnullvoid/proxmox-tui/internal/config"
)

var (
	version    = "dev"
	buildDate  = "unknown"
	commitHash = "unknown"
)

func main() {
	cfg := config.NewConfig()
	cfg.ParseFlags()

	configPath := flag.String("config", "", "Path to YAML config file")
	noCacheFlag := flag.Bool("no-cache", false, "Disable caching")
	versionFlag := flag.Bool("version", false, "Show version information")
	flag.Parse()

	if *versionFlag {
		fmt.Printf("proxmox-tui version %s\n", version)
		fmt.Printf("Build date: %s\n", buildDate)
		fmt.Printf("Commit: %s\n", commitHash)
		return
	}

	// If no config path is provided via flag, try to find it in the default location
	if *configPath == "" {
		if path, found := config.FindDefaultConfigPath(); found {
			*configPath = path
		}
	}

	if *configPath != "" {
		if err := cfg.MergeWithFile(*configPath); err != nil {
			log.Fatalf("error loading config file %s: %v", *configPath, err)
		}
	}

	cfg.SetDefaults()
	config.DebugEnabled = cfg.Debug

	if err := cfg.Validate(); err != nil {
		log.Fatal(err)
	}

	if err := app.Run(cfg, app.Options{NoCache: *noCacheFlag}); err != nil {
		log.Fatalf("error running app: %v", err)
	}
}
