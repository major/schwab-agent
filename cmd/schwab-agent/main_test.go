package main

import (
	"context"
	stderrors "errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"

	"github.com/major/schwab-agent/internal/auth"
	"github.com/major/schwab-agent/internal/client"
	"github.com/major/schwab-agent/internal/apperr"
)

// runApp builds and executes the root command without allowing urfave/cli to call os.Exit.
func runApp(t *testing.T, args ...string) (string, error) {
	t.Helper()

	var stdout strings.Builder
	app := buildApp(&stdout)
	app.Writer = &stdout
	app.ErrWriter = io.Discard
	app.ExitErrHandler = func(_ context.Context, _ *cli.Command, _ error) {}

	err := app.Run(context.Background(), args)

	return stdout.String(), err
}

// runAppWithDeps is like runApp but uses the given deps for dependency overrides.
func runAppWithDeps(t *testing.T, deps appDeps, args ...string) (string, error) {
	t.Helper()

	var stdout strings.Builder
	app := buildAppWithDeps(&stdout, deps)
	app.Writer = &stdout
	app.ErrWriter = io.Discard
	app.ExitErrHandler = func(_ context.Context, _ *cli.Command, _ error) {}

	err := app.Run(context.Background(), args)

	return stdout.String(), err
}

// writeTestConfig persists a valid auth config for Before hook tests.
func writeTestConfig(t *testing.T, path string) {
	t.Helper()

	require.NoError(t, auth.SaveConfig(path, &auth.Config{
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		CallbackURL:  "https://127.0.0.1:8182",
	}))
}

// writeTestToken persists a token file for Before hook tests.
func writeTestToken(t *testing.T, path string, tf *auth.TokenFile) {
	t.Helper()
	require.NoError(t, auth.SaveToken(path, tf))
}

func TestBuildApp_AllCommandsPresent(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	stdout, err := runApp(t, "schwab-agent", "--help")
	require.NoError(t, err)

	for _, name := range []string{"auth", "account", "order", "quote", "chain", "history", "market", "instrument", "schema"} {
		assert.Contains(t, stdout, name)
	}
}

func TestBeforeHook_SkipsAuthForAuthCommand(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	_, err := runApp(t, "schwab-agent", "auth", "status")
	require.Error(t, err)

	var authErr *apperr.AuthRequiredError
	require.ErrorAs(t, err, &authErr)

	var validationErr *apperr.ValidationError
	assert.False(t, stderrors.As(err, &validationErr))
}

func TestBeforeHook_SkipsAuthForSchemaCommand(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	stdout, err := runApp(t, "schwab-agent", "schema")
	require.NoError(t, err)
	assert.Contains(t, stdout, `"global_flags"`)
	assert.Contains(t, stdout, `"auth"`)
}

func TestBeforeHook_ReturnsAuthErrorForAPICommand(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	configPath := filepath.Join(tmpDir, "config.json")
	writeTestConfig(t, configPath)

	_, err := runApp(t, "schwab-agent", "--config", configPath, "account")
	require.Error(t, err)

	var authErr *apperr.AuthRequiredError
	require.ErrorAs(t, err, &authErr)
	assert.Equal(t, "AUTH_REQUIRED", apperr.ErrorCode(err))
	assert.Equal(t, "No authentication token found", authErr.Message)
	assert.Equal(t, "Run `schwab-agent auth login` to authenticate", authErr.Details())
	assert.Equal(t, 3, apperr.ExitCodeFor(err))
}

func TestBeforeHook_ReturnsAuthRequiredErrorForMissingConfig(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	missingConfigPath := filepath.Join(tmpDir, "missing-config.json")

	_, err := runApp(t, "schwab-agent", "--config", missingConfigPath, "account")
	require.Error(t, err)

	var authErr *apperr.AuthRequiredError
	require.ErrorAs(t, err, &authErr)
	assert.Equal(t, "AUTH_REQUIRED", apperr.ErrorCode(err))
	assert.Equal(t, "Missing required credentials: set SCHWAB_CLIENT_ID and SCHWAB_CLIENT_SECRET env vars, or add client_id and client_secret to the config file", authErr.Message)
	assert.Equal(t, "Run `schwab-agent auth login` to authenticate", authErr.Details())
	assert.Equal(t, 3, apperr.ExitCodeFor(err))
}

