package client

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"resty.dev/v3"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/major/schwab-agent/internal/apperr"
	"github.com/major/schwab-agent/internal/models"
)

func TestQuotes_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/marketdata/v1/quotes", r.URL.Path)
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		assert.Empty(t, r.Header.Get("Content-Type"))

		w.Header().Set("Content-Type", "application/json")
		response := map[string]*models.QuoteEquity{
			"AAPL": {
				Symbol: new("AAPL"),
				Quote: &models.QuoteData{
					LastPrice: new(150.25),
					BidPrice:  new(150.20),
					AskPrice:  new(150.30),
				},
			},
			"NVDA": {
				Symbol: new("NVDA"),
				Quote: &models.QuoteData{
					LastPrice: new(850.50),
					BidPrice:  new(850.40),
					AskPrice:  new(850.60),
				},
			},
		}
		require.NoError(t, json.NewEncoder(w).Encode(response))
	}))
	defer srv.Close()

	c := NewClient("test-token", WithBaseURL(srv.URL))
	result, err := c.Quotes(context.Background(), []string{"AAPL", "NVDA"}, QuoteParams{})

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
	_, err := c.Quotes(context.Background(), []string{"AAPL", "NVDA", "TSLA"}, QuoteParams{})

	require.NoError(t, err)
}

func TestQuotes_ForwardsFieldsAndIndicative(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		assert.Equal(t, "/marketdata/v1/quotes", r.URL.Path)
		assert.Equal(t, "AAPL,MSFT", query.Get("symbols"))
		assert.Equal(t, "quote,fundamental", query.Get("fields"))
		assert.Equal(t, "true", query.Get("indicative"))

		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(map[string]*models.QuoteEquity{}))
	}))
	defer srv.Close()

	c := NewClient("test-token", WithBaseURL(srv.URL))
	_, err := c.Quotes(context.Background(), []string{"AAPL", "MSFT"}, QuoteParams{
		Fields:     []string{"quote", "fundamental"},
		Indicative: true,
	})

	require.NoError(t, err)
}

func TestQuotes_EmptyResult(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(map[string]*models.QuoteEquity{}))
	}))
	defer srv.Close()

	c := NewClient("test-token", WithBaseURL(srv.URL))
	result, err := c.Quotes(context.Background(), []string{"INVALID"}, QuoteParams{})

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
		require.NoError(t, json.NewEncoder(w).Encode(response))
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
	assert.Equal(t, 150.25, *result.Quote.LastPrice)
	assert.Equal(t, int64(45000000), *result.Quote.TotalVolume)
}

func TestQuote_ForwardsFields(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/marketdata/v1/AAPL/quotes", r.URL.Path)
		assert.Equal(t, "extended,reference", r.URL.Query().Get("fields"))
		assert.Empty(t, r.Header.Get("Content-Type"))

		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(map[string]*models.QuoteEquity{
			"AAPL": {Symbol: new("AAPL")},
		}))
	}))
	defer srv.Close()

	c := NewClient("test-token", WithBaseURL(srv.URL))
	_, err := c.Quote(context.Background(), "AAPL", QuoteParams{Fields: []string{"extended", "reference"}})

	require.NoError(t, err)
}

func TestQuote_IndicativeUsesMultiQuoteEndpoint(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		assert.Equal(t, "/marketdata/v1/quotes", r.URL.Path)
		assert.Equal(t, "AAPL", query.Get("symbols"))
		assert.Equal(t, "true", query.Get("indicative"))

		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(map[string]*models.QuoteEquity{
			"AAPL": {Symbol: new("AAPL")},
		}))
	}))
	defer srv.Close()

	c := NewClient("test-token", WithBaseURL(srv.URL))
	_, err := c.Quote(context.Background(), "AAPL", QuoteParams{Indicative: true})

	require.NoError(t, err)
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
		require.NoError(t, json.NewEncoder(w).Encode(map[string]*models.QuoteEquity{}))
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

func TestQuote_404WithNonJSONBodyStillReturnsSymbolNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("<html><body>not found</body></html>"))
	}))
	defer srv.Close()

	c := NewClient("test-token", WithBaseURL(srv.URL))
	result, err := c.Quote(context.Background(), "INVALID", QuoteParams{})

	require.Error(t, err)
	assert.Nil(t, result)
	var symbolErr *apperr.SymbolNotFoundError
	require.ErrorAs(t, err, &symbolErr)
}

