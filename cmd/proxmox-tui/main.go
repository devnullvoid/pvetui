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
			fmt.Println("✅ Configuration saved. Exiting.")
		}
		os.Exit(0)
	}

	cfg.SetDefaults()
	config.DebugEnabled = cfg.Debug

	if err := cfg.Validate(); err != nil {
		// If config is missing/invalid, prompt to create default config
		fmt.Println("🔧 Configuration Setup Required")
		fmt.Println()
		fmt.Printf("It looks like this is your first time running proxmox-tui, or your configuration needs attention.\n")
		fmt.Printf("Missing: %v\n", err)
		fmt.Println()
		defaultPath := config.GetDefaultConfigPath()
		fmt.Printf("Would you like to create a default configuration file at '%s'? [Y/n] ", defaultPath)

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
			// Prompt to launch editor
			fmt.Printf("Would you like to edit the new config in the interactive editor? [Y/n] ")
			editInput, _ := reader.ReadString('\n')
			editInput = strings.TrimSpace(strings.ToLower(editInput))
			if editInput == "y" || editInput == "" {
				// Load the new config and launch the wizard
				newCfg := config.NewConfig()
				_ = newCfg.MergeWithFile(path)
				tviewApp := tview.NewApplication()
				result := make(chan bool, 1)
				wizard := components.NewConfigWizardPage(tviewApp, newCfg, path, func(c *config.Config) error {
					err := components.SaveConfigToFile(c, path)
					if err == nil {
						result <- true
					} else {
						result <- false
					}
					return err
				}, func() {
					result <- false
					tviewApp.Stop()
				})
				tviewApp.SetRoot(wizard, true)
				_ = tviewApp.Run()
				if <-result {
					fmt.Println("✅ Configuration saved. Exiting.")
				}
				// After editing (or just after creation), ask if user wants to proceed to main app
				fmt.Printf("Would you like to proceed with main application startup? [Y/n] ")
				proceedInput, _ := reader.ReadString('\n')
				proceedInput = strings.TrimSpace(strings.ToLower(proceedInput))
				if proceedInput == "y" || proceedInput == "" {
					// Reload config (in case it was edited)
					*cfg = *config.NewConfig()
					_ = cfg.MergeWithFile(path)
					cfg.SetDefaults()
					config.DebugEnabled = cfg.Debug
					fmt.Println("\n🚀 Starting Proxmox TUI...")
					// Show config source
					fmt.Printf("✅ Configuration loaded from %s\n", path)
					// Apply theme to tview global styles
					theme.ApplyCustomTheme(&cfg.Theme)
					theme.ApplyToTview()
					if err := app.RunWithStartupVerification(cfg, app.Options{NoCache: *noCacheFlag}); err != nil {
						fmt.Printf("❌ %v\n", err)
						fmt.Println()
						if strings.Contains(err.Error(), "authentication failed") {
							fmt.Println("💡 Please check your credentials in the config file:")
							fmt.Printf("   %s\n", path)
						} else if strings.Contains(err.Error(), "connection") || strings.Contains(err.Error(), "timeout") {
							fmt.Println("💡 Please check your Proxmox server address and network connectivity:")
							fmt.Printf("   Current address: %s\n", cfg.Addr)
						}
						os.Exit(1)
					}
					os.Exit(0)
				} else {
					fmt.Println("Exiting.")
					os.Exit(0)
				}
			}
			os.Exit(0)
		} else {
			fmt.Println()
			fmt.Println("Configuration setup canceled. You can configure via flags or environment variables instead.")
			os.Exit(0)
		}
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
