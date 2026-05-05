package commands

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	structclierrors "github.com/leodido/structcli/errors"

	"github.com/major/schwab-agent/internal/apperr"
	"github.com/major/schwab-agent/internal/output"
)

// mockCandleListJSON builds a CandleList JSON string with n candles.
// Close prices ramp from 100.0 to 100+n-1, giving predictable indicator output.
func mockCandleListJSON(symbol string, n int) string {
	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	candles := make([]string, n)
	for i := range n {
		dt := base.AddDate(0, 0, i).Format(time.RFC3339)
		price := 100.0 + float64(i)
		candles[i] = fmt.Sprintf(
			`{"open":%.1f,"high":%.1f,"low":%.1f,"close":%.1f,"volume":1000000,"datetimeISO8601":%q}`,
			price-0.5, price+1.0, price-1.0, price, dt,
		)
	}
	return fmt.Sprintf(`{"symbol":%q,"empty":false,"candles":[%s]}`, symbol, strings.Join(candles, ","))
}

// mockFlatCandleListJSON builds candles with no price movement. Flat histories
// produce valid all-zero MACD/ATR-style outputs after warm-up, which guards the
// dashboard against confusing "all zeros" with "no indicator data".
func mockFlatCandleListJSON(symbol string, n int) string {
	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	candles := make([]string, n)
	for i := range n {
		dt := base.AddDate(0, 0, i).Format(time.RFC3339)
		candles[i] = fmt.Sprintf(
			`{"open":100.0,"high":100.0,"low":100.0,"close":100.0,"volume":1000000,"datetimeISO8601":%q}`,
			dt,
		)
	}
	return fmt.Sprintf(`{"symbol":%q,"empty":false,"candles":[%s]}`, symbol, strings.Join(candles, ","))
}

// decodeTAEnvelope unmarshals the output buffer and returns the single symbol's
// payload for tests that focus on indicator content rather than envelope shape.
func decodeTAEnvelope(t *testing.T, buf *bytes.Buffer) (envelope output.Envelope, data map[string]any) {
	t.Helper()
	envelope, root := decodeTAEnvelopeRoot(t, buf)
	if len(root) == 1 {
		for _, raw := range root {
			entry, ok := raw.(map[string]any)
			if ok {
				return envelope, entry
			}
		}
	}

	return envelope, root
}

// decodeTAEnvelopeRoot unmarshals the output buffer into an Envelope and
// extracts the root data map without unwrapping symbol-keyed entries.
func decodeTAEnvelopeRoot(t *testing.T, buf *bytes.Buffer) (envelope output.Envelope, data map[string]any) {
	t.Helper()
	require.NoError(t, json.Unmarshal(buf.Bytes(), &envelope))

	dataBytes, err := json.Marshal(envelope.Data)
	require.NoError(t, err)

	require.NoError(t, json.Unmarshal(dataBytes, &data))
	return envelope, data
}

// mockVariedCandleListJSON builds a CandleList with price reversals so that both
// PlusDI and MinusDI produce non-zero values. Monotonic mock data gives MinusDI=0
// everywhere, which causes StripLeadingZeros to return an empty slice and breaks ADX.
func mockVariedCandleListJSON(symbol string, n int) string {
	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	candles := make([]string, n)
	for i := range n {
		dt := base.AddDate(0, 0, i).Format(time.RFC3339)
		// General uptrend with periodic pullbacks every 3rd candle
		price := 100.0 + float64(i)*0.5
		if i%3 == 0 {
			price -= 2.0
		}
		high := price + 1.5
		low := price - 1.5
		candles[i] = fmt.Sprintf(
			`{"open":%.1f,"high":%.1f,"low":%.1f,"close":%.1f,"volume":1000000,"datetimeISO8601":%q}`,
			price-0.3, high, low, price, dt,
		)
	}
	return fmt.Sprintf(`{"symbol":%q,"empty":false,"candles":[%s]}`, symbol, strings.Join(candles, ","))
}

// priceHistoryHandler returns an http.Handler that responds with mock candle data.
// Validates the request path contains /marketdata/v1/pricehistory.
func priceHistoryHandler(t *testing.T, symbol string, nCandles int) http.Handler {
	t.Helper()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/marketdata/v1/pricehistory")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(mockCandleListJSON(symbol, nCandles)))
	})
}

// mockOptionChainJSON builds a realistic option chain JSON for testing.
// underlying=150.0, ATM strike=150.0, call Mark=5.0, put Mark=4.5.
func mockOptionChainJSON(symbol string) string {
	return fmt.Sprintf(`{
		"symbol": %q,
		"underlying": {
			"mark": 150.0,
			"last": 149.5,
			"bid": 149.0,
			"ask": 150.0
		},
		"underlyingPrice": 150.0,
		"callExpDateMap": {
			"2025-06-20:30": {
				"148.0": [{"mark": 7.0, "bid": 6.9, "ask": 7.1, "strikePrice": 148.0, "daysToExpiration": 30, "inTheMoney": true}],
				"150.0": [{"mark": 5.0, "bid": 4.9, "ask": 5.1, "strikePrice": 150.0, "daysToExpiration": 30, "inTheMoney": false}],
				"152.0": [{"mark": 3.0, "bid": 2.9, "ask": 3.1, "strikePrice": 152.0, "daysToExpiration": 30, "inTheMoney": false}]
			}
		},
		"putExpDateMap": {
			"2025-06-20:30": {
				"148.0": [{"mark": 2.5, "bid": 2.4, "ask": 2.6, "strikePrice": 148.0, "daysToExpiration": 30, "inTheMoney": false}],
				"150.0": [{"mark": 4.5, "bid": 4.4, "ask": 4.6, "strikePrice": 150.0, "daysToExpiration": 30, "inTheMoney": false}],
				"152.0": [{"mark": 6.5, "bid": 6.4, "ask": 6.6, "strikePrice": 152.0, "daysToExpiration": 30, "inTheMoney": true}]
			}
		}
	}`, symbol)
}

