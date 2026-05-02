package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"

	"github.com/major/schwab-agent/internal/apperr"
	"github.com/major/schwab-agent/internal/auth"
	"github.com/major/schwab-agent/internal/models"
	"github.com/major/schwab-agent/internal/output"
)

// noopExitHandler prevents urfave/cli from calling os.Exit on returned errors.
// Kept for use by other test files (e.g. position_test.go) that still test urfave commands.
func noopExitHandler(_ context.Context, _ *cli.Command, _ error) {}

// accountMockServer creates an httptest.Server that routes requests by path.
func accountMockServer(t *testing.T, routes map[string]any) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp, ok := routes[r.URL.Path]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			require.NoError(t, json.NewEncoder(w).Encode(map[string]string{"error": "not found"}))

			return
		}
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(resp))
	}))
}

// writeAccountTestConfig writes a config file with the given default account and returns its path.
func writeAccountTestConfig(t *testing.T, dir, defaultAccount string) string {
	t.Helper()

	configPath := filepath.Join(dir, "config.json")
	cfg := &auth.Config{
		ClientID:       "test-id",
		ClientSecret:   "test-secret",
		DefaultAccount: defaultAccount,
	}
	require.NoError(t, auth.SaveConfig(configPath, cfg))

	return configPath
}

// decodeAccountEnvelope unmarshals test output into an Envelope.
func decodeAccountEnvelope(t *testing.T, data []byte) output.Envelope {
	t.Helper()

	var env output.Envelope
	require.NoError(t, json.Unmarshal(data, &env), "failed to decode envelope: %s", string(data))

	return env
}

// --- NewAccountCmd (Cobra) tests ---

func TestNewAccountCmd_List_Success(t *testing.T) {
	// Arrange
	accounts := []map[string]any{
		{"securitiesAccount": map[string]any{"type": "MARGIN", "accountNumber": "12345"}},
		{"securitiesAccount": map[string]any{"type": "CASH", "accountNumber": "67890"}},
	}
	prefs := map[string]any{
		"accounts": []map[string]any{
			{"accountNumber": "12345", "nickName": "My IRA", "primaryAccount": true},
			{"accountNumber": "67890", "nickName": "Joint Taxable", "primaryAccount": false},
		},
	}
	srv := accountMockServer(t, map[string]any{
		"/trader/v1/accounts":       accounts,
		"/trader/v1/userPreference": prefs,
	})
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewAccountCmd(testClient(t, srv), "", &buf)
	_, err := runCobraCommand(t, cmd, "list")

	// Assert
	require.NoError(t, err)

	env := decodeAccountEnvelope(t, buf.Bytes())
	assert.NotEmpty(t, env.Metadata.Timestamp)

	dataMap, ok := env.Data.(map[string]any)
	require.True(t, ok)

	accountList, ok := dataMap["accounts"].([]any)
	require.True(t, ok)
	require.Len(t, accountList, 2)

	// Verify nicknames were merged from preferences
	first, ok := accountList[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "My IRA", first["nickName"])
	assert.Equal(t, true, first["primaryAccount"])

	second, ok := accountList[1].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "Joint Taxable", second["nickName"])
	assert.Equal(t, false, second["primaryAccount"])
}

func TestNewAccountCmd_List_WithPositionsFlag(t *testing.T) {
	// Arrange
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/trader/v1/accounts" {
			// Verify the positions field is requested.
			assert.Equal(t, "positions", r.URL.Query().Get("fields"))

			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{
				"securitiesAccount": {
					"type": "MARGIN",
					"accountNumber": "12345",
					"positions": [{
						"longQuantity": 100,
						"marketValue": 15000.00,
						"instrument": {"symbol": "AAPL", "assetType": "EQUITY"}
					}]
				}
			}]`))

			return
		}
		// Preferences endpoint
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"accounts": []}`))
	}))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewAccountCmd(testClient(t, srv), "", &buf)
	_, err := runCobraCommand(t, cmd, "list", "--positions")

	// Assert
	require.NoError(t, err)

	env := decodeAccountEnvelope(t, buf.Bytes())
	dataMap, ok := env.Data.(map[string]any)
	require.True(t, ok)

	accountList, ok := dataMap["accounts"].([]any)
	require.True(t, ok)
	require.Len(t, accountList, 1)

	// Verify positions came through in the response
	acct, ok := accountList[0].(map[string]any)
	require.True(t, ok)
	sa, ok := acct["securitiesAccount"].(map[string]any)
	require.True(t, ok)
	positions, ok := sa["positions"].([]any)
	require.True(t, ok)
	assert.Len(t, positions, 1)
}

