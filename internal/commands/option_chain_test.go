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
	"github.com/major/schwab-agent/internal/models"
	"github.com/major/schwab-agent/internal/output"
)

//nolint:gocognit // table-driven test with many subtests
func TestOptionChainCompactRows(t *testing.T) {
	// Expected default column order for the compact chain view.
	wantColumns := []string{
		"expiry", "strike", "cp", "symbol",
		"bid", "ask", "mark", "delta", "iv", "oi", "volume",
	}

	tests := []struct {
		name       string
		chain      *models.OptionChain
		wantCols   []string
		wantRows   [][]any
		wantErrNil bool // true when no error is expected
	}{
		{
			name: "call only",
			chain: &models.OptionChain{
				CallExpDateMap: map[string]map[string][]*models.OptionContract{
					"2026-01-16:257": {
						"200.0": {
							{
								PutCall:        new("CALL"),
								Symbol:         new("AAPL  260116C00200000"),
								ExpirationDate: new("2026-01-16"),
								StrikePrice:    new(200.0),
								Bid:            new(12.30),
								Ask:            new(12.45),
								Mark:           new(12.375),
								Delta:          new(0.52),
								Volatility:     new(0.28),
								OpenInterest:   new(int64(1234)),
								TotalVolume:    new(int64(567)),
							},
						},
					},
				},
			},
			wantCols: wantColumns,
			wantRows: [][]any{
				{
					"2026-01-16",
					200.0,
					"CALL",
					"AAPL  260116C00200000",
					12.30,
					12.45,
					12.375,
					0.52,
					0.28,
					int64(1234),
					int64(567),
				},
			},
			wantErrNil: true,
		},
		{
			name: "put only",
			chain: &models.OptionChain{
				PutExpDateMap: map[string]map[string][]*models.OptionContract{
					"2026-01-16:257": {
						"200.0": {
							{
								PutCall:        new("PUT"),
								Symbol:         new("AAPL  260116P00200000"),
								ExpirationDate: new("2026-01-16"),
								StrikePrice:    new(200.0),
								Bid:            new(8.10),
								Ask:            new(8.25),
								Mark:           new(8.175),
								Delta:          new(-0.48),
								Volatility:     new(0.30),
								OpenInterest:   new(int64(890)),
								TotalVolume:    new(int64(123)),
							},
						},
					},
				},
			},
			wantCols: wantColumns,
			wantRows: [][]any{
				{
					"2026-01-16",
					200.0,
					"PUT",
					"AAPL  260116P00200000",
					8.10,
					8.25,
					8.175,
					-0.48,
					0.30,
					int64(890),
					int64(123),
				},
			},
			wantErrNil: true,
		},
		{
			name: "mixed call and put at same strike",
			chain: &models.OptionChain{
				CallExpDateMap: map[string]map[string][]*models.OptionContract{
					"2026-01-16:257": {
						"200.0": {
							{
								PutCall:        new("CALL"),
								Symbol:         new("AAPL  260116C00200000"),
								ExpirationDate: new("2026-01-16"),
								StrikePrice:    new(200.0),
								Bid:            new(12.30),
								Ask:            new(12.45),
								Mark:           new(12.375),
								Delta:          new(0.52),
								Volatility:     new(0.28),
								OpenInterest:   new(int64(1234)),
								TotalVolume:    new(int64(567)),
							},
						},
					},
				},
				PutExpDateMap: map[string]map[string][]*models.OptionContract{
					"2026-01-16:257": {
						"200.0": {
							{
								PutCall:        new("PUT"),
								Symbol:         new("AAPL  260116P00200000"),
								ExpirationDate: new("2026-01-16"),
								StrikePrice:    new(200.0),
								Bid:            new(8.10),
								Ask:            new(8.25),
								Mark:           new(8.175),
								Delta:          new(-0.48),
								Volatility:     new(0.30),
								OpenInterest:   new(int64(890)),
								TotalVolume:    new(int64(123)),
							},
						},
					},
				},
			},
			wantCols: wantColumns,
			wantRows: [][]any{
				// Calls before puts at same expiry+strike.
				{
					"2026-01-16",
					200.0,
					"CALL",
					"AAPL  260116C00200000",
					12.30,
					12.45,
					12.375,
					0.52,
					0.28,
					int64(1234),
					int64(567),
				},
				{
					"2026-01-16",
					200.0,
					"PUT",
					"AAPL  260116P00200000",
					8.10,
					8.25,
					8.175,
					-0.48,
					0.30,
					int64(890),
					int64(123),
				},
			},
			wantErrNil: true,
		},
		{
			name:       "empty chain nil maps",
			chain:      &models.OptionChain{},
			wantCols:   wantColumns,
			wantRows:   [][]any{},
			wantErrNil: true,
		},
		{
			name: "empty chain empty maps",
			chain: &models.OptionChain{
				CallExpDateMap: map[string]map[string][]*models.OptionContract{},
				PutExpDateMap:  map[string]map[string][]*models.OptionContract{},
			},
			wantCols:   wantColumns,
			wantRows:   [][]any{},
			wantErrNil: true,
		},
		{
			name: "deterministic sort expiry ASC strike ASC calls before puts",
			chain: &models.OptionChain{
				CallExpDateMap: map[string]map[string][]*models.OptionContract{
					// Later expiry comes first in map iteration to prove sorting.
					"2026-06-19:411": {
						"210.0": {{
							PutCall: new("CALL"), Symbol: new("AAPL  260619C00210000"),
							ExpirationDate: new("2026-06-19"), StrikePrice: new(210.0),
							Bid: new(5.0), Ask: new(5.5), Mark: new(5.25),
							Delta: new(0.40), Volatility: new(0.25),
							OpenInterest: new(int64(100)), TotalVolume: new(int64(50)),
						}},
					},
					"2026-01-16:257": {
						"210.0": {{
							PutCall: new("CALL"), Symbol: new("AAPL  260116C00210000"),
							ExpirationDate: new("2026-01-16"), StrikePrice: new(210.0),
							Bid: new(7.0), Ask: new(7.5), Mark: new(7.25),
							Delta: new(0.45), Volatility: new(0.27),
							OpenInterest: new(int64(200)), TotalVolume: new(int64(80)),
						}},
						"200.0": {{
							PutCall: new("CALL"), Symbol: new("AAPL  260116C00200000"),
							ExpirationDate: new("2026-01-16"), StrikePrice: new(200.0),
							Bid: new(12.0), Ask: new(12.5), Mark: new(12.25),
							Delta: new(0.52), Volatility: new(0.28),
							OpenInterest: new(int64(300)), TotalVolume: new(int64(90)),
						}},
					},
				},
				PutExpDateMap: map[string]map[string][]*models.OptionContract{
					"2026-01-16:257": {
						"200.0": {{
							PutCall: new("PUT"), Symbol: new("AAPL  260116P00200000"),
							ExpirationDate: new("2026-01-16"), StrikePrice: new(200.0),
							Bid: new(8.0), Ask: new(8.5), Mark: new(8.25),
							Delta: new(-0.48), Volatility: new(0.30),
							OpenInterest: new(int64(400)), TotalVolume: new(int64(70)),
						}},
					},
				},
			},
			wantCols: wantColumns,
			wantRows: [][]any{
				// 2026-01-16, strike 200: call then put
				{
					"2026-01-16",
					200.0,
					"CALL",
					"AAPL  260116C00200000",
					12.0,
					12.5,
					12.25,
					0.52,
					0.28,
					int64(300),
					int64(90),
				},
				{
					"2026-01-16",
					200.0,
					"PUT",
					"AAPL  260116P00200000",
					8.0,
					8.5,
					8.25,
					-0.48,
					0.30,
					int64(400),
					int64(70),
				},
				// 2026-01-16, strike 210: call only (no put at this strike)
				{
					"2026-01-16",
					210.0,
					"CALL",
					"AAPL  260116C00210000",
					7.0,
					7.5,
					7.25,
					0.45,
					0.27,
					int64(200),
					int64(80),
				},
				// 2026-06-19, strike 210: call only
				{
					"2026-06-19",
					210.0,
					"CALL",
					"AAPL  260619C00210000",
					5.0,
					5.5,
					5.25,
					0.40,
					0.25,
					int64(100),
					int64(50),
				},
			},
			wantErrNil: true,
		},
		{
			name: "nil pointer fields emit nil not zero",
			chain: &models.OptionChain{
				CallExpDateMap: map[string]map[string][]*models.OptionContract{
					"2026-01-16:257": {
						"200.0": {
							{
								// Only PutCall, Symbol, ExpirationDate, and StrikePrice are set.
								// All price and greek fields are nil.
								PutCall:        new("CALL"),
								Symbol:         new("AAPL  260116C00200000"),
								ExpirationDate: new("2026-01-16"),
								StrikePrice:    new(200.0),
								// Bid, Ask, Mark, Delta, Volatility, OpenInterest, TotalVolume are nil.
							},
						},
					},
				},
			},
			wantCols: wantColumns,
			wantRows: [][]any{
				// Nil pointer fields produce nil values in the row, not zero-value floats/ints.
				{"2026-01-16", 200.0, "CALL", "AAPL  260116C00200000", nil, nil, nil, nil, nil, nil, nil},
			},
			wantErrNil: true,
		},
		{
			name: "zero volume and zero OI rows included",
			chain: &models.OptionChain{
				CallExpDateMap: map[string]map[string][]*models.OptionContract{
					"2026-01-16:257": {
						"200.0": {
							{
								PutCall:        new("CALL"),
								Symbol:         new("AAPL  260116C00200000"),
								ExpirationDate: new("2026-01-16"),
								StrikePrice:    new(200.0),
								Bid:            new(0.01),
								Ask:            new(0.05),
								Mark:           new(0.03),
								Delta:          new(0.01),
								Volatility:     new(0.90),
								OpenInterest:   new(int64(0)),
								TotalVolume:    new(int64(0)),
							},
						},
					},
				},
			},
			wantCols: wantColumns,
			wantRows: [][]any{
				// Zero volume and zero OI are valid data points; the row must not be filtered out.
				{
					"2026-01-16",
					200.0,
					"CALL",
					"AAPL  260116C00200000",
					0.01,
					0.05,
					0.03,
					0.01,
					0.90,
					int64(0),
					int64(0),
				},
			},
			wantErrNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			gotCols, gotRows, err := flattenChainRows(tt.chain)

			// Assert - error expectation
			if tt.wantErrNil {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				return
			}

			// Assert - columns match expected
			assert.Equal(t, tt.wantCols, gotCols, "columns mismatch")

			// Assert - row count
			require.Len(t, gotRows, len(tt.wantRows), "row count mismatch")

			// Assert - every row length matches column count
			for i, row := range gotRows {
				assert.Len(t, row, len(gotCols), "row %d length != column count", i)
			}

			// Assert - row content
			for i, wantRow := range tt.wantRows {
				gotRow := gotRows[i]
				for j, wantVal := range wantRow {
					colName := gotCols[j]
					if wantVal == nil {
						assert.Nil(t, gotRow[j], "row %d col %q (%d): got %v, want nil", i, colName, j, gotRow[j])
					} else {
						switch w := wantVal.(type) {
						case float64:
							gotFloat, ok := gotRow[j].(float64)
							require.True(
								t,
								ok,
								"row %d col %q (%d): expected float64, got %T",
								i,
								colName,
								j,
								gotRow[j],
							)
							assert.InDelta(t, w, gotFloat, 0.001, "row %d col %q (%d)", i, colName, j)
						default:
							assert.Equal(t, wantVal, gotRow[j], "row %d col %q (%d)", i, colName, j)
						}
					}
				}
			}
		})
	}
}

