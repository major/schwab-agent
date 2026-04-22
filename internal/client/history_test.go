package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/major/schwab-agent/internal/models"
)

func TestPriceHistory_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/marketdata/v1/pricehistory", r.URL.Path)
		assert.Equal(t, "AAPL", r.URL.Query().Get("symbol"))
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		response := models.CandleList{
			Symbol: strPtr("AAPL"),
			Empty:  boolPtr(false),
			Candles: []models.Candle{
				{
					Open:     floatPtr(148.00),
					High:     floatPtr(152.00),
					Low:      floatPtr(147.50),
					Close:    floatPtr(150.25),
					Volume:   int64Ptr(45000000),
					Datetime: int64Ptr(1700000000000),
				},
				{
					Open:     floatPtr(150.50),
					High:     floatPtr(153.00),
					Low:      floatPtr(149.00),
					Close:    floatPtr(151.75),
					Volume:   int64Ptr(42000000),
					Datetime: int64Ptr(1700086400000),
				},
			},
		}
		require.NoError(t, json.NewEncoder(w).Encode(response))
	}))
	defer srv.Close()

	c := NewClient("test-token", WithBaseURL(srv.URL))
	result, err := c.PriceHistory(context.Background(), "AAPL", &HistoryParams{})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "AAPL", *result.Symbol)
	assert.False(t, *result.Empty)
	require.Len(t, result.Candles, 2)
	assert.Equal(t, 148.00, *result.Candles[0].Open)
	assert.Equal(t, 150.25, *result.Candles[0].Close)
	assert.Equal(t, int64(45000000), *result.Candles[0].Volume)
}

func TestPriceHistory_AllParams(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		assert.Equal(t, "AAPL", q.Get("symbol"))
		assert.Equal(t, "day", q.Get("periodType"))
		assert.Equal(t, "10", q.Get("period"))
		assert.Equal(t, "minute", q.Get("frequencyType"))
		assert.Equal(t, "5", q.Get("frequency"))
		assert.Equal(t, "1700000000000", q.Get("startDate"))
		assert.Equal(t, "1700100000000", q.Get("endDate"))

		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(models.CandleList{Symbol: strPtr("AAPL")}))
	}))
	defer srv.Close()

	c := NewClient("test-token", WithBaseURL(srv.URL))
	result, err := c.PriceHistory(context.Background(), "AAPL", &HistoryParams{
		PeriodType:    "day",
		Period:        "10",
		FrequencyType: "minute",
		Frequency:     "5",
		StartDate:     "1700000000000",
		EndDate:       "1700100000000",
	})

	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestPriceHistory_DateParams(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		assert.Equal(t, "AAPL", q.Get("symbol"))
		assert.Equal(t, "1700000000000", q.Get("startDate"))
		assert.Equal(t, "1700100000000", q.Get("endDate"))
		// Optional params not set should be absent
		assert.Empty(t, q.Get("periodType"))
		assert.Empty(t, q.Get("period"))

		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(models.CandleList{Symbol: strPtr("AAPL")}))
	}))
	defer srv.Close()

	c := NewClient("test-token", WithBaseURL(srv.URL))
	result, err := c.PriceHistory(context.Background(), "AAPL", &HistoryParams{
		StartDate: "1700000000000",
		EndDate:   "1700100000000",
	})

	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestPriceHistory_EmptyCandles(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		response := models.CandleList{
			Symbol:  strPtr("AAPL"),
			Empty:   boolPtr(true),
			Candles: []models.Candle{},
		}
		require.NoError(t, json.NewEncoder(w).Encode(response))
	}))
	defer srv.Close()

	c := NewClient("test-token", WithBaseURL(srv.URL))
	result, err := c.PriceHistory(context.Background(), "AAPL", &HistoryParams{})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, *result.Empty)
	assert.Empty(t, result.Candles)
}
