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

func TestOptionChain_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/marketdata/v1/chains", r.URL.Path)
		assert.Equal(t, "AAPL", r.URL.Query().Get("symbol"))
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		response := models.OptionChain{
			Symbol:          new("AAPL"),
			Status:          new("SUCCESS"),
			UnderlyingPrice: new(150.25),
			IsDelayed:       new(false),
		}
		assert.NoError(t, json.NewEncoder(w).Encode(response))
	}))
	defer srv.Close()

	c := NewClient("test-token", WithBaseURL(srv.URL))
	result, err := c.OptionChain(context.Background(), "AAPL", &ChainParams{})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "AAPL", *result.Symbol)
	assert.Equal(t, "SUCCESS", *result.Status)
	assert.InDelta(t, 150.25, *result.UnderlyingPrice, 0.001)
}

func TestOptionChain_AllParams(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		assert.Equal(t, "AAPL", q.Get("symbol"))
		assert.Equal(t, "CALL", q.Get("contractType"))
		assert.Equal(t, "10", q.Get("strikeCount"))
		assert.Equal(t, "ANALYTICAL", q.Get("strategy"))
		assert.Equal(t, "2025-01-01", q.Get("fromDate"))
		assert.Equal(t, "2025-06-30", q.Get("toDate"))

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(models.OptionChain{Symbol: new("AAPL")}))
	}))
	defer srv.Close()

	c := NewClient("test-token", WithBaseURL(srv.URL))
	result, err := c.OptionChain(context.Background(), "AAPL", &ChainParams{
		ContractType: "CALL",
		StrikeCount:  "10",
		Strategy:     "ANALYTICAL",
		FromDate:     "2025-01-01",
		ToDate:       "2025-06-30",
	})

	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestOptionChain_AdvancedParams(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		assert.Equal(t, "AAPL", q.Get("symbol"))
		assert.Equal(t, "ANALYTICAL", q.Get("strategy"))
		assert.Equal(t, "true", q.Get("includeUnderlyingQuote"))
		assert.Equal(t, "5.0", q.Get("interval"))
		assert.Equal(t, "150.0", q.Get("strike"))
		assert.Equal(t, "NTM", q.Get("range"))
		assert.Equal(t, "30.5", q.Get("volatility"))
		assert.Equal(t, "148.50", q.Get("underlyingPrice"))
		assert.Equal(t, "4.5", q.Get("interestRate"))
		assert.Equal(t, "45", q.Get("daysToExpiration"))

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(models.OptionChain{Symbol: new("AAPL")}))
	}))
	defer srv.Close()

	c := NewClient("test-token", WithBaseURL(srv.URL))
	result, err := c.OptionChain(context.Background(), "AAPL", &ChainParams{
		Strategy:               "ANALYTICAL",
		IncludeUnderlyingQuote: "true",
		Interval:               "5.0",
		Strike:                 "150.0",
		StrikeRange:            "NTM",
		Volatility:             "30.5",
		UnderlyingPrice:        "148.50",
		InterestRate:           "4.5",
		DaysToExpiration:       "45",
	})

	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestOptionChain_EmptyParams(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		// Only symbol should be present
		assert.Equal(t, "AAPL", q.Get("symbol"))
		assert.Empty(t, q.Get("contractType"))
		assert.Empty(t, q.Get("strikeCount"))
		assert.Empty(t, q.Get("strategy"))
		assert.Empty(t, q.Get("fromDate"))
		assert.Empty(t, q.Get("toDate"))
		assert.Empty(t, q.Get("includeUnderlyingQuote"))
		assert.Empty(t, q.Get("interval"))
		assert.Empty(t, q.Get("strike"))
		assert.Empty(t, q.Get("range"))
		assert.Empty(t, q.Get("volatility"))
		assert.Empty(t, q.Get("underlyingPrice"))
		assert.Empty(t, q.Get("interestRate"))
		assert.Empty(t, q.Get("daysToExpiration"))

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(models.OptionChain{Symbol: new("AAPL")}))
	}))
	defer srv.Close()

	c := NewClient("test-token", WithBaseURL(srv.URL))
	result, err := c.OptionChain(context.Background(), "AAPL", &ChainParams{})

	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestOptionChain_WithCallExpDateMap(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		response := models.OptionChain{
			Symbol: new("AAPL"),
			CallExpDateMap: map[string]map[string][]*models.OptionContract{
				"2025-06-20:30": {
					"150.0": {
						{
							Symbol:      new("AAPL_062025C150"),
							StrikePrice: new(150.0),
							Bid:         new(5.50),
							Ask:         new(5.80),
						},
					},
				},
			},
		}
		assert.NoError(t, json.NewEncoder(w).Encode(response))
	}))
	defer srv.Close()

	c := NewClient("test-token", WithBaseURL(srv.URL))
	result, err := c.OptionChain(context.Background(), "AAPL", &ChainParams{ContractType: "CALL"})

	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.CallExpDateMap)
	contracts := result.CallExpDateMap["2025-06-20:30"]["150.0"]
	require.Len(t, contracts, 1)
	assert.InDelta(t, 150.0, *contracts[0].StrikePrice, 0.001)
}

func TestExpirationChainForSymbol_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/marketdata/v1/expirationchain", r.URL.Path)
		assert.Equal(t, "AAPL", r.URL.Query().Get("symbol"))

		w.Header().Set("Content-Type", "application/json")
		response := ExpirationChain{
			ExpirationList: []ExpirationDate{
				{ExpirationDate: "2025-06-20"},
				{ExpirationDate: "2025-07-18"},
				{ExpirationDate: "2025-09-19"},
			},
		}
		assert.NoError(t, json.NewEncoder(w).Encode(response))
	}))
	defer srv.Close()

	c := NewClient("test-token", WithBaseURL(srv.URL))
	result, err := c.ExpirationChainForSymbol(context.Background(), "AAPL")

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result.ExpirationList, 3)
	assert.Equal(t, "2025-06-20", result.ExpirationList[0].ExpirationDate)
	assert.Equal(t, "2025-07-18", result.ExpirationList[1].ExpirationDate)
	assert.Equal(t, "2025-09-19", result.ExpirationList[2].ExpirationDate)
}

func TestExpirationChainForSymbol_EmptyList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(ExpirationChain{}))
	}))
	defer srv.Close()

	c := NewClient("test-token", WithBaseURL(srv.URL))
	result, err := c.ExpirationChainForSymbol(context.Background(), "AAPL")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Empty(t, result.ExpirationList)
}
