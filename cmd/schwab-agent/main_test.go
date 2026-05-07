package main

import (
	"bytes"
	stderrors "errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/major/schwab-agent/internal/apperr"
	"github.com/major/schwab-agent/internal/auth"
	"github.com/major/schwab-agent/internal/client"
	"github.com/major/schwab-agent/internal/commands"
)

func TestMain(m *testing.M) {
	envVars := []string{
		"SCHWAB_CLIENT_ID",
		"SCHWAB_CLIENT_SECRET",
		"SCHWAB_CALLBACK_URL",
		"SCHWAB_BASE_URL",
		"SCHWAB_BASE_URL_INSECURE",
	}

	type envState struct {
		value string
		set   bool
	}

	original := make(map[string]envState, len(envVars))
	for _, key := range envVars {
		value, ok := os.LookupEnv(key)
		original[key] = envState{value: value, set: ok}
		_ = os.Unsetenv(key)
	}

	exitCode := m.Run()

	for _, key := range envVars {
		if state := original[key]; state.set {
			_ = os.Setenv(key, state.value)
		} else {
			_ = os.Unsetenv(key)
		}
	}

	os.Exit(exitCode)
}

// runApp builds and executes the root command without allowing Cobra to call [os.Exit].
func runApp(t *testing.T, args ...string) (string, error) {
	t.Helper()

	var stdout strings.Builder
	app := buildApp(&stdout)
	app.SetOut(&stdout)
	app.SetErr(&stdout)
	app.SetArgs(cobraArgs(args))

	_, err := app.ExecuteC()

	return stdout.String(), err
}

// runAppWithDeps is like runApp but uses the given deps for dependency overrides.
func runAppWithDeps(t *testing.T, deps commands.RootDeps, args ...string) (string, error) {
	t.Helper()

	var stdout strings.Builder
	app := buildAppWithDeps(&stdout, deps)
	app.SetOut(&stdout)
	app.SetErr(&stdout)
	app.SetArgs(cobraArgs(args))

	_, err := app.ExecuteC()

	return stdout.String(), err
}

// cobraArgs strips the binary name used by the shared test table.
func cobraArgs(args []string) []string {
	if len(args) > 0 && args[0] == "schwab-agent" {
		return args[1:]
	}

	return args
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

	for _, name := range []string{"auth", "account", "order", "quote", "history", "market", "instrument", "completion", "symbol", "option"} {
		assert.Contains(t, stdout, name)
	}
}

func TestOptionHelp_ListsPorcelainSubcommands(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	stdout, err := runApp(t, "schwab-agent", "option", "--help")
	require.NoError(t, err)

	for _, name := range []string{"expirations", "chain", "contract"} {
		assert.Contains(t, stdout, name)
	}
}

func TestBuildApp_JSONSchemaFlagIsNotRegistered(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	stdout, err := runApp(t, "schwab-agent", "--jsonschema")
	require.Error(t, err)

	assert.Contains(t, err.Error(), "unknown flag: --jsonschema")
	assert.NotContains(t, stdout, `"$schema"`)
}

func TestBeforeHook_SkipsAuthForAuthCommand(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	_, err := runApp(t, "schwab-agent", "auth", "status")
	require.Error(t, err)

	var authErr *apperr.AuthRequiredError
	require.ErrorAs(t, err, &authErr)

	_, ok := stderrors.AsType[*apperr.ValidationError](err)
	assert.False(t, ok)
}

func TestBeforeHook_SkipsAuthForCompletionCommand(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	stdout, err := runApp(t, "schwab-agent", "completion", "bash")
	require.NoError(t, err)
	assert.Contains(t, stdout, "bash completion")
}

func TestSkipAuth_EnvVarsCommand(t *testing.T) {
	// The Cobra-native env-vars reference command must bypass PersistentPreRunE
	// because users need auth/config guidance before credentials exist.
	var buf bytes.Buffer
	app := buildAppWithDeps(&buf, commands.RootDeps{})
	app.SetArgs([]string{"env-vars"})
	_, err := app.ExecuteC()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Environment Variables")
	assert.Contains(t, output, "SCHWAB_CLIENT_ID")
}

