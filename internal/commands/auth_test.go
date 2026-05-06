package commands

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/major/schwab-agent/internal/auth"
	"github.com/major/schwab-agent/internal/client"
	"github.com/major/schwab-agent/internal/output"
)

func TestMain(m *testing.M) {
	envVars := []string{
		"SCHWAB_CLIENT_ID",
		"SCHWAB_CLIENT_SECRET",
		"SCHWAB_CALLBACK_URL",
		"SCHWAB_BASE_URL",
		"SCHWAB_BASE_URL_INSECURE",
	}

	original := make(map[string]string, len(envVars))
	for _, key := range envVars {
		original[key] = os.Getenv(key)
		_ = os.Unsetenv(key)
	}

	exitCode := m.Run()

	for _, key := range envVars {
		if value, ok := original[key]; ok && value != "" {
			_ = os.Setenv(key, value)
		} else {
			_ = os.Unsetenv(key)
		}
	}

	os.Exit(exitCode)
}

// testEnvelope mirrors the standard JSON success envelope for assertions.
type testEnvelope struct {
	Data     json.RawMessage `json:"data"`
	Metadata output.Metadata `json:"metadata"`
}

// testStatusPayload represents the auth status response body.
type testStatusPayload struct {
	Valid            bool   `json:"valid"`
	ExpiresAt        string `json:"expires_at"`
	RefreshExpiresAt string `json:"refresh_expires_at"`
	DefaultAccount   string `json:"default_account"`
	ClientID         string `json:"client_id"`
}

// testRefreshPayload represents the auth refresh response body.
type testRefreshPayload struct {
	ExpiresAt string `json:"expires_at"`
}

// testLoginPayload represents the auth login response body.
type testLoginPayload struct {
	Valid            bool   `json:"valid"`
	ExpiresAt        string `json:"expires_at"`
	RefreshExpiresAt string `json:"refresh_expires_at"`
	DefaultAccount   string `json:"default_account"`
	AuthorizationURL string `json:"authorization_url,omitempty"`
	AutoSetDefault   bool   `json:"auto_set_default"`
}

// TestNewAuthCmd_StatusWritesExpectedEnvelope verifies the Cobra status JSON shape.
func TestNewAuthCmd_StatusWritesExpectedEnvelope(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	tokenPath := filepath.Join(tmpDir, "token.json")

	// Cobra auth loads config from disk (PersistentPreRunE is skipped).
	require.NoError(t, auth.SaveConfig(configPath, &auth.Config{
		ClientID:       "abcd1234-client",
		ClientSecret:   "secret",
		DefaultAccount: "hash-123",
	}))

	now := time.Now().UTC()
	expiresAt := now.Add(30 * time.Minute).Unix()
	createdAt := now.Add(-1 * time.Hour).Unix()
	require.NoError(t, auth.SaveToken(tokenPath, &auth.TokenFile{
		CreationTimestamp: createdAt,
		Token: auth.TokenData{
			AccessToken:  "access-token",
			RefreshToken: "refresh-token",
			ExpiresIn:    1800,
			ExpiresAt:    float64(expiresAt),
		},
	}))

	var stdout bytes.Buffer
	cmd := NewAuthCmd(configPath, tokenPath, &stdout, AuthDeps{})

	_, err := runTestCommand(t, cmd, "status")
	require.NoError(t, err)

	envelope := decodeAuthEnvelope(t, stdout.Bytes())
	assert.NotEmpty(t, envelope.Metadata.Timestamp)

	var payload testStatusPayload
	require.NoError(t, json.Unmarshal(envelope.Data, &payload))
	assert.True(t, payload.Valid)
	assert.Equal(t, time.Unix(expiresAt, 0).UTC().Format(time.RFC3339), payload.ExpiresAt)
	assert.Equal(t, time.Unix(createdAt+561_600, 0).UTC().Format(time.RFC3339), payload.RefreshExpiresAt)
	assert.Equal(t, "hash-123", payload.DefaultAccount)
	assert.Equal(t, "abcd...", payload.ClientID)
}

