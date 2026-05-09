package commands

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
	// Arrange - server returns quote data for both symbols and candle data
	// for price-history requests. The quote endpoint path for multi-analyze
	// is /marketdata/v1/SYMBOL/quotes (called per-symbol).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case strings.Contains(r.URL.Path, "AAPL/quotes"):
			_, _ = w.Write([]byte(aaplQuoteEntryResponse))
		case strings.Contains(r.URL.Path, "NVDA/quotes"):
			_, _ = w.Write([]byte(`{"NVDA":{"assetMainType":"EQUITY","symbol":"NVDA","quote":{"lastPrice":800.0}}}`))
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

func TestNewAnalyzeCmd_PartialFailure_TAFails(t *testing.T) {
	// Arrange - test partial failure: AAPL succeeds completely, INVALID has quote
	// succeed but TA fail (empty candles). This verifies that partial data (quote
	// present, analysis nil) is included in the output even when there's an error.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case strings.Contains(r.URL.Path, "AAPL/quotes"):
			_, _ = w.Write([]byte(aaplQuoteEntryResponse))
		case strings.Contains(r.URL.Path, "INVALID/quotes"):
			// Quote succeeds for INVALID
			_, _ = w.Write([]byte(invalidAnalyzeQuoteEntryResponse))
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
