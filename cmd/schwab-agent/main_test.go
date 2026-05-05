package main

import (
	"bytes"
	"encoding/json"
	stderrors "errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/leodido/structcli"
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

// runApp builds and executes the root command without allowing Cobra to call os.Exit.
func runApp(t *testing.T, args ...string) (string, error) {
	t.Helper()

	var stdout strings.Builder
	app := buildApp(&stdout)
	app.SetOut(&stdout)
	app.SetErr(&stdout)
	app.SetArgs(cobraArgs(args))

	_, err := structcli.ExecuteC(app)

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

	_, err := structcli.ExecuteC(app)

	return stdout.String(), err
}

// cobraArgs strips the binary name used by the shared test table.
func cobraArgs(args []string) []string {
	if len(args) > 0 && args[0] == "schwab-agent" {
		return args[1:]
	}

	return args
}

// schemaByTitle indexes the --jsonschema=tree output by command title.
func schemaByTitle(t *testing.T, stdout string) map[string]map[string]any {
	t.Helper()

	var schemas []map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &schemas))
	require.NotEmpty(t, schemas)

	byTitle := make(map[string]map[string]any, len(schemas))
	for _, schema := range schemas {
		title, ok := schema["title"].(string)
		require.True(t, ok, "schema entry is missing a string title: %#v", schema)
		byTitle[title] = schema
	}

	return byTitle
}

// schemaStrings returns a string slice from a schema array field.
func schemaStrings(t *testing.T, schema map[string]any, key string) []string {
	t.Helper()

	values, ok := schema[key].([]any)
	require.True(t, ok, "schema %q field is not an array", key)

	result := make([]string, 0, len(values))
	for _, value := range values {
		stringValue, ok := value.(string)
		require.True(t, ok, "schema %q field contains a non-string value: %#v", key, value)
		result = append(result, stringValue)
	}

	return result
}

// schemaProperties returns the JSON Schema properties map for a command schema.
func schemaProperties(t *testing.T, schema map[string]any) map[string]any {
	t.Helper()

	properties, ok := schema["properties"].(map[string]any)
	require.True(t, ok, "schema entry is missing properties: %#v", schema)

	return properties
}

// schemaProperty returns a single named flag property from a command schema.
func schemaProperty(t *testing.T, schema map[string]any, name string) map[string]any {
	t.Helper()

	property, ok := schemaProperties(t, schema)[name].(map[string]any)
	require.True(t, ok, "schema entry is missing property %q", name)

	return property
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

	for _, name := range []string{"auth", "account", "order", "quote", "chain", "history", "market", "instrument", "completion", "symbol"} {
		assert.Contains(t, stdout, name)
	}
}

func TestJSONSchemaTreeIsCanonicalDiscoveryContract(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	stdout, err := runApp(t, "schwab-agent", "--jsonschema=tree")
	require.NoError(t, err)

	schemas := schemaByTitle(t, stdout)
	rootSchema := schemas["schwab-agent"]
	require.NotNil(t, rootSchema)

	assert.Subset(t, schemaStrings(t, rootSchema, "x-structcli-subcommands"), []string{
		"account",
		"auth",
		"chain",
		"history",
		"instrument",
		"market",
		"order",
		"position",
		"quote",
		"symbol",
		"ta",
	})
	assert.Equal(t, "a", schemaProperty(t, rootSchema, "account")["x-structcli-shorthand"])
	assert.Equal(t, "v", schemaProperty(t, rootSchema, "verbose")["x-structcli-shorthand"])
	assert.Contains(t, schemaProperties(t, rootSchema), "config")
	assert.Contains(t, schemaProperties(t, rootSchema), "token")

	symbolBuildSchema := schemas["schwab-agent symbol build"]
	require.NotNil(t, symbolBuildSchema)
	assert.Subset(t, schemaStrings(t, symbolBuildSchema, "required"), []string{"expiration", "strike", "underlying"})

	equitySchema := schemas["schwab-agent order build equity"]
	require.NotNil(t, equitySchema)
	// --action's required annotation is replaced by a OneRequired group with
	// --instruction after alias registration, so only symbol and quantity
	// remain individually required.
	assert.Subset(t, schemaStrings(t, equitySchema, "required"), []string{"quantity", "symbol"})
	assert.Subset(t, schemaStrings(t, schemaProperty(t, equitySchema, "action"), "enum"), []string{"BUY", "SELL"})
	assert.Subset(t, schemaStrings(t, schemaProperty(t, equitySchema, "type"), "enum"), []string{"LIMIT", "MARKET"})

	taSchema := schemas["schwab-agent ta sma"]
	require.NotNil(t, taSchema)
	assert.Equal(t, "daily", schemaProperty(t, taSchema, "interval")["default"])
	assert.Subset(t, schemaStrings(t, schemaProperty(t, taSchema, "interval"), "enum"), []string{"1min", "daily", "weekly"})
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
	_, err := structcli.ExecuteC(app)
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
	_, err := structcli.ExecuteC(app)
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
	assert.Equal(t, "Missing required credentials: set SCHWAB_CLIENT_ID and SCHWAB_CLIENT_SECRET env vars, or add client_id and client_secret to the config file", authErr.Message)
	assert.Equal(t, "Run `schwab-agent auth login` to authenticate", authErr.Details())
	assert.Equal(t, 3, apperr.ExitCodeFor(err))
}