// mockOptionChainTwoDTEJSON builds a chain with two expirations.
func mockOptionChainTwoDTEJSON(symbol string) string {
	return fmt.Sprintf(`{
		"symbol": %q,
		"underlying": {"mark": 150.0},
		"underlyingPrice": 150.0,
		"callExpDateMap": {
			"2025-06-20:30": {
				"150.0": [{"mark": 5.0, "bid": 4.9, "ask": 5.1, "strikePrice": 150.0, "daysToExpiration": 30}]
			},
			"2025-07-18:60": {
				"150.0": [{"mark": 7.0, "bid": 6.9, "ask": 7.1, "strikePrice": 150.0, "daysToExpiration": 60}]
			}
		},
		"putExpDateMap": {
			"2025-06-20:30": {
				"150.0": [{"mark": 4.5, "bid": 4.4, "ask": 4.6, "strikePrice": 150.0, "daysToExpiration": 30}]
			},
			"2025-07-18:60": {
				"150.0": [{"mark": 6.5, "bid": 6.4, "ask": 6.6, "strikePrice": 150.0, "daysToExpiration": 60}]
			}
		}
	}`, symbol)
}

// mockOptionChainNilMarkJSON builds a chain where ATM options need bid/ask midpoint fallback.
func mockOptionChainNilMarkJSON(symbol string) string {
	return fmt.Sprintf(`{
		"symbol": %q,
		"underlying": {"mark": 150.0},
		"underlyingPrice": 150.0,
		"callExpDateMap": {
			"2025-06-20:30": {
				"150.0": [{"bid": 4.9, "ask": 5.1, "strikePrice": 150.0, "daysToExpiration": 30}]
			}
		},
		"putExpDateMap": {
			"2025-06-20:30": {
				"150.0": [{"bid": 4.4, "ask": 4.6, "strikePrice": 150.0, "daysToExpiration": 30}]
			}
		}
	}`, symbol)
}

// mockEmptyOptionChainJSON builds a chain with no expirations.
func mockEmptyOptionChainJSON(symbol string) string {
	return fmt.Sprintf(`{
		"symbol": %q,
		"underlying": {"mark": 150.0},
		"underlyingPrice": 150.0,
		"callExpDateMap": {},
		"putExpDateMap": {}
	}`, symbol)
}

// optionChainHandler returns an http.Handler that responds with mock option chain data.
func optionChainHandler(t *testing.T, body string) http.Handler {
	t.Helper()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/marketdata/v1/chains")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	})
}

func TestTADashboard_ValidEnvelope(t *testing.T) {
	// Arrange: dashboard needs 252 candles for its long-range price context and
	// SMA 200 while still making only one price-history request.
	requestCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		assert.Contains(t, r.URL.Path, "/marketdata/v1/pricehistory")
		assert.Equal(t, "AAPL", r.URL.Query().Get("symbol"))
		assert.Equal(t, "2", r.URL.Query().Get("period"), "252 daily candles need a buffered request")

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(mockCandleListJSON("AAPL", 252)))
	}))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewTACmd(testClient(t, srv), &buf)
	_, err := runTestCommand(t, cmd, "dashboard", "AAPL")
	require.NoError(t, err)

	// Assert
	envelope, data := decodeTAEnvelope(t, &buf)
	assert.NotEmpty(t, envelope.Metadata.Timestamp)
	assert.Equal(t, 1, requestCount, "dashboard should fetch price history once")
	assert.Equal(t, "dashboard", data["indicator"])
	assert.Equal(t, "AAPL", data["symbol"])
	assert.Equal(t, "daily", data["interval"])

	parameters, ok := data["parameters"].(map[string]any)
	require.True(t, ok, "parameters should be an object")
	assert.Equal(t, []any{float64(21), float64(50), float64(200)}, parameters["sma_periods"])
	assert.InDelta(t, 14, parameters["rsi_period"], 0.1)
	assert.InDelta(t, 12, parameters["macd_fast"], 0.1)
	assert.InDelta(t, 26, parameters["macd_slow"], 0.1)
	assert.InDelta(t, 9, parameters["macd_signal"], 0.1)
	assert.InDelta(t, 20, parameters["bbands_period"], 0.1)
	assert.InDelta(t, 252, parameters["long_range"], 0.1)

	latest, ok := data["latest"].(map[string]any)
	require.True(t, ok, "latest should be an object")
	for _, key := range []string{
		"datetime", "open", "high", "low", "close", "volume",
		"sma_21", "sma_50", "sma_200", "rsi_14", "macd", "macd_signal", "macd_histogram",
		"atr_14", "atr_percent", "bbands_upper", "bbands_middle", "bbands_lower",
		"avg_volume_20", "relative_volume", "range_20_high", "range_20_low", "range_252_high", "range_252_low",
		"distance_from_sma_21_percent", "distance_from_sma_50_percent", "distance_from_sma_200_percent",
	} {
		assert.Contains(t, latest, key)
	}

	signals, ok := data["signals"].(map[string]any)
	require.True(t, ok, "signals should be an object")
	assert.Contains(t, signals, "trend")
	assert.Contains(t, signals, "momentum")
	assert.Contains(t, signals, "volatility")
	assert.Contains(t, signals, "volume")
	assert.Contains(t, signals, "close_above_sma_200")
	assert.Contains(t, signals, "macd_histogram_positive")

	values, ok := data["values"].([]any)
	require.True(t, ok, "values should be an array")
	require.Len(t, values, 1, "dashboard defaults to the latest point")
	assert.Equal(t, latest, values[0].(map[string]any), "default values row should match latest")
}

func TestTADashboard_DailyLongRangeUsesBufferedHistoryRequest(t *testing.T) {
	// Arrange: Schwab can return only 251 candles for a 1-year daily request, so
	// dashboard must request a wider period while still accepting exactly the 252
	// candles needed for a complete long-range row.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/marketdata/v1/pricehistory")
		assert.Equal(t, "GNRC", r.URL.Query().Get("symbol"))
		assert.Equal(t, "year", r.URL.Query().Get("periodType"))
		assert.Equal(t, "2", r.URL.Query().Get("period"))
		assert.Equal(t, "daily", r.URL.Query().Get("frequencyType"))
		assert.Equal(t, "1", r.URL.Query().Get("frequency"))

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(mockCandleListJSON("GNRC", 252)))
	}))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewTACmd(testClient(t, srv), &buf)
	_, err := runTestCommand(t, cmd, "dashboard", "GNRC", "--points", "1")
	require.NoError(t, err)

	// Assert
	_, data := decodeTAEnvelope(t, &buf)
	values, ok := data["values"].([]any)
	require.True(t, ok, "values should be an array")
	require.Len(t, values, 1)
}

