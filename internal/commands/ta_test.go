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

	schwabErrors "github.com/major/schwab-agent/internal/errors"
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

// decodeTAEnvelope unmarshals the output buffer into an Envelope and extracts the data map.
func decodeTAEnvelope(t *testing.T, buf *bytes.Buffer) (envelope output.Envelope, data map[string]any) {
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

func TestTASMA_ValidEnvelope(t *testing.T) {
	// Arrange
	srv := httptest.NewServer(priceHistoryHandler(t, "AAPL", 80))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := TACommand(testClient(t, srv), &buf)
	require.NoError(t, runTestCommand(t, cmd, "ta", "sma", "AAPL"))

	// Assert
	envelope, data := decodeTAEnvelope(t, &buf)
	assert.Contains(t, envelope.Metadata, "timestamp")
	assert.Equal(t, "sma", data["indicator"])
	assert.Equal(t, "AAPL", data["symbol"])
	assert.Equal(t, "daily", data["interval"])

	// Default period is 20
	assert.InDelta(t, 20, data["period"], 0.1)

	values, ok := data["values"].([]any)
	require.True(t, ok, "values should be an array")
	assert.NotEmpty(t, values)

	// Each value entry should have datetime and sma keys
	first := values[0].(map[string]any)
	assert.Contains(t, first, "datetime")
	assert.Contains(t, first, "sma")
}

func TestTASMA_PeriodFlag(t *testing.T) {
	// Arrange
	srv := httptest.NewServer(priceHistoryHandler(t, "AAPL", 50))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := TACommand(testClient(t, srv), &buf)
	require.NoError(t, runTestCommand(t, cmd, "ta", "sma", "--period", "10", "AAPL"))

	// Assert
	_, data := decodeTAEnvelope(t, &buf)
	assert.InDelta(t, 10, data["period"], 0.1)
	assert.Equal(t, "sma", data["indicator"])

	values, ok := data["values"].([]any)
	require.True(t, ok)
	// SMA(10) on 50 candles: 50 - 10 + 1 = 41 values
	assert.Len(t, values, 41)
}

func TestTASMA_PointsFlag(t *testing.T) {
	// Arrange
	srv := httptest.NewServer(priceHistoryHandler(t, "AAPL", 80))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := TACommand(testClient(t, srv), &buf)
	require.NoError(t, runTestCommand(t, cmd, "ta", "sma", "--points", "5", "AAPL"))

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
	cmd := TACommand(testClient(t, server), &buf)
	err := runTestCommand(t, cmd, "ta", "sma")

	// Assert
	require.Error(t, err)
	var valErr *schwabErrors.ValidationError
	assert.ErrorAs(t, err, &valErr)
}

func TestTAEMA_ValidEnvelope(t *testing.T) {
	// Arrange
	srv := httptest.NewServer(priceHistoryHandler(t, "MSFT", 80))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := TACommand(testClient(t, srv), &buf)
	require.NoError(t, runTestCommand(t, cmd, "ta", "ema", "MSFT"))

	// Assert
	envelope, data := decodeTAEnvelope(t, &buf)
	assert.Contains(t, envelope.Metadata, "timestamp")
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

func TestTARSI_ValidEnvelope(t *testing.T) {
	// Arrange
	srv := httptest.NewServer(priceHistoryHandler(t, "TSLA", 50))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := TACommand(testClient(t, srv), &buf)
	require.NoError(t, runTestCommand(t, cmd, "ta", "rsi", "TSLA"))

	// Assert
	envelope, data := decodeTAEnvelope(t, &buf)
	assert.Contains(t, envelope.Metadata, "timestamp")
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

func TestTARSI_MissingSymbol(t *testing.T) {
	// Arrange
	server := jsonServer(`{}`)
	defer server.Close()

	// Act
	var buf bytes.Buffer
	cmd := TACommand(testClient(t, server), &buf)
	err := runTestCommand(t, cmd, "ta", "rsi")

	// Assert
	require.Error(t, err)
	var valErr *schwabErrors.ValidationError
	assert.ErrorAs(t, err, &valErr)
}

func TestTAMACD_ValidEnvelope(t *testing.T) {
	// Arrange - 80 candles needed for MACD (slow=26, data window = max(78, 46) = 78)
	srv := httptest.NewServer(priceHistoryHandler(t, "AAPL", 80))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := TACommand(testClient(t, srv), &buf)
	require.NoError(t, runTestCommand(t, cmd, "ta", "macd", "AAPL"))

	// Assert
	envelope, data := decodeTAEnvelope(t, &buf)
	assert.Contains(t, envelope.Metadata, "timestamp")
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

func TestTAMACD_CustomFlags(t *testing.T) {
	// Arrange - fast=8, slow=21, signal=5; data window = max(63, 41) = 63
	srv := httptest.NewServer(priceHistoryHandler(t, "MSFT", 80))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := TACommand(testClient(t, srv), &buf)
	require.NoError(t, runTestCommand(t, cmd, "ta", "macd", "--fast", "8", "--slow", "21", "--signal", "5", "MSFT"))

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
	cmd := TACommand(testClient(t, server), &buf)
	err := runTestCommand(t, cmd, "ta", "macd")

	// Assert
	require.Error(t, err)
	var valErr *schwabErrors.ValidationError
	assert.ErrorAs(t, err, &valErr)
}

func TestTAATR_ValidEnvelope(t *testing.T) {
	// Arrange - 60 candles for ATR (period=14, data window = max(42, 34) = 42)
	srv := httptest.NewServer(priceHistoryHandler(t, "GOOG", 60))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := TACommand(testClient(t, srv), &buf)
	require.NoError(t, runTestCommand(t, cmd, "ta", "atr", "GOOG"))

	// Assert
	envelope, data := decodeTAEnvelope(t, &buf)
	assert.Contains(t, envelope.Metadata, "timestamp")
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

func TestTAATR_MissingSymbol(t *testing.T) {
	// Arrange
	server := jsonServer(`{}`)
	defer server.Close()

	// Act
	var buf bytes.Buffer
	cmd := TACommand(testClient(t, server), &buf)
	err := runTestCommand(t, cmd, "ta", "atr")

	// Assert
	require.Error(t, err)
	var valErr *schwabErrors.ValidationError
	assert.ErrorAs(t, err, &valErr)
}

func TestTABBands_ValidEnvelope(t *testing.T) {
	// Arrange - 80 candles for BBands (period=20, data window = max(60, 40) = 60)
	srv := httptest.NewServer(priceHistoryHandler(t, "AMZN", 80))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := TACommand(testClient(t, srv), &buf)
	require.NoError(t, runTestCommand(t, cmd, "ta", "bbands", "AMZN"))

	// Assert
	envelope, data := decodeTAEnvelope(t, &buf)
	assert.Contains(t, envelope.Metadata, "timestamp")
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
	cmd := TACommand(testClient(t, server), &buf)
	err := runTestCommand(t, cmd, "ta", "bbands")

	// Assert
	require.Error(t, err)
	var valErr *schwabErrors.ValidationError
	assert.ErrorAs(t, err, &valErr)
}

func TestTAStochastic_ValidEnvelope(t *testing.T) {
	// Arrange - k-period=14 default, data window = max(42, 34) = 42, use 60 candles for headroom
	srv := httptest.NewServer(priceHistoryHandler(t, "AAPL", 60))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := TACommand(testClient(t, srv), &buf)
	require.NoError(t, runTestCommand(t, cmd, "ta", "stoch", "AAPL"))

	// Assert
	envelope, data := decodeTAEnvelope(t, &buf)
	assert.Contains(t, envelope.Metadata, "timestamp")
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
	cmd := TACommand(testClient(t, srv), &buf)
	require.NoError(t, runTestCommand(t, cmd, "ta", "stoch", "--k-period", "10", "--smooth-k", "5", "--d-period", "5", "TSLA"))

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
	cmd := TACommand(testClient(t, server), &buf)
	err := runTestCommand(t, cmd, "ta", "stoch")

	// Assert
	require.Error(t, err)
	var valErr *schwabErrors.ValidationError
	assert.ErrorAs(t, err, &valErr)
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
	cmd := TACommand(testClient(t, srv), &buf)
	require.NoError(t, runTestCommand(t, cmd, "ta", "adx", "GOOG"))

	// Assert
	envelope, data := decodeTAEnvelope(t, &buf)
	assert.Contains(t, envelope.Metadata, "timestamp")
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
	cmd := TACommand(testClient(t, server), &buf)
	err := runTestCommand(t, cmd, "ta", "adx")

	// Assert
	require.Error(t, err)
	var valErr *schwabErrors.ValidationError
	assert.ErrorAs(t, err, &valErr)
}

func TestTAVWAP_ValidEnvelope(t *testing.T) {
	// Arrange
	srv := httptest.NewServer(priceHistoryHandler(t, "AAPL", 80))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := TACommand(testClient(t, srv), &buf)
	require.NoError(t, runTestCommand(t, cmd, "ta", "vwap", "AAPL"))

	// Assert
	envelope, data := decodeTAEnvelope(t, &buf)
	assert.Contains(t, envelope.Metadata, "timestamp")
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
	cmd := TACommand(testClient(t, srv), &buf)
	require.NoError(t, runTestCommand(t, cmd, "ta", "vwap", "AAPL", "--interval", "5min"))

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
	cmd := TACommand(testClient(t, srv), &buf)
	err := runTestCommand(t, cmd, "ta", "vwap")

	// Assert
	require.Error(t, err)
	var valErr *schwabErrors.ValidationError
	require.ErrorAs(t, err, &valErr)
	assert.Contains(t, valErr.Error(), "symbol")
}

func TestTAVWAP_NoPointsFlag(t *testing.T) {
	// Arrange - use 80 candles, request only 5 points
	srv := httptest.NewServer(priceHistoryHandler(t, "AAPL", 80))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := TACommand(testClient(t, srv), &buf)
	require.NoError(t, runTestCommand(t, cmd, "ta", "vwap", "AAPL", "--points", "5"))

	// Assert
	_, data := decodeTAEnvelope(t, &buf)
	values, ok := data["values"].([]any)
	require.True(t, ok)
	assert.Len(t, values, 5)
}

func TestTAHV_ValidEnvelope(t *testing.T) {
	// Arrange - period+21 passed to fetchAndValidateCandles, which applies max(p*3, p+20).
	// For default period=20: 41*3=123 candles required. Use 200 for headroom.
	srv := httptest.NewServer(priceHistoryHandler(t, "AAPL", 200))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := TACommand(testClient(t, srv), &buf)
	require.NoError(t, runTestCommand(t, cmd, "ta", "hv", "AAPL"))

	// Assert
	envelope, data := decodeTAEnvelope(t, &buf)
	assert.Contains(t, envelope.Metadata, "timestamp")
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
	// Arrange - period=30, so 51 passed to fetchAndValidateCandles -> max(153, 71)=153 required.
	srv := httptest.NewServer(priceHistoryHandler(t, "AAPL", 200))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := TACommand(testClient(t, srv), &buf)
	require.NoError(t, runTestCommand(t, cmd, "ta", "hv", "AAPL", "--period", "30"))

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
	cmd := TACommand(testClient(t, srv), &buf)
	err := runTestCommand(t, cmd, "ta", "hv")

	// Assert
	require.Error(t, err)
	var valErr *schwabErrors.ValidationError
	require.ErrorAs(t, err, &valErr)
	assert.Contains(t, valErr.Error(), "symbol")
}

func TestTAHV_InvalidPeriod(t *testing.T) {
	// Arrange
	srv := httptest.NewServer(priceHistoryHandler(t, "AAPL", 100))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := TACommand(testClient(t, srv), &buf)
	err := runTestCommand(t, cmd, "ta", "hv", "AAPL", "--period", "0")

	// Assert - period=0 should produce a ValidationError from ta.HistoricalVolatility
	require.Error(t, err)
	var valErr *schwabErrors.ValidationError
	require.ErrorAs(t, err, &valErr)
}

func TestTAExpectedMove_ValidEnvelope(t *testing.T) {
	// Arrange
	srv := httptest.NewServer(optionChainHandler(t, mockOptionChainJSON("AAPL")))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := TACommand(testClient(t, srv), &buf)
	require.NoError(t, runTestCommand(t, cmd, "ta", "expected-move", "AAPL"))

	// Assert
	envelope, data := decodeTAEnvelope(t, &buf)
	assert.Contains(t, envelope.Metadata, "timestamp")
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
	cmd := TACommand(testClient(t, srv), &buf)
	require.NoError(t, runTestCommand(t, cmd, "ta", "expected-move", "AAPL", "--dte", "50"))

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
	cmd := TACommand(testClient(t, srv), &buf)
	err := runTestCommand(t, cmd, "ta", "expected-move")

	// Assert
	require.Error(t, err)
	var valErr *schwabErrors.ValidationError
	require.ErrorAs(t, err, &valErr)
	assert.Contains(t, valErr.Error(), "symbol")
}

func TestTAExpectedMove_NilMark_FallbackBidAsk(t *testing.T) {
	// Arrange
	srv := httptest.NewServer(optionChainHandler(t, mockOptionChainNilMarkJSON("AAPL")))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := TACommand(testClient(t, srv), &buf)
	require.NoError(t, runTestCommand(t, cmd, "ta", "expected-move", "AAPL"))

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
	cmd := TACommand(testClient(t, srv), &buf)
	err := runTestCommand(t, cmd, "ta", "expected-move", "AAPL")

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no options available")
}