// TestNewAuthCmd_RefreshCallsRefresh verifies refresh delegation and persistence.
func TestNewAuthCmd_RefreshCallsRefresh(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	tokenPath := filepath.Join(tmpDir, "token.json")

	// Write config to disk for Cobra auth (no pre-loaded config).
	require.NoError(t, auth.SaveConfig(configPath, &auth.Config{
		ClientID:     "client-id",
		ClientSecret: "client-secret",
	}))

	originalToken := &auth.TokenFile{
		CreationTimestamp: 1_650_000_000,
		Token: auth.TokenData{
			AccessToken:  "old-token",
			RefreshToken: "refresh-token",
			ExpiresAt:    float64(1_650_000_100),
		},
	}
	require.NoError(t, auth.SaveToken(tokenPath, originalToken))

	called := false
	deps := AuthDeps{
		RefreshAccessToken: func(cfg *auth.Config, tf *auth.TokenFile, endpoint string) (*auth.TokenFile, error) {
			called = true
			assert.Equal(t, "client-id", cfg.ClientID)
			assert.Equal(t, originalToken.Token.RefreshToken, tf.Token.RefreshToken)
			assert.Equal(t, "", endpoint)

			return &auth.TokenFile{
				CreationTimestamp: tf.CreationTimestamp,
				Token: auth.TokenData{
					AccessToken:  "new-token",
					RefreshToken: tf.Token.RefreshToken,
					ExpiresAt:    float64(1_650_004_200),
				},
			}, nil
		},
	}

	var stdout bytes.Buffer
	cmd := NewAuthCmd(configPath, tokenPath, &stdout, deps)

	_, err := runTestCommand(t, cmd, "refresh")
	require.NoError(t, err)
	assert.True(t, called)

	persisted, err := auth.LoadToken(tokenPath)
	require.NoError(t, err)
	assert.Equal(t, "new-token", persisted.Token.AccessToken)

	envelope := decodeAuthEnvelope(t, stdout.Bytes())
	var payload testRefreshPayload
	require.NoError(t, json.Unmarshal(envelope.Data, &payload))
	assert.Equal(t, time.Unix(1_650_004_200, 0).UTC().Format(time.RFC3339), payload.ExpiresAt)
}

// TestNewAuthCmd_LoginAutoSetsDefaultAccount verifies login output and default account behavior.
func TestNewAuthCmd_LoginAutoSetsDefaultAccount(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	tokenPath := filepath.Join(tmpDir, "token.json")

	require.NoError(t, auth.SaveConfig(configPath, &auth.Config{
		ClientID:       "client-id",
		ClientSecret:   "client-secret",
		CallbackURL:    "https://127.0.0.1:8182",
		DefaultAccount: "",
	}))

	expiresAt := time.Now().UTC().Add(45 * time.Minute).Unix()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/oauth/token":
			assert.Equal(t, http.MethodPost, r.Method)
			_, _ = io.WriteString(
				w,
				`{"access_token":"access-token","token_type":"Bearer","expires_in":1800,"refresh_token":"refresh-token","scope":"api"}`,
			)
		case "/trader/v1/accounts/accountNumbers":
			assert.Equal(t, "Bearer access-token", r.Header.Get("Authorization"))
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `[{"accountNumber":"123456789","hashValue":"hash-abc"}]`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	deps := AuthDeps{
		OAuthTokenEndpoint: func() string { return server.URL + "/v1/oauth/token" },
		NewAccountClient: func(token string, _ *auth.Config) accountNumbersClient {
			return client.NewClient(token, client.WithBaseURL(server.URL))
		},
		RunLogin: func(_ *auth.Config, targetTokenPath string, tokenEndpoint string, openBrowser bool, w io.Writer) error {
			assert.False(t, openBrowser)
			_, err := fmt.Fprintln(w, "https://example.com/authorize")
			require.NoError(t, err)

			response, err := http.Post(
				tokenEndpoint,
				"application/x-www-form-urlencoded",
				bytes.NewBufferString("grant_type=authorization_code"),
			)
			if err != nil {
				return err
			}
			defer response.Body.Close()

			var token auth.TokenData
			if err := json.NewDecoder(response.Body).Decode(&token); err != nil {
				return err
			}

			token.ExpiresAt = float64(expiresAt)

			return auth.SaveToken(targetTokenPath, &auth.TokenFile{
				CreationTimestamp: time.Now().UTC().Add(-1 * time.Hour).Unix(),
				Token:             token,
			})
		},
	}

	var stdout bytes.Buffer
	cmd := NewAuthCmd(configPath, tokenPath, &stdout, deps)

	_, err := runTestCommand(t, cmd, "login", "--no-browser")
	require.NoError(t, err)

	savedConfig, err := auth.LoadConfig(configPath)
	require.NoError(t, err)
	assert.Equal(t, "hash-abc", savedConfig.DefaultAccount)

	envelope := decodeAuthEnvelope(t, stdout.Bytes())
	var payload testLoginPayload
	require.NoError(t, json.Unmarshal(envelope.Data, &payload))
	assert.True(t, payload.Valid)
	assert.Equal(t, "hash-abc", payload.DefaultAccount)
	assert.Equal(t, "https://example.com/authorize", payload.AuthorizationURL)
	assert.True(t, payload.AutoSetDefault)
}