func TestTADashboard_SingleSymbolReturnsSymbolMap(t *testing.T) {
	// Arrange: every TA command uses the same symbol-keyed shape, even when only
	// one symbol is requested, so LLM callers do not need one-symbol branching.
	srv := httptest.NewServer(priceHistoryHandler(t, "AAPL", 252))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewTACmd(testClient(t, srv), &buf)
	_, err := runTestCommand(t, cmd, "dashboard", "AAPL")
	require.NoError(t, err)

	// Assert
	_, data := decodeTAEnvelopeRoot(t, &buf)
	entry, ok := data["AAPL"].(map[string]any)
	require.True(t, ok, "single-symbol dashboard should be keyed by symbol")
	assert.Equal(t, "dashboard", entry["indicator"])
	assert.Equal(t, "AAPL", entry["symbol"])
}

func TestTADashboard_PointsFlag(t *testing.T) {
	// Arrange
	srv := httptest.NewServer(priceHistoryHandler(t, "MSFT", 254))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewTACmd(testClient(t, srv), &buf)
	_, err := runTestCommand(t, cmd, "dashboard", "--points", "3", "MSFT")
	require.NoError(t, err)

	// Assert
	_, data := decodeTAEnvelope(t, &buf)
	assert.Equal(t, "dashboard", data["indicator"])
	assert.Equal(t, "MSFT", data["symbol"])

	values, ok := data["values"].([]any)
	require.True(t, ok, "values should be an array")
	require.Len(t, values, 3, "--points 3 should return three dashboard rows")

	latest, ok := data["latest"].(map[string]any)
	require.True(t, ok, "latest should be an object")
	assert.Equal(t, latest, values[2].(map[string]any), "latest should match the final limited row")

	first := values[0].(map[string]any)
	assert.InDelta(t, 352.0, first["range_252_high"], 0.1, "first row should use candles 0-251")
	assert.InDelta(t, 99.0, first["range_252_low"], 0.1, "first row should use candles 0-251")
	assert.InDelta(t, 354.0, latest["range_252_high"], 0.1, "latest row should use candles 2-253")
	assert.InDelta(t, 101.0, latest["range_252_low"], 0.1, "latest row should use candles 2-253")
}

func TestTADashboard_FlatHistoryDoesNotPanic(t *testing.T) {
	// Arrange
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/marketdata/v1/pricehistory")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(mockFlatCandleListJSON("FLAT", 252)))
	}))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewTACmd(testClient(t, srv), &buf)
	_, err := runTestCommand(t, cmd, "dashboard", "FLAT")
	require.NoError(t, err)

	// Assert
	_, data := decodeTAEnvelope(t, &buf)
	latest, ok := data["latest"].(map[string]any)
	require.True(t, ok, "latest should be an object")
	assert.InDelta(t, 0.0, latest["macd"], 0.1)
	assert.InDelta(t, 0.0, latest["macd_signal"], 0.1)
	assert.InDelta(t, 0.0, latest["macd_histogram"], 0.1)
	assert.InDelta(t, 0.0, latest["atr_14"], 0.1)
	assert.InDelta(t, 100.0, latest["range_252_high"], 0.1)
	assert.InDelta(t, 100.0, latest["range_252_low"], 0.1)
}

func TestTADashboard_InsufficientHistory(t *testing.T) {
	// Arrange
	srv := httptest.NewServer(priceHistoryHandler(t, "AAPL", 100))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewTACmd(testClient(t, srv), &buf)
	_, err := runTestCommand(t, cmd, "dashboard", "AAPL")

	// Assert
	require.Error(t, err)
	var validationErr *apperr.ValidationError
	assert.ErrorAs(t, err, &validationErr)
}

func TestTASMA_ValidEnvelope(t *testing.T) {
	// Arrange
	srv := httptest.NewServer(priceHistoryHandler(t, "AAPL", 80))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewTACmd(testClient(t, srv), &buf)
	_, err := runTestCommand(t, cmd, "sma", "AAPL")
	require.NoError(t, err)

	// Assert
	envelope, data := decodeTAEnvelope(t, &buf)
	assert.NotEmpty(t, envelope.Metadata.Timestamp)
	assert.Equal(t, "sma", data["indicator"])
	assert.Equal(t, "AAPL", data["symbol"])
	assert.Equal(t, "daily", data["interval"])

	// Default period is 20
	assert.InDelta(t, 20, data["period"], 0.1)

	values, ok := data["values"].([]any)
	require.True(t, ok, "values should be an array")
	assert.Len(t, values, 1, "TA time-series commands default to the latest point")

	// Each value entry should have datetime and sma keys
	first := values[0].(map[string]any)
	assert.Contains(t, first, "datetime")
	assert.Contains(t, first, "sma")
}

func TestTASMA_SingleSymbolReturnsSymbolMap(t *testing.T) {
	// Arrange: simple indicators use the same normalized root as the dashboard
	// and batch requests so agents can always read data.<SYMBOL>.
	srv := httptest.NewServer(priceHistoryHandler(t, "AAPL", 80))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewTACmd(testClient(t, srv), &buf)
	_, err := runTestCommand(t, cmd, "sma", "AAPL")
	require.NoError(t, err)

	// Assert
	_, data := decodeTAEnvelopeRoot(t, &buf)
	entry, ok := data["AAPL"].(map[string]any)
	require.True(t, ok, "single-symbol sma should be keyed by symbol")
	assert.Equal(t, "sma", entry["indicator"])
	assert.Equal(t, "AAPL", entry["symbol"])
}

func TestTASMA_PeriodFlag(t *testing.T) {
	// Arrange
	srv := httptest.NewServer(priceHistoryHandler(t, "AAPL", 50))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewTACmd(testClient(t, srv), &buf)
	_, err := runTestCommand(t, cmd, "sma", "--period", "10", "--points", "0", "AAPL")
	require.NoError(t, err)

	// Assert
	_, data := decodeTAEnvelope(t, &buf)
	assert.InDelta(t, 10, data["period"], 0.1)
	assert.Equal(t, "sma", data["indicator"])

	values, ok := data["values"].([]any)
	require.True(t, ok)
	// SMA(10) on 50 candles: 50 - 10 + 1 = 41 values (--points 0 returns all)
	assert.Len(t, values, 41)
}

func TestTASMA_PointsFlag(t *testing.T) {
	// Arrange
	srv := httptest.NewServer(priceHistoryHandler(t, "AAPL", 80))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewTACmd(testClient(t, srv), &buf)
	_, err := runTestCommand(t, cmd, "sma", "--points", "5", "AAPL")
	require.NoError(t, err)

	// Assert
	_, data := decodeTAEnvelope(t, &buf)
	values, ok := data["values"].([]any)
	require.True(t, ok)
	assert.Len(t, values, 5, "--points 5 should limit output to 5 entries")
}

