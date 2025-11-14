// Package config provides encryption utilities for sensitive configuration fields.
//
// This module implements age-based encryption for sensitive data when SOPS is not used.
// It allows users to manually edit config files with cleartext, which will be
// automatically encrypted after successful connection verification.
package config

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"filippo.io/age"
)

const (
	// encryptionPrefix marks encrypted fields in the config file.
	encryptionPrefix = "age1:"
	// identityFileName is the name of the age identity file stored in the config directory.
	identityFileName = ".age-identity"
	// recipientFileName is the name of the age recipient file stored in the config directory.
	recipientFileName = ".age-recipient"
)

// getOrCreateAgeIdentity returns an age identity for encryption/decryption.
// Creates a new identity if one doesn't exist, storing it in the config directory.
func getOrCreateAgeIdentity() (age.Identity, age.Recipient, error) {
	configDir := getConfigDir()
	identityPath := filepath.Join(configDir, identityFileName)
	recipientPath := filepath.Join(configDir, recipientFileName)

	// Try to load existing identity
	if data, err := os.ReadFile(identityPath); err == nil {
		identities, err := age.ParseIdentities(strings.NewReader(string(data)))
		if err != nil {
			return nil, nil, fmt.Errorf("parse existing identity: %w", err)
		}
		if len(identities) == 0 {
			return nil, nil, fmt.Errorf("no identity found in file")
		}
		identity := identities[0]
		// Try to load recipient from file, or derive from identity
		var recipient age.Recipient
		if recipientData, err := os.ReadFile(recipientPath); err == nil {
			recipients, err := age.ParseRecipients(strings.NewReader(string(recipientData)))
			if err == nil && len(recipients) > 0 {
				recipient = recipients[0]
			}
		}
		// If recipient file doesn't exist or parsing failed, try to get from identity
		if recipient == nil {
			// For X25519Identity, we can extract the recipient
			if x25519Identity, ok := identity.(*age.X25519Identity); ok {
				recipient = x25519Identity.Recipient()
			} else {
				return nil, nil, fmt.Errorf("unsupported identity type, cannot get recipient")
			}
		}
		return identity, recipient, nil
	}

	// Create new identity
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		return nil, nil, fmt.Errorf("generate identity: %w", err)
	}

	recipient := identity.Recipient()

	// Ensure config directory exists
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		return nil, nil, fmt.Errorf("create config directory: %w", err)
	}

	// Save identity (private key)
	identityStr := identity.String()
	if err := os.WriteFile(identityPath, []byte(identityStr), 0o600); err != nil {
		return nil, nil, fmt.Errorf("save identity: %w", err)
	}

	// Save recipient (public key) for reference
	recipientStr := recipient.String()
	if err := os.WriteFile(recipientPath, []byte(recipientStr), 0o600); err != nil {
		return nil, nil, fmt.Errorf("save recipient: %w", err)
	}

	return identity, recipient, nil
}

// isEncrypted checks if a string value is encrypted (starts with encryption prefix).
func isEncrypted(value string) bool {
	return strings.HasPrefix(value, encryptionPrefix)
}

// EncryptField encrypts a sensitive field value using age encryption.
// Returns the encrypted value with the encryption prefix, or an error.
func EncryptField(value string) (string, error) {
	if value == "" {
		return "", nil
	}

	if isEncrypted(value) {
		// Already encrypted, return as-is
		return value, nil
	}

	_, recipient, err := getOrCreateAgeIdentity()
	if err != nil {
		return "", fmt.Errorf("get age identity: %w", err)
	}

	// Encrypt the value
	var encrypted bytes.Buffer

	encryptWriter, err := age.Encrypt(&encrypted, recipient)
	if err != nil {
		return "", fmt.Errorf("create encrypt writer: %w", err)
	}

	if _, err := encryptWriter.Write([]byte(value)); err != nil {
		return "", fmt.Errorf("write to encrypt: %w", err)
	}

	if err := encryptWriter.Close(); err != nil {
		return "", fmt.Errorf("close encrypt writer: %w", err)
	}

	// Base64 encode the encrypted data for safe YAML storage
	encryptedData := base64.StdEncoding.EncodeToString(encrypted.Bytes())
	return encryptionPrefix + encryptedData, nil
}

