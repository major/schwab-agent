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

// screenTestChain builds a chain with multiple contracts spanning two
// expirations and a range of strikes, suitable for exercising screen filters.
func screenTestChain() string {
	return chainAPIResponse(205.0,
		map[string]map[string][]*models.OptionContract{
			// Near-term expiration (~30 DTE from test perspective).
			"2026-06-19:42": {
				"200.0": {{
					PutCall: new("CALL"), Symbol: new("AAPL  260619C00200000"),
					ExpirationDate: new("2026-06-19"), StrikePrice: new(200.0),
					Bid: new(12.0), Ask: new(12.50), Mark: new(12.25),
					Delta: new(0.55), Volatility: new(0.28),
					OpenInterest: new(int64(5000)), TotalVolume: new(int64(1200)),
				}},
				"210.0": {{
					PutCall: new("CALL"), Symbol: new("AAPL  260619C00210000"),
					ExpirationDate: new("2026-06-19"), StrikePrice: new(210.0),
					Bid: new(5.0), Ask: new(5.50), Mark: new(5.25),
					Delta: new(0.35), Volatility: new(0.30),
					OpenInterest: new(int64(3000)), TotalVolume: new(int64(800)),
				}},
				"220.0": {{
					PutCall: new("CALL"), Symbol: new("AAPL  260619C00220000"),
					ExpirationDate: new("2026-06-19"), StrikePrice: new(220.0),
					Bid: new(1.50), Ask: new(2.00), Mark: new(1.75),
					Delta: new(0.15), Volatility: new(0.35),
					OpenInterest: new(int64(100)), TotalVolume: new(int64(10)),
				}},
			},
			// Farther-term expiration.
			"2026-09-18:133": {
				"200.0": {{
					PutCall: new("CALL"), Symbol: new("AAPL  260918C00200000"),
					ExpirationDate: new("2026-09-18"), StrikePrice: new(200.0),
					Bid: new(15.0), Ask: new(15.80), Mark: new(15.40),
					Delta: new(0.60), Volatility: new(0.26),
					OpenInterest: new(int64(2000)), TotalVolume: new(int64(500)),
				}},
			},
		},
		map[string]map[string][]*models.OptionContract{
			"2026-06-19:42": {
				"200.0": {{
					PutCall: new("PUT"), Symbol: new("AAPL  260619P00200000"),
					ExpirationDate: new("2026-06-19"), StrikePrice: new(200.0),
					Bid: new(7.0), Ask: new(7.50), Mark: new(7.25),
					Delta: new(-0.45), Volatility: new(0.29),
					OpenInterest: new(int64(4000)), TotalVolume: new(int64(900)),
				}},
				"190.0": {{
					PutCall: new("PUT"), Symbol: new("AAPL  260619P00190000"),
					ExpirationDate: new("2026-06-19"), StrikePrice: new(190.0),
					Bid: new(3.0), Ask: new(3.40), Mark: new(3.20),
					Delta: new(-0.25), Volatility: new(0.32),
					OpenInterest: new(int64(6000)), TotalVolume: new(int64(1500)),
				}},
			},
		},
	)
}

// screenServer returns an httptest server that serves the test chain for any
// chains endpoint request.
func screenServer(body string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/marketdata/v1/chains" {
			_, _ = w.Write([]byte(body))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
}