func TestTASMA_MissingSymbol(t *testing.T) {
	// Arrange
	server := jsonServer(`{}`)
	defer server.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewTACmd(testClient(t, server), &buf)
	_, err := runTestCommand(t, cmd, "sma")

	// Assert
	require.Error(t, err)
}

// TestTASMA_LargePeriodScalesHistory verifies that SMA with a large period (e.g. 200)
// works without requesting excessive history. SMA only needs exactly period candles,
// so SMA 200 fits in a 1-year window (252 trading days). Before the fix for #12, the
// generic formula over-requested 600 candles and then failed validation.
func TestTASMA_LargePeriodScalesHistory(t *testing.T) {
	// Arrange: handler that verifies SMA 200 only requests 1 year of data
	// (SMA needs exactly period candles, and 252 > 200).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/marketdata/v1/pricehistory")

		// SMA 200 needs 200 candles. 1 year of daily data provides 252,
		// which is sufficient. No need to request multiple years.
		period := r.URL.Query().Get("period")
		assert.Equal(t, "1", period,
			"SMA 200 needs 200 candles; 1 year (252 trading days) is sufficient")

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(mockCandleListJSON("IBM", 252)))
	}))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewTACmd(testClient(t, srv), &buf)
	_, err := runTestCommand(t, cmd, "sma", "--period", "200", "--points", "1", "IBM")
	require.NoError(t, err)

	// Assert
	_, data := decodeTAEnvelope(t, &buf)
	assert.Equal(t, "sma", data["indicator"])
	assert.Equal(t, "IBM", data["symbol"])
	assert.InDelta(t, 200, data["period"], 0.1)

	values, ok := data["values"].([]any)
	require.True(t, ok, "values should be an array")
	assert.Len(t, values, 1, "--points 1 should limit output to 1 entry")
}

func TestTASMA_MultipleCommaSeparatedPeriods(t *testing.T) {
	// Arrange: only one price-history request should be needed because the
	// command fetches enough candles for the largest requested SMA period.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/marketdata/v1/pricehistory")
		assert.Equal(t, "1", r.URL.Query().Get("period"))

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(mockCandleListJSON("AAPL", 252)))
	}))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewTACmd(testClient(t, srv), &buf)
	_, err := runTestCommand(t, cmd, "sma", "--period", "21,50,200", "--points", "1", "AAPL")
	require.NoError(t, err)

	// Assert
	_, data := decodeTAEnvelope(t, &buf)
	assert.Equal(t, "sma", data["indicator"])
	assert.Equal(t, "AAPL", data["symbol"])
	assert.NotContains(t, data, "period", "multi-period output should use periods")

	periods, ok := data["periods"].([]any)
	require.True(t, ok, "periods should be an array")
	assert.Equal(t, []any{float64(21), float64(50), float64(200)}, periods)

	values, ok := data["values"].([]any)
	require.True(t, ok, "values should be an array")
	require.Len(t, values, 1, "--points 1 should return one merged row")

	latest := values[0].(map[string]any)
	assert.Contains(t, latest, "datetime")
	assert.Contains(t, latest, "sma_21")
	assert.Contains(t, latest, "sma_50")
	assert.Contains(t, latest, "sma_200")
}

func TestTAEMA_ValidEnvelope(t *testing.T) {
	// Arrange
	srv := httptest.NewServer(priceHistoryHandler(t, "MSFT", 80))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewTACmd(testClient(t, srv), &buf)
	_, err := runTestCommand(t, cmd, "ema", "MSFT")
	require.NoError(t, err)

	// Assert
	envelope, data := decodeTAEnvelope(t, &buf)
	assert.NotEmpty(t, envelope.Metadata.Timestamp)
	assert.Equal(t, "ema", data["indicator"])
	assert.Equal(t, "MSFT", data["symbol"])
	assert.Equal(t, "daily", data["interval"])
	assert.InDelta(t, 20, data["period"], 0.1)

	values, ok := data["values"].([]any)
	require.True(t, ok, "values should be an array")
	assert.NotEmpty(t, values)

	// Each value entry should have datetime and ema keys
	first := values[0].(map[string]any)
	assert.Contains(t, first, "datetime")
	assert.Contains(t, first, "ema")
}

func TestTAEMA_MultipleRepeatedPeriods(t *testing.T) {
	// Arrange
	srv := httptest.NewServer(priceHistoryHandler(t, "MSFT", 120))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewTACmd(testClient(t, srv), &buf)
	_, err := runTestCommand(t, cmd, "ema", "--period", "12", "--period", "26", "--points", "2", "MSFT")
	require.NoError(t, err)

	// Assert
	_, data := decodeTAEnvelope(t, &buf)
	assert.Equal(t, "ema", data["indicator"])

	periods, ok := data["periods"].([]any)
	require.True(t, ok, "periods should be an array")
	assert.Equal(t, []any{float64(12), float64(26)}, periods)

	values, ok := data["values"].([]any)
	require.True(t, ok, "values should be an array")
	require.Len(t, values, 2)

	latest := values[1].(map[string]any)
	assert.Contains(t, latest, "ema_12")
	assert.Contains(t, latest, "ema_26")
}

func TestTASimplePeriodValidation(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "zero period",
			args: []string{"sma", "--period", "0", "AAPL"},
		},
		{
			name: "duplicate period",
			args: []string{"sma", "--period", "21,21", "AAPL"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			server := jsonServer(`{}`)
			defer server.Close()

			// Act
			var buf bytes.Buffer
			cmd := NewTACmd(testClient(t, server), &buf)
			_, err := runTestCommand(t, cmd, tt.args...)

			// Assert: validation now runs inside structcli.Unmarshal via
			// simpleTAOpts.Validate(), producing structcli's ValidationError.
			require.Error(t, err)
			var valErr *structclierrors.ValidationError
			assert.ErrorAs(t, err, &valErr)
		})
	}
}