func TestBeforeHook_ReturnsAuthExpiredErrorForStaleRefreshToken(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	configPath := filepath.Join(tmpDir, "schwab-agent", "config.json")
	tokenPath := filepath.Join(tmpDir, "schwab-agent", "token.json")
	writeTestConfig(t, configPath)
	writeTestToken(t, tokenPath, &auth.TokenFile{
		CreationTimestamp: time.Now().Add(-(157 * time.Hour)).Unix(),
		Token: auth.TokenData{
			AccessToken:  "expired-access-token",
			RefreshToken: "stale-refresh-token",
			ExpiresIn:    1800,
			ExpiresAt:    float64(time.Now().Add(-time.Hour).Unix()),
		},
	})

	_, err := runApp(t, "schwab-agent", "--config", configPath, "--token", tokenPath, "account")
	require.Error(t, err)

	var authErr *apperr.AuthExpiredError
	require.ErrorAs(t, err, &authErr)
	assert.Equal(t, "Run `schwab-agent auth login` to re-authenticate", authErr.Details())
	assert.Equal(t, 3, apperr.ExitCodeFor(err))
}

func TestBeforeHook_RefreshesExpiredToken(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	configPath := filepath.Join(tmpDir, "schwab-agent", "config.json")
	tokenPath := filepath.Join(tmpDir, "schwab-agent", "token.json")
	writeTestConfig(t, configPath)
	writeTestToken(t, tokenPath, &auth.TokenFile{
		CreationTimestamp: time.Now().Add(-time.Hour).Unix(),
		Token: auth.TokenData{
			AccessToken:  "expired-access-token",
			RefreshToken: "refresh-token",
			ExpiresIn:    1800,
			ExpiresAt:    float64(time.Now().Add(-time.Hour).Unix()),
		},
	})

	var refreshCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth/token":
			refreshCalls.Add(1)
			assert.Equal(t, http.MethodPost, r.Method)
			assert.Equal(t, "Basic dGVzdC1jbGllbnQ6dGVzdC1zZWNyZXQ=", r.Header.Get("Authorization"))
			_, _ = w.Write([]byte(`{"access_token":"fresh-access-token","token_type":"Bearer","expires_in":1800,"refresh_token":"refresh-token","scope":"api"}`))
		case "/trader/v1/accounts/accountNumbers":
			assert.Equal(t, "Bearer fresh-access-token", r.Header.Get("Authorization"))
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"accountNumber":"123456789","hashValue":"hash-123"}]`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	deps := defaultAppDeps()
	deps.tokenRefreshEndpoint = func() string { return server.URL + "/oauth/token" }
	deps.newClient = func(token string, _ ...client.Option) *client.Client {
		return client.NewClient(token, client.WithBaseURL(server.URL))
	}

	_, err := runAppWithDeps(t, deps, "schwab-agent", "--config", configPath, "--token", tokenPath, "account", "numbers")
	require.NoError(t, err)
	assert.Equal(t, int32(1), refreshCalls.Load())

	refreshed, loadErr := auth.LoadToken(tokenPath)
	require.NoError(t, loadErr)
	assert.Equal(t, "fresh-access-token", refreshed.Token.AccessToken)
	assert.True(t, refreshed.Token.ExpiresAt > float64(time.Now().Unix()))
}

func freshToken() *auth.TokenFile {
	return &auth.TokenFile{
		CreationTimestamp: time.Now().Add(-time.Hour).Unix(),
		Token: auth.TokenData{
			AccessToken:  "fresh-access-token",
			RefreshToken: "refresh-token",
			ExpiresIn:    1800,
			ExpiresAt:    float64(time.Now().Add(30 * time.Minute).Unix()),
		},
	}
}

func TestIntegration_ValidToken_QuoteGet(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	configPath := filepath.Join(tmpDir, "schwab-agent", "config.json")
	tokenPath := filepath.Join(tmpDir, "schwab-agent", "token.json")
	writeTestConfig(t, configPath)
	writeTestToken(t, tokenPath, freshToken())

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer fresh-access-token", r.Header.Get("Authorization"))

		switch {
		case r.URL.Path == "/marketdata/v1/AAPL/quotes":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"AAPL":{"assetMainType":"EQUITY","symbol":"AAPL","bidPrice":150.0,"askPrice":151.0,"lastPrice":150.5,"totalVolume":1000000}}`))
		case r.URL.Path == "/marketdata/v1/quotes" && r.URL.Query().Get("symbols") == "AAPL":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"AAPL":{"assetMainType":"EQUITY","symbol":"AAPL","bidPrice":150.0,"askPrice":151.0,"lastPrice":150.5,"totalVolume":1000000}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	deps := defaultAppDeps()
	deps.newClient = func(token string, _ ...client.Option) *client.Client {
		return client.NewClient(token, client.WithBaseURL(server.URL))
	}

	stdout, err := runAppWithDeps(t, deps, "schwab-agent", "--config", configPath, "--token", tokenPath, "quote", "get", "AAPL")
	require.NoError(t, err)
	assert.Contains(t, stdout, `"data"`)
	assert.Contains(t, stdout, `"AAPL"`)
}