func TestNewAccountCmd_List_WithoutPositionsFlag_NoFieldsParam(t *testing.T) {
	// Arrange
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/trader/v1/accounts" {
			// No fields param when --positions is not passed.
			assert.Empty(t, r.URL.Query().Get("fields"))

			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"securitiesAccount": {"type": "MARGIN", "accountNumber": "12345"}}]`))

			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"accounts": []}`))
	}))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewAccountCmd(testClient(t, srv), "", &buf)
	_, err := runCobraCommand(t, cmd, "list")

	// Assert
	require.NoError(t, err)
}

func TestNewAccountCmd_List_PreferencesFailure_StillReturnsAccounts(t *testing.T) {
	// Preferences endpoint returns 404, but account list should still succeed
	// without nicknames.
	accounts := []map[string]any{
		{"securitiesAccount": map[string]any{"type": "MARGIN", "accountNumber": "12345"}},
	}
	srv := accountMockServer(t, map[string]any{
		"/trader/v1/accounts": accounts,
		// No /trader/v1/userPreference route - will 404
	})
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewAccountCmd(testClient(t, srv), "", &buf)
	_, err := runCobraCommand(t, cmd, "list")

	// Assert
	require.NoError(t, err)

	env := decodeAccountEnvelope(t, buf.Bytes())
	dataMap, ok := env.Data.(map[string]any)
	require.True(t, ok)

	accountList, ok := dataMap["accounts"].([]any)
	require.True(t, ok)
	require.Len(t, accountList, 1)

	// nickName should be absent since preferences failed
	first, ok := accountList[0].(map[string]any)
	require.True(t, ok)
	assert.NotContains(t, first, "nickName")
}

