package cli

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestRootCommand(t *testing.T) {
	// Test that the root command is properly configured
	if RootCmd.Use != "pvetui" {
		t.Errorf("Expected root command use to be 'pvetui', got '%s'", RootCmd.Use)
	}

	if RootCmd.Short == "" {
		t.Error("Expected root command to have a short description")
	}

	if RootCmd.Long == "" {
		t.Error("Expected root command to have a long description")
	}
}

func TestConfigWizardCommand(t *testing.T) {
	// Find the config-wizard command
	var configWizardCmd *cobra.Command
	for _, cmd := range RootCmd.Commands() {
		if cmd.Use == "config-wizard" {
			configWizardCmd = cmd
			break
		}
	}

	if configWizardCmd == nil {
		t.Error("Expected config-wizard command to be added to root command")
		return
	}

	if configWizardCmd.Short == "" {
		t.Error("Expected config-wizard command to have a short description")
	}

	if configWizardCmd.Long == "" {
		t.Error("Expected config-wizard command to have a long description")
	}
}

func TestPersistentFlags(t *testing.T) {
	// Test that key persistent flags are present
	expectedFlags := []string{
		"config",
		"profile",
		"no-cache",
		"version",
		"config-wizard",
		"list-profiles",
		"addr",
		"user",
		"password",
		"token-id",
		"token-secret",
		"realm",
		"insecure",
		"api-path",
		"ssh-user",
		"debug",
		"cache-dir",
	}

	for _, flagName := range expectedFlags {
		if RootCmd.PersistentFlags().Lookup(flagName) == nil {
			t.Errorf("Expected persistent flag '%s' to be present", flagName)
		}
	}
}
