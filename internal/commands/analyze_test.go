package commands

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/major/schwab-agent/internal/apperr"
	"github.com/major/schwab-agent/internal/client"
	"github.com/major/schwab-agent/internal/output"
)

const invalidAnalyzeQuoteEntryResponse = `{
	"INVALID": {
		"assetMainType": "EQUITY",
		"symbol": "INVALID",
		"quote": {"lastPrice": 100.0}
	}
}`

// analyzeServer returns an [httptest.Server] that handles both quote and
// price-history requests. quoteBody is keyed by symbol path suffix
// (e.g., "/marketdata/v1/AAPL/quotes"), and candleBody applies to all
// price-history requests.
func analyzeServer(t *testing.T, quoteBody, candleBody string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case strings.Contains(r.URL.Path, "/quotes"):
			_, _ = w.Write([]byte(quoteBody))
		case strings.Contains(r.URL.Path, "/pricehistory"):
			_, _ = w.Write([]byte(candleBody))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func TestNewAnalyzeCmd_Metadata(t *testing.T) {
	// Arrange
	ref := &client.Ref{}
	var buf bytes.Buffer
	cmd := NewAnalyzeCmd(ref, &buf)

	// Assert
	assert.Equal(t, "analyze SYMBOL [SYMBOL...]", cmd.Use)
	assert.Equal(t, "market-data", cmd.GroupID)
	assert.Empty(t, cmd.Aliases, "analyze should not have aliases")
	assert.Contains(t, cmd.Short, "quote")
	assert.Contains(t, cmd.Short, "technical analysis")
}

func TestNewAnalyzeCmd_Flags(t *testing.T) {
	// Arrange
	ref := &client.Ref{}
	var buf bytes.Buffer
	cmd := NewAnalyzeCmd(ref, &buf)

	// Assert - registered flags exist with correct defaults
	intervalFlag := cmd.Flags().Lookup("interval")
	require.NotNil(t, intervalFlag, "--interval flag should be registered")
	assert.Equal(t, "daily", intervalFlag.DefValue)

	pointsFlag := cmd.Flags().Lookup("points")
	require.NotNil(t, pointsFlag, "--points flag should be registered")
	assert.Equal(t, "1", pointsFlag.DefValue)

	latestOnlyFlag := cmd.Flags().Lookup("latest-only")
	require.NotNil(t, latestOnlyFlag, "--latest-only flag should be registered")
	assert.Equal(t, "false", latestOnlyFlag.DefValue)

	compactFlag := cmd.Flags().Lookup("compact")
	require.NotNil(t, compactFlag, "--compact flag should be registered")
	assert.Equal(t, "false", compactFlag.DefValue)
}

func TestNewAnalyzeCmd_NoArgs(t *testing.T) {
	// Arrange
	ref := &client.Ref{}
	var buf bytes.Buffer
	cmd := NewAnalyzeCmd(ref, &buf)

	// Act
	_, err := runTestCommand(t, cmd)

	// Assert - MinimumNArgs(1) triggers an error
	require.Error(t, err)
}

func TestNewAnalyzeCmd_HelpText(t *testing.T) {
	// Arrange
	ref := &client.Ref{}
	var buf bytes.Buffer
	cmd := NewAnalyzeCmd(ref, &buf)

	// Act
	helpOutput, err := runTestCommand(t, cmd, "--help")
	require.NoError(t, err)

	// Assert
	assert.Contains(t, helpOutput, "analyze SYMBOL")
	assert.Contains(t, helpOutput, "quote")
	assert.Contains(t, helpOutput, "ta dashboard")
	assert.Contains(t, helpOutput, "--interval")
	assert.Contains(t, helpOutput, "--points")
	assert.Contains(t, helpOutput, "--latest-only")
	assert.Contains(t, helpOutput, "--compact")
}

func TestNewAnalyzeCmd_SingleSymbol(t *testing.T) {
	// Arrange - server handles both quote and price-history endpoints
	quoteJSON := aaplQuoteEntryResponse
	candleJSON := mockCandleListJSON("AAPL", 252)
	srv := analyzeServer(t, quoteJSON, candleJSON)
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewAnalyzeCmd(testClientWithMarketData(t, srv), &buf)
	_, err := runTestCommand(t, cmd, "AAPL")
	require.NoError(t, err)

	// Assert - envelope shape: {"data": {"AAPL": {"quote": {...}, "analysis": {...}}}}
	var envelope output.Envelope
	require.NoError(t, json.Unmarshal(buf.Bytes(), &envelope))
	assert.NotEmpty(t, envelope.Metadata.Timestamp)
	assert.Empty(t, envelope.Errors)

	data, ok := envelope.Data.(map[string]any)
	require.True(t, ok, "data should be a map")
	require.Contains(t, data, "AAPL")

	symbolData, ok := data["AAPL"].(map[string]any)
	require.True(t, ok, "AAPL entry should be a map")
	assert.NotNil(t, symbolData["quote"], "quote field should be present")
	assert.NotNil(t, symbolData["analysis"], "analysis field should be present")

	// Verify analysis contains dashboard fields
	analysis, ok := symbolData["analysis"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "dashboard", analysis["indicator"])
	assert.Equal(t, "AAPL", analysis["symbol"])
}

func TestNewAnalyzeCmd_MultiSymbol(t *testing.T) {
	// Arrange - server returns quote data for both symbols from one batched
	// /quotes request and candle data for each price-history request.
	quoteRequests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.URL.Path == "/marketdata/v1/quotes":
			quoteRequests++
			assert.ElementsMatch(t, []string{"AAPL", "NVDA"}, strings.Split(r.URL.Query().Get("symbols"), ","))
			_, _ = w.Write([]byte(`{
				"AAPL":{"assetMainType":"EQUITY","symbol":"AAPL","quote":{"lastPrice":150.0}},
				"NVDA":{"assetMainType":"EQUITY","symbol":"NVDA","quote":{"lastPrice":800.0}}
			}`))
		case strings.Contains(r.URL.Path, "/quotes"):
			t.Fatalf("analyze should batch multi-symbol quotes, got path %s", r.URL.Path)
		case strings.Contains(r.URL.Path, "/pricehistory"):
			// Determine symbol from query param
			sym := r.URL.Query().Get("symbol")
			if sym == "" {
				sym = "AAPL"
			}
			_, _ = w.Write([]byte(mockCandleListJSON(sym, 252)))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewAnalyzeCmd(testClientWithMarketData(t, srv), &buf)
	_, err := runTestCommand(t, cmd, "AAPL", "NVDA")
	require.NoError(t, err)

	// Assert - both symbols in data map
	var envelope output.Envelope
	require.NoError(t, json.Unmarshal(buf.Bytes(), &envelope))
	assert.Empty(t, envelope.Errors)
	assert.Equal(t, 1, quoteRequests, "multi-symbol analyze should fetch quotes once")

	data, ok := envelope.Data.(map[string]any)
	require.True(t, ok)
	assert.Contains(t, data, "AAPL")
	assert.Contains(t, data, "NVDA")

	// Verify each symbol has quote + analysis
	for _, sym := range []string{"AAPL", "NVDA"} {
		symbolData, symbolOK := data[sym].(map[string]any)
		require.True(t, symbolOK, "%s entry should be a map", sym)
		assert.NotNil(t, symbolData["quote"], "%s quote should be present", sym)
		assert.NotNil(t, symbolData["analysis"], "%s analysis should be present", sym)
	}
}

func TestNewAnalyzeCmd_MultiSymbolFetchesTAConcurrently(t *testing.T) {
	// Arrange - quote batching happens before TA, then price-history requests
	// should overlap instead of making each symbol wait for the previous one.
	var activeHistoryRequests atomic.Int64
	var maxActiveHistoryRequests atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.URL.Path == "/marketdata/v1/quotes":
			_, _ = w.Write([]byte(`{
				"AAPL":{"assetMainType":"EQUITY","symbol":"AAPL","quote":{"lastPrice":150.0}},
				"MSFT":{"assetMainType":"EQUITY","symbol":"MSFT","quote":{"lastPrice":410.0}},
				"NVDA":{"assetMainType":"EQUITY","symbol":"NVDA","quote":{"lastPrice":800.0}}
			}`))
		case strings.Contains(r.URL.Path, "/pricehistory"):
			active := activeHistoryRequests.Add(1)
			for {
				maxActive := maxActiveHistoryRequests.Load()
				if active <= maxActive || maxActiveHistoryRequests.CompareAndSwap(maxActive, active) {
					break
				}
			}
			defer activeHistoryRequests.Add(-1)

			// Hold the handler briefly so parallel requests are observable. If analyze
			// regresses to sequential TA, maxActiveHistoryRequests remains 1.
			time.Sleep(25 * time.Millisecond)
			sym := r.URL.Query().Get("symbol")
			_, _ = w.Write([]byte(mockCandleListJSON(sym, 252)))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewAnalyzeCmd(testClientWithMarketData(t, srv), &buf)
	_, err := runTestCommand(t, cmd, "AAPL", "MSFT", "NVDA")
	require.NoError(t, err)

	// Assert
	assert.Greater(t, maxActiveHistoryRequests.Load(), int64(1), "multi-symbol analyze should overlap TA requests")
}

func TestNewAnalyzeCmd_LatestOnlyOmitsDashboardValues(t *testing.T) {
	// Arrange
	srv := analyzeServer(t, aaplQuoteEntryResponse, mockCandleListJSON("AAPL", 252))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewAnalyzeCmd(testClientWithMarketData(t, srv), &buf)
	_, err := runTestCommand(t, cmd, "AAPL", "--latest-only")
	require.NoError(t, err)

	// Assert
	var envelope output.Envelope
	require.NoError(t, json.Unmarshal(buf.Bytes(), &envelope))
	data, ok := envelope.Data.(map[string]any)
	require.True(t, ok, "data should be a map")
	symbolData, ok := data["AAPL"].(map[string]any)
	require.True(t, ok, "AAPL entry should be a map")
	analysis, ok := symbolData["analysis"].(map[string]any)
	require.True(t, ok, "analysis should be a map")
	assert.NotNil(t, analysis["latest"], "latest should still be present")
	assert.NotContains(t, analysis, "values", "latest-only should omit duplicate values row")
}

func TestNewAnalyzeCmd_CompactOutput(t *testing.T) {
	// Arrange
	quoteJSON := `{"AAPL":{"assetMainType":"EQUITY","symbol":"AAPL","quote":{"bidPrice":149.9,"askPrice":150.1,"lastPrice":150.0,"mark":150.0,"netPercentChange":1.25,"totalVolume":1234567,"quoteTime":1710000000000}}}`
	srv := analyzeServer(t, quoteJSON, mockCandleListJSON("AAPL", 252))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewAnalyzeCmd(testClientWithMarketData(t, srv), &buf)
	_, err := runTestCommand(t, cmd, "AAPL", "--compact")
	require.NoError(t, err)

	// Assert
	var envelope output.Envelope
	require.NoError(t, json.Unmarshal(buf.Bytes(), &envelope))
	data, ok := envelope.Data.(map[string]any)
	require.True(t, ok, "data should be a map")
	symbolData, ok := data["AAPL"].(map[string]any)
	require.True(t, ok, "AAPL entry should be a compact map")

	quote, ok := symbolData["quote"].(map[string]any)
	require.True(t, ok, "compact quote should be a map")
	assert.Equal(t, "AAPL", quote["symbol"])
	assert.InDelta(t, 149.9, quote["bid"], 0.001)
	assert.InDelta(t, 150.1, quote["ask"], 0.001)
	assert.InDelta(t, 150.0, quote["last"], 0.001)
	assert.InDelta(t, 1.25, quote["netPercentChange"], 0.001)
	assert.InDelta(t, 1234567, quote["volume"], 0.001)

	technical, ok := symbolData["technical"].(map[string]any)
	require.True(t, ok, "compact technical fields should be present")
	assert.Contains(t, technical, "close")
	assert.Contains(t, technical, "distance_from_sma_200_percent")
	assert.Contains(t, technical, "rsi_14")
	assert.Contains(t, technical, "atr_percent")
	assert.Contains(t, technical, "relative_volume")

	signals, ok := symbolData["signals"].(map[string]any)
	require.True(t, ok, "compact signals should be present")
	assert.Contains(t, signals, "trend")
	assert.Contains(t, signals, "momentum")
	assert.NotContains(t, symbolData, "analysis", "compact output should omit full dashboard")
}

func TestNewAnalyzeCmd_PartialFailure_TAFails(t *testing.T) {
	// Arrange - test partial failure: AAPL succeeds completely, INVALID has quote
	// succeed but TA fail (empty candles). This verifies that partial data (quote
	// present, analysis nil) is included in the output even when there's an error.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.URL.Path == "/marketdata/v1/quotes":
			// Quotes succeed for both symbols in one batch.
			_, _ = w.Write([]byte(`{
				"AAPL":{"assetMainType":"EQUITY","symbol":"AAPL","quote":{"lastPrice":150.0}},
				"INVALID":{"assetMainType":"EQUITY","symbol":"INVALID","quote":{"lastPrice":100.0}}
			}`))
		case strings.Contains(r.URL.Path, "/pricehistory"):
			// Return valid candles for AAPL, empty for INVALID (TA will fail)
			sym := r.URL.Query().Get("symbol")
			if sym == "INVALID" {
				_, _ = w.Write([]byte(`{"symbol":"INVALID","empty":true,"candles":[]}`))
			} else {
				_, _ = w.Write([]byte(mockCandleListJSON("AAPL", 252)))
			}
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewAnalyzeCmd(testClientWithMarketData(t, srv), &buf)
	_, err := runTestCommand(t, cmd, "AAPL", "INVALID")
	require.NoError(t, err)

	// Assert - partial response: AAPL succeeds, INVALID has partial data (quote present, analysis nil) + error
	var envelope output.Envelope
	require.NoError(t, json.Unmarshal(buf.Bytes(), &envelope))

	data, ok := envelope.Data.(map[string]any)
	require.True(t, ok)
	assert.Contains(t, data, "AAPL", "AAPL should be in partial data")

	// INVALID should also be in data with partial result (quote succeeded, TA failed)
	assert.Contains(t, data, "INVALID", "INVALID should be in data with partial result")
	invalidData, ok := data["INVALID"].(map[string]any)
	require.True(t, ok, "INVALID entry should be a map")
	// Quote should be present, analysis should be nil (TA failed on empty candles)
	assert.NotNil(t, invalidData["quote"], "INVALID quote should be present (quote succeeded)")
	assert.Nil(t, invalidData["analysis"], "INVALID analysis should be nil (TA failed on empty candles)")

	// Errors should be reported for INVALID
	require.NotEmpty(t, envelope.Errors, "should have partial errors")
	assert.Equal(t, 2, envelope.Metadata.Requested)
	assert.Equal(t, 2, envelope.Metadata.Returned)
}

func TestNewAnalyzeCmd_SingleSymbolTAInsufficientCandles(t *testing.T) {
	// Arrange - issue #150: a young symbol can have a valid quote but fewer
	// candles than ta dashboard needs. analyze should keep the quote and report a
	// partial failure instead of returning a command-level validation error.
	srv := analyzeServer(t, invalidAnalyzeQuoteEntryResponse, mockCandleListJSON("INVALID", 190))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewAnalyzeCmd(testClientWithMarketData(t, srv), &buf)
	_, err := runTestCommand(t, cmd, "INVALID")
	require.NoError(t, err)

	// Assert
	var envelope output.Envelope
	require.NoError(t, json.Unmarshal(buf.Bytes(), &envelope))

	data, ok := envelope.Data.(map[string]any)
	require.True(t, ok, "data should be a map")
	require.Contains(t, data, "INVALID")

	symbolData, ok := data["INVALID"].(map[string]any)
	require.True(t, ok, "INVALID entry should be a map")
	assert.NotNil(t, symbolData["quote"], "quote should be present when quote fetch succeeds")
	assert.Nil(t, symbolData["analysis"], "analysis should be nil when TA lacks enough candles")

	require.Len(t, envelope.Errors, 1)
	assert.Contains(t, envelope.Errors[0], "INVALID")
	assert.Contains(t, envelope.Errors[0], "dashboard requires at least 252 candles, got 190")
	assert.Equal(t, 1, envelope.Metadata.Requested)
	assert.Equal(t, 1, envelope.Metadata.Returned)
}

func TestNewAnalyzeCmd_QuoteHTTPNotFound(t *testing.T) {
	// Arrange - schwab-go turns a 404 quote response into an API error. analyze
	// should preserve the existing quote command behavior by mapping that to the
	// user-facing SymbolNotFoundError type.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case strings.Contains(r.URL.Path, "/quotes"):
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"symbol not found"}`))
		case strings.Contains(r.URL.Path, "/pricehistory"):
			_, _ = w.Write([]byte(mockCandleListJSON("MISSING", 252)))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewAnalyzeCmd(testClientWithMarketData(t, srv), &buf)
	_, err := runTestCommand(t, cmd, "MISSING")

	// Assert
	var symErr *apperr.SymbolNotFoundError
	require.ErrorAs(t, err, &symErr)
}

func TestNewAnalyzeCmd_QuoteResponseMissingSymbol(t *testing.T) {
	// Arrange - Schwab can return a successful quote envelope that still omits
	// the requested symbol. Treat that as the same symbol-not-found condition as
	// quote.go rather than returning a nil quote with successful analysis.
	srv := analyzeServer(
		t,
		`{"OTHER":{"assetMainType":"EQUITY","symbol":"OTHER","quote":{"lastPrice":1.0}}}`,
		mockCandleListJSON("MISSING", 252),
	)
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewAnalyzeCmd(testClientWithMarketData(t, srv), &buf)
	_, err := runTestCommand(t, cmd, "MISSING")

	// Assert
	var symErr *apperr.SymbolNotFoundError
	require.ErrorAs(t, err, &symErr)
}

func TestNewAnalyzeCmd_SingleSymbolQuoteFailsTASucceeds(t *testing.T) {
	// Arrange - single-symbol analyze should preserve successful TA output when a
	// non-symbol quote failure happens. This matches multi-symbol partial output
	// behavior without weakening the SymbolNotFoundError hard-fail cases above.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case strings.Contains(r.URL.Path, "/quotes"):
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"message":"temporary quote failure"}`))
		case strings.Contains(r.URL.Path, "/pricehistory"):
			_, _ = w.Write([]byte(mockCandleListJSON("AAPL", 252)))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewAnalyzeCmd(testClientWithMarketData(t, srv), &buf)
	_, err := runTestCommand(t, cmd, "AAPL")
	require.NoError(t, err)

	// Assert
	var envelope output.Envelope
	require.NoError(t, json.Unmarshal(buf.Bytes(), &envelope))

	data, ok := envelope.Data.(map[string]any)
	require.True(t, ok, "data should be a map")
	require.Contains(t, data, "AAPL")

	symbolData, ok := data["AAPL"].(map[string]any)
	require.True(t, ok, "AAPL entry should be a map")
	assert.Nil(t, symbolData["quote"], "quote should be nil when quote fetch fails")
	assert.NotNil(t, symbolData["analysis"], "analysis should be present when TA succeeds")

	require.Len(t, envelope.Errors, 1)
	assert.Contains(t, envelope.Errors[0], "AAPL")
	assert.Contains(t, envelope.Errors[0], "quote failed")
	assert.Equal(t, 1, envelope.Metadata.Requested)
	assert.Equal(t, 1, envelope.Metadata.Returned)
}