func TestTARSI_ValidEnvelope(t *testing.T) {
	// Arrange
	srv := httptest.NewServer(priceHistoryHandler(t, "TSLA", 50))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewTACmd(testClient(t, srv), &buf)
	_, err := runTestCommand(t, cmd, "rsi", "TSLA")
	require.NoError(t, err)

	// Assert
	envelope, data := decodeTAEnvelope(t, &buf)
	assert.NotEmpty(t, envelope.Metadata.Timestamp)
	assert.Equal(t, "rsi", data["indicator"])
	assert.Equal(t, "TSLA", data["symbol"])

	// Default RSI period is 14
	assert.InDelta(t, 14, data["period"], 0.1)

	values, ok := data["values"].([]any)
	require.True(t, ok, "values should be an array")
	assert.NotEmpty(t, values)

	// Each RSI value should be in 0-100 range
	for i, v := range values {
		entry := v.(map[string]any)
		assert.Contains(t, entry, "datetime")
		assert.Contains(t, entry, "rsi")
		rsiVal := entry["rsi"].(float64)
		assert.GreaterOrEqual(t, rsiVal, 0.0, "RSI[%d] should be >= 0", i)
		assert.LessOrEqual(t, rsiVal, 100.0, "RSI[%d] should be <= 100", i)
	}
}

func TestTARSI_MultipleCommaSeparatedPeriods(t *testing.T) {
	// Arrange
	srv := httptest.NewServer(priceHistoryHandler(t, "TSLA", 120))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewTACmd(testClient(t, srv), &buf)
	_, err := runTestCommand(t, cmd, "rsi", "--period", "14,21,28", "--points", "2", "TSLA")
	require.NoError(t, err)

	// Assert
	_, data := decodeTAEnvelope(t, &buf)
	assert.Equal(t, "rsi", data["indicator"])
	assert.Equal(t, "TSLA", data["symbol"])
	assert.NotContains(t, data, "period", "multi-period output should use periods")

	periods, ok := data["periods"].([]any)
	require.True(t, ok, "periods should be an array")
	assert.Equal(t, []any{float64(14), float64(21), float64(28)}, periods)

	values, ok := data["values"].([]any)
	require.True(t, ok, "values should be an array")
	require.Len(t, values, 2)

	latest := values[1].(map[string]any)
	assert.Contains(t, latest, "datetime")
	assert.Contains(t, latest, "rsi_14")
	assert.Contains(t, latest, "rsi_21")
	assert.Contains(t, latest, "rsi_28")
}

func TestTARSI_MissingSymbol(t *testing.T) {
	// Arrange
	server := jsonServer(`{}`)
	defer server.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewTACmd(testClient(t, server), &buf)
	_, err := runTestCommand(t, cmd, "rsi")

	// Assert
	require.Error(t, err)
}

func TestTAMACD_ValidEnvelope(t *testing.T) {
	// Arrange - MACD needs (slow+signal)*2 = (26+9)*2 = 70 candles; 80 provides headroom
	srv := httptest.NewServer(priceHistoryHandler(t, "AAPL", 80))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewTACmd(testClient(t, srv), &buf)
	_, err := runTestCommand(t, cmd, "macd", "AAPL")
	require.NoError(t, err)

	// Assert
	envelope, data := decodeTAEnvelope(t, &buf)
	assert.NotEmpty(t, envelope.Metadata.Timestamp)
	assert.Equal(t, "macd", data["indicator"])
	assert.Equal(t, "AAPL", data["symbol"])
	assert.Equal(t, "daily", data["interval"])

	// Default MACD parameters
	assert.InDelta(t, 12, data["fast"], 0.1)
	assert.InDelta(t, 26, data["slow"], 0.1)
	assert.InDelta(t, 9, data["signal"], 0.1)

	values, ok := data["values"].([]any)
	require.True(t, ok, "values should be an array")
	assert.NotEmpty(t, values)

	// Each value entry should have datetime, macd, signal, and histogram keys
	first := values[0].(map[string]any)
	assert.Contains(t, first, "datetime")
	assert.Contains(t, first, "macd")
	assert.Contains(t, first, "signal")
	assert.Contains(t, first, "histogram")
}

func TestTAMACD_SingleSymbolReturnsSymbolMap(t *testing.T) {
	// Arrange: generic-helper indicators should preserve the same normalized root
	// shape as simple indicators and dashboard output.
	srv := httptest.NewServer(priceHistoryHandler(t, "AAPL", 80))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewTACmd(testClient(t, srv), &buf)
	_, err := runTestCommand(t, cmd, "macd", "AAPL")
	require.NoError(t, err)

	// Assert
	_, data := decodeTAEnvelopeRoot(t, &buf)
	entry, ok := data["AAPL"].(map[string]any)
	require.True(t, ok, "single-symbol macd should be keyed by symbol")
	assert.Equal(t, "macd", entry["indicator"])
	assert.Equal(t, "AAPL", entry["symbol"])
}

func TestTAMACD_CustomFlags(t *testing.T) {
	// Arrange - fast=8, slow=21, signal=5; required = (21+5)*2 = 52 candles
	srv := httptest.NewServer(priceHistoryHandler(t, "MSFT", 80))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewTACmd(testClient(t, srv), &buf)
	_, err := runTestCommand(t, cmd, "macd", "--fast", "8", "--slow", "21", "--signal", "5", "MSFT")
	require.NoError(t, err)

	// Assert
	_, data := decodeTAEnvelope(t, &buf)
	assert.Equal(t, "macd", data["indicator"])
	assert.Equal(t, "MSFT", data["symbol"])
	assert.InDelta(t, 8, data["fast"], 0.1)
	assert.InDelta(t, 21, data["slow"], 0.1)
	assert.InDelta(t, 5, data["signal"], 0.1)

	values, ok := data["values"].([]any)
	require.True(t, ok)
	assert.NotEmpty(t, values)
}

func TestTAMACD_MissingSymbol(t *testing.T) {
	// Arrange
	server := jsonServer(`{}`)
	defer server.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewTACmd(testClient(t, server), &buf)
	_, err := runTestCommand(t, cmd, "macd")

	// Assert
	require.Error(t, err)
}

func TestTAATR_ValidEnvelope(t *testing.T) {
	// Arrange - ATR needs period*3 = 14*3 = 42 candles; 60 provides headroom
	srv := httptest.NewServer(priceHistoryHandler(t, "GOOG", 60))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewTACmd(testClient(t, srv), &buf)
	_, err := runTestCommand(t, cmd, "atr", "GOOG")
	require.NoError(t, err)

	// Assert
	envelope, data := decodeTAEnvelope(t, &buf)
	assert.NotEmpty(t, envelope.Metadata.Timestamp)
	assert.Equal(t, "atr", data["indicator"])
	assert.Equal(t, "GOOG", data["symbol"])
	assert.Equal(t, "daily", data["interval"])
	assert.InDelta(t, 14, data["period"], 0.1)

	values, ok := data["values"].([]any)
	require.True(t, ok, "values should be an array")
	assert.NotEmpty(t, values)

	// Each ATR value should be positive and have datetime + atr keys
	for i, v := range values {
		entry := v.(map[string]any)
		assert.Contains(t, entry, "datetime")
		assert.Contains(t, entry, "atr")
		atrVal := entry["atr"].(float64)
		assert.Greater(t, atrVal, 0.0, "ATR[%d] should be positive", i)
	}
}

