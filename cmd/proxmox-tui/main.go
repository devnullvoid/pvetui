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
		fmt.Println("🔧 Configuration Setup Required")
		fmt.Println()
		fmt.Printf("It looks like this is your first time running proxmox-tui, or your configuration needs attention.\n")
		fmt.Printf("Missing: %v\n", err)
		fmt.Println()
		fmt.Printf("Would you like to create a default configuration file at '%s'? [Y/n] ", config.GetDefaultConfigPath())

		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(strings.ToLower(input))

		if input == "y" || input == "" {
			fmt.Println()
			path, createErr := config.CreateDefaultConfigFile()
			if createErr != nil {
				log.Fatalf("Error creating config file: %v", createErr)
			}
			fmt.Printf("✅ Success! Configuration file created at %s\n", path)
			fmt.Println()
			fmt.Println("Please edit it with your Proxmox details and run the application again.")
		} else {
			fmt.Println()
			fmt.Println("Configuration setup canceled. You can configure via flags or environment variables instead.")
		}
		os.Exit(0)
	}

	// Start the application with startup verification
	fmt.Println("🚀 Starting Proxmox TUI...")

	// Show config source
	if *configPath != "" {
		fmt.Printf("✅ Configuration loaded from %s\n", *configPath)
	} else {
		fmt.Println("✅ Configuration loaded from environment variables")
	}

	if err := app.RunWithStartupVerification(cfg, app.Options{NoCache: *noCacheFlag}); err != nil {
		fmt.Printf("❌ %v\n", err)
		fmt.Println()
		if strings.Contains(err.Error(), "authentication failed") {
			fmt.Println("💡 Please check your credentials in the config file:")
			if *configPath != "" {
				fmt.Printf("   %s\n", *configPath)
			} else {
				fmt.Printf("   %s\n", config.GetDefaultConfigPath())
			}
		} else if strings.Contains(err.Error(), "connection") || strings.Contains(err.Error(), "timeout") {
			fmt.Println("💡 Please check your Proxmox server address and network connectivity:")
			fmt.Printf("   Current address: %s\n", cfg.Addr)
		}
		os.Exit(1)
	}
}
