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
	"github.com/devnullvoid/proxmox-tui/internal/logger"
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
	var configPath, profile string
	var noCache, version, configWizard bool

	// Bootstrap flags
	flag.StringVar(&configPath, "config", "", "Path to YAML config file")
	flag.StringVar(&configPath, "c", "", "Short for --config")
	flag.StringVar(&profile, "profile", "", "Connection profile to use (overrides default_profile)")
	flag.StringVar(&profile, "p", "", "Short for --profile")
	flag.BoolVar(&noCache, "no-cache", false, "Disable caching")
	flag.BoolVar(&noCache, "n", false, "Short for --no-cache")
	flag.BoolVar(&version, "version", false, "Show version information")
	flag.BoolVar(&version, "v", false, "Short for --version")
	flag.BoolVar(&configWizard, "config-wizard", false, "Launch interactive config wizard and exit")
	flag.BoolVar(&configWizard, "w", false, "Short for --config-wizard")

	// Config flags (these will be applied to the config object later)
	flag.String("addr", "", "Proxmox API URL (env PROXMOX_ADDR)")
	flag.String("a", "", "Short for --addr")
	flag.String("user", "", "Proxmox username (env PROXMOX_USER)")
	flag.String("u", "", "Short for --user")
	flag.String("password", "", "Proxmox password (env PROXMOX_PASSWORD)")
	flag.String("pass", "", "Short for --password")
	flag.String("token-id", "", "Proxmox API token ID (env PROXMOX_TOKEN_ID)")
	flag.String("tid", "", "Short for --token-id")
	flag.String("token-secret", "", "Proxmox API token secret (env PROXMOX_TOKEN_SECRET)")
	flag.String("ts", "", "Short for --token-secret")
	flag.String("realm", "", "Proxmox realm (env PROXMOX_REALM)")
	flag.String("r", "", "Short for --realm")
	flag.Bool("insecure", false, "Skip TLS verification (env PROXMOX_INSECURE)")
	flag.Bool("i", false, "Short for --insecure")
	flag.String("api-path", "", "Proxmox API path (env PROXMOX_API_PATH)")
	flag.String("ap", "", "Short for --api-path")
	flag.String("ssh-user", "", "SSH username (env PROXMOX_SSH_USER)")
	flag.String("su", "", "Short for --ssh-user")
	flag.Bool("debug", false, "Enable debug logging (env PROXMOX_DEBUG)")
	flag.Bool("d", false, "Short for --debug")
	flag.String("cache-dir", "", "Cache directory path (env PROXMOX_CACHE_DIR)")
	flag.String("cd", "", "Short for --cache-dir")

	flag.Parse()

	return BootstrapOptions{
		ConfigPath:   configPath,
		Profile:      profile,
		NoCache:      noCache,
		Version:      version,
		ConfigWizard: configWizard,
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
	// Apply command line flags to config
	applyFlagsToConfig(cfg)

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
	logger.SetDebugEnabled(cfg.Debug)

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

// applyFlagsToConfig applies command line flags to the config object
func applyFlagsToConfig(cfg *config.Config) {
	// Get flag values and apply them to config if they were set
	if addr := flag.Lookup("addr"); addr != nil && addr.Value.String() != "" {
		cfg.Addr = addr.Value.String()
	}
	if user := flag.Lookup("user"); user != nil && user.Value.String() != "" {
		cfg.User = user.Value.String()
	}
	if password := flag.Lookup("password"); password != nil && password.Value.String() != "" {
		cfg.Password = password.Value.String()
	}
	if tokenID := flag.Lookup("token-id"); tokenID != nil && tokenID.Value.String() != "" {
		cfg.TokenID = tokenID.Value.String()
	}
	if tokenSecret := flag.Lookup("token-secret"); tokenSecret != nil && tokenSecret.Value.String() != "" {
		cfg.TokenSecret = tokenSecret.Value.String()
	}
	if realm := flag.Lookup("realm"); realm != nil && realm.Value.String() != "" {
		cfg.Realm = realm.Value.String()
	}
	if insecure := flag.Lookup("insecure"); insecure != nil && insecure.Value.String() == "true" {
		cfg.Insecure = true
	}
	if apiPath := flag.Lookup("api-path"); apiPath != nil && apiPath.Value.String() != "" {
		cfg.ApiPath = apiPath.Value.String()
	}
	if sshUser := flag.Lookup("ssh-user"); sshUser != nil && sshUser.Value.String() != "" {
		cfg.SSHUser = sshUser.Value.String()
	}
	if debug := flag.Lookup("debug"); debug != nil && debug.Value.String() == "true" {
		cfg.Debug = true
	}
	if cacheDir := flag.Lookup("cache-dir"); cacheDir != nil && cacheDir.Value.String() != "" {
		cfg.CacheDir = cacheDir.Value.String()
	}
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
	if err := app.RunWithStartupVerification(result.Config, result.ConfigPath, appOpts); err != nil {
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
