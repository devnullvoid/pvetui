package components

import (
	"fmt"
	"os"

	"github.com/devnullvoid/pvetui/internal/config"
)

// SaveConfigPreservingSOPS persists the current app config and re-encrypts with
// SOPS when the source config file was originally SOPS-encrypted.
func (a *App) SaveConfigPreservingSOPS() error {
	configPath, found := config.FindDefaultConfigPath()
	if !found {
		configPath = config.GetDefaultConfigPath()
	}

	wasSOPS := false
	if data, err := os.ReadFile(configPath); err == nil {
		wasSOPS = config.IsSOPSEncrypted(configPath, data)
	}

	if err := SaveConfigToFile(&a.config, configPath); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	if wasSOPS {
		if err := a.reEncryptConfigIfNeeded(configPath); err != nil {
			return fmt.Errorf("re-encrypt config: %w", err)
		}
	}

	return nil
}
