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

func TestMovers_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/marketdata/v1/movers/$SPX", r.URL.Path)
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		response := models.ScreenerResponse{
			Screeners: []models.Screener{
				{
					Symbol:           new("AAPL"),
					Description:      new("Apple Inc"),
					LastPrice:        new(150.25),
					NetChange:        new(2.50),
					NetPercentChange: new(0.0169),
					Volume:           new(int64(12000000)),
					TotalVolume:      new(int64(45000000)),
					Trades:           new(int64(100000)),
					MarketShare:      new(26.67),
				},
				{
					Symbol:           new("NVDA"),
					Description:      new("NVIDIA Corp"),
					LastPrice:        new(850.50),
					NetChange:        new(15.00),
					NetPercentChange: new(0.0179),
					Volume:           new(int64(8000000)),
					TotalVolume:      new(int64(30000000)),
					Trades:           new(int64(75000)),
					MarketShare:      new(26.67),
				},
			},
		}
		require.NoError(t, json.NewEncoder(w).Encode(response))
	}))
	defer srv.Close()

	c := NewClient("test-token", WithBaseURL(srv.URL))
	result, err := c.Movers(context.Background(), "$SPX", MoversParams{})

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result.Screeners, 2)
	assert.Equal(t, "AAPL", *result.Screeners[0].Symbol)
	assert.Equal(t, 2.50, *result.Screeners[0].NetChange)
	assert.Equal(t, "NVDA", *result.Screeners[1].Symbol)
}

func TestMovers_WithParams(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/marketdata/v1/movers/$DJI", r.URL.Path)
		q := r.URL.Query()
		assert.Equal(t, "VOLUME", q.Get("sort"))
		assert.Equal(t, "5", q.Get("frequency"))

		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(models.ScreenerResponse{}))
	}))
	defer srv.Close()

	c := NewClient("test-token", WithBaseURL(srv.URL))
	result, err := c.Movers(context.Background(), "$DJI", MoversParams{
		Sort:      "VOLUME",
		Frequency: "5",
	})

	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestMovers_NoOptionalParams(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/marketdata/v1/movers/$COMPX", r.URL.Path)
		// No query params should be present
		assert.Empty(t, r.URL.RawQuery)

		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(models.ScreenerResponse{}))
	}))
	defer srv.Close()

	c := NewClient("test-token", WithBaseURL(srv.URL))
	result, err := c.Movers(context.Background(), "$COMPX", MoversParams{})

	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestMovers_EmptyScreeners(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(models.ScreenerResponse{Screeners: []models.Screener{}}))
	}))
	defer srv.Close()

	c := NewClient("test-token", WithBaseURL(srv.URL))
	result, err := c.Movers(context.Background(), "$SPX", MoversParams{})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Empty(t, result.Screeners)
}