func TestQuote_JSONContentTypeWithCharsetSucceeds(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		require.NoError(t, json.NewEncoder(w).Encode(map[string]*models.QuoteEquity{
			"AAPL": {Symbol: new("AAPL")},
		}))
	}))
	defer srv.Close()

	c := NewClient("test-token", WithBaseURL(srv.URL))
	result, err := c.Quote(context.Background(), "AAPL", QuoteParams{})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "AAPL", *result.Symbol)
}

func TestQuote_BaseURLPrefixPreserved(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/proxy/marketdata/v1/AAPL/quotes", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(map[string]*models.QuoteEquity{
			"AAPL": {Symbol: new("AAPL")},
		}))
	}))
	defer srv.Close()

	c := NewClient("test-token", WithBaseURL(srv.URL+"/proxy"))
	_, err := c.Quote(context.Background(), "AAPL", QuoteParams{})

	require.NoError(t, err)
}

func TestQuote_UsesUpdatedToken(t *testing.T) {
	var seenAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(map[string]*models.QuoteEquity{
			"AAPL": {Symbol: new("AAPL")},
		}))
	}))
	defer srv.Close()

	c := NewClient("old-token", WithBaseURL(srv.URL))
	c.token = "new-token"
	_, err := c.Quote(context.Background(), "AAPL", QuoteParams{})

	require.NoError(t, err)
	assert.Equal(t, "Bearer new-token", seenAuth)
}

func TestQuote_UsesConfiguredTLSClient(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(map[string]*models.QuoteEquity{
			"AAPL": {Symbol: new("AAPL")},
		}))
	}))
	defer srv.Close()

	c := NewClient("test-token", WithBaseURL(srv.URL), WithTLSConfig(&tls.Config{InsecureSkipVerify: true}))
	result, err := c.Quote(context.Background(), "AAPL", QuoteParams{})

	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestSchwabGoHTTPClientPreservesRestyTimeout(t *testing.T) {
	c := NewClient("test-token")

	httpClient := c.schwabGoHTTPClient()

	assert.Equal(t, c.resty.Timeout(), httpClient.Timeout)
}

func TestQuote_500ReturnsHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"message":"upstream failed"}`))
	}))
	defer srv.Close()

	c := NewClient("test-token", WithBaseURL(srv.URL))
	result, err := c.Quote(context.Background(), "AAPL", QuoteParams{})

	require.Error(t, err)
	assert.Nil(t, result)
	var httpErr *apperr.HTTPError
	require.ErrorAs(t, err, &httpErr)
	assert.Equal(t, http.StatusInternalServerError, httpErr.StatusCode)
	// schwab-go currently exposes the HTTP status text here instead of the raw
	// response body. This test still matters because the quote adapter must let
	// non-2xx bodies reach schwab-go's APIError mapping instead of rejecting them
	// as non-JSON success responses in quoteSafeTransport.
	assert.Contains(t, httpErr.Body, http.StatusText(http.StatusInternalServerError))
}

func TestQuote_502WithNonJSONBodyStillReturnsHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("<html><body>bad gateway</body></html>"))
	}))
	defer srv.Close()

	c := NewClient("test-token", WithBaseURL(srv.URL))
	result, err := c.Quote(context.Background(), "AAPL", QuoteParams{})

	require.Error(t, err)
	assert.Nil(t, result)
	var httpErr *apperr.HTTPError
	require.ErrorAs(t, err, &httpErr)
	assert.Equal(t, http.StatusBadGateway, httpErr.StatusCode)
}

func TestQuote_RejectsNonJSONSuccessResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("<html><body>maintenance window</body></html>"))
	}))
	defer srv.Close()

	c := NewClient("test-token", WithBaseURL(srv.URL))
	result, err := c.Quote(context.Background(), "AAPL", QuoteParams{})

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), `unexpected Content-Type "text/html; charset=utf-8"`)
	assert.Contains(t, err.Error(), "maintenance window")
}

func TestQuote_RejectsOversizedSuccessResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(bytes.Repeat([]byte(" "), maxResponseSize+1))
	}))
	defer srv.Close()

	c := NewClient("test-token", WithBaseURL(srv.URL))
	result, err := c.Quote(context.Background(), "AAPL", QuoteParams{})

	require.Error(t, err)
	assert.Nil(t, result)
	assert.True(t, errors.Is(err, resty.ErrReadExceedsThresholdLimit), "expected %v, got %v", resty.ErrReadExceedsThresholdLimit, err)
}