func TestSkipAuth_ConfigKeysCommand(t *testing.T) {
	// The generated config-key reference reads the live Cobra tree without
	// requiring a token file or API client.
	var buf bytes.Buffer
	app := buildAppWithDeps(&buf, commands.RootDeps{})
	app.SetArgs([]string{"config-keys"})
	_, err := app.ExecuteC()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Configuration Keys")
	assert.Contains(t, output, "schwab-agent (global)")
	assert.Contains(t, output, "--account")
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
	assert.Equal(
		t,
		"Missing required credentials: set SCHWAB_CLIENT_ID and SCHWAB_CLIENT_SECRET env vars, or add client_id and client_secret to the config file",
		authErr.Message,
	)
	assert.Equal(t, "Run `schwab-agent auth login` to authenticate", authErr.Details())
	assert.Equal(t, 3, apperr.ExitCodeFor(err))
}

func TestBeforeHook_ReturnsValidationErrorForInvalidBaseURLConfig(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	configPath := filepath.Join(tmpDir, "invalid-config.json")
	require.NoError(
		t,
		os.WriteFile(
			configPath,
			[]byte(`{"client_id":"test-client","client_secret":"test-secret","base_url":"://bad"}`),
			0o600,
		),
	)

	_, err := runApp(t, "schwab-agent", "--config", configPath, "account")
	require.Error(t, err)

	var validationErr *apperr.ValidationError
	require.ErrorAs(t, err, &validationErr)
	assert.Contains(t, err.Error(), "invalid base_url")
	assert.Equal(t, 1, apperr.ExitCodeFor(err))
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
			ExpiresAt:    time.Now().Add(-time.Hour).Unix(),
		},
	})

	_, err := runApp(t, "schwab-agent", "--config", configPath, "--token", tokenPath, "account")
	require.Error(t, err)

	var authErr *apperr.AuthExpiredError
	require.ErrorAs(t, err, &authErr)
	assert.Equal(t, "Run `schwab-agent auth login` to re-authenticate", authErr.Details())
	assert.Equal(t, 3, apperr.ExitCodeFor(err))
}

func TestAuthStatus_UsesRuntimeConfigAndTokenFlags(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	defaultConfigPath := filepath.Join(tmpDir, "schwab-agent", "config.json")
	defaultTokenPath := filepath.Join(tmpDir, "schwab-agent", "token.json")
	runtimeConfigPath := filepath.Join(tmpDir, "runtime-config.json")
	runtimeTokenPath := filepath.Join(tmpDir, "runtime-token.json")

	writeTestConfig(t, defaultConfigPath)
	writeTestToken(t, defaultTokenPath, freshToken())
	require.NoError(t, auth.SaveConfig(runtimeConfigPath, &auth.Config{
		ClientID:     "runtime-client-id",
		ClientSecret: "runtime-secret",
	}))
	writeTestToken(t, runtimeTokenPath, freshToken())

	stdout, err := runApp(t,
		"schwab-agent",
		"--config", runtimeConfigPath,
		"--token", runtimeTokenPath,
		"auth", "status",
	)
	require.NoError(t, err)
	assert.Contains(t, stdout, `"client_id":"runt..."`)
}

