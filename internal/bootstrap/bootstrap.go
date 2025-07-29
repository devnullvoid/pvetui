// Package bootstrap handles application initialization and startup logic.
//
// This package provides a clean separation between the main entry point
// and the actual application startup process, making the code more
// testable and maintainable.
package bootstrap

import (
	"flag"
	"fmt"
	"strings"

	"github.com/devnullvoid/proxmox-tui/internal/app"
	"github.com/devnullvoid/proxmox-tui/internal/config"
	"github.com/devnullvoid/proxmox-tui/internal/onboarding"
	"github.com/devnullvoid/proxmox-tui/internal/profile"
	"github.com/devnullvoid/proxmox-tui/internal/ui/components"
	"github.com/devnullvoid/proxmox-tui/internal/ui/theme"
	"github.com/devnullvoid/proxmox-tui/internal/version"
)

// BootstrapOptions contains all the options for bootstrapping the application.
type BootstrapOptions struct {
	ConfigPath   string
	Profile      string
	NoCache      bool
	Version      bool
	ConfigWizard bool
}

// BootstrapResult contains the result of the bootstrap process.
type BootstrapResult struct {
	Config     *config.Config
	ConfigPath string
	Profile    string
	NoCache    bool
}

// ParseFlags parses command line flags and returns bootstrap options.
func ParseFlags() BootstrapOptions {
	configPathFlag := flag.String("config", "", "Path to YAML config file")
	profileFlag := flag.String("profile", "", "Connection profile to use (overrides default_profile)")
	noCacheFlag := flag.Bool("no-cache", false, "Disable caching")
	versionFlag := flag.Bool("version", false, "Show version information")
	configWizardFlag := flag.Bool("config-wizard", false, "Launch interactive config wizard and exit")
	flag.Parse()

	return BootstrapOptions{
		ConfigPath:   *configPathFlag,
		Profile:      *profileFlag,
		NoCache:      *noCacheFlag,
		Version:      *versionFlag,
		ConfigWizard: *configWizardFlag,
	}
}

// Bootstrap handles the complete application bootstrap process.
func Bootstrap(opts BootstrapOptions) (*BootstrapResult, error) {
	// Handle version flag
	if opts.Version {
		printVersion()
		return nil, nil
	}

	// Initialize configuration
	cfg := config.NewConfig()
	cfg.ParseFlags()

	// Resolve configuration path
	configPath := resolveConfigPath(opts.ConfigPath)
	if configPath != "" {
		if err := cfg.MergeWithFile(configPath); err != nil {
			return nil, fmt.Errorf("failed to load config file: %w", err)
		}
	}

	// Handle profile selection
	selectedProfile, err := profile.ResolveProfile(opts.Profile, cfg)
	if err != nil {
		return nil, fmt.Errorf("profile resolution failed: %w", err)
	}

	// Apply selected profile
	if selectedProfile != "" {
		if err := cfg.ApplyProfile(selectedProfile); err != nil {
			return nil, fmt.Errorf("could not select profile '%s': %w", selectedProfile, err)
		}
	}

	// Handle config wizard
	if opts.ConfigWizard {
		if err := handleConfigWizard(cfg, configPath, selectedProfile); err != nil {
			return nil, fmt.Errorf("config wizard failed: %w", err)
		}
		return nil, nil
	}

	// Set defaults and validate
	cfg.SetDefaults()
	config.DebugEnabled = cfg.Debug

	// Handle validation errors with onboarding
	if err := cfg.Validate(); err != nil {
		if err := onboarding.HandleValidationError(cfg, configPath, opts.NoCache, selectedProfile); err != nil {
			return nil, fmt.Errorf("onboarding failed: %w", err)
		}
		return nil, nil
	}

	return &BootstrapResult{
		Config:     cfg,
		ConfigPath: configPath,
		Profile:    selectedProfile,
		NoCache:    opts.NoCache,
	}, nil
}

// StartApplication starts the main application with the given configuration.
func StartApplication(result *BootstrapResult) error {
	if result == nil {
		return fmt.Errorf("bootstrap result is nil")
	}

	fmt.Println("\n🚀 Starting Proxmox TUI...")

	if result.ConfigPath != "" {
		fmt.Printf("✅ Configuration loaded from %s\n", result.ConfigPath)
	} else {
		fmt.Println("✅ Configuration loaded from environment variables")
	}

	// Apply theme configuration
	theme.ApplyCustomTheme(&result.Config.Theme)
	theme.ApplyToTview()

	appOpts := app.Options{NoCache: result.NoCache}
	if err := app.RunWithStartupVerification(result.Config, appOpts); err != nil {
		return handleStartupError(err, result.Config)
	}

	fmt.Println("🚪 Exiting.")
	return nil
}

// resolveConfigPath resolves the configuration file path.
func resolveConfigPath(flagPath string) string {
	if flagPath != "" {
		return flagPath
	}

	if path, found := config.FindDefaultConfigPath(); found {
		return path
	}

	return ""
}

// handleConfigWizard launches the configuration wizard.
func handleConfigWizard(cfg *config.Config, configPath string, activeProfile string) error {
	res := components.LaunchConfigWizard(cfg, configPath, activeProfile)

	switch {
	case res.SopsEncrypted:
		fmt.Printf("✅ Configuration saved and encrypted with SOPS: %s\n", configPath)
	case res.Saved:
		fmt.Println("✅ Configuration saved.")
	case res.Canceled:
		fmt.Println("🚪 Exiting.")
	}

	return nil
}

// handleStartupError provides user-friendly error messages for startup failures.
func handleStartupError(err error, cfg *config.Config) error {
	fmt.Printf("❌ %v\n", err)
	fmt.Println()

	if strings.Contains(err.Error(), "authentication failed") {
		fmt.Println("💡 Please check your credentials in the config file:")
		fmt.Printf("   %s\n", config.GetDefaultConfigPath())
	} else if strings.Contains(err.Error(), "connection") || strings.Contains(err.Error(), "timeout") {
		fmt.Println("💡 Please check your Proxmox server address and network connectivity:")
		fmt.Printf("   Current address: %s\n", cfg.Addr)
	}

	return err
}

// printVersion prints version information.
func printVersion() {
	info := version.GetBuildInfo()
	fmt.Printf("proxmox-tui version %s\n", info.Version)
	fmt.Printf("Build date: %s\n", info.BuildDate)
	fmt.Printf("Commit: %s\n", info.Commit)
	fmt.Printf("Go version: %s\n", info.GoVersion)
	fmt.Printf("OS/Arch: %s/%s\n", info.OS, info.Arch)
}