func TestBeforeHook_ReturnsValidationErrorForInvalidBaseURLConfig(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	configPath := filepath.Join(tmpDir, "invalid-config.json")
	require.NoError(t, os.WriteFile(configPath, []byte(`{"client_id":"test-client","client_secret":"test-secret","base_url":"://bad"}`), 0o600))

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

	deps := commands.DefaultRootDeps()
	deps.TokenRefreshEndpoint = func(_ *auth.Config) string { return server.URL + "/oauth/token" }
	deps.NewClient = func(token string, _ ...client.Option) *client.Client {
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
			ExpiresAt:    float64(time.Now().Add(-time.Hour).Unix()),
		},
	})

	var refreshCalls atomic.Int32
	proxy := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/proxy/v1/oauth/token":
			refreshCalls.Add(1)
			assert.Equal(t, http.MethodPost, r.Method)
			assert.Equal(t, "Basic dGVzdC1jbGllbnQ6dGVzdC1zZWNyZXQ=", r.Header.Get("Authorization"))
			_, _ = w.Write([]byte(`{"access_token":"fresh-access-token","token_type":"Bearer","expires_in":1800,"refresh_token":"refresh-token","scope":"api"}`))
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

	_, err := runAppWithDeps(t, commands.DefaultRootDeps(), "schwab-agent", "--config", configPath, "--token", tokenPath, "account", "numbers")
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

	deps := commands.DefaultRootDeps()
	deps.NewClient = func(token string, _ ...client.Option) *client.Client {
		return client.NewClient(token, client.WithBaseURL(server.URL))
	}

	stdout, err := runAppWithDeps(t, deps, "schwab-agent", "--config", configPath, "--token", tokenPath, "quote", "get", "AAPL")
	require.NoError(t, err)
	assert.Contains(t, stdout, `"data"`)
	assert.Contains(t, stdout, `"AAPL"`)
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

func TestDebugOptions_TextFormat(t *testing.T) {
	// --debug-options should print text flag attribution without requiring
	// auth credentials or a valid token.
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	stdout, err := runApp(t, "schwab-agent", "--debug-options", "account", "list")
	require.NoError(t, err)

	assert.Contains(t, stdout, "Command:")
	assert.Contains(t, stdout, "Flags:")
}

func TestDebugOptions_JSONFormat(t *testing.T) {
	// --debug-options=json should produce structured JSON output.
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	stdout, err := runApp(t, "schwab-agent", "--debug-options=json", "account", "list")
	require.NoError(t, err)

	assert.Contains(t, stdout, `"command"`)
	assert.Contains(t, stdout, `"flags"`)
	assert.Contains(t, stdout, `"source"`)
}

func TestDebugOptions_SkipsAuth(t *testing.T) {
	// Debug mode on a command that normally requires auth should not attempt
	// authentication at all (no config/token files needed).
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	// Point config/token at non-existent paths to prove auth is skipped.
	stdout, err := runApp(t,
		"schwab-agent",
		"--debug-options",
		"--config", filepath.Join(tmpDir, "does-not-exist.json"),
		"--token", filepath.Join(tmpDir, "no-token.json"),
		"account", "list",
	)
	require.NoError(t, err)

	// Should get debug output, not an auth error.
	assert.Contains(t, stdout, "Command:")
	assert.Contains(t, stdout, "Flags:")
}

func TestDebugOptions_BareFlag(t *testing.T) {
	// Bare --debug-options (no value) should default to text format.
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	stdout, err := runApp(t, "schwab-agent", "--debug-options", "account", "list")
	require.NoError(t, err)

	assert.Contains(t, stdout, "Command:")
	assert.Contains(t, stdout, "Flags:")
}