func TestIntegration_OrderConfirmGate(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	configPath := filepath.Join(tmpDir, "schwab-agent", "config.json")
	tokenPath := filepath.Join(tmpDir, "schwab-agent", "token.json")
	writeTestConfig(t, configPath)
	writeTestToken(t, tokenPath, freshToken())
	// Enable mutable operations so the test reaches the confirm gate.
	// Without this, requireMutableEnabled fires before requireConfirm and the
	// test never exercises the --confirm flag logic.
	require.NoError(t, auth.SaveConfig(configPath, &auth.Config{
		ClientID:                   "test-client",
		ClientSecret:               "test-secret",
		CallbackURL:                "https://127.0.0.1:8182",
		DefaultAccount:             "hash-123",
		IAlsoLikeToLiveDangerously: true,
	}))

	_, err := runApp(t,
		"schwab-agent",
		"--config", configPath,
		"--token", tokenPath,
		"order", "place", "equity",
		"--symbol", "AAPL",
		"--action", "BUY",
		"--quantity", "10",
		"--type", "MARKET",
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "confirm")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer fresh-access-token", r.Header.Get("Authorization"))

		if r.URL.Path != "/trader/v1/accounts/hash-123/orders" {
			http.NotFound(w, r)
			return
		}

		assert.Equal(t, http.MethodPost, r.Method)
		w.Header().Set("Location", "/trader/v1/accounts/hash-123/orders/98765")
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	deps := defaultAppDeps()
	deps.newClient = func(token string, _ ...client.Option) *client.Client {
		return client.NewClient(token, client.WithBaseURL(server.URL))
	}

	stdout, err := runAppWithDeps(t, deps,
		"schwab-agent",
		"--config", configPath,
		"--token", tokenPath,
		"order", "place", "equity",
		"--symbol", "AAPL",
		"--action", "BUY",
		"--quantity", "10",
		"--type", "MARKET",
		"--confirm",
	)
	require.NoError(t, err)
	assert.Contains(t, stdout, `"orderId":98765`)
}

func TestBuildApp_VersionFlag(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	stdout, err := runApp(t, "schwab-agent", "--version")
	require.NoError(t, err)
	assert.Contains(t, stdout, "dev")
}
