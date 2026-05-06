package commands

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/major/schwab-agent/internal/apperr"
	"github.com/major/schwab-agent/internal/client"
	"github.com/major/schwab-agent/internal/models"
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
	cmd := NewPositionCmd(c, configPath, &buf)

	_, err := runTestCommand(t, cmd, "list", "--account", "HASH123")
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
	cmd := NewPositionCmd(c, configPath, &buf)

	// No --account flag: should use config default.
	_, err := runTestCommand(t, cmd, "list")
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
	cmd := NewPositionCmd(c, "", &buf)

	_, err := runTestCommand(t, cmd, "list", "--all-accounts")
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
	cmd := NewPositionCmd(c, "", &buf)

	_, err := runTestCommand(t, cmd, "list", "--all-accounts")
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
	cmd := NewPositionCmd(c, configPath, &buf)

	_, err := runTestCommand(t, cmd, "list", "--account", "HASH123")
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
	cmd := NewPositionCmd(c, configPath, &buf)

	_, err := runTestCommand(t, cmd, "list", "--all-accounts", "--account", "HASH456")
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
	cmd := NewPositionCmd(c, configPath, &buf)

	_, err := runTestCommand(t, cmd, "list")
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
	cmd := NewPositionCmd(c, configPath, &buf)

	_, err := runTestCommand(t, cmd, "list", "--account", "HASH123")
	require.NoError(t, err)
	assert.Equal(t, "positions", capturedFields)
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
	cmd := NewPositionCmd(c, configPath, &buf)

	_, err := runTestCommand(t, cmd, "list", "--account", "HASH123")
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

func TestPositionListFiltersSymbolsAndSortsByValue(t *testing.T) {
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
					"longOpenProfitLoss": -250.0,
					"instrument":         map[string]any{"symbol": "MSFT", "assetType": "EQUITY"},
				},
				{
					"longQuantity":       20.0,
					"averagePrice":       300.0,
					"marketValue":        4000.0,
					"longOpenProfitLoss": -500.0,
					"instrument":         map[string]any{"symbol": "TSLA", "assetType": "EQUITY"},
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
	cmd := NewPositionCmd(c, configPath, &buf)

	_, err := runTestCommand(t, cmd, "list", "--account", "HASH123", "--symbol", "msft,aapl", "--sort", "value-desc")
	require.NoError(t, err)

	env := decodeAccountEnvelope(t, buf.Bytes())
	assert.Equal(t, 2, env.Metadata.Returned)

	positions := positionListFromEnvelope(t, env.Data)
	require.Len(t, positions, 2)
	assert.Equal(t, "AAPL", positionSymbol(t, positions[0]))
	assert.Equal(t, "MSFT", positionSymbol(t, positions[1]))
}

func TestPositionListFiltersPnLAndSortsAscending(t *testing.T) {
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
					"longOpenProfitLoss": -250.0,
					"instrument":         map[string]any{"symbol": "MSFT", "assetType": "EQUITY"},
				},
				{
					"longQuantity":       20.0,
					"averagePrice":       300.0,
					"marketValue":        4000.0,
					"longOpenProfitLoss": -500.0,
					"instrument":         map[string]any{"symbol": "TSLA", "assetType": "EQUITY"},
				},
				{
					"longQuantity": 10.0,
					"averagePrice": 50.0,
					"marketValue":  500.0,
					"instrument":   map[string]any{"symbol": "VTI", "assetType": "EQUITY"},
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
	cmd := NewPositionCmd(c, configPath, &buf)

	_, err := runTestCommand(
		t,
		cmd,
		"list",
		"--account",
		"HASH123",
		"--losers-only",
		"--min-pnl",
		"-600",
		"--max-pnl",
		"-100",
		"--sort",
		"pnl-asc",
	)
	require.NoError(t, err)

	env := decodeAccountEnvelope(t, buf.Bytes())
	assert.Equal(t, 2, env.Metadata.Returned)

	positions := positionListFromEnvelope(t, env.Data)
	require.Len(t, positions, 2)
	assert.Equal(t, "TSLA", positionSymbol(t, positions[0]))
	assert.Equal(t, -500.0, positions[0]["unrealizedPnL"])
	assert.Equal(t, "MSFT", positionSymbol(t, positions[1]))
	assert.Equal(t, -250.0, positions[1]["unrealizedPnL"])
}