// TestNewAuthCmd_LoginUsesConfiguredProxy verifies login uses proxy settings from config.
func TestNewAuthCmd_LoginUsesConfiguredProxy(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	tokenPath := filepath.Join(tmpDir, "token.json")

	proxy := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/proxy/trader/v1/accounts/accountNumbers":
			assert.Equal(t, "Bearer access-token", r.Header.Get("Authorization"))
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `[{"accountNumber":"123456789","hashValue":"hash-proxy"}]`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer proxy.Close()

	require.NoError(t, auth.SaveConfig(configPath, &auth.Config{
		ClientID:        "client-id",
		ClientSecret:    "client-secret",
		CallbackURL:     "https://127.0.0.1:8182",
		BaseURL:         proxy.URL + "/proxy/",
		BaseURLInsecure: true,
	}))

	// Only override RunLogin; NewAccountClient uses the default which reads
	// BaseURL and BaseURLInsecure from the config loaded off disk.
	deps := AuthDeps{
		RunLogin: func(_ *auth.Config, targetTokenPath string, _ string, openBrowser bool, w io.Writer) error {
			assert.False(t, openBrowser)
			_, err := fmt.Fprintln(w, "https://proxy.example.com/proxy/v1/oauth/authorize")
			require.NoError(t, err)

			return auth.SaveToken(targetTokenPath, &auth.TokenFile{
				CreationTimestamp: time.Now().UTC().Add(-1 * time.Hour).Unix(),
				Token: auth.TokenData{
					AccessToken:  "access-token",
					TokenType:    "Bearer",
					ExpiresIn:    1800,
					RefreshToken: "refresh-token",
					Scope:        "api",
					ExpiresAt:    float64(time.Now().UTC().Add(30 * time.Minute).Unix()),
				},
			})
		},
	}

	var stdout bytes.Buffer
	cmd := NewAuthCmd(configPath, tokenPath, &stdout, deps)

	_, err := runTestCommand(t, cmd, "login", "--no-browser")
	require.NoError(t, err)

	savedConfig, err := auth.LoadConfig(configPath)
	require.NoError(t, err)
	assert.Equal(t, "hash-proxy", savedConfig.DefaultAccount)

	envelope := decodeAuthEnvelope(t, stdout.Bytes())
	var payload testLoginPayload
	require.NoError(t, json.Unmarshal(envelope.Data, &payload))
	assert.Equal(t, "hash-proxy", payload.DefaultAccount)
	assert.Equal(t, "https://proxy.example.com/proxy/v1/oauth/authorize", payload.AuthorizationURL)
	assert.True(t, payload.AutoSetDefault)
}

// TestAuthAccountSetDefaultCommand_WritesSuccess verifies the account helper command.
func TestAuthAccountSetDefaultCommand_WritesSuccess(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	require.NoError(t, auth.SaveConfig(configPath, &auth.Config{
		ClientID:     "client-id",
		ClientSecret: "client-secret",
		CallbackURL:  "https://127.0.0.1:8182",
	}))

	var stdout bytes.Buffer
	cmd := newAccountSetDefaultCmd(configPath, &stdout)

	_, err := runTestCommand(t, cmd, "hash-xyz")
	require.NoError(t, err)

	savedConfig, err := auth.LoadConfig(configPath)
	require.NoError(t, err)
	assert.Equal(t, "hash-xyz", savedConfig.DefaultAccount)

	envelope := decodeAuthEnvelope(t, stdout.Bytes())
	assert.NotEmpty(t, envelope.Metadata.Timestamp)
}

