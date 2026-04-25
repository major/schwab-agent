package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/major/schwab-agent/internal/apperr"
	"github.com/major/schwab-agent/internal/client"
	"github.com/major/schwab-agent/internal/models"
	"github.com/major/schwab-agent/internal/output"
)

func TestPositionListSingleAccount(t *testing.T) {
	account := map[string]any{
		"securitiesAccount": map[string]any{
			"type":          "MARGIN",
			"accountNumber": "12345",
			"positions": []map[string]any{
				{
					"longQuantity":       100.0,
					"averagePrice":       150.0,
					"marketValue":        16000.0,
					"longOpenProfitLoss": 1000.0,
					"instrument":         map[string]any{"symbol": "AAPL", "assetType": "EQUITY"},
				},
				{
					"longQuantity":       50.0,
					"averagePrice":       200.0,
					"marketValue":        11000.0,
					"longOpenProfitLoss": 1000.0,
					"instrument":         map[string]any{"symbol": "MSFT", "assetType": "EQUITY"},
				},
			},
		},
	}
	prefs := map[string]any{
		"accounts": []map[string]any{
			{"accountNumber": "12345", "nickName": "My IRA", "primaryAccount": true},
		},
	}

	srv := accountMockServer(t, map[string]any{
		"/trader/v1/accounts/HASH123": account,
		"/trader/v1/userPreference":   prefs,
	})
	defer srv.Close()

	c := &client.Ref{Client: client.NewClient("test-token", client.WithBaseURL(srv.URL))}
	configPath := writeAccountTestConfig(t, t.TempDir(), "")

	var buf bytes.Buffer
	cmd := PositionCommand(c, configPath, &buf)
	cmd.ExitErrHandler = noopExitHandler

	err := cmd.Run(context.Background(), []string{"position", "list", "--account", "HASH123"})
	require.NoError(t, err)

	env := decodeAccountEnvelope(t, buf.Bytes())
	assert.Equal(t, "HASH123", env.Metadata.Account)
	assert.Equal(t, 2, env.Metadata.Returned)

	dataMap, ok := env.Data.(map[string]any)
	require.True(t, ok)

	positions, ok := dataMap["positions"].([]any)
	require.True(t, ok)
	require.Len(t, positions, 2)

	// Verify first position has account info and computed fields.
	pos := positions[0].(map[string]any)
	assert.Equal(t, "12345", pos["accountNumber"])
	assert.Equal(t, "HASH123", pos["accountHash"])
	assert.Equal(t, "My IRA", pos["accountNickName"])
	assert.Equal(t, 15000.0, pos["totalCostBasis"]) // 150 * 100
	assert.Equal(t, 1000.0, pos["unrealizedPnL"])   // longOpenProfitLoss only

	// Verify instrument came through.
	inst, ok := pos["instrument"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "AAPL", inst["symbol"])
}

func TestPositionListSingleAccount_UsesConfigDefault(t *testing.T) {
	account := map[string]any{
		"securitiesAccount": map[string]any{
			"type":          "MARGIN",
			"accountNumber": "12345",
			"positions": []map[string]any{
				{
					"longQuantity": 10.0,
					"averagePrice": 50.0,
					"instrument":   map[string]any{"symbol": "VTI", "assetType": "EQUITY"},
				},
			},
		},
	}

	srv := accountMockServer(t, map[string]any{
		"/trader/v1/accounts/DEFAULT_HASH": account,
		"/trader/v1/userPreference":        map[string]any{"accounts": []any{}},
	})
	defer srv.Close()

	c := &client.Ref{Client: client.NewClient("test-token", client.WithBaseURL(srv.URL))}
	configPath := writeAccountTestConfig(t, t.TempDir(), "DEFAULT_HASH")

	var buf bytes.Buffer
	cmd := PositionCommand(c, configPath, &buf)
	cmd.ExitErrHandler = noopExitHandler

	// No --account flag: should use config default.
	err := cmd.Run(context.Background(), []string{"position", "list"})
	require.NoError(t, err)

	env := decodeAccountEnvelope(t, buf.Bytes())
	assert.Equal(t, "DEFAULT_HASH", env.Metadata.Account)
	assert.Equal(t, 1, env.Metadata.Returned)
}

