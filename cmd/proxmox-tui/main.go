package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

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
		fmt.Printf("Configuration is missing or invalid: %v\n", err)
		fmt.Printf("Would you like to create a default configuration file at '%s'? [Y/n] ", config.GetDefaultConfigPath())

		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(strings.ToLower(input))

		if input == "y" || input == "" {
			path, createErr := config.CreateDefaultConfigFile()
			if createErr != nil {
				log.Fatalf("Error creating config file: %v", createErr)
			}
			fmt.Printf("✅ Success! Configuration file created at %s.\n", path)
			fmt.Println("Please edit it with your Proxmox details and run the application again.")
		} else {
			fmt.Println("Configuration setup canceled. You can configure via flags or environment variables instead.")
		}
		os.Exit(0)
	}

	if err := app.Run(cfg, app.Options{NoCache: *noCacheFlag}); err != nil {
		log.Fatalf("error running app: %v", err)
	}
}
