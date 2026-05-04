package commands

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewOptionCmdTicketGet(t *testing.T) {
	// Arrange - Schwab returns quote data from one endpoint and a narrowly
	// filtered chain from another. The command should turn those two API calls
	// into one compact agent-facing ticket.
	var chainQuery map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/marketdata/v1/AAPL/quotes":
			_, _ = w.Write([]byte(`{
				"AAPL": {
					"symbol": "AAPL",
					"quote": {"lastPrice": 199.50, "bidPrice": 199.45, "askPrice": 199.55},
					"reference": {"description": "Apple Inc"}
				}
			}`))
		case "/marketdata/v1/chains":
			chainQuery = firstQueryValues(r)
			_, _ = w.Write([]byte(`{
				"symbol": "AAPL",
				"status": "SUCCESS",
				"isDelayed": false,
				"underlyingPrice": 199.5,
				"numberOfContracts": 1,
				"underlying": {"symbol": "AAPL", "last": 199.5, "mark": 199.5},
				"callExpDateMap": {
					"2026-01-16:257": {
						"200.0": [{
							"putCall": "CALL",
							"symbol": "AAPL  260116C00200000",
							"description": "AAPL Jan 16 2026 200 Call",
							"bid": 12.30,
							"ask": 12.45,
							"mark": 12.375,
							"strikePrice": 200,
							"expirationDate": "2026-01-16",
							"delta": 0.52,
							"openInterest": 1234
						}]
					}
				}
			}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	buf := &bytes.Buffer{}
	cmd := NewOptionCmd(testClient(t, server), buf)
	cmd.SetArgs([]string{"ticket", "get", "AAPL", "--expiration", "2026-01-16", "--strike", "200", "--call"})

	// Act
	err := cmd.Execute()

	// Assert
	require.NoError(t, err)
	assert.Equal(t, map[string]string{
		"contractType":           "CALL",
		"fromDate":               "2026-01-16",
		"includeUnderlyingQuote": "true",
		"strategy":               "SINGLE",
		"strike":                 "200",
		"symbol":                 "AAPL",
		"toDate":                 "2026-01-16",
	}, chainQuery)

	var envelope struct {
		Data struct {
			Symbol          string  `json:"symbol"`
			Expiration      string  `json:"expiration"`
			Strike          float64 `json:"strike"`
			PutCall         string  `json:"putCall"`
			OCCSymbol       string  `json:"occSymbol"`
			UnderlyingQuote struct {
				Quote struct {
					LastPrice float64 `json:"lastPrice"`
				} `json:"quote"`
			} `json:"underlyingQuote"`
			Chain struct {
				UnderlyingPrice   float64 `json:"underlyingPrice"`
				NumberOfContracts int     `json:"numberOfContracts"`
			} `json:"chain"`
			Contracts []struct {
				Symbol string  `json:"symbol"`
				Mark   float64 `json:"mark"`
			} `json:"contracts"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(buf.Bytes(), &envelope))
	assert.Equal(t, "AAPL", envelope.Data.Symbol)
	assert.Equal(t, "2026-01-16", envelope.Data.Expiration)
	assert.Equal(t, 200.0, envelope.Data.Strike)
	assert.Equal(t, "CALL", envelope.Data.PutCall)
	assert.Equal(t, "AAPL  260116C00200000", envelope.Data.OCCSymbol)
	assert.Equal(t, 199.50, envelope.Data.UnderlyingQuote.Quote.LastPrice)
	assert.Equal(t, 199.50, envelope.Data.Chain.UnderlyingPrice)
	assert.Equal(t, 1, envelope.Data.Chain.NumberOfContracts)
	require.Len(t, envelope.Data.Contracts, 1)
	assert.Equal(t, "AAPL  260116C00200000", envelope.Data.Contracts[0].Symbol)
	assert.Equal(t, 12.375, envelope.Data.Contracts[0].Mark)
}

func TestNewOptionCmdTicketGetMissingContract(t *testing.T) {
	// Arrange - the filtered chain is valid JSON but has no matching strike.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/marketdata/v1/AAPL/quotes":
			_, _ = w.Write([]byte(`{"AAPL": {"symbol": "AAPL", "quote": {"lastPrice": 199.50}}}`))
		case "/marketdata/v1/chains":
			_, _ = w.Write([]byte(`{"symbol": "AAPL", "putExpDateMap": {}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	cmd := NewOptionCmd(testClient(t, server), &bytes.Buffer{})

	// Act
	_, err := runTestCommand(t, cmd, "ticket", "get", "AAPL", "--expiration", "2026-01-16", "--strike", "200", "--put")

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "option contract not found")
}

func firstQueryValues(r *http.Request) map[string]string {
	values := r.URL.Query()
	result := make(map[string]string, len(values))
	for key, value := range values {
		if len(value) > 0 {
			result[key] = value[0]
		}
	}
	return result
}