// DecryptField decrypts an encrypted field value.
// Returns the decrypted value, or the original value if not encrypted.
func DecryptField(value string) (string, error) {
	if value == "" {
		return "", nil
	}

	if !isEncrypted(value) {
		// Not encrypted, return as-is
		return value, nil
	}

	// Remove prefix and decode base64
	encryptedData := strings.TrimPrefix(value, encryptionPrefix)
	decoded, err := base64.StdEncoding.DecodeString(encryptedData)
	if err != nil {
		return "", fmt.Errorf("decode base64: %w", err)
	}

	identity, _, err := getOrCreateAgeIdentity()
	if err != nil {
		return "", fmt.Errorf("get age identity: %w", err)
	}

	// Decrypt the value
	decryptReader, err := age.Decrypt(bytes.NewReader(decoded), identity)
	if err != nil {
		return "", fmt.Errorf("create decrypt reader: %w", err)
	}

	var decrypted bytes.Buffer
	if _, err := io.Copy(&decrypted, decryptReader); err != nil {
		return "", fmt.Errorf("read decrypted data: %w", err)
	}

	return decrypted.String(), nil
}

// EncryptSensitiveFields encrypts all sensitive fields in a ProfileConfig.
// Fields that are already encrypted are left unchanged.
func EncryptSensitiveFields(profile *ProfileConfig) error {
	var err error

	if profile.Password != "" && !isEncrypted(profile.Password) {
		profile.Password, err = EncryptField(profile.Password)
		if err != nil {
			return fmt.Errorf("encrypt password: %w", err)
		}
	}

	if profile.TokenSecret != "" && !isEncrypted(profile.TokenSecret) {
		profile.TokenSecret, err = EncryptField(profile.TokenSecret)
		if err != nil {
			return fmt.Errorf("encrypt token_secret: %w", err)
		}
	}

	return nil
}

// DecryptSensitiveFields decrypts all sensitive fields in a ProfileConfig.
// Fields that are not encrypted are left unchanged.
func DecryptSensitiveFields(profile *ProfileConfig) error {
	var err error

	if profile.Password != "" {
		profile.Password, err = DecryptField(profile.Password)
		if err != nil {
			return fmt.Errorf("decrypt password: %w", err)
		}
	}

	if profile.TokenSecret != "" {
		profile.TokenSecret, err = DecryptField(profile.TokenSecret)
		if err != nil {
			return fmt.Errorf("decrypt token_secret: %w", err)
		}
	}

	return nil
}

// EncryptConfigSensitiveFields encrypts sensitive fields in the entire Config.
// This includes both profile-based and legacy fields.
func EncryptConfigSensitiveFields(cfg *Config) error {
	// Encrypt profile fields
	for name, profile := range cfg.Profiles {
		if err := EncryptSensitiveFields(&profile); err != nil {
			return fmt.Errorf("encrypt profile %s: %w", name, err)
		}
		cfg.Profiles[name] = profile
	}

	// Encrypt legacy fields
	if cfg.Password != "" && !isEncrypted(cfg.Password) {
		var err error
		cfg.Password, err = EncryptField(cfg.Password)
		if err != nil {
			return fmt.Errorf("encrypt legacy password: %w", err)
		}
	}

	if cfg.TokenSecret != "" && !isEncrypted(cfg.TokenSecret) {
		var err error
		cfg.TokenSecret, err = EncryptField(cfg.TokenSecret)
		if err != nil {
			return fmt.Errorf("encrypt legacy token_secret: %w", err)
		}
	}

	return nil
}

// DecryptConfigSensitiveFields decrypts sensitive fields in the entire Config.
// This includes both profile-based and legacy fields.
func DecryptConfigSensitiveFields(cfg *Config) error {
	// Decrypt profile fields
	for name, profile := range cfg.Profiles {
		if err := DecryptSensitiveFields(&profile); err != nil {
			return fmt.Errorf("decrypt profile %s: %w", name, err)
		}
		cfg.Profiles[name] = profile
	}

	// Decrypt legacy fields
	if cfg.Password != "" {
		var err error
		cfg.Password, err = DecryptField(cfg.Password)
		if err != nil {
			return fmt.Errorf("decrypt legacy password: %w", err)
		}
	}

	if cfg.TokenSecret != "" {
		var err error
		cfg.TokenSecret, err = DecryptField(cfg.TokenSecret)
		if err != nil {
			return fmt.Errorf("decrypt legacy token_secret: %w", err)
		}
	}

	return nil
}