func TestNewAccountCmd_List_APIError(t *testing.T) {
	// Arrange
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"server error"}`))
	}))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewAccountCmd(testClient(t, srv), "", &buf)
	_, err := runCobraCommand(t, cmd, "list")

	// Assert
	require.Error(t, err)
	_, ok := errors.AsType[*apperr.HTTPError](err)
	assert.True(t, ok)
}

func TestNewAccountCmd_Numbers_Success(t *testing.T) {
	// Arrange
	numbers := []map[string]any{
		{"accountNumber": "12345", "hashValue": "HASH_ABC"},
		{"accountNumber": "67890", "hashValue": "HASH_DEF"},
	}
	srv := accountMockServer(t, map[string]any{
		"/trader/v1/accounts/accountNumbers": numbers,
	})
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewAccountCmd(testClient(t, srv), "", &buf)
	_, err := runCobraCommand(t, cmd, "numbers")

	// Assert
	require.NoError(t, err)

	env := decodeAccountEnvelope(t, buf.Bytes())
	assert.NotEmpty(t, env.Metadata.Timestamp)

	dataMap, ok := env.Data.(map[string]any)
	require.True(t, ok)

	accountList, ok := dataMap["accounts"].([]any)
	require.True(t, ok)
	assert.Len(t, accountList, 2)
}

func TestNewAccountCmd_Get_WithPositionsFlag(t *testing.T) {
	// Arrange
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/trader/v1/accounts/MY_HASH" {
			assert.Equal(t, "positions", r.URL.Query().Get("fields"))

			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"securitiesAccount": {
					"type": "MARGIN",
					"accountNumber": "12345",
					"positions": [{
						"longQuantity": 200,
						"marketValue": 30000.00,
						"instrument": {"symbol": "MSFT", "assetType": "EQUITY"}
					}]
				}
			}`))

			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"accounts": []}`))
	}))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewAccountCmd(testClient(t, srv), "", &buf)
	_, err := runCobraCommand(t, cmd, "get", "--positions", "MY_HASH")

	// Assert
	require.NoError(t, err)

	env := decodeAccountEnvelope(t, buf.Bytes())
	assert.Equal(t, "MY_HASH", env.Metadata.Account)

	dataMap, ok := env.Data.(map[string]any)
	require.True(t, ok)
	sa, ok := dataMap["securitiesAccount"].(map[string]any)
	require.True(t, ok)
	positions, ok := sa["positions"].([]any)
	require.True(t, ok)
	assert.Len(t, positions, 1)
}

func TestNewAccountCmd_Get_FlagOverridesAll(t *testing.T) {
	// Arrange - mock server only responds to FLAG_HASH, proving the flag was used
	srv := accountMockServer(t, map[string]any{
		"/trader/v1/accounts/FLAG_HASH": map[string]any{
			"securitiesAccount": map[string]any{"type": "MARGIN", "accountNumber": "11111"},
		},
		"/trader/v1/userPreference": map[string]any{
			"accounts": []map[string]any{
				{"accountNumber": "11111", "nickName": "Test Account", "primaryAccount": true},
			},
		},
	})
	defer srv.Close()

	configPath := writeAccountTestConfig(t, t.TempDir(), "CONFIG_HASH")

	// Act
	var buf bytes.Buffer
	cmd := NewAccountCmd(testClient(t, srv), configPath, &buf)
	// Persistent --account flag on parent should override both positional arg and config default
	_, err := runCobraCommand(t, cmd, "--account", "FLAG_HASH", "get", "ARG_HASH")

	// Assert
	require.NoError(t, err)

	env := decodeAccountEnvelope(t, buf.Bytes())
	assert.Equal(t, "FLAG_HASH", env.Metadata.Account)
}

func TestNewAccountCmd_Get_PositionalArg(t *testing.T) {
	// Arrange
	srv := accountMockServer(t, map[string]any{
		"/trader/v1/accounts/ARG_HASH": map[string]any{
			"securitiesAccount": map[string]any{"type": "MARGIN", "accountNumber": "22222"},
		},
		"/trader/v1/userPreference": map[string]any{"accounts": []map[string]any{}},
	})
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewAccountCmd(testClient(t, srv), "", &buf)
	_, err := runCobraCommand(t, cmd, "get", "ARG_HASH")

	// Assert
	require.NoError(t, err)

	env := decodeAccountEnvelope(t, buf.Bytes())
	assert.Equal(t, "ARG_HASH", env.Metadata.Account)
}

func TestNewAccountCmd_Get_ConfigDefault(t *testing.T) {
	// Arrange
	srv := accountMockServer(t, map[string]any{
		"/trader/v1/accounts/CONFIG_HASH": map[string]any{
			"securitiesAccount": map[string]any{"type": "CASH", "accountNumber": "33333"},
		},
		"/trader/v1/userPreference": map[string]any{"accounts": []map[string]any{}},
	})
	defer srv.Close()

	configPath := writeAccountTestConfig(t, t.TempDir(), "CONFIG_HASH")

	// Act
	var buf bytes.Buffer
	cmd := NewAccountCmd(testClient(t, srv), configPath, &buf)
	_, err := runCobraCommand(t, cmd, "get")

	// Assert
	require.NoError(t, err)

	env := decodeAccountEnvelope(t, buf.Bytes())
	assert.Equal(t, "CONFIG_HASH", env.Metadata.Account)
}

func TestNewAccountCmd_Get_NoAccount_Error(t *testing.T) {
	// Arrange
	srv := accountMockServer(t, map[string]any{})
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewAccountCmd(testClient(t, srv), "", &buf)
	_, err := runCobraCommand(t, cmd, "get")

	// Assert
	require.Error(t, err)

	notFoundErr, ok := errors.AsType[*apperr.AccountNotFoundError](err)
	require.True(t, ok)
	assert.Contains(t, notFoundErr.Message, "no account specified")
	assert.Contains(t, notFoundErr.Details(), "schwab-agent account numbers")
	assert.Contains(t, notFoundErr.Details(), "schwab-agent account set-default")
}

func TestNewAccountCmd_Get_MetadataContainsHash(t *testing.T) {
	// Arrange
	srv := accountMockServer(t, map[string]any{
		"/trader/v1/accounts/MY_HASH": map[string]any{
			"securitiesAccount": map[string]any{"type": "MARGIN"},
		},
		"/trader/v1/userPreference": map[string]any{"accounts": []map[string]any{}},
	})
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewAccountCmd(testClient(t, srv), "", &buf)
	_, err := runCobraCommand(t, cmd, "get", "MY_HASH")

	// Assert
	require.NoError(t, err)

	env := decodeAccountEnvelope(t, buf.Bytes())
	assert.Equal(t, "MY_HASH", env.Metadata.Account)
	assert.NotEmpty(t, env.Metadata.Timestamp)
}

func TestNewAccountCmd_Get_APIError(t *testing.T) {
	// Arrange - 404 from Account method becomes AccountNotFoundError
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"not found"}`))
	}))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewAccountCmd(testClient(t, srv), "", &buf)
	_, err := runCobraCommand(t, cmd, "get", "BAD_HASH")

	// Assert
	require.Error(t, err)
	_, ok := errors.AsType[*apperr.AccountNotFoundError](err)
	assert.True(t, ok)
}

