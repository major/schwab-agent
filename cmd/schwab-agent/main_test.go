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
	structclimcp "github.com/leodido/structcli/mcp"
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

func TestSkipAuth_HelpTopic(t *testing.T) {
	// env-vars help topic must bypass PersistentPreRunE (no token/config needed).
	var buf bytes.Buffer
	app := buildAppWithDeps(&buf, commands.RootDeps{})
	app.SetArgs([]string{"env-vars"})
	_, err := structcli.ExecuteC(app)
	require.NoError(t, err)

	// Verify output contains help topic content.
	output := buf.String()
	assert.Contains(t, output, "Environment Variables")
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

	_, err := runApp(t, "schwab-agent", "price-history")
	require.Error(t, err)

	assert.Contains(t, err.Error(), `unknown command "price-history"`)
	assert.Contains(t, err.Error(), "Did you mean this?")
	assert.Contains(t, err.Error(), "history")
	assert.Equal(t, 1, apperr.ExitCodeFor(err))
}

func TestUnknownCommand_WithUnknownFlags(t *testing.T) {
	// When an unknown command is used with flags not defined on the root
	// command, Cobra produces a misleading flag error. The flag error handler
	// should intercept this and report the unknown command instead.
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	_, err := runApp(t, "schwab-agent", "price-history", "get", "AAPL", "--period-type", "month")
	require.Error(t, err)

	assert.Contains(t, err.Error(), `unknown command "price-history"`)
	assert.Contains(t, err.Error(), "Did you mean this?")
	assert.Contains(t, err.Error(), "history")
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

// --- MCP server tests ---

// runMCP sends newline-delimited JSON-RPC requests to the MCP server and returns
// the raw stdout output. The MCP server reads from stdin and writes responses to
// stdout, terminating when stdin reaches EOF.
func runMCP(t *testing.T, requests ...string) (string, error) {
	t.Helper()

	input := strings.Join(requests, "\n") + "\n"
	var stdout bytes.Buffer
	app := buildApp(&stdout)
	app.SetIn(strings.NewReader(input))
	app.SetOut(&stdout)
	app.SetArgs([]string{"--mcp"})

	_, err := structcli.ExecuteC(app)

	return stdout.String(), err
}

// runMCPWithDeps is like runMCP but injects custom dependencies.
func runMCPWithDeps(t *testing.T, deps commands.RootDeps, requests ...string) (string, error) {
	t.Helper()

	input := strings.Join(requests, "\n") + "\n"
	var stdout bytes.Buffer
	app := buildAppWithDeps(&stdout, deps)
	app.SetIn(strings.NewReader(input))
	app.SetOut(&stdout)
	app.SetArgs([]string{"--mcp"})

	_, err := structcli.ExecuteC(app)

	return stdout.String(), err
}

// mcpResponse represents a JSON-RPC 2.0 response for test decoding.
type mcpResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// parseMCPResponses decodes newline-delimited JSON-RPC responses from MCP output.
func parseMCPResponses(t *testing.T, output string) []mcpResponse {
	t.Helper()

	var responses []mcpResponse
	dec := json.NewDecoder(strings.NewReader(output))
	for dec.More() {
		var resp mcpResponse
		require.NoError(t, dec.Decode(&resp))
		responses = append(responses, resp)
	}

	return responses
}

func TestMCP_ToolsList(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	output, err := runMCP(t,
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"clientInfo":{"name":"test"}}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
	)
	require.NoError(t, err)

	responses := parseMCPResponses(t, output)
	require.Len(t, responses, 2)

	// Verify initialize response.
	var initResult structclimcp.InitializeResult
	require.NoError(t, json.Unmarshal(responses[0].Result, &initResult))
	assert.Equal(t, structclimcp.ProtocolVersion, initResult.ProtocolVersion)
	assert.Equal(t, "schwab-agent", initResult.ServerInfo.Name)

	// Verify tools/list response.
	var toolsList structclimcp.ToolsListResult
	require.NoError(t, json.Unmarshal(responses[1].Result, &toolsList))

	toolNames := make([]string, len(toolsList.Tools))
	for i, tool := range toolsList.Tools {
		toolNames[i] = tool.Name
	}

	// Spot-check expected tools are present.
	for _, expected := range []string{
		"quote-get", "account-list", "symbol-build",
		"ta-sma", "order-place-equity",
	} {
		assert.Contains(t, toolNames, expected, "expected tool %q in tools/list", expected)
	}

	// Verify excluded tools are absent.
	for _, excluded := range []string{
		"auth-login",
		"completion-bash", "completion-zsh",
		"completion-fish", "completion-powershell",
	} {
		assert.NotContains(t, toolNames, excluded, "excluded tool %q should not be in tools/list", excluded)
	}

	// Non-excluded auth commands should still be available.
	assert.Contains(t, toolNames, "auth-status")
	assert.Contains(t, toolNames, "auth-refresh")
}

func TestMCP_ToolCall_SymbolBuild(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	output, err := runMCP(t,
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"clientInfo":{"name":"test"}}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"symbol-build","arguments":{"underlying":"AAPL","expiration":"2026-12-19","strike":200,"call":true}}}`,
	)
	require.NoError(t, err)

	responses := parseMCPResponses(t, output)
	require.Len(t, responses, 2)

	var result structclimcp.ToolCallResult
	require.NoError(t, json.Unmarshal(responses[1].Result, &result))
	assert.False(t, result.IsError, "symbol-build should succeed without auth")
	require.NotEmpty(t, result.Content)
	// symbol build outputs a JSON envelope with the OCC symbol.
	assert.Contains(t, result.Content[0].Text, "AAPL")
}

func TestMCP_ToolCall_AuthRequired(t *testing.T) {
	// Calling an API-dependent tool without credentials should return an
	// MCP error response (isError: true) rather than crashing.
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	output, err := runMCP(t,
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"clientInfo":{"name":"test"}}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"account-list","arguments":{}}}`,
	)
	require.NoError(t, err)

	responses := parseMCPResponses(t, output)
	require.Len(t, responses, 2)

	var result structclimcp.ToolCallResult
	require.NoError(t, json.Unmarshal(responses[1].Result, &result))
	assert.True(t, result.IsError, "tool call without credentials should return isError: true")
	require.NotEmpty(t, result.Content)
	assert.Contains(t, result.Content[0].Text, "credentials")
}

func TestMCP_ToolCall_WithAuth(t *testing.T) {
	// End-to-end: authenticated MCP tool call returns real data.
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	configPath := filepath.Join(tmpDir, "schwab-agent", "config.json")
	tokenPath := filepath.Join(tmpDir, "schwab-agent", "token.json")
	writeTestConfig(t, configPath)
	writeTestToken(t, tokenPath, freshToken())

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/trader/v1/accounts/accountNumbers":
			_, _ = w.Write([]byte(`[{"accountNumber":"123456789","hashValue":"hash-123"}]`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	deps := commands.DefaultRootDeps()
	deps.NewClient = func(token string, _ ...client.Option) *client.Client {
		return client.NewClient(token, client.WithBaseURL(server.URL))
	}

	output, err := runMCPWithDeps(t, deps,
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"clientInfo":{"name":"test"}}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"account-numbers","arguments":{}}}`,
	)
	require.NoError(t, err)

	responses := parseMCPResponses(t, output)
	require.Len(t, responses, 2)

	var result structclimcp.ToolCallResult
	require.NoError(t, json.Unmarshal(responses[1].Result, &result))
	assert.False(t, result.IsError, "authenticated tool call should succeed")
	require.NotEmpty(t, result.Content)
	assert.Contains(t, result.Content[0].Text, "hash-123")
}