func TestTAATR_MultipleSymbolsReturnsSymbolMap(t *testing.T) {
	// Arrange: price history is single-symbol at the Schwab API, so the CLI loops
	// over requested symbols and returns one symbol-keyed envelope to callers.
	requestCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		assert.Contains(t, r.URL.Path, "/marketdata/v1/pricehistory")
		symbol := r.URL.Query().Get("symbol")
		require.Contains(t, []string{"AAPL", "MSFT"}, symbol)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(mockCandleListJSON(symbol, 60)))
	}))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewTACmd(testClient(t, srv), &buf)
	_, err := runTestCommand(t, cmd, "atr", "AAPL", "MSFT")
	require.NoError(t, err)

	// Assert
	envelope, data := decodeTAEnvelope(t, &buf)
	assert.NotEmpty(t, envelope.Metadata.Timestamp)
	assert.Equal(t, 2, requestCount)
	assert.Empty(t, envelope.Errors)

	for _, symbol := range []string{"AAPL", "MSFT"} {
		entry, ok := data[symbol].(map[string]any)
		require.True(t, ok, "%s output should be an object", symbol)
		assert.Equal(t, "atr", entry["indicator"])
		assert.Equal(t, symbol, entry["symbol"])
		assert.NotEmpty(t, entry["values"])
	}
}