// --- SetDefault subcommand tests ---

func TestNewAccountCmd_SetDefault_Success(t *testing.T) {
	// Arrange - write a config so SetDefaultAccount has a file to update
	tmpDir := t.TempDir()
	configPath := writeAccountTestConfig(t, tmpDir, "OLD_HASH")

	// Act
	var buf bytes.Buffer
	cmd := NewAccountCmd(testClient(t, jsonServer(`{}`)), configPath, &buf)
	_, err := runCobraCommand(t, cmd, "set-default", "NEW_HASH")

	// Assert
	require.NoError(t, err)

	env := decodeAccountEnvelope(t, buf.Bytes())
	dataMap, ok := env.Data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "NEW_HASH", dataMap["default_account"])

	// Verify the config file was updated
	cfg, loadErr := auth.LoadConfig(configPath)
	require.NoError(t, loadErr)
	assert.Equal(t, "NEW_HASH", cfg.DefaultAccount)
}

func TestNewAccountCmd_SetDefault_MissingHash(t *testing.T) {
	// Arrange
	tmpDir := t.TempDir()
	configPath := writeAccountTestConfig(t, tmpDir, "")

	// Act
	var buf bytes.Buffer
	cmd := NewAccountCmd(testClient(t, jsonServer(`{}`)), configPath, &buf)
	_, err := runCobraCommand(t, cmd, "set-default")

	// Assert
	require.Error(t, err)
	var valErr *apperr.ValidationError
	assert.ErrorAs(t, err, &valErr)
}

func TestNewAccountCmd_SetDefault_NoSafetyGuard(t *testing.T) {
	// Verify that set-default works without requireMutableEnabled.
	// This is intentional: setting a default account is a config change,
	// not a trading operation.
	tmpDir := t.TempDir()
	configPath := writeAccountTestConfig(t, tmpDir, "")

	// Act - no "i-also-like-to-live-dangerously" in config, should still succeed
	var buf bytes.Buffer
	cmd := NewAccountCmd(testClient(t, jsonServer(`{}`)), configPath, &buf)
	_, err := runCobraCommand(t, cmd, "set-default", "SOME_HASH")

	// Assert
	require.NoError(t, err)
}

// --- Enrichment helper tests (framework-agnostic) ---

func TestEnrichAccountsWithPreferences(t *testing.T) {
	acctNum1 := "12345"
	acctNum2 := "67890"
	nick1 := "My IRA"
	nick2 := "Joint"
	primary := true
	notPrimary := false

	accounts := []models.Account{
		{SecuritiesAccount: &models.SecuritiesAccount{AccountNumber: &acctNum1}},
		{SecuritiesAccount: &models.SecuritiesAccount{AccountNumber: &acctNum2}},
	}

	prefs := &models.UserPreference{
		Accounts: []models.UserPreferenceAccount{
			{AccountNumber: &acctNum1, NickName: &nick1, PrimaryAccount: &primary},
			{AccountNumber: &acctNum2, NickName: &nick2, PrimaryAccount: &notPrimary},
		},
	}

	enrichAccountsWithPreferences(accounts, prefs)

	require.NotNil(t, accounts[0].NickName)
	assert.Equal(t, "My IRA", *accounts[0].NickName)
	require.NotNil(t, accounts[0].PrimaryAccount)
	assert.True(t, *accounts[0].PrimaryAccount)

	require.NotNil(t, accounts[1].NickName)
	assert.Equal(t, "Joint", *accounts[1].NickName)
	require.NotNil(t, accounts[1].PrimaryAccount)
	assert.False(t, *accounts[1].PrimaryAccount)
}

func TestEnrichAccountsWithPreferences_NilPrefs(t *testing.T) {
	acctNum := "12345"
	accounts := []models.Account{
		{SecuritiesAccount: &models.SecuritiesAccount{AccountNumber: &acctNum}},
	}

	// Should not panic with nil preferences
	enrichAccountsWithPreferences(accounts, nil)
	assert.Nil(t, accounts[0].NickName)
}

