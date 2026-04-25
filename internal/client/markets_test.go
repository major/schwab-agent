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

func TestMarkets_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/marketdata/v1/markets", r.URL.Path)
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		// The API requires a "markets" query param with comma-separated market names.
		q := r.URL.Query()
		assert.Equal(t, "equity,option", q.Get("markets"))

		w.Header().Set("Content-Type", "application/json")
		// The real API returns a doubly-nested map: market -> product -> hours.
		response := map[string]map[string]models.MarketHours{
			"equity": {
				"EQ": {
					MarketType: new("EQUITY"),
					IsOpen:     new(true),
					Date:       new("2025-04-21"),
				},
			},
			"option": {
				"EQO": {
					MarketType: new("OPTION"),
					IsOpen:     new(true),
					Date:       new("2025-04-21"),
				},
			},
		}
		require.NoError(t, json.NewEncoder(w).Encode(response))
	}))
	defer srv.Close()

	c := NewClient("test-token", WithBaseURL(srv.URL))
	result, err := c.Markets(context.Background(), []string{"equity", "option"})

	require.NoError(t, err)
	require.Len(t, result, 2)
	require.Contains(t, result, "equity")
	assert.Equal(t, "EQUITY", *result["equity"]["EQ"].MarketType)
	assert.True(t, *result["equity"]["EQ"].IsOpen)
	require.Contains(t, result, "option")
	assert.Equal(t, "OPTION", *result["option"]["EQO"].MarketType)
}

func TestMarkets_EmptyResult(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(map[string]map[string]models.MarketHours{}))
	}))
	defer srv.Close()

	c := NewClient("test-token", WithBaseURL(srv.URL))
	result, err := c.Markets(context.Background(), []string{"equity"})

	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestMarket_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/marketdata/v1/markets/equity", r.URL.Path)
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		// The real API wraps even single-market responses in the same
		// doubly-nested structure: market -> product -> hours.
		response := map[string]map[string]models.MarketHours{
			"equity": {
				"EQ": {
					MarketType:  new("EQUITY"),
					ProductName: new("equity"),
					IsOpen:      new(true),
					Date:        new("2025-04-21"),
					SessionHours: &models.SessionHours{
						RegularMarket: []models.MarketSession{
							{
								Start: new("2025-04-21T09:30:00-04:00"),
								End:   new("2025-04-21T16:00:00-04:00"),
							},
						},
					},
				},
			},
		}
		require.NoError(t, json.NewEncoder(w).Encode(response))
	}))
	defer srv.Close()

	c := NewClient("test-token", WithBaseURL(srv.URL))
	result, err := c.Market(context.Background(), "equity")

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Contains(t, result, "equity")
	eq := result["equity"]["EQ"]
	assert.Equal(t, "EQUITY", *eq.MarketType)
	assert.True(t, *eq.IsOpen)
	require.NotNil(t, eq.SessionHours)
	require.Len(t, eq.SessionHours.RegularMarket, 1)
	assert.Equal(t, "2025-04-21T09:30:00-04:00", *eq.SessionHours.RegularMarket[0].Start)
}

func TestMarket_404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"not found"}`))
	}))
	defer srv.Close()

	c := NewClient("test-token", WithBaseURL(srv.URL))
	result, err := c.Market(context.Background(), "invalid")

	require.Error(t, err)
	assert.Nil(t, result)
}