func TestPositionListAllAccounts(t *testing.T) {
	accounts := []map[string]any{
		{
			"securitiesAccount": map[string]any{
				"type":          "MARGIN",
				"accountNumber": "11111",
				"positions": []map[string]any{
					{
						"longQuantity":       100.0,
						"averagePrice":       150.0,
						"longOpenProfitLoss": 500.0,
						"instrument":         map[string]any{"symbol": "AAPL", "assetType": "EQUITY"},
					},
				},
			},
		},
		{
			"securitiesAccount": map[string]any{
				"type":          "CASH",
				"accountNumber": "22222",
				"positions": []map[string]any{
					{
						"longQuantity":       200.0,
						"averagePrice":       50.0,
						"longOpenProfitLoss": 2000.0,
						"instrument":         map[string]any{"symbol": "VTI", "assetType": "EQUITY"},
					},
					{
						"shortQuantity":       10.0,
						"averagePrice":        300.0,
						"shortOpenProfitLoss": -500.0,
						"instrument":          map[string]any{"symbol": "TSLA", "assetType": "EQUITY"},
					},
				},
			},
		},
	}
	accountNumbers := []map[string]any{
		{"accountNumber": "11111", "hashValue": "HASH_A"},
		{"accountNumber": "22222", "hashValue": "HASH_B"},
	}
	prefs := map[string]any{
		"accounts": []map[string]any{
			{"accountNumber": "11111", "nickName": "Trading", "primaryAccount": true},
			{"accountNumber": "22222", "nickName": "Retirement", "primaryAccount": false},
		},
	}

	srv := accountMockServer(t, map[string]any{
		"/trader/v1/accounts":                accounts,
		"/trader/v1/accounts/accountNumbers": accountNumbers,
		"/trader/v1/userPreference":          prefs,
	})
	defer srv.Close()

	c := &client.Ref{Client: client.NewClient("test-token", client.WithBaseURL(srv.URL))}

	var buf bytes.Buffer
	cmd := PositionCommand(c, "", &buf)
	cmd.ExitErrHandler = noopExitHandler

	err := cmd.Run(context.Background(), []string{"position", "list", "--all-accounts"})
	require.NoError(t, err)

	env := decodeAccountEnvelope(t, buf.Bytes())
	assert.Empty(t, env.Metadata.Account, "all-accounts mode should not set account in metadata")
	assert.Equal(t, 3, env.Metadata.Returned)

	dataMap, ok := env.Data.(map[string]any)
	require.True(t, ok)

	positions, ok := dataMap["positions"].([]any)
	require.True(t, ok)
	require.Len(t, positions, 3)

	// First position: account 11111 / HASH_A / "Trading"
	pos0 := positions[0].(map[string]any)
	assert.Equal(t, "11111", pos0["accountNumber"])
	assert.Equal(t, "HASH_A", pos0["accountHash"])
	assert.Equal(t, "Trading", pos0["accountNickName"])

	inst0, ok := pos0["instrument"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "AAPL", inst0["symbol"])

	// Second position: account 22222 / HASH_B / "Retirement"
	pos1 := positions[1].(map[string]any)
	assert.Equal(t, "22222", pos1["accountNumber"])
	assert.Equal(t, "HASH_B", pos1["accountHash"])
	assert.Equal(t, "Retirement", pos1["accountNickName"])

	// Third position: short TSLA in account 22222
	pos2 := positions[2].(map[string]any)
	assert.Equal(t, "22222", pos2["accountNumber"])
	assert.Equal(t, -500.0, pos2["unrealizedPnL"])

	inst2, ok := pos2["instrument"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "TSLA", inst2["symbol"])
}

func TestPositionListAllAccounts_PreferencesFailure(t *testing.T) {
	accounts := []map[string]any{
		{
			"securitiesAccount": map[string]any{
				"type":          "MARGIN",
				"accountNumber": "12345",
				"positions": []map[string]any{
					{
						"longQuantity": 10.0,
						"averagePrice": 100.0,
						"instrument":   map[string]any{"symbol": "SPY", "assetType": "EQUITY"},
					},
				},
			},
		},
	}
	accountNumbers := []map[string]any{
		{"accountNumber": "12345", "hashValue": "HASH_X"},
	}

	// No /trader/v1/userPreference route: will 404.
	srv := accountMockServer(t, map[string]any{
		"/trader/v1/accounts":                accounts,
		"/trader/v1/accounts/accountNumbers": accountNumbers,
	})
	defer srv.Close()

	c := &client.Ref{Client: client.NewClient("test-token", client.WithBaseURL(srv.URL))}

	var buf bytes.Buffer
	cmd := PositionCommand(c, "", &buf)
	cmd.ExitErrHandler = noopExitHandler

	err := cmd.Run(context.Background(), []string{"position", "list", "--all-accounts"})
	require.NoError(t, err)

	env := decodeAccountEnvelope(t, buf.Bytes())
	assert.Equal(t, 1, env.Metadata.Returned)

	dataMap, ok := env.Data.(map[string]any)
	require.True(t, ok, "expected data to be map[string]any")
	positions, ok := dataMap["positions"].([]any)
	require.True(t, ok, "expected positions to be []any")
	pos, ok := positions[0].(map[string]any)
	require.True(t, ok, "expected position to be map[string]any")

	// Nickname should be absent since preferences failed.
	_, hasNick := pos["accountNickName"]
	assert.False(t, hasNick, "nickname should be omitted when preferences fail")
}