func TestEnrichAccountsWithPreferences_NoMatch(t *testing.T) {
	acctNum := "12345"
	otherNum := "99999"
	nick := "Other Account"

	accounts := []models.Account{
		{SecuritiesAccount: &models.SecuritiesAccount{AccountNumber: &acctNum}},
	}

	prefs := &models.UserPreference{
		Accounts: []models.UserPreferenceAccount{
			{AccountNumber: &otherNum, NickName: &nick},
		},
	}

	enrichAccountsWithPreferences(accounts, prefs)
	assert.Nil(t, accounts[0].NickName, "account with no matching preference should not be enriched")
}

func TestEnrichAccountWithPreferences_SingleAccount(t *testing.T) {
	acctNum := "12345"
	nick := "My Account"
	primary := true

	account := &models.Account{
		SecuritiesAccount: &models.SecuritiesAccount{AccountNumber: &acctNum},
	}

	prefs := &models.UserPreference{
		Accounts: []models.UserPreferenceAccount{
			{AccountNumber: &acctNum, NickName: &nick, PrimaryAccount: &primary},
		},
	}

	enrichAccountWithPreferences(account, prefs)

	require.NotNil(t, account.NickName)
	assert.Equal(t, "My Account", *account.NickName)
	require.NotNil(t, account.PrimaryAccount)
	assert.True(t, *account.PrimaryAccount)
}

// --- Transaction subcommand tests ---

func TestNewAccountCmd_Transaction_List_WithAccountFlag(t *testing.T) {
	// Arrange
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Contains(t, r.URL.Path, "/trader/v1/accounts/abc123/transactions")

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"activityId":1001,"description":"BUY 100 AAPL"}]`))
	}))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewAccountCmd(testClient(t, srv), "/nonexistent/config.json", &buf)
	_, err := runCobraCommand(t, cmd, "transaction", "list", "--account", "abc123")

	// Assert
	require.NoError(t, err)

	var envelope output.Envelope
	require.NoError(t, json.Unmarshal(buf.Bytes(), &envelope))
	assert.NotNil(t, envelope.Data)
	assert.NotEmpty(t, envelope.Metadata.Timestamp)
}

func TestNewAccountCmd_Transaction_List_WithFilters(t *testing.T) {
	// Arrange
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		assert.Equal(t, "TRADE", q.Get("types"))
		assert.Equal(t, "2024-01-01", q.Get("startDate"))
		assert.Equal(t, "2024-01-31", q.Get("endDate"))

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewAccountCmd(testClient(t, srv), "/nonexistent/config.json", &buf)
	_, err := runCobraCommand(t, cmd,
		"transaction", "list",
		"--account", "abc123",
		"--types", "TRADE",
		"--from", "2024-01-01",
		"--to", "2024-01-31",
	)

	// Assert
	require.NoError(t, err)
}

func TestNewAccountCmd_Transaction_List_DefaultAccount(t *testing.T) {
	// Arrange
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/trader/v1/accounts/default-hash-123/transactions")

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	configData := []byte(`{"client_id":"test","client_secret":"test","default_account":"default-hash-123"}`)
	require.NoError(t, os.WriteFile(configPath, configData, 0o600))

	// Act
	var buf bytes.Buffer
	cmd := NewAccountCmd(testClient(t, srv), configPath, &buf)
	_, err := runCobraCommand(t, cmd, "transaction", "list")

	// Assert
	require.NoError(t, err)
}

func TestNewAccountCmd_Transaction_List_NoAccount(t *testing.T) {
	// Arrange
	server := jsonServer(`[]`)
	defer server.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewAccountCmd(testClient(t, server), "/nonexistent/config.json", &buf)
	_, err := runCobraCommand(t, cmd, "transaction", "list")

	// Assert
	require.Error(t, err)

	var accountErr *apperr.AccountNotFoundError
	assert.ErrorAs(t, err, &accountErr)
}

func TestNewAccountCmd_Transaction_Get_Success(t *testing.T) {
	// Arrange
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/trader/v1/accounts/abc123/transactions/1001", r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"activityId":1001,"description":"BUY 100 AAPL","netAmount":-15000.00}`))
	}))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewAccountCmd(testClient(t, srv), "/nonexistent/config.json", &buf)
	_, err := runCobraCommand(t, cmd,
		"transaction", "get", "--account", "abc123", "1001",
	)

	// Assert
	require.NoError(t, err)

	var envelope output.Envelope
	require.NoError(t, json.Unmarshal(buf.Bytes(), &envelope))
	assert.NotNil(t, envelope.Data)
	assert.NotEmpty(t, envelope.Metadata.Timestamp)
}