func TestOptionChainFieldProjection(t *testing.T) {
	// validFields is the allowlist from the design doc.
	validFields := []string{
		"expiry", "strike", "cp", "symbol",
		"bid", "ask", "mark", "last",
		"delta", "gamma", "theta", "vega", "rho",
		"iv", "oi", "volume", "itm", "dte",
	}

	tests := []struct {
		name            string
		requestedFields []string
		wantColumns     []string
		wantErr         bool
		wantErrContains string
	}{
		{
			name:            "default fields",
			requestedFields: nil,
			wantColumns: []string{
				"expiry",
				"strike",
				"cp",
				"symbol",
				"bid",
				"ask",
				"mark",
				"delta",
				"iv",
				"oi",
				"volume",
			},
			wantErr: false,
		},
		{
			name:            "custom fields exact order",
			requestedFields: []string{"strike", "bid", "ask"},
			wantColumns:     []string{"strike", "bid", "ask"},
			wantErr:         false,
		},
		{
			name:            "identity fields omitted when not requested",
			requestedFields: []string{"bid", "ask"},
			wantColumns:     []string{"bid", "ask"},
			wantErr:         false,
		},
		{
			name:            "unknown field returns validation error",
			requestedFields: []string{"expiry", "nope"},
			wantErr:         true,
			wantErrContains: "nope",
		},
		{
			name:            "all valid fields accepted",
			requestedFields: validFields,
			wantColumns:     validFields,
			wantErr:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act - projectChainFields resolves the column list from a requested field list.
			// When requestedFields is nil, it returns the default column order.
			gotColumns, err := resolveChainColumns(tt.requestedFields)

			// Assert
			if tt.wantErr {
				require.Error(t, err)
				if tt.wantErrContains != "" {
					assert.Contains(t, err.Error(), tt.wantErrContains)
				}
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantColumns, gotColumns)
		})
	}
}