func TestOptionScreenCommand(t *testing.T) {
	t.Run("default output with no filters", func(t *testing.T) {
		// Arrange
		server := screenServer(screenTestChain())
		defer server.Close()

		var buf bytes.Buffer
		cmd := NewOptionCmd(testClient(t, server), &buf)

		// Act
		_, err := runTestCommand(t, cmd, "screen", "AAPL")

		// Assert
		require.NoError(t, err)

		var envelope output.Envelope
		require.NoError(t, json.Unmarshal(buf.Bytes(), &envelope))

		data, ok := envelope.Data.(map[string]any)
		require.True(t, ok)

		assert.Equal(t, "AAPL", data["underlying"])

		// Default columns include spreadPct and dte.
		cols, ok := data["columns"].([]any)
		require.True(t, ok)
		assert.Contains(t, cols, "spreadPct")
		assert.Contains(t, cols, "dte")

		// Should return all 6 contracts (4 calls + 2 puts).
		rowCount, ok := data["rowCount"].(float64)
		require.True(t, ok)
		assert.InDelta(t, 6.0, rowCount, 0.001)

		// Metadata fields.
		totalScanned, ok := data["totalScanned"].(float64)
		require.True(t, ok)
		assert.InDelta(t, 6.0, totalScanned, 0.001)
	})

	t.Run("type filter CALL only", func(t *testing.T) {
		// Arrange - verify API receives contractType=CALL.
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if r.URL.Path == "/marketdata/v1/chains" {
				assert.Equal(t, "CALL", r.URL.Query().Get("contractType"))
				_, _ = w.Write([]byte(screenTestChain()))
				return
			}
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		var buf bytes.Buffer
		cmd := NewOptionCmd(testClient(t, server), &buf)

		// Act
		_, err := runTestCommand(t, cmd, "screen", "AAPL", "--type", "CALL")

		// Assert
		require.NoError(t, err)
	})

	t.Run("delta filter narrows results", func(t *testing.T) {
		// Arrange
		server := screenServer(screenTestChain())
		defer server.Close()

		var buf bytes.Buffer
		cmd := NewOptionCmd(testClient(t, server), &buf)

		// Act - filter for delta 0.30 to 0.60 (should match 3 calls: 0.35, 0.55, 0.60).
		_, err := runTestCommand(t, cmd, "screen", "AAPL", "--type", "CALL",
			"--delta-min", "0.30", "--delta-max", "0.60")

		// Assert
		require.NoError(t, err)

		var envelope output.Envelope
		require.NoError(t, json.Unmarshal(buf.Bytes(), &envelope))

		data, ok := envelope.Data.(map[string]any)
		require.True(t, ok)

		rows, ok := data["rows"].([]any)
		require.True(t, ok)
		assert.Len(t, rows, 3, "expected 3 calls with delta in [0.30, 0.60]")
	})

	t.Run("delta-max zero filters out all positive delta", func(t *testing.T) {
		// Verifies that --delta-max 0 is honored as an explicit bound, not
		// treated as "unset". With delta-max=0, only contracts with delta <= 0
		// should pass. In the test chain, puts have negative delta (-0.45, -0.25),
		// and all calls have positive delta (0.15, 0.35, 0.55, 0.60).

		// Arrange
		server := screenServer(screenTestChain())
		defer server.Close()

		var buf bytes.Buffer
		cmd := NewOptionCmd(testClient(t, server), &buf)

		// Act - delta-max 0 should exclude all calls (positive delta).
		_, err := runTestCommand(t, cmd, "screen", "AAPL", "--delta-max", "0")

		// Assert
		require.NoError(t, err)

		var envelope output.Envelope
		require.NoError(t, json.Unmarshal(buf.Bytes(), &envelope))

		data, ok := envelope.Data.(map[string]any)
		require.True(t, ok)

		rows, ok := data["rows"].([]any)
		require.True(t, ok)
		// Only puts pass: PUT 190@0619 (delta=-0.25), PUT 200@0619 (delta=-0.45).
		assert.Len(t, rows, 2, "expected only 2 puts with delta <= 0")

		// Verify filtersApplied includes delta-max=0.
		filters, ok := data["filtersApplied"].([]any)
		require.True(t, ok)
		assert.Contains(t, filters, "delta-max=0")
	})

	t.Run("dte-max zero is honored for 0DTE screening", func(t *testing.T) {
		// Verifies that --dte-max 0 filters to 0DTE only, not treated as "unset".
		// The test chain has expirations far in the future, so --dte-max 0 should
		// exclude everything.

		// Arrange
		server := screenServer(screenTestChain())
		defer server.Close()

		var buf bytes.Buffer
		cmd := NewOptionCmd(testClient(t, server), &buf)

		// Act - dte-max 0 with no matching 0DTE contracts should return empty.
		_, err := runTestCommand(t, cmd, "screen", "AAPL", "--dte-max", "0")

		// Assert
		require.NoError(t, err)

		var envelope output.Envelope
		require.NoError(t, json.Unmarshal(buf.Bytes(), &envelope))

		data, ok := envelope.Data.(map[string]any)
		require.True(t, ok)

		rows, ok := data["rows"].([]any)
		require.True(t, ok)
		assert.Empty(t, rows, "expected no contracts with DTE <= 0 (all expirations are in the future)")

		// Verify filtersApplied includes dte-max=0.
		filters, ok := data["filtersApplied"].([]any)
		require.True(t, ok)
		assert.Contains(t, filters, "dte-max=0")
	})

	t.Run("min-bid filter removes low-premium contracts", func(t *testing.T) {
		// Arrange
		server := screenServer(screenTestChain())
		defer server.Close()

		var buf bytes.Buffer
		cmd := NewOptionCmd(testClient(t, server), &buf)

		// Act - min-bid $6.00 filters out low-bid contracts.
		_, err := runTestCommand(t, cmd, "screen", "AAPL", "--min-bid", "6.0")

		// Assert
		require.NoError(t, err)

		var envelope output.Envelope
		require.NoError(t, json.Unmarshal(buf.Bytes(), &envelope))

		data, ok := envelope.Data.(map[string]any)
		require.True(t, ok)

		rows, ok := data["rows"].([]any)
		require.True(t, ok)
		// Only contracts with bid >= 6.0: 12.0, 5.0->no, 1.50->no, 15.0, 7.0, 3.0->no.
		// Remaining: CALL 200@0619(12.0), CALL 200@0918(15.0), PUT 200@0619(7.0) = 3.
		assert.Len(t, rows, 3, "expected 3 contracts with bid >= 6.0")
	})

	t.Run("max-ask filter caps premium", func(t *testing.T) {
		// Arrange
		server := screenServer(screenTestChain())
		defer server.Close()

		var buf bytes.Buffer
		cmd := NewOptionCmd(testClient(t, server), &buf)

		// Act - max-ask $4.00.
		_, err := runTestCommand(t, cmd, "screen", "AAPL", "--max-ask", "4.0")

		// Assert
		require.NoError(t, err)

		var envelope output.Envelope
		require.NoError(t, json.Unmarshal(buf.Bytes(), &envelope))

		data, ok := envelope.Data.(map[string]any)
		require.True(t, ok)

		rows, ok := data["rows"].([]any)
		require.True(t, ok)
		// Contracts with ask <= 4.0: CALL 220@0619(2.00), PUT 190@0619(3.40) = 2.
		assert.Len(t, rows, 2, "expected 2 contracts with ask <= 4.0")
	})

	t.Run("min-volume filter", func(t *testing.T) {
		// Arrange
		server := screenServer(screenTestChain())
		defer server.Close()

		var buf bytes.Buffer
		cmd := NewOptionCmd(testClient(t, server), &buf)

		// Act - min-volume 1000.
		_, err := runTestCommand(t, cmd, "screen", "AAPL", "--min-volume", "1000")

		// Assert
		require.NoError(t, err)

		var envelope output.Envelope
		require.NoError(t, json.Unmarshal(buf.Bytes(), &envelope))

		data, ok := envelope.Data.(map[string]any)
		require.True(t, ok)

		rows, ok := data["rows"].([]any)
		require.True(t, ok)
		// Contracts with volume >= 1000: CALL 200@0619(1200), PUT 190@0619(1500) = 2.
		assert.Len(t, rows, 2, "expected 2 contracts with volume >= 1000")
	})

	t.Run("min-oi filter", func(t *testing.T) {
		// Arrange
		server := screenServer(screenTestChain())
		defer server.Close()

		var buf bytes.Buffer
		cmd := NewOptionCmd(testClient(t, server), &buf)

		// Act - min-oi 4000.
		_, err := runTestCommand(t, cmd, "screen", "AAPL", "--min-oi", "4000")

		// Assert
		require.NoError(t, err)

		var envelope output.Envelope
		require.NoError(t, json.Unmarshal(buf.Bytes(), &envelope))

		data, ok := envelope.Data.(map[string]any)
		require.True(t, ok)

		rows, ok := data["rows"].([]any)
		require.True(t, ok)
		// Contracts with OI >= 4000: CALL 200@0619(5000), PUT 200@0619(4000), PUT 190@0619(6000) = 3.
		assert.Len(t, rows, 3, "expected 3 contracts with OI >= 4000")
	})

	t.Run("max-spread-pct filter", func(t *testing.T) {
		// Arrange - create a chain with one tight and one wide spread.
		tightWideChain := chainAPIResponse(205.0,
			map[string]map[string][]*models.OptionContract{
				"2026-06-19:42": {
					"200.0": {{
						PutCall: new("CALL"), Symbol: new("AAPL  260619C00200000"),
						ExpirationDate: new("2026-06-19"), StrikePrice: new(200.0),
						// Tight spread: (12.10 - 12.00) / 12.05 = 0.83%.
						Bid: new(12.0), Ask: new(12.10), Mark: new(12.05),
						Delta: new(0.55), Volatility: new(0.28),
						OpenInterest: new(int64(5000)), TotalVolume: new(int64(1200)),
					}},
					"220.0": {{
						PutCall: new("CALL"), Symbol: new("AAPL  260619C00220000"),
						ExpirationDate: new("2026-06-19"), StrikePrice: new(220.0),
						// Wide spread: (2.00 - 0.50) / 1.25 = 120%.
						Bid: new(0.50), Ask: new(2.0), Mark: new(1.25),
						Delta: new(0.10), Volatility: new(0.40),
						OpenInterest: new(int64(100)), TotalVolume: new(int64(10)),
					}},
				},
			},
			nil,
		)
		server := screenServer(tightWideChain)
		defer server.Close()

		var buf bytes.Buffer
		cmd := NewOptionCmd(testClient(t, server), &buf)

		// Act - max-spread-pct 5.0 keeps only the tight spread.
		_, err := runTestCommand(t, cmd, "screen", "AAPL", "--max-spread-pct", "5.0")

		// Assert
		require.NoError(t, err)

		var envelope output.Envelope
		require.NoError(t, json.Unmarshal(buf.Bytes(), &envelope))

		data, ok := envelope.Data.(map[string]any)
		require.True(t, ok)

		rows, ok := data["rows"].([]any)
		require.True(t, ok)
		assert.Len(t, rows, 1, "expected 1 contract with spread <= 5%")
	})

	t.Run("combined filters narrow results", func(t *testing.T) {
		// Arrange
		server := screenServer(screenTestChain())
		defer server.Close()

		var buf bytes.Buffer
		cmd := NewOptionCmd(testClient(t, server), &buf)

		// Act - PUT, delta -0.50 to -0.20, min-bid 2.0, min-oi 1000.
		_, err := runTestCommand(t, cmd, "screen", "AAPL",
			"--type", "PUT",
			"--delta-min", "-0.50",
			"--delta-max", "-0.20",
			"--min-bid", "2.0",
			"--min-oi", "1000",
		)

		// Assert
		require.NoError(t, err)

		var envelope output.Envelope
		require.NoError(t, json.Unmarshal(buf.Bytes(), &envelope))

		data, ok := envelope.Data.(map[string]any)
		require.True(t, ok)

		rows, ok := data["rows"].([]any)
		require.True(t, ok)
		// Both puts pass: PUT 200@0619(delta=-0.45,bid=7.0,OI=4000), PUT 190@0619(delta=-0.25,bid=3.0,OI=6000).
		assert.Len(t, rows, 2, "expected 2 puts matching all filters")

		// Check filtersApplied metadata.
		filters, ok := data["filtersApplied"].([]any)
		require.True(t, ok)
		assert.Contains(t, filters, "type=PUT")
		assert.Contains(t, filters, "delta-min=-0.5")
		assert.Contains(t, filters, "delta-max=-0.2")
		assert.Contains(t, filters, "min-bid=2")
		assert.Contains(t, filters, "min-oi=1000")
	})

	t.Run("limit caps output rows", func(t *testing.T) {
		// Arrange
		server := screenServer(screenTestChain())
		defer server.Close()

		var buf bytes.Buffer
		cmd := NewOptionCmd(testClient(t, server), &buf)

		// Act - limit to 2 rows.
		_, err := runTestCommand(t, cmd, "screen", "AAPL", "--limit", "2")

		// Assert
		require.NoError(t, err)

		var envelope output.Envelope
		require.NoError(t, json.Unmarshal(buf.Bytes(), &envelope))

		data, ok := envelope.Data.(map[string]any)
		require.True(t, ok)

		rows, ok := data["rows"].([]any)
		require.True(t, ok)
		assert.Len(t, rows, 2, "expected 2 rows with --limit 2")
		assert.Equal(t, 2, envelope.Metadata.Returned)
	})

	t.Run("sort by delta descending", func(t *testing.T) {
		// Arrange
		server := screenServer(screenTestChain())
		defer server.Close()

		var buf bytes.Buffer
		cmd := NewOptionCmd(testClient(t, server), &buf)

		// Act - sort by delta descending, limit to 3 to verify ordering.
		_, err := runTestCommand(t, cmd, "screen", "AAPL", "--sort", "delta:desc", "--limit", "3")

		// Assert
		require.NoError(t, err)

		var envelope output.Envelope
		require.NoError(t, json.Unmarshal(buf.Bytes(), &envelope))

		data, ok := envelope.Data.(map[string]any)
		require.True(t, ok)

		rows, ok := data["rows"].([]any)
		require.True(t, ok)
		require.Len(t, rows, 3)

		// Find the delta column index.
		cols, ok := data["columns"].([]any)
		require.True(t, ok)
		deltaIdx := -1
		for i, c := range cols {
			if c == "delta" {
				deltaIdx = i
				break
			}
		}
		require.GreaterOrEqual(t, deltaIdx, 0, "delta column must exist")

		// Verify descending delta order.
		row0, _ := rows[0].([]any)
		row1, _ := rows[1].([]any)
		row2, _ := rows[2].([]any)
		delta0, _ := row0[deltaIdx].(float64)
		delta1, _ := row1[deltaIdx].(float64)
		delta2, _ := row2[deltaIdx].(float64)
		assert.GreaterOrEqual(t, delta0, delta1, "delta should be descending")
		assert.GreaterOrEqual(t, delta1, delta2, "delta should be descending")
	})

	t.Run("custom fields projection", func(t *testing.T) {
		// Arrange
		server := screenServer(screenTestChain())
		defer server.Close()

		var buf bytes.Buffer
		cmd := NewOptionCmd(testClient(t, server), &buf)

		// Act - request only four fields including spreadPct.
		_, err := runTestCommand(t, cmd, "screen", "AAPL", "--fields", "strike,bid,ask,spreadPct")

		// Assert
		require.NoError(t, err)

		var envelope output.Envelope
		require.NoError(t, json.Unmarshal(buf.Bytes(), &envelope))

		data, ok := envelope.Data.(map[string]any)
		require.True(t, ok)

		cols, ok := data["columns"].([]any)
		require.True(t, ok)
		assert.Equal(t, []any{"strike", "bid", "ask", "spreadPct"}, cols)

		rows, ok := data["rows"].([]any)
		require.True(t, ok)
		require.NotEmpty(t, rows)

		// Each row should have exactly 4 values.
		row, ok := rows[0].([]any)
		require.True(t, ok)
		assert.Len(t, row, 4)
	})

	t.Run("mid field aliases mark output", func(t *testing.T) {
		// Arrange
		server := screenServer(screenTestChain())
		defer server.Close()

		var markBuf bytes.Buffer
		markCmd := NewOptionCmd(testClient(t, server), &markBuf)

		var midBuf bytes.Buffer
		midCmd := NewOptionCmd(testClient(t, server), &midBuf)

		// Act
		_, markErr := runTestCommand(t, markCmd, "screen", "AAPL", "--fields", "strike,delta,bid,ask,mark")
		_, midErr := runTestCommand(t, midCmd, "screen", "AAPL", "--fields", "strike,delta,bid,ask,mid")

		// Assert
		require.NoError(t, markErr)
		require.NoError(t, midErr)
		assert.JSONEq(t, markBuf.String(), midBuf.String())

		var envelope output.Envelope
		require.NoError(t, json.Unmarshal(midBuf.Bytes(), &envelope))

		data, ok := envelope.Data.(map[string]any)
		require.True(t, ok)

		cols, ok := data["columns"].([]any)
		require.True(t, ok)
		assert.Equal(t, []any{"strike", "delta", "bid", "ask", "mark"}, cols)
	})

	t.Run("mid and mark fields deduplicate after aliasing", func(t *testing.T) {
		// Arrange
		server := screenServer(screenTestChain())
		defer server.Close()

		var buf bytes.Buffer
		cmd := NewOptionCmd(testClient(t, server), &buf)

		// Act
		_, err := runTestCommand(t, cmd, "screen", "AAPL", "--fields", "strike,mark,mid")

		// Assert
		require.NoError(t, err)

		var envelope output.Envelope
		require.NoError(t, json.Unmarshal(buf.Bytes(), &envelope))

		data, ok := envelope.Data.(map[string]any)
		require.True(t, ok)

		cols, ok := data["columns"].([]any)
		require.True(t, ok)
		assert.Equal(t, []any{"strike", "mark"}, cols)
	})

	t.Run("empty chain returns SymbolNotFoundError", func(t *testing.T) {
		// Arrange
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"symbol":"AAPL","status":"SUCCESS"}`))
		}))
		defer server.Close()

		var buf bytes.Buffer
		cmd := NewOptionCmd(testClient(t, server), &buf)

		// Act
		_, err := runTestCommand(t, cmd, "screen", "AAPL")

		// Assert
		require.Error(t, err)
		var symbolErr *apperr.SymbolNotFoundError
		require.ErrorAs(t, err, &symbolErr)
	})

	t.Run("missing symbol argument", func(t *testing.T) {
		// Arrange
		server := jsonServer(`{}`)
		defer server.Close()

		var buf bytes.Buffer
		cmd := NewOptionCmd(testClient(t, server), &buf)

		// Act
		_, err := runTestCommand(t, cmd, "screen")

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "accepts 1 arg(s), received 0")
	})

	t.Run("invalid field returns validation error", func(t *testing.T) {
		// Arrange
		server := screenServer(screenTestChain())
		defer server.Close()

		var buf bytes.Buffer
		cmd := NewOptionCmd(testClient(t, server), &buf)

		// Act
		_, err := runTestCommand(t, cmd, "screen", "AAPL", "--fields", "strike,bogus")

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "bogus")
	})
}

func TestOptionScreenValidation(t *testing.T) {
	tests := []struct {
		name        string
		opts        optionScreenOpts
		wantErr     bool
		errContains string
	}{
		{
			name:    "valid defaults",
			opts:    optionScreenOpts{},
			wantErr: false,
		},
		{
			name:    "valid type CALL",
			opts:    optionScreenOpts{Type: "CALL"},
			wantErr: false,
		},
		{
			name:        "invalid type",
			opts:        optionScreenOpts{Type: "STRADDLE"},
			wantErr:     true,
			errContains: "invalid type",
		},
		{
			name:        "negative dte-min",
			opts:        optionScreenOpts{DTEMin: -1},
			wantErr:     true,
			errContains: "dte-min must be >= 0",
		},
		{
			name:        "dte-min exceeds dte-max",
			opts:        optionScreenOpts{DTEMin: 60, DTEMax: 30, dteMinSet: true, dteMaxSet: true},
			wantErr:     true,
			errContains: "dte-min (60) must be <= dte-max (30)",
		},
		{
			name:        "delta-min exceeds delta-max",
			opts:        optionScreenOpts{DeltaMin: 0.5, DeltaMax: 0.2, deltaMinSet: true, deltaMaxSet: true},
			wantErr:     true,
			errContains: "delta-min (0.5) must be <= delta-max (0.2)",
		},
		{
			name:    "valid put delta range",
			opts:    optionScreenOpts{DeltaMin: -0.30, DeltaMax: -0.20},
			wantErr: false,
		},
		{
			name:    "valid delta-min zero with delta-max set",
			opts:    optionScreenOpts{DeltaMin: 0, DeltaMax: 0.50, deltaMinSet: true, deltaMaxSet: true},
			wantErr: false,
		},
		{
			name:        "negative min-bid",
			opts:        optionScreenOpts{MinBid: -1.0},
			wantErr:     true,
			errContains: "min-bid must be >= 0",
		},
		{
			name:        "negative limit",
			opts:        optionScreenOpts{Limit: -1},
			wantErr:     true,
			errContains: "limit must be >= 0",
		},
		{
			name:        "invalid sort format",
			opts:        optionScreenOpts{Sort: "delta"},
			wantErr:     true,
			errContains: "sort must be field:direction",
		},
		{
			name:        "invalid sort field",
			opts:        optionScreenOpts{Sort: "bogus:asc"},
			wantErr:     true,
			errContains: "unsupported sort field",
		},
		{
			name:        "invalid sort direction",
			opts:        optionScreenOpts{Sort: "delta:up"},
			wantErr:     true,
			errContains: "sort direction must be asc or desc",
		},
		{
			name:    "valid sort",
			opts:    optionScreenOpts{Sort: "volume:desc"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			errs := validateOptionScreenOpts(&tt.opts)

			// Assert
			if tt.wantErr {
				require.NotEmpty(t, errs, "expected validation errors")
				found := false
				for _, e := range errs {
					if assert.Contains(t, e.Error(), tt.errContains) {
						found = true
					}
				}
				assert.True(t, found, "expected error containing %q", tt.errContains)
			} else {
				assert.Empty(t, errs, "expected no validation errors")
			}
		})
	}
}

func TestComputeSpreadPct(t *testing.T) {
	tests := []struct {
		name     string
		contract *models.OptionContract
		want     float64
	}{
		{
			name: "normal spread",
			contract: &models.OptionContract{
				Bid: new(12.0), Ask: new(12.50), Mark: new(12.25),
			},
			want: 4.08, // (12.50 - 12.00) / 12.25 * 100 = 4.08...
		},
		{
			name: "zero mark",
			contract: &models.OptionContract{
				Bid: new(0.0), Ask: new(0.05), Mark: new(0.0),
			},
			want: 0,
		},
		{
			name:     "nil fields",
			contract: &models.OptionContract{},
			want:     0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computeSpreadPct(tt.contract)
			assert.InDelta(t, tt.want, got, 0.1)
		})
	}
}

func TestScreenSortSplitFlag(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{input: "delta:asc", want: []string{"delta", "asc"}},
		{input: "volume:desc", want: []string{"volume", "desc"}},
		{input: "spreadPct:asc", want: []string{"spreadPct", "asc"}},
		{input: "nodirection", want: []string{"nodirection"}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := splitSortFlag(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}