// decodeAuthEnvelope parses a success envelope from auth command output.
func decodeAuthEnvelope(t *testing.T, raw []byte) testEnvelope {
	t.Helper()

	var envelope testEnvelope
	require.NoError(t, json.Unmarshal(raw, &envelope))
	require.NotNil(t, envelope.Data)
	return envelope
}

func TestUnixSecondsToRFC3339(t *testing.T) {
	tests := []struct {
		name     string
		seconds  float64
		expected string
	}{
		{"positive timestamp", 1_700_000_000, "2023-11-14T22:13:20Z"},
		{"zero returns empty", 0, ""},
		{"negative returns empty", -1, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, unixSecondsToRFC3339(tt.seconds))
		})
	}
}

func TestRefreshExpiryRFC3339(t *testing.T) {
	tests := []struct {
		name     string
		tf       *auth.TokenFile
		expected string
	}{
		{"nil token file", nil, ""},
		{"zero creation timestamp", &auth.TokenFile{CreationTimestamp: 0}, ""},
		{"negative creation timestamp", &auth.TokenFile{CreationTimestamp: -1}, ""},
		{
			"valid timestamp adds 561600 seconds",
			&auth.TokenFile{CreationTimestamp: 1_700_000_000},
			time.Unix(1_700_000_000+561_600, 0).UTC().Format(time.RFC3339),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, refreshExpiryRFC3339(tt.tf))
		})
	}
}

func TestRedactClientID(t *testing.T) {
	tests := []struct {
		name     string
		clientID string
		expected string
	}{
		{"empty string", "", ""},
		{"whitespace only", "   ", ""},
		{"short ID (4 chars)", "abcd", "abcd..."},
		{"normal ID", "abcd1234-efgh", "abcd..."},
		{"very short ID (2 chars)", "ab", "ab..."},
		{"leading/trailing spaces trimmed", "  abcd1234  ", "abcd..."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, redactClientID(tt.clientID))
		})
	}
}

func TestConfigDefaultAccount(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *auth.Config
		expected string
	}{
		{"nil config", nil, ""},
		{"empty default account", &auth.Config{}, ""},
		{"whitespace only", &auth.Config{DefaultAccount: "  "}, ""},
		{"valid account", &auth.Config{DefaultAccount: "hash-abc"}, "hash-abc"},
		{"trims whitespace", &auth.Config{DefaultAccount: "  hash-abc  "}, "hash-abc"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, configDefaultAccount(tt.cfg))
		})
	}
}

func TestRequireAuthConfig_UsesProvidedConfig(t *testing.T) {
	// When config already has client credentials, it returns the same config.
	cfg := &auth.Config{ClientID: "id", ClientSecret: "secret"}
	got, err := requireAuthConfig(cfg, "/nonexistent")
	require.NoError(t, err)
	assert.Equal(t, cfg, got)
}

func TestRequireAuthConfig_LoadsFromDisk(t *testing.T) {
	// Clear env vars so LoadConfig only sees disk state.
	t.Setenv("SCHWAB_CLIENT_ID", "")
	t.Setenv("SCHWAB_CLIENT_SECRET", "")
	t.Setenv("SCHWAB_CALLBACK_URL", "")

	// When config is missing credentials, it falls back to disk.
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	require.NoError(t, auth.SaveConfig(configPath, &auth.Config{
		ClientID:     "disk-id",
		ClientSecret: "disk-secret",
	}))

	got, err := requireAuthConfig(&auth.Config{}, configPath)
	require.NoError(t, err)
	assert.Equal(t, "disk-id", got.ClientID)
}

func TestRequireAuthConfig_ErrorOnMissingFile(t *testing.T) {
	// Clear env vars so LoadConfig can't succeed from env alone.
	t.Setenv("SCHWAB_CLIENT_ID", "")
	t.Setenv("SCHWAB_CLIENT_SECRET", "")
	t.Setenv("SCHWAB_CALLBACK_URL", "")

	_, err := requireAuthConfig(&auth.Config{}, "/nonexistent/config.json")
	assert.Error(t, err)
}

func TestOptionalAuthConfig_UsesProvided(t *testing.T) {
	cfg := &auth.Config{ClientID: "provided"}
	got := optionalAuthConfig(cfg, "/nonexistent")
	assert.Equal(t, cfg, got)
}