// chainAPIResponse builds a minimal JSON chain response with the given
// call/put expiration-date maps. Includes an underlying price for ATM
// centering tests.
func chainAPIResponse(underlyingPrice float64, calls, puts map[string]map[string][]*models.OptionContract) string {
	chain := models.OptionChain{
		Symbol:          new("AAPL"),
		Status:          new("SUCCESS"),
		UnderlyingPrice: new(underlyingPrice),
		CallExpDateMap:  calls,
		PutExpDateMap:   puts,
	}
	b, _ := json.Marshal(chain)
	return string(b)
}

func TestOptionChainCommand(t *testing.T) {
	t.Run("successful compact output with default fields", func(t *testing.T) {
		// Arrange - mock server with one call at one expiration.
		body := chainAPIResponse(205.0,
			map[string]map[string][]*models.OptionContract{
				"2026-06-19:43": {
					"200.0": {{
						PutCall: new("CALL"), Symbol: new("AAPL  260619C00200000"),
						ExpirationDate: new("2026-06-19"), StrikePrice: new(200.0),
						Bid: new(12.0), Ask: new(12.5), Mark: new(12.25),
						Delta: new(0.55), Volatility: new(0.28),
						OpenInterest: new(int64(500)), TotalVolume: new(int64(100)),
					}},
				},
			},
			nil,
		)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if r.URL.Path == "/marketdata/v1/chains" {
				assert.Equal(t, "AAPL", r.URL.Query().Get("symbol"))
				_, _ = w.Write([]byte(body))
				return
			}
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		var buf bytes.Buffer
		cmd := NewOptionCmd(testClient(t, server), &buf)

		// Act
		_, err := runTestCommand(t, cmd, "chain", "AAPL")

		// Assert
		require.NoError(t, err)

		var envelope output.Envelope
		require.NoError(t, json.Unmarshal(buf.Bytes(), &envelope))

		data, ok := envelope.Data.(map[string]any)
		require.True(t, ok, "data should be a map")

		assert.Equal(t, "AAPL", data["underlying"])
		assert.Equal(t, "2026-06-19", data["expiration"])

		// Default columns: 11 fields.
		cols, ok := data["columns"].([]any)
		require.True(t, ok, "columns should be a slice")
		assert.Len(t, cols, 11, "expected 11 default columns")

		rows, ok := data["rows"].([]any)
		require.True(t, ok, "rows should be a slice")
		assert.Len(t, rows, 1, "expected 1 row")

		rowCount, ok := data["rowCount"].(float64)
		require.True(t, ok)
		assert.InDelta(t, 1.0, rowCount, 0.001)

		assert.Equal(t, 1, envelope.Metadata.Returned)
	})

	t.Run("custom fields projection", func(t *testing.T) {
		// Arrange - chain with one call contract.
		body := chainAPIResponse(205.0,
			map[string]map[string][]*models.OptionContract{
				"2026-06-19:43": {
					"200.0": {{
						PutCall: new("CALL"), Symbol: new("AAPL  260619C00200000"),
						ExpirationDate: new("2026-06-19"), StrikePrice: new(200.0),
						Bid: new(12.0), Ask: new(12.5), Mark: new(12.25),
						Delta: new(0.55), Gamma: new(0.03), Volatility: new(0.28),
						OpenInterest: new(int64(500)), TotalVolume: new(int64(100)),
					}},
				},
			},
			nil,
		)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(body))
		}))
		defer server.Close()

		var buf bytes.Buffer
		cmd := NewOptionCmd(testClient(t, server), &buf)

		// Act - request only three fields.
		_, err := runTestCommand(t, cmd, "chain", "AAPL", "--fields", "strike,bid,delta")

		// Assert
		require.NoError(t, err)

		var envelope output.Envelope
		require.NoError(t, json.Unmarshal(buf.Bytes(), &envelope))

		data, ok := envelope.Data.(map[string]any)
		require.True(t, ok)

		cols, ok := data["columns"].([]any)
		require.True(t, ok)
		assert.Equal(t, []any{"strike", "bid", "delta"}, cols)

		rows, ok := data["rows"].([]any)
		require.True(t, ok)
		require.Len(t, rows, 1)

		// Each row should have exactly 3 values.
		row, ok := rows[0].([]any)
		require.True(t, ok)
		assert.Len(t, row, 3)
	})

	t.Run("strike-count larger than available returns all", func(t *testing.T) {
		// Arrange - chain with 2 unique strikes, request 10.
		// Use a different underlying price (200.0) to exercise ATM centering
		// and satisfy unparam lint for chainAPIResponse.
		body := chainAPIResponse(200.0,
			map[string]map[string][]*models.OptionContract{
				"2026-06-19:43": {
					"200.0": {{
						PutCall: new("CALL"), Symbol: new("AAPL  260619C00200000"),
						ExpirationDate: new("2026-06-19"), StrikePrice: new(200.0),
						Bid: new(12.0), Ask: new(12.5), Mark: new(12.25),
						Delta: new(0.55), Volatility: new(0.28),
						OpenInterest: new(int64(500)), TotalVolume: new(int64(100)),
					}},
					"210.0": {{
						PutCall: new("CALL"), Symbol: new("AAPL  260619C00210000"),
						ExpirationDate: new("2026-06-19"), StrikePrice: new(210.0),
						Bid: new(7.0), Ask: new(7.5), Mark: new(7.25),
						Delta: new(0.40), Volatility: new(0.25),
						OpenInterest: new(int64(300)), TotalVolume: new(int64(50)),
					}},
				},
			},
			nil,
		)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(body))
		}))
		defer server.Close()

		var buf bytes.Buffer
		cmd := NewOptionCmd(testClient(t, server), &buf)

		// Act - request 10 strikes but only 2 exist.
		_, err := runTestCommand(t, cmd, "chain", "AAPL", "--strike-count", "10")

		// Assert - no error, returns all 2 rows.
		require.NoError(t, err)

		var envelope output.Envelope
		require.NoError(t, json.Unmarshal(buf.Bytes(), &envelope))

		data, ok := envelope.Data.(map[string]any)
		require.True(t, ok)

		rows, ok := data["rows"].([]any)
		require.True(t, ok)
		assert.Len(t, rows, 2, "should return all rows when strike-count exceeds available")

		// metadata.requested = 10, metadata.returned = 2.
		assert.Equal(t, 10, envelope.Metadata.Requested)
		assert.Equal(t, 2, envelope.Metadata.Returned)
	})

	t.Run("empty chain returns SymbolNotFoundError", func(t *testing.T) {
		// Arrange - mock server returning empty chain.
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"symbol":"AAPL","status":"SUCCESS"}`))
		}))
		defer server.Close()

		var buf bytes.Buffer
		cmd := NewOptionCmd(testClient(t, server), &buf)

		// Act
		_, err := runTestCommand(t, cmd, "chain", "AAPL")

		// Assert
		require.Error(t, err)
		var symbolErr *apperr.SymbolNotFoundError
		require.ErrorAs(t, err, &symbolErr)
		assert.Contains(t, err.Error(), "no option chain data for AAPL")
	})

	t.Run("missing symbol argument", func(t *testing.T) {
		// Arrange
		server := jsonServer(`{}`)
		defer server.Close()

		var buf bytes.Buffer
		cmd := NewOptionCmd(testClient(t, server), &buf)

		// Act
		_, err := runTestCommand(t, cmd, "chain")

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "accepts 1 arg(s), received 0")
	})

	t.Run("type flag filters contract type in API request", func(t *testing.T) {
		// Arrange - verify the API receives contractType=CALL. Pass an empty
		// but non-nil puts map to avoid unparam lint on chainAPIResponse.
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if r.URL.Path == "/marketdata/v1/chains" {
				assert.Equal(t, "CALL", r.URL.Query().Get("contractType"))
				body := chainAPIResponse(205.0,
					map[string]map[string][]*models.OptionContract{
						"2026-06-19:43": {
							"200.0": {{
								PutCall: new("CALL"), Symbol: new("AAPL  260619C00200000"),
								ExpirationDate: new("2026-06-19"), StrikePrice: new(200.0),
								Bid: new(12.0), Ask: new(12.5), Mark: new(12.25),
								Delta: new(0.55), Volatility: new(0.28),
								OpenInterest: new(int64(500)), TotalVolume: new(int64(100)),
							}},
						},
					},
					map[string]map[string][]*models.OptionContract{},
				)
				_, _ = w.Write([]byte(body))
				return
			}
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		var buf bytes.Buffer
		cmd := NewOptionCmd(testClient(t, server), &buf)

		// Act
		_, err := runTestCommand(t, cmd, "chain", "AAPL", "--type", "CALL")

		// Assert
		require.NoError(t, err)
	})
}