func TestPositionListNoPositions(t *testing.T) {
	// Account exists but has no positions.
	account := map[string]any{
		"securitiesAccount": map[string]any{
			"type":          "MARGIN",
			"accountNumber": "12345",
		},
	}

	srv := accountMockServer(t, map[string]any{
		"/trader/v1/accounts/HASH123": account,
		"/trader/v1/userPreference":   map[string]any{"accounts": []any{}},
	})
	defer srv.Close()

	c := &client.Ref{Client: client.NewClient("test-token", client.WithBaseURL(srv.URL))}
	configPath := writeAccountTestConfig(t, t.TempDir(), "")

	var buf bytes.Buffer
	cmd := PositionCommand(c, configPath, &buf)
	cmd.ExitErrHandler = noopExitHandler

	err := cmd.Run(context.Background(), []string{"position", "list", "--account", "HASH123"})
	require.NoError(t, err)

	env := decodeAccountEnvelope(t, buf.Bytes())
	assert.Equal(t, 0, env.Metadata.Returned)

	dataMap, ok := env.Data.(map[string]any)
	require.True(t, ok, "expected data to be map[string]any")
	positions, ok := dataMap["positions"].([]any)
	require.True(t, ok, "expected positions to be []any")
	assert.Empty(t, positions, "empty positions should return empty array, not null")
}

func TestPositionListMutuallyExclusiveFlags(t *testing.T) {
	// --all-accounts and --account should not be used together.
	srv := accountMockServer(t, map[string]any{})
	defer srv.Close()

	c := &client.Ref{Client: client.NewClient("test-token", client.WithBaseURL(srv.URL))}
	configPath := writeAccountTestConfig(t, t.TempDir(), "HASH123")

	var buf bytes.Buffer
	cmd := PositionCommand(c, configPath, &buf)
	cmd.ExitErrHandler = noopExitHandler

	err := cmd.Run(context.Background(), []string{"position", "list", "--all-accounts", "--account", "HASH456"})
	require.Error(t, err)

	var ve *apperr.ValidationError
	require.ErrorAs(t, err, &ve, "should return ValidationError for mutually exclusive flags")
	assert.Contains(t, err.Error(), "mutually exclusive")
}

func TestPositionListNoAccountConfigured(t *testing.T) {
	srv := accountMockServer(t, map[string]any{})
	defer srv.Close()

	c := &client.Ref{Client: client.NewClient("test-token", client.WithBaseURL(srv.URL))}
	// Config with no default account.
	configPath := writeAccountTestConfig(t, t.TempDir(), "")

	var buf bytes.Buffer
	cmd := PositionCommand(c, configPath, &buf)
	cmd.ExitErrHandler = noopExitHandler

	err := cmd.Run(context.Background(), []string{"position", "list"})
	require.Error(t, err, "should error when no account is specified or configured")
}

func TestPositionListRequestsPositionsField(t *testing.T) {
	// Verify the client sends ?fields=positions to the API.
	var capturedFields string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/trader/v1/accounts/HASH123":
			capturedFields = r.URL.Query().Get("fields")

			w.Header().Set("Content-Type", "application/json")
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"securitiesAccount": map[string]any{
					"type":          "MARGIN",
					"accountNumber": "12345",
				},
			}))
		case "/trader/v1/userPreference":
			w.Header().Set("Content-Type", "application/json")
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"accounts": []any{}}))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	c := &client.Ref{Client: client.NewClient("test-token", client.WithBaseURL(srv.URL))}
	configPath := writeAccountTestConfig(t, t.TempDir(), "")

	var buf bytes.Buffer
	cmd := PositionCommand(c, configPath, &buf)
	cmd.ExitErrHandler = noopExitHandler

	err := cmd.Run(context.Background(), []string{"position", "list", "--account", "HASH123"})
	require.NoError(t, err)
	assert.Equal(t, "positions", capturedFields)
}