func TestTAATR_MultipleSymbolsPartialError(t *testing.T) {
	// Arrange: one bad ticker should not discard successful indicator results for
	// the rest of a portfolio-wide audit.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/marketdata/v1/pricehistory")
		symbol := r.URL.Query().Get("symbol")
		if symbol == "BOGUS" {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"symbol not found"}`))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(mockCandleListJSON(symbol, 60)))
	}))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewTACmd(testClient(t, srv), &buf)
	_, err := runTestCommand(t, cmd, "atr", "AAPL", "BOGUS")
	require.NoError(t, err)

	// Assert
	envelope, data := decodeTAEnvelopeRoot(t, &buf)
	assert.Equal(t, 2, envelope.Metadata.Requested)
	assert.Equal(t, 1, envelope.Metadata.Returned)
	require.Len(t, envelope.Errors, 1)
	assert.Contains(t, envelope.Errors[0], "BOGUS")
	assert.Contains(t, data, "AAPL")
	assert.NotContains(t, data, "BOGUS")
}

func TestTAATR_MissingSymbol(t *testing.T) {
	// Arrange
	server := jsonServer(`{}`)
	defer server.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewTACmd(testClient(t, server), &buf)
	_, err := runTestCommand(t, cmd, "atr")

	// Assert
	require.Error(t, err)
}

func TestTABBands_ValidEnvelope(t *testing.T) {
	// Arrange - BBands needs exactly period = 20 candles; 80 provides headroom
	srv := httptest.NewServer(priceHistoryHandler(t, "AMZN", 80))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewTACmd(testClient(t, srv), &buf)
	_, err := runTestCommand(t, cmd, "bbands", "AMZN")
	require.NoError(t, err)

	// Assert
	envelope, data := decodeTAEnvelope(t, &buf)
	assert.NotEmpty(t, envelope.Metadata.Timestamp)
	assert.Equal(t, "bbands", data["indicator"])
	assert.Equal(t, "AMZN", data["symbol"])
	assert.Equal(t, "daily", data["interval"])
	assert.InDelta(t, 20, data["period"], 0.1)
	assert.InDelta(t, 2.0, data["std_dev"], 0.01)

	values, ok := data["values"].([]any)
	require.True(t, ok, "values should be an array")
	assert.NotEmpty(t, values)

	// Each value should have datetime, upper, middle, lower with upper > middle > lower
	for i, v := range values {
		entry := v.(map[string]any)
		assert.Contains(t, entry, "datetime")
		assert.Contains(t, entry, "upper")
		assert.Contains(t, entry, "middle")
		assert.Contains(t, entry, "lower")
		upper := entry["upper"].(float64)
		middle := entry["middle"].(float64)
		lower := entry["lower"].(float64)
		assert.GreaterOrEqual(t, upper, middle, "BBands[%d] upper >= middle", i)
		assert.GreaterOrEqual(t, middle, lower, "BBands[%d] middle >= lower", i)
	}
}

func TestTABBands_MissingSymbol(t *testing.T) {
	// Arrange
	server := jsonServer(`{}`)
	defer server.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewTACmd(testClient(t, server), &buf)
	_, err := runTestCommand(t, cmd, "bbands")

	// Assert
	require.Error(t, err)
}

func TestTAStochastic_ValidEnvelope(t *testing.T) {
	// Arrange - Stochastic needs k+smoothK+d = 14+3+3 = 20 candles; 60 provides headroom
	srv := httptest.NewServer(priceHistoryHandler(t, "AAPL", 60))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewTACmd(testClient(t, srv), &buf)
	_, err := runTestCommand(t, cmd, "stoch", "AAPL")
	require.NoError(t, err)

	// Assert
	envelope, data := decodeTAEnvelope(t, &buf)
	assert.NotEmpty(t, envelope.Metadata.Timestamp)
	assert.Equal(t, "stoch", data["indicator"])
	assert.Equal(t, "AAPL", data["symbol"])
	assert.Equal(t, "daily", data["interval"])

	// Default stochastic parameters
	assert.InDelta(t, 14, data["k_period"], 0.1)
	assert.InDelta(t, 3, data["smooth_k"], 0.1)
	assert.InDelta(t, 3, data["d_period"], 0.1)

	values, ok := data["values"].([]any)
	require.True(t, ok, "values should be an array")
	assert.NotEmpty(t, values)

	// Each value entry should have datetime, slowk, slowd keys
	first := values[0].(map[string]any)
	assert.Contains(t, first, "datetime")
	assert.Contains(t, first, "slowk")
	assert.Contains(t, first, "slowd")
}

func TestTAStochastic_CustomFlags(t *testing.T) {
	// Arrange - custom k-period=10, smooth-k=5, d-period=5
	srv := httptest.NewServer(priceHistoryHandler(t, "TSLA", 60))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewTACmd(testClient(t, srv), &buf)
	_, err := runTestCommand(t, cmd, "stoch", "--k-period", "10", "--smooth-k", "5", "--d-period", "5", "TSLA")
	require.NoError(t, err)

	// Assert
	_, data := decodeTAEnvelope(t, &buf)
	assert.Equal(t, "stoch", data["indicator"])
	assert.Equal(t, "TSLA", data["symbol"])
	assert.InDelta(t, 10, data["k_period"], 0.1)
	assert.InDelta(t, 5, data["smooth_k"], 0.1)
	assert.InDelta(t, 5, data["d_period"], 0.1)

	values, ok := data["values"].([]any)
	require.True(t, ok)
	assert.NotEmpty(t, values)
}

func TestTAStochastic_MissingSymbol(t *testing.T) {
	// Arrange
	server := jsonServer(`{}`)
	defer server.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewTACmd(testClient(t, server), &buf)
	_, err := runTestCommand(t, cmd, "stoch")

	// Assert
	require.Error(t, err)
}

func TestTAADX_ValidEnvelope(t *testing.T) {
	// Arrange - ADX has double lookback (period for DI + period for ADX itself),
	// so period=14 needs ~2*14=28 bars unstable period. Use 100 candles for plenty of headroom.
	// Must use varied data: monotonic prices give MinusDI=0 everywhere, which
	// StripLeadingZeros turns into an empty slice, breaking alignment.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/marketdata/v1/pricehistory")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(mockVariedCandleListJSON("GOOG", 100)))
	}))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewTACmd(testClient(t, srv), &buf)
	_, err := runTestCommand(t, cmd, "adx", "GOOG")
	require.NoError(t, err)

	// Assert
	envelope, data := decodeTAEnvelope(t, &buf)
	assert.NotEmpty(t, envelope.Metadata.Timestamp)
	assert.Equal(t, "adx", data["indicator"])
	assert.Equal(t, "GOOG", data["symbol"])
	assert.Equal(t, "daily", data["interval"])
	assert.InDelta(t, 14, data["period"], 0.1)

	values, ok := data["values"].([]any)
	require.True(t, ok, "values should be an array")
	assert.NotEmpty(t, values)

	// Each value entry should have datetime, adx, plus_di, minus_di keys
	first := values[0].(map[string]any)
	assert.Contains(t, first, "datetime")
	assert.Contains(t, first, "adx")
	assert.Contains(t, first, "plus_di")
	assert.Contains(t, first, "minus_di")
}

func TestTAADX_MissingSymbol(t *testing.T) {
	// Arrange
	server := jsonServer(`{}`)
	defer server.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewTACmd(testClient(t, server), &buf)
	_, err := runTestCommand(t, cmd, "adx")

	// Assert
	require.Error(t, err)
}

func TestTAVWAP_ValidEnvelope(t *testing.T) {
	// Arrange
	srv := httptest.NewServer(priceHistoryHandler(t, "AAPL", 80))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewTACmd(testClient(t, srv), &buf)
	_, err := runTestCommand(t, cmd, "vwap", "AAPL")
	require.NoError(t, err)

	// Assert
	envelope, data := decodeTAEnvelope(t, &buf)
	assert.NotEmpty(t, envelope.Metadata.Timestamp)
	assert.Equal(t, "vwap", data["indicator"])
	assert.Equal(t, "AAPL", data["symbol"])
	assert.Equal(t, "daily", data["interval"])
	// VWAP has no period - period=0 in output
	assert.InDelta(t, 0, data["period"], 0.1)
	values, ok := data["values"].([]any)
	require.True(t, ok)
	assert.NotEmpty(t, values)
	// Each value entry has "datetime" and "vwap" keys
	first := values[0].(map[string]any)
	assert.Contains(t, first, "datetime")
	assert.Contains(t, first, "vwap")
}

func TestTAVWAP_WithInterval(t *testing.T) {
	// Arrange
	srv := httptest.NewServer(priceHistoryHandler(t, "AAPL", 80))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewTACmd(testClient(t, srv), &buf)
	_, err := runTestCommand(t, cmd, "vwap", "AAPL", "--interval", "5min")
	require.NoError(t, err)

	// Assert
	_, data := decodeTAEnvelope(t, &buf)
	assert.Equal(t, "5min", data["interval"])
}

func TestTAVWAP_MissingSymbol(t *testing.T) {
	// Arrange
	srv := httptest.NewServer(priceHistoryHandler(t, "AAPL", 80))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewTACmd(testClient(t, srv), &buf)
	_, err := runTestCommand(t, cmd, "vwap")

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "arg")
}

func TestTAVWAP_DefaultsToLatestPoint(t *testing.T) {
	// Arrange - use 80 candles so the default proves it returns the latest point,
	// not the whole computed series.
	srv := httptest.NewServer(priceHistoryHandler(t, "AAPL", 80))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewTACmd(testClient(t, srv), &buf)
	_, err := runTestCommand(t, cmd, "vwap", "AAPL")
	require.NoError(t, err)

	// Assert
	_, data := decodeTAEnvelope(t, &buf)
	values, ok := data["values"].([]any)
	require.True(t, ok)
	assert.Len(t, values, 1)
}

func TestTAHV_ValidEnvelope(t *testing.T) {
	// Arrange - HV passes period+21 = 41 directly as requiredCandles. Use 200 for headroom.
	srv := httptest.NewServer(priceHistoryHandler(t, "AAPL", 200))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewTACmd(testClient(t, srv), &buf)
	_, err := runTestCommand(t, cmd, "hv", "AAPL")
	require.NoError(t, err)

	// Assert
	envelope, data := decodeTAEnvelope(t, &buf)
	assert.NotEmpty(t, envelope.Metadata.Timestamp)
	assert.Equal(t, "hv", data["indicator"])
	assert.Equal(t, "AAPL", data["symbol"])
	assert.Equal(t, "daily", data["interval"])
	assert.InDelta(t, 20, data["period"], 0.1)
	// All scalar vol fields must be present
	assert.Contains(t, data, "daily_vol")
	assert.Contains(t, data, "weekly_vol")
	assert.Contains(t, data, "monthly_vol")
	assert.Contains(t, data, "annualized_vol")
	assert.Contains(t, data, "percentile_rank")
	assert.Contains(t, data, "regime")
	assert.Contains(t, data, "min_vol")
	assert.Contains(t, data, "max_vol")
	assert.Contains(t, data, "mean_vol")
	// Percentile rank must be in [0, 100]
	rank, ok := data["percentile_rank"].(float64)
	require.True(t, ok)
	assert.GreaterOrEqual(t, rank, 0.0)
	assert.LessOrEqual(t, rank, 100.0)
	// Regime must be a valid string
	regime, ok := data["regime"].(string)
	require.True(t, ok)
	assert.NotEmpty(t, regime)
}

func TestTAHV_WithPeriod(t *testing.T) {
	// Arrange - period=30, so period+21 = 51 passed directly as requiredCandles.
	srv := httptest.NewServer(priceHistoryHandler(t, "AAPL", 200))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewTACmd(testClient(t, srv), &buf)
	_, err := runTestCommand(t, cmd, "hv", "AAPL", "--period", "30")
	require.NoError(t, err)

	// Assert
	_, data := decodeTAEnvelope(t, &buf)
	assert.InDelta(t, 30, data["period"], 0.1)
}

func TestTAHV_MissingSymbol(t *testing.T) {
	// Arrange
	srv := httptest.NewServer(priceHistoryHandler(t, "AAPL", 100))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewTACmd(testClient(t, srv), &buf)
	_, err := runTestCommand(t, cmd, "hv")

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "arg")
}

func TestTAHV_InvalidPeriod(t *testing.T) {
	// Arrange
	srv := httptest.NewServer(priceHistoryHandler(t, "AAPL", 100))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewTACmd(testClient(t, srv), &buf)
	_, err := runTestCommand(t, cmd, "hv", "AAPL", "--period", "0")

	// Assert - period=0 should produce a ValidationError from ta.HistoricalVolatility
	require.Error(t, err)
	var valErr *apperr.ValidationError
	require.ErrorAs(t, err, &valErr)
}

func TestTAExpectedMove_ValidEnvelope(t *testing.T) {
	// Arrange
	srv := httptest.NewServer(optionChainHandler(t, mockOptionChainJSON("AAPL")))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewTACmd(testClient(t, srv), &buf)
	_, err := runTestCommand(t, cmd, "expected-move", "AAPL")
	require.NoError(t, err)

	// Assert
	envelope, data := decodeTAEnvelope(t, &buf)
	assert.NotEmpty(t, envelope.Metadata.Timestamp)
	assert.Equal(t, "expected-move", data["indicator"])
	assert.Equal(t, "AAPL", data["symbol"])
	assert.Contains(t, data, "underlying_price")
	assert.Contains(t, data, "expiration")
	assert.Contains(t, data, "dte")
	assert.Contains(t, data, "straddle_price")
	assert.Contains(t, data, "expected_move")
	assert.Contains(t, data, "adjusted_move")
	assert.Contains(t, data, "upper_1x")
	assert.Contains(t, data, "lower_1x")
	assert.Contains(t, data, "upper_2x")
	assert.Contains(t, data, "lower_2x")
	assert.InDelta(t, 9.5, data["straddle_price"], 0.01)
	assert.InDelta(t, 8.075, data["adjusted_move"], 0.01)
}

func TestTAExpectedMove_WithDTE(t *testing.T) {
	// Arrange
	srv := httptest.NewServer(optionChainHandler(t, mockOptionChainTwoDTEJSON("AAPL")))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewTACmd(testClient(t, srv), &buf)
	_, err := runTestCommand(t, cmd, "expected-move", "AAPL", "--dte", "50")
	require.NoError(t, err)

	// Assert
	_, data := decodeTAEnvelope(t, &buf)
	assert.InDelta(t, 60, data["dte"], 0.1)
	assert.Equal(t, "2025-07-18", data["expiration"])
}

func TestTAExpectedMove_MissingSymbol(t *testing.T) {
	// Arrange
	srv := httptest.NewServer(optionChainHandler(t, mockOptionChainJSON("AAPL")))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewTACmd(testClient(t, srv), &buf)
	_, err := runTestCommand(t, cmd, "expected-move")

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "arg")
}

func TestTAExpectedMove_NilMark_FallbackBidAsk(t *testing.T) {
	// Arrange
	srv := httptest.NewServer(optionChainHandler(t, mockOptionChainNilMarkJSON("AAPL")))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewTACmd(testClient(t, srv), &buf)
	_, err := runTestCommand(t, cmd, "expected-move", "AAPL")
	require.NoError(t, err)

	// Assert
	_, data := decodeTAEnvelope(t, &buf)
	assert.InDelta(t, 9.5, data["straddle_price"], 0.01)
}

func TestTAExpectedMove_EmptyChain(t *testing.T) {
	// Arrange
	srv := httptest.NewServer(optionChainHandler(t, mockEmptyOptionChainJSON("AAPL")))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewTACmd(testClient(t, srv), &buf)
	_, err := runTestCommand(t, cmd, "expected-move", "AAPL")

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no options available")
}

func TestTASMA_APIError(t *testing.T) {
	// Arrange: server returns 500 for price history request.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewTACmd(testClient(t, srv), &buf)
	_, err := runTestCommand(t, cmd, "sma", "IBM", "--period", "20")

	// Assert
	require.Error(t, err)

	var httpErr *apperr.HTTPError
	require.ErrorAs(t, err, &httpErr)
}

func TestTASMA_InsufficientCandles(t *testing.T) {
	// SMA 20 requires 20 candles; returning only 10 should fail validation.
	srv := httptest.NewServer(priceHistoryHandler(t, "IBM", 10))
	defer srv.Close()

	var buf bytes.Buffer
	cmd := NewTACmd(testClient(t, srv), &buf)
	_, err := runTestCommand(t, cmd, "sma", "IBM", "--period", "20")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "requires at least 20 candles")
}

func TestTAEMA_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	var buf bytes.Buffer
	cmd := NewTACmd(testClient(t, srv), &buf)
	_, err := runTestCommand(t, cmd, "ema", "IBM", "--period", "20")

	require.Error(t, err)

	var httpErr *apperr.HTTPError
	require.ErrorAs(t, err, &httpErr)
}

func TestMustParseFloat(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want float64
	}{
		{"valid integer", "100", 100.0},
		{"valid decimal", "3.14", 3.14},
		{"empty string", "", 0},
		{"invalid string", "notanumber", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mustParseFloat(tt.in)
			assert.Equal(t, tt.want, got)
		})
	}
}
