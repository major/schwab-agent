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
	"github.com/major/schwab-agent/internal/ptr"
)

func TestSearchInstruments_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/marketdata/v1/instruments", r.URL.Path)
		assert.Equal(t, "AAPL", r.URL.Query().Get("symbol"))
		assert.Equal(t, "symbol-search", r.URL.Query().Get("projection"))
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		response := models.InstrumentResponse{
			Instruments: []models.Instrument{
				{
					Cusip:       ptr.To("037833100"),
					Symbol:      ptr.To("AAPL"),
					Description: ptr.To("Apple Inc"),
					Exchange:    ptr.To("NASDAQ"),
					AssetType:   ptr.To("EQUITY"),
				},
			},
		}
		require.NoError(t, json.NewEncoder(w).Encode(response))
	}))
	defer srv.Close()

	c := NewClient("test-token", WithBaseURL(srv.URL))
	result, err := c.SearchInstruments(context.Background(), "AAPL", "symbol-search")

	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, "AAPL", *result[0].Symbol)
	assert.Equal(t, "037833100", *result[0].Cusip)
	assert.Equal(t, "Apple Inc", *result[0].Description)
}

func TestSearchInstruments_MultipleResults(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "AA", r.URL.Query().Get("symbol"))
		assert.Equal(t, "symbol-regex", r.URL.Query().Get("projection"))

		w.Header().Set("Content-Type", "application/json")
		response := models.InstrumentResponse{
			Instruments: []models.Instrument{
				{Symbol: ptr.To("AAPL"), Description: ptr.To("Apple Inc")},
				{Symbol: ptr.To("AAL"), Description: ptr.To("American Airlines")},
				{Symbol: ptr.To("AAXJ"), Description: ptr.To("iShares MSCI All Country Asia")},
			},
		}
		require.NoError(t, json.NewEncoder(w).Encode(response))
	}))
	defer srv.Close()

	c := NewClient("test-token", WithBaseURL(srv.URL))
	result, err := c.SearchInstruments(context.Background(), "AA", "symbol-regex")

	require.NoError(t, err)
	require.Len(t, result, 3)
}

func TestSearchInstruments_EmptyResult(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(models.InstrumentResponse{Instruments: []models.Instrument{}}))
	}))
	defer srv.Close()

	c := NewClient("test-token", WithBaseURL(srv.URL))
	result, err := c.SearchInstruments(context.Background(), "ZZZZZ", "symbol-search")

	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestGetInstrument_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/marketdata/v1/instruments/037833100", r.URL.Path)
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		// The Schwab API returns {"instruments": [...]} even for single-CUSIP lookups.
		response := models.InstrumentResponse{
			Instruments: []models.Instrument{
				{
					Cusip:       ptr.To("037833100"),
					Symbol:      ptr.To("AAPL"),
					Description: ptr.To("Apple Inc"),
					Exchange:    ptr.To("NASDAQ"),
					AssetType:   ptr.To("EQUITY"),
				},
			},
		}
		require.NoError(t, json.NewEncoder(w).Encode(response))
	}))
	defer srv.Close()

	c := NewClient("test-token", WithBaseURL(srv.URL))
	result, err := c.GetInstrument(context.Background(), "037833100")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "037833100", *result.Cusip)
	assert.Equal(t, "AAPL", *result.Symbol)
	assert.Equal(t, "Apple Inc", *result.Description)
}

func TestGetInstrument_404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"not found"}`))
	}))
	defer srv.Close()

	c := NewClient("test-token", WithBaseURL(srv.URL))
	result, err := c.GetInstrument(context.Background(), "000000000")

	require.Error(t, err)
	assert.Nil(t, result)
}