func TestBeforeHook_RefreshesExpiredToken(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	configPath := filepath.Join(tmpDir, "schwab-agent", "config.json")
	tokenPath := filepath.Join(tmpDir, "schwab-agent", "token.json")
	require.NoError(t, auth.SaveConfig(configPath, &auth.Config{
		ClientID:        "test-client",
		ClientSecret:    "test-secret",
		CallbackURL:     "https://127.0.0.1:8182",
		BaseURLInsecure: true,
	}))
	writeTestToken(t, tokenPath, &auth.TokenFile{
		CreationTimestamp: time.Now().Add(-time.Hour).Unix(),
		Token: auth.TokenData{
			AccessToken:  "expired-access-token",
			RefreshToken: "refresh-token",
			ExpiresIn:    1800,
			ExpiresAt:    time.Now().Add(-time.Hour).Unix(),
		},
	})

	var refreshCalls atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth/token":
			refreshCalls.Add(1)
			assert.Equal(t, http.MethodPost, r.Method)
			assert.Equal(t, "Basic dGVzdC1jbGllbnQ6dGVzdC1zZWNyZXQ=", r.Header.Get("Authorization"))
			_, _ = w.Write(
				[]byte(
					`{"access_token":"fresh-access-token","token_type":"Bearer","expires_in":1800,"refresh_token":"refresh-token","scope":"api"}`,
				),
			)
		case "/trader/v1/accounts/accountNumbers":
			assert.Equal(t, "Bearer fresh-access-token", r.Header.Get("Authorization"))
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"accountNumber":"123456789","hashValue":"hash-123"}]`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	deps := commands.DefaultRootDeps()
	deps.TokenRefreshEndpoint = func(_ *auth.Config) string { return server.URL + "/oauth/token" }
	deps.NewClient = func(token string, opts ...client.Option) *client.Client {
		return client.NewClient(token, append(opts, client.WithBaseURL(server.URL))...)
	}

	_, err := runAppWithDeps(
		t,
		deps,
		"schwab-agent",
		"--config",
		configPath,
		"--token",
		tokenPath,
		"account",
		"numbers",
	)
	require.NoError(t, err)
	assert.Equal(t, int32(1), refreshCalls.Load())

	refreshed, loadErr := auth.LoadToken(tokenPath)
	require.NoError(t, loadErr)
	assert.Equal(t, "fresh-access-token", refreshed.Token.AccessToken)
	assert.Greater(t, refreshed.Token.ExpiresAt, time.Now().Unix())
}

func TestBeforeHook_UsesConfiguredProxyForRefreshAndAPIRequests(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	configPath := filepath.Join(tmpDir, "schwab-agent", "config.json")
	tokenPath := filepath.Join(tmpDir, "schwab-agent", "token.json")
	writeTestToken(t, tokenPath, &auth.TokenFile{
		CreationTimestamp: time.Now().Add(-time.Hour).Unix(),
		Token: auth.TokenData{
			AccessToken:  "expired-access-token",
			RefreshToken: "refresh-token",
			ExpiresIn:    1800,
			ExpiresAt:    time.Now().Add(-time.Hour).Unix(),
		},
	})

	var refreshCalls atomic.Int32
	proxy := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/proxy/v1/oauth/token":
			refreshCalls.Add(1)
			assert.Equal(t, http.MethodPost, r.Method)
			assert.Equal(t, "Basic dGVzdC1jbGllbnQ6dGVzdC1zZWNyZXQ=", r.Header.Get("Authorization"))
			_, _ = w.Write(
				[]byte(
					`{"access_token":"fresh-access-token","token_type":"Bearer","expires_in":1800,"refresh_token":"refresh-token","scope":"api"}`,
				),
			)
		case "/proxy/trader/v1/accounts/accountNumbers":
			assert.Equal(t, "Bearer fresh-access-token", r.Header.Get("Authorization"))
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"accountNumber":"123456789","hashValue":"hash-123"}]`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer proxy.Close()

	require.NoError(t, auth.SaveConfig(configPath, &auth.Config{
		ClientID:        "test-client",
		ClientSecret:    "test-secret",
		CallbackURL:     "https://127.0.0.1:8182",
		BaseURL:         proxy.URL + "/proxy/",
		BaseURLInsecure: true,
	}))

	_, err := runAppWithDeps(
		t,
		commands.DefaultRootDeps(),
		"schwab-agent",
		"--config",
		configPath,
		"--token",
		tokenPath,
		"account",
		"numbers",
	)
	require.NoError(t, err)
	assert.Equal(t, int32(1), refreshCalls.Load())

	refreshed, loadErr := auth.LoadToken(tokenPath)
	require.NoError(t, loadErr)
	assert.Equal(t, "fresh-access-token", refreshed.Token.AccessToken)
	assert.Greater(t, refreshed.Token.ExpiresAt, time.Now().Unix())
}

func freshToken() *auth.TokenFile {
	return &auth.TokenFile{
		CreationTimestamp: time.Now().Add(-time.Hour).Unix(),
		Token: auth.TokenData{
			AccessToken:  "fresh-access-token",
			RefreshToken: "refresh-token",
			ExpiresIn:    1800,
			ExpiresAt:    time.Now().Add(30 * time.Minute).Unix(),
		},
	}
}

