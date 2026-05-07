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

	schwabauth "github.com/major/schwab-go/schwab/auth"

	"github.com/major/schwab-agent/internal/apperr"
)

// ErrMissingCredentials is a sentinel error wrapped by the ValidationError
// returned from LoadConfig when client_id or client_secret are absent. Callers
// can use [errors.Is] with ErrMissingCredentials to distinguish missing
// credentials from other validation failures (e.g., invalid base_url) without
// matching on error message text.
var ErrMissingCredentials = errors.New("missing required credentials")

const (
	defaultBaseURL     = "https://api.schwabapi.com"
	defaultCallbackURL = "https://127.0.0.1:8182"
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

// TLSConfig returns a TLS configuration for OAuth and API HTTP clients.
// When base_url_insecure is true we skip certificate verification so local
// self-signed proxy setups can terminate TLS without requiring a custom trust
// store on every developer machine. Returns nil for standard secure connections.
func (cfg *Config) TLSConfig() *tls.Config {
	if cfg == nil || !cfg.BaseURLInsecure {
		return nil
	}
	return &tls.Config{
		InsecureSkipVerify: true, //nolint:gosec // base_url_insecure is an explicit opt-in for local self-signed proxies.
	}
}

// schwabAuthConfig adapts schwab-agent's app-owned config to schwab-go's
// OAuth-only config. The optional tokenEndpoint keeps old tests and injected
// dependencies able to point directly at a token endpoint while production code
// derives OAuth endpoints from base_url.
func (cfg *Config) schwabAuthConfig(tokenEndpoint string) (schwabauth.Config, error) {
	if cfg == nil {
		return schwabauth.Config{}, errors.New("auth config is required")
	}

	oauthBaseURL := tokenEndpointOAuthBaseURL(tokenEndpoint)
	if oauthBaseURL == "" {
		var err error
		oauthBaseURL, err = schwabauth.OAuthBaseURLFromAPIBaseURL(cfg.APIBaseURL())
		if err != nil {
			return schwabauth.Config{}, fmt.Errorf("derive OAuth base URL: %w", err)
		}
	}

	callbackURL := cfg.CallbackURL
	if callbackURL == "" {
		callbackURL = defaultCallbackURL
	}

	schwabCfg := schwabauth.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		CallbackURL:  callbackURL,
		OAuthBaseURL: oauthBaseURL,
	}
	if err := schwabCfg.Validate(); err != nil {
		return schwabauth.Config{}, fmt.Errorf("validate schwab auth config: %w", err)
	}

	return schwabCfg, nil
}

// oauthHTTPClient returns the HTTP client used by schwab-go for outbound OAuth
// token exchange and refresh requests. Keeping this adapter preserves
// schwab-agent's base_url_insecure proxy behavior while letting schwab-go own
// request construction, response parsing, and refresh-token age checks.
func (cfg *Config) oauthHTTPClient() *http.Client {
	if cfg == nil {
		return &http.Client{Timeout: oauthHTTPTimeout}
	}

	client := &http.Client{Timeout: oauthHTTPTimeout}
	if tlsCfg := cfg.TLSConfig(); tlsCfg != nil {
		// Clone DefaultTransport so insecure proxy testing only changes TLS
		// verification. This keeps proxy env vars, connection pooling, keep-alives,
		// and other standard transport defaults intact for schwab-go OAuth calls.
		transport, ok := http.DefaultTransport.(*http.Transport)
		if !ok {
			client.Transport = &http.Transport{TLSClientConfig: tlsCfg}
			return client
		}

		transport = transport.Clone()
		transport.TLSClientConfig = tlsCfg
		client.Transport = transport
	}

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
		return "", errors.New("base_url must include scheme and host")
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
		if unmarshalErr := json.Unmarshal(data, cfg); unmarshalErr != nil {
			return nil, fmt.Errorf("failed to parse config file: %w", unmarshalErr)
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
		baseURLInsecure, parseErr := strconv.ParseBool(strings.TrimSpace(rawBaseURLInsecure))
		if parseErr != nil {
			return nil, apperr.NewValidationError(
				"Invalid SCHWAB_BASE_URL_INSECURE: expected true or false",
				parseErr,
			)
		}

		cfg.BaseURLInsecure = baseURLInsecure
	}

	// Set default callback URL if still empty
	if cfg.CallbackURL == "" {
		cfg.CallbackURL = defaultCallbackURL
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
			ErrMissingCredentials,
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
	//nolint:gosec // G117: config files intentionally store client_secret with 0600 permissions.
	data, err := json.MarshalIndent(
		cfg,
		"",
		"  ",
	)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write file with 0600 permissions
	if writeErr := os.WriteFile(configPath, data, 0o600); writeErr != nil {
		return fmt.Errorf("failed to write config file: %w", writeErr)
	}

	return nil
}

// SetDefaultAccount loads config, sets DefaultAccount, and saves it.
func SetDefaultAccount(configPath, hash string) error {
	// Load existing config (or create empty one if missing)
	cfg, err := LoadConfig(configPath)
	if err != nil {
		// If validation error (missing credentials) or file doesn't exist, create a minimal config
		if _, ok := errors.AsType[*apperr.ValidationError](err); ok || os.IsNotExist(err) {
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
