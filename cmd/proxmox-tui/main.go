package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/devnullvoid/proxmox-tui/internal/app"
	"github.com/devnullvoid/proxmox-tui/internal/config"
	"github.com/devnullvoid/proxmox-tui/internal/ui/components"
	"github.com/devnullvoid/proxmox-tui/internal/ui/theme"
	"github.com/rivo/tview"
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
	configWizardFlag := flag.Bool("config-wizard", false, "Launch interactive config wizard and exit")
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

	// Pre-fill cfg from config file if present
	if *configPath != "" {
		_ = cfg.MergeWithFile(*configPath) // ignore error, just pre-fill what we can
	}

	// If --config-wizard is set, launch the wizard and exit
	if *configWizardFlag {
		app := tview.NewApplication()
		result := make(chan bool, 1) // true=saved, false=cancelled
		wizard := components.NewConfigWizardPage(app, cfg, *configPath, func(c *config.Config) error {
			err := components.SaveConfigToFile(c, *configPath)
			if err == nil {
				result <- true
			} else {
				result <- false
			}
			return err
		}, func() {
			result <- false
			app.Stop()
		})
		app.SetRoot(wizard, true)
		_ = app.Run()
		if <-result {
			fmt.Println("Configuration saved. Exiting.")
		}
		os.Exit(0)
	}

	cfg.SetDefaults()
	config.DebugEnabled = cfg.Debug

	if err := cfg.Validate(); err != nil {
		// Launch the config wizard if config is missing/invalid
		fmt.Println("🔧 Configuration Setup Required (launching wizard)")
		app := tview.NewApplication()
		result := make(chan bool, 1)
		wizard := components.NewConfigWizardPage(app, cfg, *configPath, func(c *config.Config) error {
			err := components.SaveConfigToFile(c, *configPath)
			if err == nil {
				result <- true
			} else {
				result <- false
			}
			return err
		}, func() {
			result <- false
			app.Stop()
		})
		app.SetRoot(wizard, true)
		_ = app.Run()
		if <-result {
			fmt.Println("Configuration saved. Exiting.")
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

	// Apply theme to tview global styles
	theme.ApplyCustomTheme(&cfg.Theme)
	theme.ApplyToTview()

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