func TestIntegration_ValidToken_QuoteGet(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	configPath := filepath.Join(tmpDir, "schwab-agent", "config.json")
	tokenPath := filepath.Join(tmpDir, "schwab-agent", "token.json")
	writeTestToken(t, tokenPath, freshToken())

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer fresh-access-token", r.Header.Get("Authorization"))

		switch {
		case r.URL.Path == "/marketdata/v1/AAPL/quotes":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(
				[]byte(
					`{"AAPL":{"assetMainType":"EQUITY","symbol":"AAPL","quote":{"bidPrice":150.0,"askPrice":151.0,"lastPrice":150.5,"totalVolume":1000000}}}`,
				),
			)
		case r.URL.Path == "/marketdata/v1/quotes" && r.URL.Query().Get("symbols") == "AAPL":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(
				[]byte(
					`{"AAPL":{"assetMainType":"EQUITY","symbol":"AAPL","quote":{"bidPrice":150.0,"askPrice":151.0,"lastPrice":150.5,"totalVolume":1000000}}}`,
				),
			)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	require.NoError(t, auth.SaveConfig(configPath, &auth.Config{
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		BaseURL:      server.URL,
		CallbackURL:  "https://127.0.0.1:8182",
	}))

	deps := commands.DefaultRootDeps()
	deps.NewClient = func(token string, _ ...client.Option) *client.Client {
		return client.NewClient(token, client.WithBaseURL(server.URL))
	}

	stdout, err := runAppWithDeps(
		t,
		deps,
		"schwab-agent",
		"--config",
		configPath,
		"--token",
		tokenPath,
		"quote",
		"get",
		"AAPL",
	)
	require.NoError(t, err)
	assert.Contains(t, stdout, `"data"`)
	assert.Contains(t, stdout, `"AAPL"`)
	assert.Contains(t, stdout, `"quote"`)
	assert.Contains(t, stdout, `"lastPrice":150.5`)
}

func TestUnknownCommand_SuggestsClosestMatch(t *testing.T) {
	// When an unknown command is used without conflicting flags, the Before
	// hook should catch it and return a clear error with a suggestion.
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	_, err := runApp(t, "schwab-agent", "frobnicate")
	require.Error(t, err)

	assert.Contains(t, err.Error(), `unknown command "frobnicate"`)
	assert.Equal(t, 1, apperr.ExitCodeFor(err))
}

func TestUnknownCommand_WithUnknownFlags(t *testing.T) {
	// When an unknown command is used with flags not defined on the root
	// command, Cobra produces a misleading flag error. The flag error handler
	// should intercept this and report the unknown command instead.
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	_, err := runApp(t, "schwab-agent", "frobnicate", "get", "AAPL", "--period-type", "month")
	require.Error(t, err)

	assert.Contains(t, err.Error(), `unknown command "frobnicate"`)
	assert.Equal(t, 1, apperr.ExitCodeFor(err))
}

func TestUnknownCommand_CompletelyWrongName(t *testing.T) {
	// Even a totally unrecognizable command name should report "unknown
	// command" rather than a confusing auth or flag error.
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	_, err := runApp(t, "schwab-agent", "frobnicate")
	require.Error(t, err)

	assert.Contains(t, err.Error(), `unknown command "frobnicate"`)
	assert.Equal(t, 1, apperr.ExitCodeFor(err))
}

func TestKnownCommand_StillWorks(t *testing.T) {
	// Sanity check: a known command that requires auth should still produce
	// an auth error, not an unknown command error.
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	configPath := filepath.Join(tmpDir, "config.json")
	writeTestConfig(t, configPath)

	_, err := runApp(t, "schwab-agent", "--config", configPath, "account")
	require.Error(t, err)

	var authErr *apperr.AuthRequiredError
	require.ErrorAs(t, err, &authErr)
}

func TestPriceHistoryAlias_Works(t *testing.T) {
	// Verify that "price-history" is a valid alias for "history" command.
	// The alias should be recognized and produce the same help output as "history".
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	configPath := filepath.Join(tmpDir, "config.json")
	writeTestConfig(t, configPath)

	// Running "price-history --help" should work (not produce unknown command error)
	stdout, err := runApp(t, "schwab-agent", "--config", configPath, "price-history", "--help")
	require.NoError(t, err)

	// Should contain help text for the history command
	assert.Contains(t, stdout, "Retrieve price history for a symbol")
}

func TestBuildApp_VersionFlag(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	stdout, err := runApp(t, "schwab-agent", "--version")
	require.NoError(t, err)
	assert.Contains(t, stdout, "dev")
}
