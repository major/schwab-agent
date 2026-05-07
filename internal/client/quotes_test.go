package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/major/schwab-agent/internal/apperr"
	"github.com/major/schwab-agent/internal/models"
)

func TestQuote_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/marketdata/v1/quotes", r.URL.Path)
		assert.Equal(t, "AAPL", r.URL.Query().Get("symbols"))
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		response := map[string]*models.QuoteEquity{
			"AAPL": {
				Symbol: new("AAPL"),
				Quote: &models.QuoteData{
					LastPrice:   new(150.25),
					BidPrice:    new(150.20),
					AskPrice:    new(150.30),
					TotalVolume: new(int64(45000000)),
				},
				Reference: &models.QuoteReference{
					Description: new("Apple Inc"),
				},
			},
		}
		assert.NoError(t, json.NewEncoder(w).Encode(response))
	}))
	defer srv.Close()

	c := NewClient("test-token", WithBaseURL(srv.URL))
	result, err := c.Quote(context.Background(), "AAPL", QuoteParams{})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "AAPL", *result.Symbol)
	require.NotNil(t, result.Reference)
	assert.Equal(t, "Apple Inc", *result.Reference.Description)
	require.NotNil(t, result.Quote)
	assert.InDelta(t, 150.25, *result.Quote.LastPrice, 0.001)
	assert.Equal(t, int64(45000000), *result.Quote.TotalVolume)
}

func TestQuote_404_ReturnsSymbolNotFoundError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"not found"}`))
	}))
	defer srv.Close()

	c := NewClient("test-token", WithBaseURL(srv.URL))
	result, err := c.Quote(context.Background(), "INVALID", QuoteParams{})

	require.Error(t, err)
	assert.Nil(t, result)

	var symbolErr *apperr.SymbolNotFoundError
	require.ErrorAs(t, err, &symbolErr)
	assert.Contains(t, symbolErr.Error(), "symbol INVALID not found")
}

func TestQuote_MissingSymbolInResponse(t *testing.T) {
	// API returns 200 but the map doesn't contain the requested symbol.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]*models.QuoteEquity{}))
	}))
	defer srv.Close()

	c := NewClient("test-token", WithBaseURL(srv.URL))
	result, err := c.Quote(context.Background(), "MISSING", QuoteParams{})

	require.Error(t, err)
	assert.Nil(t, result)

	var symbolErr *apperr.SymbolNotFoundError
	require.ErrorAs(t, err, &symbolErr)
	assert.Contains(t, symbolErr.Error(), "symbol MISSING not found")
}

func TestQuote_401_ReturnsAuthExpiredError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"token expired"}`))
	}))
	defer srv.Close()

	c := NewClient("bad-token", WithBaseURL(srv.URL))
	result, err := c.Quote(context.Background(), "AAPL", QuoteParams{})

	require.Error(t, err)
	assert.Nil(t, result)

	var authErr *apperr.AuthExpiredError
	require.ErrorAs(t, err, &authErr)
}