func TestComputePositionFields(t *testing.T) {
	t.Run("long position with P&L", func(t *testing.T) {
		entry := positionEntry{
			Position: models.Position{
				AveragePrice:       new(150.0),
				LongQuantity:       new(100.0),
				LongOpenProfitLoss: new(1000.0),
			},
		}
		computePositionFields(&entry)

		require.NotNil(t, entry.TotalCostBasis)
		assert.Equal(t, 15000.0, *entry.TotalCostBasis)

		require.NotNil(t, entry.UnrealizedPnL)
		assert.Equal(t, 1000.0, *entry.UnrealizedPnL)

		require.NotNil(t, entry.UnrealizedPnLPct)
		assert.InDelta(t, 6.6667, *entry.UnrealizedPnLPct, 0.001)
	})

	t.Run("short position with P&L", func(t *testing.T) {
		entry := positionEntry{
			Position: models.Position{
				AveragePrice:        new(300.0),
				ShortQuantity:       new(10.0),
				ShortOpenProfitLoss: new(-500.0),
			},
		}
		computePositionFields(&entry)

		require.NotNil(t, entry.TotalCostBasis)
		assert.Equal(t, 3000.0, *entry.TotalCostBasis)

		require.NotNil(t, entry.UnrealizedPnL)
		assert.Equal(t, -500.0, *entry.UnrealizedPnL)

		require.NotNil(t, entry.UnrealizedPnLPct)
		assert.InDelta(t, -16.6667, *entry.UnrealizedPnLPct, 0.001)
	})

	t.Run("both long and short P&L", func(t *testing.T) {
		entry := positionEntry{
			Position: models.Position{
				AveragePrice:        new(100.0),
				LongQuantity:        new(50.0),
				ShortQuantity:       new(10.0),
				LongOpenProfitLoss:  new(500.0),
				ShortOpenProfitLoss: new(-200.0),
			},
		}
		computePositionFields(&entry)

		require.NotNil(t, entry.TotalCostBasis)
		assert.Equal(t, 6000.0, *entry.TotalCostBasis) // 100 * (50 + 10)

		require.NotNil(t, entry.UnrealizedPnL)
		assert.Equal(t, 300.0, *entry.UnrealizedPnL) // 500 + (-200)

		require.NotNil(t, entry.UnrealizedPnLPct)
		assert.InDelta(t, 5.0, *entry.UnrealizedPnLPct, 0.001) // 300/6000*100
	})

	t.Run("nil averagePrice skips cost basis", func(t *testing.T) {
		entry := positionEntry{
			Position: models.Position{
				LongQuantity:       new(100.0),
				LongOpenProfitLoss: new(500.0),
			},
		}
		computePositionFields(&entry)

		assert.Nil(t, entry.TotalCostBasis)
		require.NotNil(t, entry.UnrealizedPnL)
		assert.Equal(t, 500.0, *entry.UnrealizedPnL)
		// No P&L pct because no cost basis.
		assert.Nil(t, entry.UnrealizedPnLPct)
	})

	t.Run("zero quantity skips cost basis", func(t *testing.T) {
		entry := positionEntry{
			Position: models.Position{
				AveragePrice: new(150.0),
				// No quantity fields set (both nil, default to 0).
			},
		}
		computePositionFields(&entry)

		assert.Nil(t, entry.TotalCostBasis)
	})

	t.Run("no P&L fields", func(t *testing.T) {
		entry := positionEntry{
			Position: models.Position{
				AveragePrice: new(150.0),
				LongQuantity: new(100.0),
			},
		}
		computePositionFields(&entry)

		require.NotNil(t, entry.TotalCostBasis)
		assert.Equal(t, 15000.0, *entry.TotalCostBasis)
		assert.Nil(t, entry.UnrealizedPnL)
		assert.Nil(t, entry.UnrealizedPnLPct)
	})

	t.Run("all fields nil", func(t *testing.T) {
		entry := positionEntry{}
		computePositionFields(&entry)

		assert.Nil(t, entry.TotalCostBasis)
		assert.Nil(t, entry.UnrealizedPnL)
		assert.Nil(t, entry.UnrealizedPnLPct)
	})
}