func TestPositionListRejectsInvalidPnLRange(t *testing.T) {
	srv := accountMockServer(t, map[string]any{})
	defer srv.Close()

	c := &client.Ref{Client: client.NewClient("test-token", client.WithBaseURL(srv.URL))}
	configPath := writeAccountTestConfig(t, t.TempDir(), "HASH123")

	var buf bytes.Buffer
	cmd := NewPositionCmd(c, configPath, &buf)

	_, err := runTestCommand(t, cmd, "list", "--min-pnl", "100", "--max-pnl", "-100")
	require.Error(t, err)

	var ve *apperr.ValidationError
	require.ErrorAs(t, err, &ve)
	assert.Contains(t, err.Error(), "--min-pnl cannot be greater than --max-pnl")
}

// ---------------------------------------------------------------------------
// Pure unit tests (framework-agnostic, no migration needed)
// ---------------------------------------------------------------------------

func positionListFromEnvelope(t *testing.T, data any) []map[string]any {
	t.Helper()

	dataMap, ok := data.(map[string]any)
	require.True(t, ok, "expected data to be map[string]any")
	positionsRaw, ok := dataMap["positions"].([]any)
	require.True(t, ok, "expected positions to be []any")

	positions := make([]map[string]any, 0, len(positionsRaw))
	for _, raw := range positionsRaw {
		position, ok := raw.(map[string]any)
		require.True(t, ok, "expected position to be map[string]any")
		positions = append(positions, position)
	}

	return positions
}

func positionSymbol(t *testing.T, position map[string]any) string {
	t.Helper()

	instrument, ok := position["instrument"].(map[string]any)
	require.True(t, ok, "expected instrument to be map[string]any")
	symbol, ok := instrument["symbol"].(string)
	require.True(t, ok, "expected instrument symbol to be string")

	return symbol
}

func TestComputePositionFields(t *testing.T) {
	t.Run("long position with P&L", func(t *testing.T) {
		entry := positionEntry{
			Position: models.Position{
				AveragePrice:       &testFloat150,
				LongQuantity:       &testFloat100,
				LongOpenProfitLoss: &testFloat1000,
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
				AveragePrice:        &testFloat300,
				ShortQuantity:       &testFloat10,
				ShortOpenProfitLoss: &testFloatNeg500,
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
				AveragePrice:        &testFloat100,
				LongQuantity:        &testFloat50,
				ShortQuantity:       &testFloat10,
				LongOpenProfitLoss:  &testFloat500,
				ShortOpenProfitLoss: &testFloatNeg200,
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
				LongQuantity:       &testFloat100,
				LongOpenProfitLoss: &testFloat500,
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
				AveragePrice: &testFloat150,
				// No quantity fields set (both nil, default to 0).
			},
		}
		computePositionFields(&entry)

		assert.Nil(t, entry.TotalCostBasis)
	})

	t.Run("no P&L fields", func(t *testing.T) {
		entry := positionEntry{
			Position: models.Position{
				AveragePrice: &testFloat150,
				LongQuantity: &testFloat100,
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

var (
	testFloat5      = 5.0
	testFloat10     = 10.0
	testFloat50     = 50.0
	testFloat100    = 100.0
	testFloat150    = 150.0
	testFloat200    = 200.0
	testFloat300    = 300.0
	testFloat500    = 500.0
	testFloat1000   = 1000.0
	testFloatNeg200 = -200.0
	testFloatNeg500 = -500.0

	testString11111   = "11111"
	testString22222   = "22222"
	testStringAAPL    = "AAPL"
	testStringTSLA    = "TSLA"
	testStringTrading = "Trading"
)

func TestFlattenAccountPositions(t *testing.T) {
	accounts := []models.Account{
		{
			SecuritiesAccount: &models.SecuritiesAccount{
				AccountNumber: &testString11111,
				Positions: []models.Position{
					{
						LongQuantity: &testFloat100,
						AveragePrice: &testFloat50,
						Instrument:   &models.AccountsInstrument{Symbol: &testStringAAPL},
					},
				},
			},
			NickName: &testStringTrading,
		},
		{
			// Account with no SecuritiesAccount: should be skipped.
			SecuritiesAccount: nil,
		},
		{
			SecuritiesAccount: &models.SecuritiesAccount{
				AccountNumber: &testString22222,
				Positions: []models.Position{
					{
						ShortQuantity: &testFloat5,
						AveragePrice:  &testFloat200,
						Instrument:    &models.AccountsInstrument{Symbol: &testStringTSLA},
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