func TestOptionalAuthConfig_NilFallsBackToFile(t *testing.T) {
	// Clear env vars so LoadConfig only sees disk state.
	t.Setenv("SCHWAB_CLIENT_ID", "")
	t.Setenv("SCHWAB_CLIENT_SECRET", "")
	t.Setenv("SCHWAB_CALLBACK_URL", "")

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	// LoadConfig requires both client_id and client_secret to be non-empty.
	require.NoError(t, auth.SaveConfig(configPath, &auth.Config{
		ClientID:     "from-disk",
		ClientSecret: "secret",
	}))

	got := optionalAuthConfig(nil, configPath)
	assert.Equal(t, "from-disk", got.ClientID)
}

func TestOptionalAuthConfig_NilWithMissingFileReturnsEmpty(t *testing.T) {
	// Clear env vars so LoadConfig can't succeed from env alone.
	t.Setenv("SCHWAB_CLIENT_ID", "")
	t.Setenv("SCHWAB_CLIENT_SECRET", "")
	t.Setenv("SCHWAB_CALLBACK_URL", "")

	got := optionalAuthConfig(nil, "/nonexistent/config.json")
	assert.NotNil(t, got)
	assert.Empty(t, got.ClientID)
}

func TestDefaultAuthConfigPath_ReturnsConfigPath(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	path := defaultAuthConfigPath()
	assert.Equal(t, filepath.Join(tmpDir, "schwab-agent", "config.json"), path)
}

func TestDefaultAuthConfigPath_FallsBackToHomeDir(t *testing.T) {
	// When XDG_CONFIG_HOME is unset, defaultAuthConfigPath falls back to
	// os.UserHomeDir() + .config/schwab-agent/config.json.
	t.Setenv("XDG_CONFIG_HOME", "")

	path := defaultAuthConfigPath()
	// The path should end with .config/schwab-agent/config.json regardless
	// of the home directory prefix.
	assert.Contains(t, path, filepath.Join(".config", "schwab-agent", "config.json"))
}

func TestNewAuthCmd_RefreshMissingConfig(t *testing.T) {
	// When the config file doesn't exist, refresh should fail.
	// Clear env vars so LoadConfig can't succeed from environment alone.
	t.Setenv("SCHWAB_CLIENT_ID", "")
	t.Setenv("SCHWAB_CLIENT_SECRET", "")
	t.Setenv("SCHWAB_CALLBACK_URL", "")

	var stdout bytes.Buffer
	cmd := NewAuthCmd("/nonexistent/config.json", "/nonexistent/token.json", &stdout, AuthDeps{})

	_, err := runTestCommand(t, cmd, "refresh")
	require.Error(t, err)
}

func TestNewAuthCmd_RefreshMissingToken(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	// Cobra auth loads config from disk, so write it.
	require.NoError(t, auth.SaveConfig(configPath, &auth.Config{
		ClientID:     "id",
		ClientSecret: "secret",
	}))

	var stdout bytes.Buffer
	cmd := NewAuthCmd(configPath, "/nonexistent/token.json", &stdout, AuthDeps{})

	_, err := runTestCommand(t, cmd, "refresh")
	require.Error(t, err)
}

func TestNewAuthCmd_RefreshFails(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	tokenPath := filepath.Join(tmpDir, "token.json")

	require.NoError(t, auth.SaveConfig(configPath, &auth.Config{
		ClientID:     "id",
		ClientSecret: "secret",
	}))

	require.NoError(t, auth.SaveToken(tokenPath, &auth.TokenFile{
		CreationTimestamp: time.Now().Unix(),
		Token: auth.TokenData{
			AccessToken:  "old-token",
			RefreshToken: "refresh-token",
			ExpiresAt:    float64(time.Now().Unix() - 100),
		},
	}))

	deps := AuthDeps{
		RefreshAccessToken: func(_ *auth.Config, _ *auth.TokenFile, _ string) (*auth.TokenFile, error) {
			return nil, errors.New("refresh failed: server error")
		},
	}

	var stdout bytes.Buffer
	cmd := NewAuthCmd(configPath, tokenPath, &stdout, deps)

	_, err := runTestCommand(t, cmd, "refresh")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "refresh failed")
}

func TestAccountSetDefaultCommand_MissingHash(t *testing.T) {
	var stdout bytes.Buffer
	cmd := newAccountSetDefaultCmd("/whatever/config.json", &stdout)

	_, err := runTestCommand(t, cmd)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "account hash is required")
}