func TestNewAnalyzeCmd_SingleSymbolBothFail(t *testing.T) {
	// Arrange - when neither quote nor TA produces usable data, analyze should
	// return the command-level error instead of writing an empty partial envelope.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case strings.Contains(r.URL.Path, "/quotes"):
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"message":"temporary quote failure"}`))
		case strings.Contains(r.URL.Path, "/pricehistory"):
			_, _ = w.Write([]byte(`{"symbol":"AAPL","empty":true,"candles":[]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewAnalyzeCmd(testClientWithMarketData(t, srv), &buf)
	_, err := runTestCommand(t, cmd, "AAPL")

	// Assert
	var httpErr *apperr.HTTPError
	require.ErrorAs(t, err, &httpErr)
	assert.Empty(t, buf.String(), "analyze should not write a partial envelope when both data sources fail")
}

func TestNewAnalyzeCmd_InvalidInterval(t *testing.T) {
	// Arrange
	ref := &client.Ref{}
	var buf bytes.Buffer
	cmd := NewAnalyzeCmd(ref, &buf)

	// Act
	_, err := runTestCommand(t, cmd, "--interval", "bogus", "AAPL")

	// Assert - normalizeFlagValidationErrorFunc wraps invalid enum values
	var valErr *apperr.ValidationError
	require.ErrorAs(t, err, &valErr)
	assert.Contains(t, valErr.Error(), "bogus")
}