func TestFlattenAccountPositions(t *testing.T) {
	accounts := []models.Account{
		{
			SecuritiesAccount: &models.SecuritiesAccount{
				AccountNumber: new("11111"),
				Positions: []models.Position{
					{
						LongQuantity: new(100.0),
						AveragePrice: new(50.0),
						Instrument:   &models.AccountsInstrument{Symbol: new("AAPL")},
					},
				},
			},
			NickName: new("Trading"),
		},
		{
			// Account with no SecuritiesAccount: should be skipped.
			SecuritiesAccount: nil,
		},
		{
			SecuritiesAccount: &models.SecuritiesAccount{
				AccountNumber: new("22222"),
				Positions: []models.Position{
					{
						ShortQuantity: new(5.0),
						AveragePrice:  new(200.0),
						Instrument:    &models.AccountsInstrument{Symbol: new("TSLA")},
					},
				},
			},
		},
	}

	numberToHash := map[string]string{
		"11111": "HASH_A",
		"22222": "HASH_B",
	}

	entries := flattenAccountPositions(accounts, numberToHash)
	require.Len(t, entries, 2)

	assert.Equal(t, "11111", entries[0].AccountNumber)
	assert.Equal(t, "HASH_A", entries[0].AccountHash)
	assert.Equal(t, "Trading", entries[0].AccountNickName)
	assert.Equal(t, "AAPL", *entries[0].Instrument.Symbol)

	assert.Equal(t, "22222", entries[1].AccountNumber)
	assert.Equal(t, "HASH_B", entries[1].AccountHash)
	assert.Empty(t, entries[1].AccountNickName)
	assert.Equal(t, "TSLA", *entries[1].Instrument.Symbol)
}

func TestFlattenAccountPositions_Empty(t *testing.T) {
	entries := flattenAccountPositions(nil, nil)
	require.NotNil(t, entries, "should return empty slice, not nil")
	assert.Empty(t, entries)
}

func TestPositionListComputedFieldsInOutput(t *testing.T) {
	// End-to-end test verifying computed fields appear in the JSON output.
	account := map[string]any{
		"securitiesAccount": map[string]any{
			"type":          "MARGIN",
			"accountNumber": "12345",
			"positions": []map[string]any{
				{
					"longQuantity":       100.0,
					"averagePrice":       150.0,
					"longOpenProfitLoss": 1500.0,
					"marketValue":        16500.0,
					"instrument":         map[string]any{"symbol": "AAPL", "assetType": "EQUITY"},
				},
			},
		},
	}

	srv := accountMockServer(t, map[string]any{
		"/trader/v1/accounts/HASH123": account,
		"/trader/v1/userPreference":   map[string]any{"accounts": []any{}},
	})
	defer srv.Close()

	c := &client.Ref{Client: client.NewClient("test-token", client.WithBaseURL(srv.URL))}
	configPath := writeAccountTestConfig(t, t.TempDir(), "")

	var buf bytes.Buffer
	cmd := PositionCommand(c, configPath, &buf)
	cmd.ExitErrHandler = noopExitHandler

	err := cmd.Run(context.Background(), []string{"position", "list", "--account", "HASH123"})
	require.NoError(t, err)

	// Decode the raw JSON to verify computed fields are present.
	var raw map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &raw))

	dataMap, ok := raw["data"].(map[string]any)
	require.True(t, ok, "expected data to be map[string]any")
	positions, ok := dataMap["positions"].([]any)
	require.True(t, ok, "expected positions to be []any")
	pos, ok := positions[0].(map[string]any)
	require.True(t, ok, "expected position to be map[string]any")

	assert.Equal(t, 15000.0, pos["totalCostBasis"])                   // 150 * 100
	assert.Equal(t, 1500.0, pos["unrealizedPnL"])                     // longOpenProfitLoss
	assert.InDelta(t, 10.0, pos["unrealizedPnLPct"].(float64), 0.001) // 1500/15000*100

	// Original API fields should still be present.
	assert.Equal(t, 100.0, pos["longQuantity"])
	assert.Equal(t, 150.0, pos["averagePrice"])
	assert.Equal(t, 16500.0, pos["marketValue"])
}

// Suppress unused import warnings for output package.
var _ = output.Envelope{}
