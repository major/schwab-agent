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
	assert.NotEmpty(t, envelope.Metadata.Timestamp)
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
	require.NoError(t, runTestCommand(t, cmd, "ta", "sma", "--period", "10", "--points", "0", "AAPL"))

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
	var valErr *apperr.ValidationError
	assert.ErrorAs(t, err, &valErr)
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

		// SMA 200 needs exactly 200 candles. 1 year of daily data provides 252,
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
	cmd := TACommand(testClient(t, srv), &buf)
	require.NoError(t, runTestCommand(t, cmd, "ta", "sma", "--period", "200", "--points", "1", "IBM"))

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
	cmd := TACommand(testClient(t, srv), &buf)
	require.NoError(t, runTestCommand(t, cmd, "ta", "sma", "--period", "21,50,200", "--points", "1", "AAPL"))

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
	cmd := TACommand(testClient(t, srv), &buf)
	require.NoError(t, runTestCommand(t, cmd, "ta", "ema", "MSFT"))

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
	cmd := TACommand(testClient(t, srv), &buf)
	require.NoError(t, runTestCommand(t, cmd, "ta", "ema", "--period", "12", "--period", "26", "--points", "2", "MSFT"))

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
			args: []string{"ta", "sma", "--period", "0", "AAPL"},
		},
		{
			name: "duplicate period",
			args: []string{"ta", "sma", "--period", "21,21", "AAPL"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			server := jsonServer(`{}`)
			defer server.Close()

			// Act
			var buf bytes.Buffer
			cmd := TACommand(testClient(t, server), &buf)
			err := runTestCommand(t, cmd, tt.args...)

			// Assert
			require.Error(t, err)
			var valErr *apperr.ValidationError
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
	cmd := TACommand(testClient(t, srv), &buf)
	require.NoError(t, runTestCommand(t, cmd, "ta", "rsi", "TSLA"))

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
	cmd := TACommand(testClient(t, srv), &buf)
	require.NoError(t, runTestCommand(t, cmd, "ta", "rsi", "--period", "14,21,28", "--points", "2", "TSLA"))

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
	cmd := TACommand(testClient(t, server), &buf)
	err := runTestCommand(t, cmd, "ta", "rsi")

	// Assert
	require.Error(t, err)
	var valErr *apperr.ValidationError
	assert.ErrorAs(t, err, &valErr)
}

func TestTAMACD_ValidEnvelope(t *testing.T) {
	// Arrange - MACD needs (slow+signal)*2 = (26+9)*2 = 70 candles; 80 provides headroom
	srv := httptest.NewServer(priceHistoryHandler(t, "AAPL", 80))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := TACommand(testClient(t, srv), &buf)
	require.NoError(t, runTestCommand(t, cmd, "ta", "macd", "AAPL"))

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

func TestTAMACD_CustomFlags(t *testing.T) {
	// Arrange - fast=8, slow=21, signal=5; required = (21+5)*2 = 52 candles
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
	var valErr *apperr.ValidationError
	assert.ErrorAs(t, err, &valErr)
}

func TestTAATR_ValidEnvelope(t *testing.T) {
	// Arrange - ATR needs period*3 = 14*3 = 42 candles; 60 provides headroom
	srv := httptest.NewServer(priceHistoryHandler(t, "GOOG", 60))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := TACommand(testClient(t, srv), &buf)
	require.NoError(t, runTestCommand(t, cmd, "ta", "atr", "GOOG"))

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
	var valErr *apperr.ValidationError
	assert.ErrorAs(t, err, &valErr)
}

func TestTABBands_ValidEnvelope(t *testing.T) {
	// Arrange - BBands needs exactly period = 20 candles; 80 provides headroom
	srv := httptest.NewServer(priceHistoryHandler(t, "AMZN", 80))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := TACommand(testClient(t, srv), &buf)
	require.NoError(t, runTestCommand(t, cmd, "ta", "bbands", "AMZN"))

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
	cmd := TACommand(testClient(t, server), &buf)
	err := runTestCommand(t, cmd, "ta", "bbands")

	// Assert
	require.Error(t, err)
	var valErr *apperr.ValidationError
	assert.ErrorAs(t, err, &valErr)
}

func TestTAStochastic_ValidEnvelope(t *testing.T) {
	// Arrange - Stochastic needs k+smoothK+d = 14+3+3 = 20 candles; 60 provides headroom
	srv := httptest.NewServer(priceHistoryHandler(t, "AAPL", 60))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := TACommand(testClient(t, srv), &buf)
	require.NoError(t, runTestCommand(t, cmd, "ta", "stoch", "AAPL"))

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
	var valErr *apperr.ValidationError
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
	cmd := TACommand(testClient(t, server), &buf)
	err := runTestCommand(t, cmd, "ta", "adx")

	// Assert
	require.Error(t, err)
	var valErr *apperr.ValidationError
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
	var valErr *apperr.ValidationError
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
	// Arrange - HV passes period+21 = 41 directly as requiredCandles. Use 200 for headroom.
	srv := httptest.NewServer(priceHistoryHandler(t, "AAPL", 200))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := TACommand(testClient(t, srv), &buf)
	require.NoError(t, runTestCommand(t, cmd, "ta", "hv", "AAPL"))

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
	var valErr *apperr.ValidationError
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
	var valErr *apperr.ValidationError
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
	var valErr *apperr.ValidationError
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

func TestTASMA_APIError(t *testing.T) {
	// Arrange: server returns 500 for price history request.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := TACommand(testClient(t, srv), &buf)
	err := runTestCommand(t, cmd, "ta", "sma", "IBM", "--period", "20")

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
	cmd := TACommand(testClient(t, srv), &buf)
	err := runTestCommand(t, cmd, "ta", "sma", "IBM", "--period", "20")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "requires at least 20 candles")
}

func TestTAEMA_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	var buf bytes.Buffer
	cmd := TACommand(testClient(t, srv), &buf)
	err := runTestCommand(t, cmd, "ta", "ema", "IBM", "--period", "20")

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

// --- Cobra (NewTACmd) tests ---

func TestNewTACmd_SMA_ValidEnvelope(t *testing.T) {
	// Arrange
	srv := httptest.NewServer(priceHistoryHandler(t, "AAPL", 80))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewTACmd(testClient(t, srv), &buf)
	_, err := runCobraCommand(t, cmd, "sma", "AAPL")

	// Assert
	require.NoError(t, err)
	envelope, data := decodeTAEnvelope(t, &buf)
	assert.NotEmpty(t, envelope.Metadata.Timestamp)
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

func TestNewTACmd_SMA_PeriodFlag(t *testing.T) {
	// Arrange
	srv := httptest.NewServer(priceHistoryHandler(t, "AAPL", 50))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewTACmd(testClient(t, srv), &buf)
	_, err := runCobraCommand(t, cmd, "sma", "--period", "10", "--points", "0", "AAPL")

	// Assert
	require.NoError(t, err)
	_, data := decodeTAEnvelope(t, &buf)
	assert.InDelta(t, 10, data["period"], 0.1)
	assert.Equal(t, "sma", data["indicator"])

	values, ok := data["values"].([]any)
	require.True(t, ok)
	// SMA(10) on 50 candles: 50 - 10 + 1 = 41 values (--points 0 returns all)
	assert.Len(t, values, 41)
}

func TestNewTACmd_SMA_PointsFlag(t *testing.T) {
	// Arrange
	srv := httptest.NewServer(priceHistoryHandler(t, "AAPL", 80))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewTACmd(testClient(t, srv), &buf)
	_, err := runCobraCommand(t, cmd, "sma", "--points", "5", "AAPL")

	// Assert
	require.NoError(t, err)
	_, data := decodeTAEnvelope(t, &buf)
	values, ok := data["values"].([]any)
	require.True(t, ok)
	assert.Len(t, values, 5, "--points 5 should limit output to 5 entries")
}

func TestNewTACmd_SMA_MissingSymbol(t *testing.T) {
	// Arrange
	server := jsonServer(`{}`)
	defer server.Close()

	// Act
	cmd := NewTACmd(testClient(t, server), &bytes.Buffer{})
	_, err := runCobraCommand(t, cmd, "sma")

	// Assert - Cobra's ExactArgs(1) rejects missing symbol
	require.Error(t, err)
}

// TestNewTACmd_SMA_LargePeriodScalesHistory verifies that SMA with a large period (e.g. 200)
// works without requesting excessive history. SMA only needs exactly period candles,
// so SMA 200 fits in a 1-year window (252 trading days).
func TestNewTACmd_SMA_LargePeriodScalesHistory(t *testing.T) {
	// Arrange: handler that verifies SMA 200 only requests 1 year of data
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
	_, err := runCobraCommand(t, cmd, "sma", "--period", "200", "--points", "1", "IBM")

	// Assert
	require.NoError(t, err)
	_, data := decodeTAEnvelope(t, &buf)
	assert.Equal(t, "sma", data["indicator"])
	assert.Equal(t, "IBM", data["symbol"])
	assert.InDelta(t, 200, data["period"], 0.1)

	values, ok := data["values"].([]any)
	require.True(t, ok, "values should be an array")
	assert.Len(t, values, 1, "--points 1 should limit output to 1 entry")
}

func TestNewTACmd_SMA_MultipleCommaSeparatedPeriods(t *testing.T) {
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
	_, err := runCobraCommand(t, cmd, "sma", "--period", "21,50,200", "--points", "1", "AAPL")

	// Assert
	require.NoError(t, err)
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

func TestNewTACmd_EMA_ValidEnvelope(t *testing.T) {
	// Arrange
	srv := httptest.NewServer(priceHistoryHandler(t, "MSFT", 80))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewTACmd(testClient(t, srv), &buf)
	_, err := runCobraCommand(t, cmd, "ema", "MSFT")

	// Assert
	require.NoError(t, err)
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

func TestNewTACmd_EMA_MultipleRepeatedPeriods(t *testing.T) {
	// Arrange
	srv := httptest.NewServer(priceHistoryHandler(t, "MSFT", 120))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewTACmd(testClient(t, srv), &buf)
	_, err := runCobraCommand(t, cmd, "ema", "--period", "12", "--period", "26", "--points", "2", "MSFT")

	// Assert
	require.NoError(t, err)
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

func TestNewTACmd_SimplePeriodValidation(t *testing.T) {
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
			_, err := runCobraCommand(t, cmd, tt.args...)

			// Assert
			require.Error(t, err)
			var valErr *apperr.ValidationError
			assert.ErrorAs(t, err, &valErr)
		})
	}
}

func TestNewTACmd_RSI_ValidEnvelope(t *testing.T) {
	// Arrange
	srv := httptest.NewServer(priceHistoryHandler(t, "TSLA", 50))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewTACmd(testClient(t, srv), &buf)
	_, err := runCobraCommand(t, cmd, "rsi", "TSLA")

	// Assert
	require.NoError(t, err)
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

func TestNewTACmd_RSI_MultipleCommaSeparatedPeriods(t *testing.T) {
	// Arrange
	srv := httptest.NewServer(priceHistoryHandler(t, "TSLA", 120))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewTACmd(testClient(t, srv), &buf)
	_, err := runCobraCommand(t, cmd, "rsi", "--period", "14,21,28", "--points", "2", "TSLA")

	// Assert
	require.NoError(t, err)
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

func TestNewTACmd_RSI_MissingSymbol(t *testing.T) {
	// Arrange
	server := jsonServer(`{}`)
	defer server.Close()

	// Act
	cmd := NewTACmd(testClient(t, server), &bytes.Buffer{})
	_, err := runCobraCommand(t, cmd, "rsi")

	// Assert - Cobra's ExactArgs(1) rejects missing symbol
	require.Error(t, err)
}

func TestNewTACmd_MACD_ValidEnvelope(t *testing.T) {
	// Arrange - MACD needs (slow+signal)*2 = (26+9)*2 = 70 candles; 80 provides headroom
	srv := httptest.NewServer(priceHistoryHandler(t, "AAPL", 80))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewTACmd(testClient(t, srv), &buf)
	_, err := runCobraCommand(t, cmd, "macd", "AAPL")

	// Assert
	require.NoError(t, err)
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

func TestNewTACmd_MACD_CustomFlags(t *testing.T) {
	// Arrange - fast=8, slow=21, signal=5; required = (21+5)*2 = 52 candles
	srv := httptest.NewServer(priceHistoryHandler(t, "MSFT", 80))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewTACmd(testClient(t, srv), &buf)
	_, err := runCobraCommand(t, cmd, "macd", "--fast", "8", "--slow", "21", "--signal", "5", "MSFT")

	// Assert
	require.NoError(t, err)
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

func TestNewTACmd_MACD_MissingSymbol(t *testing.T) {
	// Arrange
	server := jsonServer(`{}`)
	defer server.Close()

	// Act
	cmd := NewTACmd(testClient(t, server), &bytes.Buffer{})
	_, err := runCobraCommand(t, cmd, "macd")

	// Assert - Cobra's ExactArgs(1) rejects missing symbol
	require.Error(t, err)
}

func TestNewTACmd_ATR_ValidEnvelope(t *testing.T) {
	// Arrange - ATR needs period*3 = 14*3 = 42 candles; 60 provides headroom
	srv := httptest.NewServer(priceHistoryHandler(t, "GOOG", 60))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewTACmd(testClient(t, srv), &buf)
	_, err := runCobraCommand(t, cmd, "atr", "GOOG")

	// Assert
	require.NoError(t, err)
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

func TestNewTACmd_ATR_MissingSymbol(t *testing.T) {
	// Arrange
	server := jsonServer(`{}`)
	defer server.Close()

	// Act
	cmd := NewTACmd(testClient(t, server), &bytes.Buffer{})
	_, err := runCobraCommand(t, cmd, "atr")

	// Assert - Cobra's ExactArgs(1) rejects missing symbol
	require.Error(t, err)
}

func TestNewTACmd_BBands_ValidEnvelope(t *testing.T) {
	// Arrange - BBands needs exactly period = 20 candles; 80 provides headroom
	srv := httptest.NewServer(priceHistoryHandler(t, "AMZN", 80))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewTACmd(testClient(t, srv), &buf)
	_, err := runCobraCommand(t, cmd, "bbands", "AMZN")

	// Assert
	require.NoError(t, err)
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

func TestNewTACmd_BBands_MissingSymbol(t *testing.T) {
	// Arrange
	server := jsonServer(`{}`)
	defer server.Close()

	// Act
	cmd := NewTACmd(testClient(t, server), &bytes.Buffer{})
	_, err := runCobraCommand(t, cmd, "bbands")

	// Assert - Cobra's ExactArgs(1) rejects missing symbol
	require.Error(t, err)
}

func TestNewTACmd_Stochastic_ValidEnvelope(t *testing.T) {
	// Arrange - Stochastic needs k+smoothK+d = 14+3+3 = 20 candles; 60 provides headroom
	srv := httptest.NewServer(priceHistoryHandler(t, "AAPL", 60))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewTACmd(testClient(t, srv), &buf)
	_, err := runCobraCommand(t, cmd, "stoch", "AAPL")

	// Assert
	require.NoError(t, err)
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

func TestNewTACmd_Stochastic_CustomFlags(t *testing.T) {
	// Arrange - custom k-period=10, smooth-k=5, d-period=5
	srv := httptest.NewServer(priceHistoryHandler(t, "TSLA", 60))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewTACmd(testClient(t, srv), &buf)
	_, err := runCobraCommand(t, cmd, "stoch", "--k-period", "10", "--smooth-k", "5", "--d-period", "5", "TSLA")

	// Assert
	require.NoError(t, err)
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

func TestNewTACmd_Stochastic_MissingSymbol(t *testing.T) {
	// Arrange
	server := jsonServer(`{}`)
	defer server.Close()

	// Act
	cmd := NewTACmd(testClient(t, server), &bytes.Buffer{})
	_, err := runCobraCommand(t, cmd, "stoch")

	// Assert - Cobra's ExactArgs(1) rejects missing symbol
	require.Error(t, err)
}

func TestNewTACmd_ADX_ValidEnvelope(t *testing.T) {
	// Arrange - ADX has double lookback; use varied data to avoid MinusDI=0.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/marketdata/v1/pricehistory")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(mockVariedCandleListJSON("GOOG", 100)))
	}))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewTACmd(testClient(t, srv), &buf)
	_, err := runCobraCommand(t, cmd, "adx", "GOOG")

	// Assert
	require.NoError(t, err)
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

func TestNewTACmd_ADX_MissingSymbol(t *testing.T) {
	// Arrange
	server := jsonServer(`{}`)
	defer server.Close()

	// Act
	cmd := NewTACmd(testClient(t, server), &bytes.Buffer{})
	_, err := runCobraCommand(t, cmd, "adx")

	// Assert - Cobra's ExactArgs(1) rejects missing symbol
	require.Error(t, err)
}

func TestNewTACmd_VWAP_ValidEnvelope(t *testing.T) {
	// Arrange
	srv := httptest.NewServer(priceHistoryHandler(t, "AAPL", 80))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewTACmd(testClient(t, srv), &buf)
	_, err := runCobraCommand(t, cmd, "vwap", "AAPL")

	// Assert
	require.NoError(t, err)
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

func TestNewTACmd_VWAP_WithInterval(t *testing.T) {
	// Arrange
	srv := httptest.NewServer(priceHistoryHandler(t, "AAPL", 80))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewTACmd(testClient(t, srv), &buf)
	_, err := runCobraCommand(t, cmd, "vwap", "AAPL", "--interval", "5min")

	// Assert
	require.NoError(t, err)
	_, data := decodeTAEnvelope(t, &buf)
	assert.Equal(t, "5min", data["interval"])
}

func TestNewTACmd_VWAP_MissingSymbol(t *testing.T) {
	// Arrange
	srv := httptest.NewServer(priceHistoryHandler(t, "AAPL", 80))
	defer srv.Close()

	// Act
	cmd := NewTACmd(testClient(t, srv), &bytes.Buffer{})
	_, err := runCobraCommand(t, cmd, "vwap")

	// Assert - Cobra's ExactArgs(1) rejects missing symbol
	require.Error(t, err)
}

func TestNewTACmd_VWAP_PointsFlag(t *testing.T) {
	// Arrange - use 80 candles, request only 5 points
	srv := httptest.NewServer(priceHistoryHandler(t, "AAPL", 80))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewTACmd(testClient(t, srv), &buf)
	_, err := runCobraCommand(t, cmd, "vwap", "AAPL", "--points", "5")

	// Assert
	require.NoError(t, err)
	_, data := decodeTAEnvelope(t, &buf)
	values, ok := data["values"].([]any)
	require.True(t, ok)
	assert.Len(t, values, 5)
}

func TestNewTACmd_HV_ValidEnvelope(t *testing.T) {
	// Arrange - HV passes period+21 = 41 directly as requiredCandles. Use 200 for headroom.
	srv := httptest.NewServer(priceHistoryHandler(t, "AAPL", 200))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewTACmd(testClient(t, srv), &buf)
	_, err := runCobraCommand(t, cmd, "hv", "AAPL")

	// Assert
	require.NoError(t, err)
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

func TestNewTACmd_HV_WithPeriod(t *testing.T) {
	// Arrange - period=30, so period+21 = 51 passed directly as requiredCandles.
	srv := httptest.NewServer(priceHistoryHandler(t, "AAPL", 200))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewTACmd(testClient(t, srv), &buf)
	_, err := runCobraCommand(t, cmd, "hv", "AAPL", "--period", "30")

	// Assert
	require.NoError(t, err)
	_, data := decodeTAEnvelope(t, &buf)
	assert.InDelta(t, 30, data["period"], 0.1)
}

func TestNewTACmd_HV_MissingSymbol(t *testing.T) {
	// Arrange
	srv := httptest.NewServer(priceHistoryHandler(t, "AAPL", 100))
	defer srv.Close()

	// Act
	cmd := NewTACmd(testClient(t, srv), &bytes.Buffer{})
	_, err := runCobraCommand(t, cmd, "hv")

	// Assert - Cobra's ExactArgs(1) rejects missing symbol
	require.Error(t, err)
}

func TestNewTACmd_HV_InvalidPeriod(t *testing.T) {
	// Arrange
	srv := httptest.NewServer(priceHistoryHandler(t, "AAPL", 100))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewTACmd(testClient(t, srv), &buf)
	_, err := runCobraCommand(t, cmd, "hv", "AAPL", "--period", "0")

	// Assert - period=0 should produce a ValidationError from ta.HistoricalVolatility
	require.Error(t, err)
	var valErr *apperr.ValidationError
	require.ErrorAs(t, err, &valErr)
}

func TestNewTACmd_ExpectedMove_ValidEnvelope(t *testing.T) {
	// Arrange
	srv := httptest.NewServer(optionChainHandler(t, mockOptionChainJSON("AAPL")))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewTACmd(testClient(t, srv), &buf)
	_, err := runCobraCommand(t, cmd, "expected-move", "AAPL")

	// Assert
	require.NoError(t, err)
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

func TestNewTACmd_ExpectedMove_WithDTE(t *testing.T) {
	// Arrange
	srv := httptest.NewServer(optionChainHandler(t, mockOptionChainTwoDTEJSON("AAPL")))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewTACmd(testClient(t, srv), &buf)
	_, err := runCobraCommand(t, cmd, "expected-move", "AAPL", "--dte", "50")

	// Assert
	require.NoError(t, err)
	_, data := decodeTAEnvelope(t, &buf)
	assert.InDelta(t, 60, data["dte"], 0.1)
	assert.Equal(t, "2025-07-18", data["expiration"])
}

func TestNewTACmd_ExpectedMove_MissingSymbol(t *testing.T) {
	// Arrange
	srv := httptest.NewServer(optionChainHandler(t, mockOptionChainJSON("AAPL")))
	defer srv.Close()

	// Act
	cmd := NewTACmd(testClient(t, srv), &bytes.Buffer{})
	_, err := runCobraCommand(t, cmd, "expected-move")

	// Assert - Cobra's ExactArgs(1) rejects missing symbol
	require.Error(t, err)
}

func TestNewTACmd_ExpectedMove_NilMark_FallbackBidAsk(t *testing.T) {
	// Arrange
	srv := httptest.NewServer(optionChainHandler(t, mockOptionChainNilMarkJSON("AAPL")))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewTACmd(testClient(t, srv), &buf)
	_, err := runCobraCommand(t, cmd, "expected-move", "AAPL")

	// Assert
	require.NoError(t, err)
	_, data := decodeTAEnvelope(t, &buf)
	assert.InDelta(t, 9.5, data["straddle_price"], 0.01)
}

func TestNewTACmd_ExpectedMove_EmptyChain(t *testing.T) {
	// Arrange
	srv := httptest.NewServer(optionChainHandler(t, mockEmptyOptionChainJSON("AAPL")))
	defer srv.Close()

	// Act
	cmd := NewTACmd(testClient(t, srv), &bytes.Buffer{})
	_, err := runCobraCommand(t, cmd, "expected-move", "AAPL")

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no options available")
}

func TestNewTACmd_SMA_APIError(t *testing.T) {
	// Arrange: server returns 500 for price history request.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewTACmd(testClient(t, srv), &buf)
	_, err := runCobraCommand(t, cmd, "sma", "--period", "20", "IBM")

	// Assert
	require.Error(t, err)

	var httpErr *apperr.HTTPError
	require.ErrorAs(t, err, &httpErr)
}

func TestNewTACmd_SMA_InsufficientCandles(t *testing.T) {
	// SMA 20 requires 20 candles; returning only 10 should fail validation.
	srv := httptest.NewServer(priceHistoryHandler(t, "IBM", 10))
	defer srv.Close()

	var buf bytes.Buffer
	cmd := NewTACmd(testClient(t, srv), &buf)
	_, err := runCobraCommand(t, cmd, "sma", "--period", "20", "IBM")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "requires at least 20 candles")
}

func TestNewTACmd_EMA_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	var buf bytes.Buffer
	cmd := NewTACmd(testClient(t, srv), &buf)
	_, err := runCobraCommand(t, cmd, "ema", "--period", "20", "IBM")

	require.Error(t, err)

	var httpErr *apperr.HTTPError
	require.ErrorAs(t, err, &httpErr)
}
