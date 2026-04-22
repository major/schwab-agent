// Package auth handles Schwab API authentication and configuration.
package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/major/schwab-agent/internal/apperr"
)

// Config holds Schwab API credentials and settings.
type Config struct {
	ClientID                  string `json:"client_id"`
	ClientSecret              string `json:"client_secret"`
	CallbackURL               string `json:"callback_url"`
	DefaultAccount            string `json:"default_account"`
	IAlsoLikeToLiveDangerously bool   `json:"i-also-like-to-live-dangerously"`
}

// LoadConfig reads config from file and applies env var overrides.
// Default path: ~/.config/schwab-agent/config.json
// Priority: env vars > config file > defaults
// Returns ValidationError if ClientID or ClientSecret are empty after all sources.
func LoadConfig(configPath string) (*Config, error) {
	cfg := &Config{}

	// Try to read config file if it exists
	data, err := os.ReadFile(configPath)
	if err != nil {
		if !os.IsNotExist(err) {
			// File exists but couldn't be read
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
		// File doesn't exist, start with empty config
	} else {
		// File exists, parse it
		if err := json.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("failed to parse config file: %w", err)
		}
	}

	// Apply env var overrides
	if clientID := os.Getenv("SCHWAB_CLIENT_ID"); clientID != "" {
		cfg.ClientID = clientID
	}

	if clientSecret := os.Getenv("SCHWAB_CLIENT_SECRET"); clientSecret != "" {
		cfg.ClientSecret = clientSecret
	}

	if callbackURL := os.Getenv("SCHWAB_CALLBACK_URL"); callbackURL != "" {
		cfg.CallbackURL = callbackURL
	}

	// Set default callback URL if still empty
	if cfg.CallbackURL == "" {
		cfg.CallbackURL = "https://127.0.0.1:8182"
	}

	// Validate required fields
	if cfg.ClientID == "" || cfg.ClientSecret == "" {
		return nil, apperr.NewValidationError(
			"Missing required credentials: set SCHWAB_CLIENT_ID and SCHWAB_CLIENT_SECRET env vars, or add client_id and client_secret to the config file",
			nil,
		)
	}

	return cfg, nil
}

// SaveConfig writes config to file with 0600 permissions.
// Creates parent directory with 0700 if missing.
func SaveConfig(configPath string, cfg *Config) error {
	// Create parent directory if it doesn't exist
	parentDir := filepath.Dir(configPath)
	if err := os.MkdirAll(parentDir, 0o700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Marshal config to JSON
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write file with 0600 permissions
	if err := os.WriteFile(configPath, data, 0o600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// SetDefaultAccount loads config, sets DefaultAccount, and saves it.
func SetDefaultAccount(configPath, hash string) error {
	// Load existing config (or create empty one if missing)
	cfg, err := LoadConfig(configPath)
	if err != nil {
		// If validation error (missing credentials) or file doesn't exist, create a minimal config
		var valErr *apperr.ValidationError
		if errors.As(err, &valErr) || os.IsNotExist(err) {
			// For validation errors or missing file, create empty config
			cfg = &Config{}
		} else {
			// Some other error occurred
			return err
		}
	}

	// Set the default account
	cfg.DefaultAccount = hash

	// Save the updated config
	return SaveConfig(configPath, cfg)
}
