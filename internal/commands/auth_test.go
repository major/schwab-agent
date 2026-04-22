//go:build task16

package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/major/schwab-agent/internal/auth"
	"github.com/major/schwab-agent/internal/client"
)

// testEnvelope mirrors the standard JSON success envelope for assertions.
type testEnvelope struct {
	Data     json.RawMessage `json:"data"`
	Metadata map[string]any  `json:"metadata"`
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

// TestAuthStatusCommand_WritesExpectedEnvelope verifies the status JSON shape.
func TestAuthStatusCommand_WritesExpectedEnvelope(t *testing.T) {
	tmpDir := t.TempDir()
	tokenPath := filepath.Join(tmpDir, "token.json")
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
	cmd := AuthCommand(&auth.Config{
		ClientID:       "abcd1234-client",
		ClientSecret:   "secret",
		DefaultAccount: "hash-123",
	}, tokenPath, &stdout)

	err := cmd.Run(context.Background(), []string{"auth", "status"})
	require.NoError(t, err)

	envelope := decodeAuthEnvelope(t, stdout.Bytes())
	assert.Contains(t, envelope.Metadata, "timestamp")

	var payload testStatusPayload
	require.NoError(t, json.Unmarshal(envelope.Data, &payload))
	assert.True(t, payload.Valid)
	assert.Equal(t, time.Unix(expiresAt, 0).UTC().Format(time.RFC3339), payload.ExpiresAt)
	assert.Equal(t, time.Unix(createdAt+561_600, 0).UTC().Format(time.RFC3339), payload.RefreshExpiresAt)
	assert.Equal(t, "hash-123", payload.DefaultAccount)
	assert.Equal(t, "abcd...", payload.ClientID)
}

// TestAuthRefreshCommand_CallsRefresh verifies refresh delegation and persistence.
func TestAuthRefreshCommand_CallsRefresh(t *testing.T) {
	tmpDir := t.TempDir()
	tokenPath := filepath.Join(tmpDir, "token.json")
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
	deps := defaultAuthDeps()
	deps.refreshAccessToken = func(cfg *auth.Config, tf *auth.TokenFile, endpoint string) (*auth.TokenFile, error) {
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
	}

	var stdout bytes.Buffer
	cmd := newAuthCommand(&auth.Config{ClientID: "client-id", ClientSecret: "client-secret"}, tokenPath, &stdout, deps)

	err := cmd.Run(context.Background(), []string{"auth", "refresh"})
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

// TestAuthLoginCommand_AutoSetsDefaultAccount verifies login output and default account behavior.
func TestAuthLoginCommand_AutoSetsDefaultAccount(t *testing.T) {
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
			_, _ = io.WriteString(w, `{"access_token":"access-token","token_type":"Bearer","expires_in":1800,"refresh_token":"refresh-token","scope":"api"}`)
		case "/trader/v1/accounts/accountNumbers":
			assert.Equal(t, "Bearer access-token", r.Header.Get("Authorization"))
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `[{"accountNumber":"123456789","hashValue":"hash-abc"}]`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	deps := defaultAuthDeps()
	deps.configPath = func() string { return configPath }
	deps.oauthTokenEndpoint = func() string { return server.URL + "/v1/oauth/token" }
	deps.newAccountClient = func(token string) accountNumbersClient {
		return client.NewClient(token, client.WithBaseURL(server.URL))
	}
	deps.runLogin = func(cfg *auth.Config, targetTokenPath string, tokenEndpoint string, openBrowser bool, w io.Writer) error {
		assert.False(t, openBrowser)
		_, err := fmt.Fprintln(w, "https://example.com/authorize")
		require.NoError(t, err)

		response, err := http.Post(tokenEndpoint, "application/x-www-form-urlencoded", bytes.NewBufferString("grant_type=authorization_code"))
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
			Token:            token,
		})
	}

	var stdout bytes.Buffer
	cmd := newAuthCommand(&auth.Config{
		ClientID:     "client-id",
		ClientSecret: "client-secret",
		CallbackURL:  "https://127.0.0.1:8182",
	}, tokenPath, &stdout, deps)

	err := cmd.Run(context.Background(), []string{"auth", "login", "--no-browser"})
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
	cmd := AccountSetDefaultCommand(configPath, &stdout)

	err := cmd.Run(context.Background(), []string{"set-default", "hash-xyz"})
	require.NoError(t, err)

	savedConfig, err := auth.LoadConfig(configPath)
	require.NoError(t, err)
	assert.Equal(t, "hash-xyz", savedConfig.DefaultAccount)

	envelope := decodeAuthEnvelope(t, stdout.Bytes())
	assert.Contains(t, envelope.Metadata, "timestamp")
}

// decodeAuthEnvelope parses a success envelope from auth command output.
func decodeAuthEnvelope(t *testing.T, raw []byte) testEnvelope {
	t.Helper()

	var envelope testEnvelope
	require.NoError(t, json.Unmarshal(raw, &envelope))
	require.NotNil(t, envelope.Data)
	return envelope
}


