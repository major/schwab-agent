// Package auth handles Schwab API authentication and configuration.
package auth

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/major/schwab-agent/internal/apperr"
)

const (
	defaultBaseURL = "https://api.schwabapi.com"
)

// Config holds Schwab API credentials and settings.
type Config struct {
	ClientID                   string `json:"client_id"`
	ClientSecret               string `json:"client_secret"`
	BaseURL                    string `json:"base_url"`
	BaseURLInsecure            bool   `json:"base_url_insecure"`
	CallbackURL                string `json:"callback_url"`
	DefaultAccount             string `json:"default_account"`
	IAlsoLikeToLiveDangerously bool   `json:"i-also-like-to-live-dangerously"`
}

// APIBaseURL returns the normalized REST/OAuth base URL, defaulting to Schwab's
// production host when the config leaves base_url empty.
func (cfg *Config) APIBaseURL() string {
	if cfg == nil {
		return defaultBaseURL
	}

	normalizedBaseURL, err := normalizeBaseURL(cfg.BaseURL)
	if err != nil {
		trimmedBaseURL := strings.TrimSpace(cfg.BaseURL)
		if trimmedBaseURL != "" {
			return trimmedBaseURL
		}

		return defaultBaseURL
	}

	return normalizedBaseURL
}

// OAuthAuthorizeURL derives the authorize endpoint from base_url so a
// Schwab-compatible proxy can front both REST and OAuth traffic consistently.
func (cfg *Config) OAuthAuthorizeURL() string {
	return cfg.resolveAPIPath("/v1/oauth/authorize")
}

// OAuthTokenURL derives the token/refresh endpoint from base_url. We keep this
// separate from callback_url because the callback is still the local loopback
// listener, while authorize/token traffic may be routed through a proxy.
func (cfg *Config) OAuthTokenURL() string {
	return cfg.resolveAPIPath("/v1/oauth/token")
}

// HTTPClient returns an outbound HTTP client for REST or OAuth requests.
// When base_url_insecure is true we skip certificate verification so local
// self-signed proxy setups can terminate TLS without requiring a custom trust
// store on every developer machine.
func (cfg *Config) HTTPClient(timeout time.Duration) *http.Client {
	insecure := cfg != nil && cfg.BaseURLInsecure
	client := &http.Client{Timeout: timeout}

	if !insecure {
		return client
	}

	transport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return client
	}

	clonedTransport := transport.Clone()
	clonedTransport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // base_url_insecure is an explicit opt-in for local self-signed proxies
	client.Transport = clonedTransport

	return client
}

// resolveAPIPath joins an API path onto the normalized base URL while
// preserving any proxy path prefix present in base_url.
func (cfg *Config) resolveAPIPath(endpointPath string) string {
	baseURL, err := url.Parse(cfg.APIBaseURL())
	if err != nil {
		return cfg.APIBaseURL()
	}

	baseURL.Path = path.Join(strings.TrimRight(baseURL.Path, "/"), endpointPath)
	baseURL.RawPath = ""
	baseURL.RawQuery = ""
	baseURL.Fragment = ""

	return baseURL.String()
}

// normalizeBaseURL trims whitespace, enforces an absolute URL, and removes a
// trailing slash so downstream code can safely append REST/OAuth paths.
func normalizeBaseURL(rawBaseURL string) (string, error) {
	rawBaseURL = strings.TrimSpace(rawBaseURL)
	if rawBaseURL == "" {
		rawBaseURL = defaultBaseURL
	}

	parsedURL, err := url.Parse(rawBaseURL)
	if err != nil {
		return "", fmt.Errorf("parse base_url: %w", err)
	}

	if parsedURL.Scheme == "" || parsedURL.Host == "" {
		return "", fmt.Errorf("base_url must include scheme and host")
	}

	parsedURL.Path = strings.TrimRight(parsedURL.Path, "/")
	parsedURL.RawPath = ""
	parsedURL.RawQuery = ""
	parsedURL.Fragment = ""

	return parsedURL.String(), nil
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

	if baseURL := os.Getenv("SCHWAB_BASE_URL"); baseURL != "" {
		cfg.BaseURL = baseURL
	}

	if rawBaseURLInsecure, ok := os.LookupEnv("SCHWAB_BASE_URL_INSECURE"); ok {
		baseURLInsecure, err := strconv.ParseBool(strings.TrimSpace(rawBaseURLInsecure))
		if err != nil {
			return nil, apperr.NewValidationError(
				"Invalid SCHWAB_BASE_URL_INSECURE: expected true or false",
				err,
			)
		}

		cfg.BaseURLInsecure = baseURLInsecure
	}

	// Set default callback URL if still empty
	if cfg.CallbackURL == "" {
		cfg.CallbackURL = "https://127.0.0.1:8182"
	}

	normalizedBaseURL, err := normalizeBaseURL(cfg.BaseURL)
	if err != nil {
		return nil, apperr.NewValidationError("invalid base_url", err)
	}
	cfg.BaseURL = normalizedBaseURL

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
	data, err := json.MarshalIndent(cfg, "", "  ") //nolint:gosec // G117: config file intentionally stores client_secret with 0600 perms
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