func TestNewAccountCmd_Transaction_Get_MissingTxnID(t *testing.T) {
	// Arrange
	server := jsonServer(`{}`)
	defer server.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewAccountCmd(testClient(t, server), "/nonexistent/config.json", &buf)
	_, err := runCobraCommand(t, cmd, "transaction", "get", "--account", "abc123")

	// Assert
	require.Error(t, err)

	var valErr *apperr.ValidationError
	assert.ErrorAs(t, err, &valErr)
}

func TestNewAccountCmd_Transaction_Get_InvalidTxnID(t *testing.T) {
	// Arrange
	server := jsonServer(`{}`)
	defer server.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewAccountCmd(testClient(t, server), "/nonexistent/config.json", &buf)
	_, err := runCobraCommand(t, cmd, "transaction", "get", "--account", "abc123", "not-a-number")

	// Assert
	require.Error(t, err)

	var valErr *apperr.ValidationError
	assert.ErrorAs(t, err, &valErr)
}

func TestNewAccountCmd_Transaction_Get_NoAccount(t *testing.T) {
	// Arrange
	server := jsonServer(`{}`)
	defer server.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewAccountCmd(testClient(t, server), "/nonexistent/config.json", &buf)
	_, err := runCobraCommand(t, cmd, "transaction", "get", "1001")

	// Assert
	require.Error(t, err)

	var accountErr *apperr.AccountNotFoundError
	assert.ErrorAs(t, err, &accountErr)
}

func TestNewAccountCmd_NoSubcommand(t *testing.T) {
	// Arrange
	server := jsonServer(`{}`)
	defer server.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewAccountCmd(testClient(t, server), "", &buf)
	_, err := runCobraCommand(t, cmd)

	// Assert
	require.Error(t, err)
	var valErr *apperr.ValidationError
	assert.ErrorAs(t, err, &valErr)
	assert.Contains(t, err.Error(), "requires a subcommand")
}

// --- resolveAccount tests (framework-agnostic) ---

func TestResolveAccount_FlagTakesPriority(t *testing.T) {
	account, err := resolveAccount("flag-account", "/nonexistent/config.json", nil)
	require.NoError(t, err)
	assert.Equal(t, "flag-account", account)
}

func TestResolveAccount_PositionalArgBeforeConfig(t *testing.T) {
	account, err := resolveAccount("", "/nonexistent/config.json", []string{"positional-account"})
	require.NoError(t, err)
	assert.Equal(t, "positional-account", account)
}

func TestResolveAccount_FlagBeforePositionalArg(t *testing.T) {
	account, err := resolveAccount("flag-account", "/nonexistent/config.json", []string{"positional-account"})
	require.NoError(t, err)
	assert.Equal(t, "flag-account", account)
}

func TestResolveAccount_FallbackToConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	// LoadConfig validates client_id/client_secret, so we must include them
	// for the config to load successfully and expose default_account.
	configData := []byte(`{"client_id":"test","client_secret":"test","default_account":"config-account"}`)
	require.NoError(t, os.WriteFile(configPath, configData, 0o600))

	account, err := resolveAccount("", configPath, nil)
	require.NoError(t, err)
	assert.Equal(t, "config-account", account)
}

func TestResolveAccount_NoAccountError(t *testing.T) {
	account, err := resolveAccount("", "/nonexistent/config.json", nil)
	require.Error(t, err)
	assert.Empty(t, account)

	var accountErr *apperr.AccountNotFoundError
	assert.ErrorAs(t, err, &accountErr)
}

func TestResolveAccount_TrimsWhitespace(t *testing.T) {
	account, err := resolveAccount("  spaced-account  ", "/nonexistent/config.json", nil)
	require.NoError(t, err)
	assert.Equal(t, "spaced-account", account)
}

func TestResolveAccount_EmptyPositionalArgsSkipped(t *testing.T) {
	// Empty positional args should be treated like nil - fall through to config/error.
	account, err := resolveAccount("", "/nonexistent/config.json", []string{})
	require.Error(t, err)
	assert.Empty(t, account)
}

func TestResolveAccount_WhitespaceOnlyPositionalArgSkipped(t *testing.T) {
	account, err := resolveAccount("", "/nonexistent/config.json", []string{"  "})
	require.Error(t, err)
	assert.Empty(t, account)
}
