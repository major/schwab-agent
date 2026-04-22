package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	schwabErrors "github.com/major/schwab-agent/internal/errors"
	"github.com/major/schwab-agent/internal/models"
)

func TestQuotes_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/marketdata/v1/quotes", r.URL.Path)
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		response := map[string]*models.QuoteEquity{
			"AAPL": {
				Symbol: strPtr("AAPL"),
				Quote: &models.QuoteData{
					LastPrice: floatPtr(150.25),
					BidPrice:  floatPtr(150.20),
					AskPrice:  floatPtr(150.30),
				},
			},
			"NVDA": {
				Symbol: strPtr("NVDA"),
				Quote: &models.QuoteData{
					LastPrice: floatPtr(850.50),
					BidPrice:  floatPtr(850.40),
					AskPrice:  floatPtr(850.60),
				},
			},
		}
		require.NoError(t, json.NewEncoder(w).Encode(response))
	}))
	defer srv.Close()

	c := NewClient("test-token", WithBaseURL(srv.URL))
	result, err := c.Quotes(context.Background(), []string{"AAPL", "NVDA"})

	require.NoError(t, err)
	require.Len(t, result, 2)
	assert.Equal(t, "AAPL", *result["AAPL"].Symbol)
	require.NotNil(t, result["AAPL"].Quote)
	assert.Equal(t, 150.25, *result["AAPL"].Quote.LastPrice)
	assert.Equal(t, "NVDA", *result["NVDA"].Symbol)
	require.NotNil(t, result["NVDA"].Quote)
	assert.Equal(t, 850.50, *result["NVDA"].Quote.LastPrice)
}

func TestQuotes_CommaSeparatedSymbols(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify symbols are comma-separated in query param
		symbols := r.URL.Query().Get("symbols")
		assert.Equal(t, "AAPL,NVDA,TSLA", symbols)

		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(map[string]*models.QuoteEquity{}))
	}))
	defer srv.Close()

	c := NewClient("test-token", WithBaseURL(srv.URL))
	_, err := c.Quotes(context.Background(), []string{"AAPL", "NVDA", "TSLA"})

	require.NoError(t, err)
}

func TestQuotes_EmptyResult(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(map[string]*models.QuoteEquity{}))
	}))
	defer srv.Close()

	c := NewClient("test-token", WithBaseURL(srv.URL))
	result, err := c.Quotes(context.Background(), []string{"INVALID"})

	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestQuote_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/marketdata/v1/AAPL/quotes", r.URL.Path)
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		response := map[string]*models.QuoteEquity{
			"AAPL": {
				Symbol: strPtr("AAPL"),
				Quote: &models.QuoteData{
					LastPrice:   floatPtr(150.25),
					BidPrice:    floatPtr(150.20),
					AskPrice:    floatPtr(150.30),
					TotalVolume: int64Ptr(45000000),
				},
				Reference: &models.QuoteReference{
					Description: strPtr("Apple Inc"),
				},
			},
		}
		require.NoError(t, json.NewEncoder(w).Encode(response))
	}))
	defer srv.Close()

	c := NewClient("test-token", WithBaseURL(srv.URL))
	result, err := c.Quote(context.Background(), "AAPL")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "AAPL", *result.Symbol)
	require.NotNil(t, result.Reference)
	assert.Equal(t, "Apple Inc", *result.Reference.Description)
	require.NotNil(t, result.Quote)
	assert.Equal(t, 150.25, *result.Quote.LastPrice)
	assert.Equal(t, int64(45000000), *result.Quote.TotalVolume)
}

func TestQuote_404_ReturnsSymbolNotFoundError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"not found"}`))
	}))
	defer srv.Close()

	c := NewClient("test-token", WithBaseURL(srv.URL))
	result, err := c.Quote(context.Background(), "INVALID")

	require.Error(t, err)
	assert.Nil(t, result)

	var symbolErr *schwabErrors.SymbolNotFoundError
	require.ErrorAs(t, err, &symbolErr)
	assert.Contains(t, symbolErr.Error(), "symbol INVALID not found")
}

func TestQuote_MissingSymbolInResponse(t *testing.T) {
	// API returns 200 but the map doesn't contain the requested symbol.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(map[string]*models.QuoteEquity{}))
	}))
	defer srv.Close()

	c := NewClient("test-token", WithBaseURL(srv.URL))
	result, err := c.Quote(context.Background(), "MISSING")

	require.Error(t, err)
	assert.Nil(t, result)

	var symbolErr *schwabErrors.SymbolNotFoundError
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
	result, err := c.Quote(context.Background(), "AAPL")

	require.Error(t, err)
	assert.Nil(t, result)

	var authErr *schwabErrors.AuthExpiredError
	require.ErrorAs(t, err, &authErr)
}

// int64Ptr is a test helper for creating *int64 values.
func int64Ptr(i int64) *int64 {
	return &i
}
