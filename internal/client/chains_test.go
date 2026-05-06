package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/major/schwab-go/schwab/marketdata"

	"github.com/major/schwab-agent/internal/models"
)

func TestOptionChainUsesSchwabGoParamsAndStableChainFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/marketdata/v1/chains", r.URL.Path)

		q := r.URL.Query()
		assert.Equal(t, "AAPL", q.Get("symbol"))
		assert.Equal(t, "CALL", q.Get("contractType"))
		assert.Equal(t, "10", q.Get("strikeCount"))
		assert.Equal(t, "true", q.Get("includeUnderlyingQuote"))
		assert.Equal(t, "SINGLE", q.Get("strategy"))
		assert.Equal(t, "5", q.Get("interval"))
		assert.Equal(t, "150", q.Get("strike"))
		assert.Equal(t, "NTM", q.Get("range"))
		assert.Equal(t, "2026-01-16", q.Get("fromDate"))
		assert.Equal(t, "2026-01-16", q.Get("toDate"))
		assert.Equal(t, "30.5", q.Get("volatility"))
		assert.Equal(t, "148.5", q.Get("underlyingPrice"))
		assert.Equal(t, "4.5", q.Get("interestRate"))
		assert.Equal(t, "45", q.Get("daysToExpiration"))

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"symbol": "AAPL",
			"status": "SUCCESS",
			"underlyingPrice": 150.25,
			"numberOfContracts": 1,
			"callExpDateMap": {
				"2026-01-16:257": {
					"150.0": [{
						"symbol": "AAPL  260116C00150000",
						"strikePrice": 150,
						"bid": 5.50,
						"ask": 5.80,
						"mark": 5.65,
						"inTheMoney": true
					}]
				}
			}
		}`))
	}))
	t.Cleanup(server.Close)

	c := NewClient("test-token", WithBaseURL(server.URL))
	result, err := c.OptionChain(context.Background(), &marketdata.OptionChainParams{
		Symbol:                 "AAPL",
		ContractType:           marketdata.OptionChainContractTypeCall,
		StrikeCount:            10,
		IncludeUnderlyingQuote: true,
		Strategy:               marketdata.OptionChainStrategySingle,
		Interval:               5,
		Strike:                 150,
		Range:                  marketdata.OptionChainRangeNearTheMoney,
		FromDate:               "2026-01-16",
		ToDate:                 "2026-01-16",
		Volatility:             30.5,
		UnderlyingPrice:        148.5,
		InterestRate:           4.5,
		DaysToExpiration:       45,
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "AAPL", *result.Symbol)
	assert.Equal(t, "SUCCESS", *result.Status)
	assert.InDelta(t, 150.25, *result.UnderlyingPrice, 0.001)
	assert.Equal(t, 1, *result.NumberOfContracts)

	contracts := result.CallExpDateMap["2026-01-16:257"]["150.0"]
	require.Len(t, contracts, 1)
	assert.InDelta(t, 5.50, *contracts[0].Bid, 0.001)
	assert.InDelta(t, 5.80, *contracts[0].Ask, 0.001)
	assert.InDelta(t, 5.65, *contracts[0].Mark, 0.001)
	assert.True(t, *contracts[0].InTheMoney)
}

func TestOptionContractUnmarshalAcceptsSchwabGoPriceNames(t *testing.T) {
	var contract models.OptionContract
	err := json.Unmarshal([]byte(`{
		"symbol": "AAPL  260116C00150000",
		"bidPrice": 5.50,
		"askPrice": 5.80,
		"lastPrice": 5.60,
		"markPrice": 5.65,
		"isInTheMoney": true,
		"isMini": false,
		"isNonStandard": true,
		"isPennyPilot": true
	}`), &contract)

	require.NoError(t, err)
	assert.InDelta(t, 5.50, *contract.Bid, 0.001)
	assert.InDelta(t, 5.80, *contract.Ask, 0.001)
	assert.InDelta(t, 5.60, *contract.Last, 0.001)
	assert.InDelta(t, 5.65, *contract.Mark, 0.001)
	assert.True(t, *contract.InTheMoney)
	assert.False(t, *contract.Mini)
	assert.True(t, *contract.NonStandard)
	assert.True(t, *contract.PennyPilot)
}

func TestExpirationChainForSymbolPreservesExpirationDateOutput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/marketdata/v1/expirationchain", r.URL.Path)
		assert.Equal(t, "AAPL", r.URL.Query().Get("symbol"))

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"expirationList":[{"expiration":"2026-01-16"}]}`))
	}))
	t.Cleanup(server.Close)

	c := NewClient("test-token", WithBaseURL(server.URL))
	result, err := c.ExpirationChainForSymbol(context.Background(), "AAPL")

	require.NoError(t, err)
	require.Len(t, result.ExpirationList, 1)
	assert.Equal(t, "2026-01-16", result.ExpirationList[0].ExpirationDate)
}

func TestOptionChainQueryParamsOmitsZeroValues(t *testing.T) {
	params := optionChainQueryParams(&marketdata.OptionChainParams{Symbol: "AAPL"})

	assert.Equal(t, map[string]string{"symbol": "AAPL"}, params)
	assert.Nil(t, optionChainQueryParams(nil))
}

func TestExpirationDateUnmarshalPrefersExpirationDate(t *testing.T) {
	var expiration ExpirationDate
	err := json.Unmarshal([]byte(`{"expirationDate":"2026-01-16","expiration":"2026-02-20"}`), &expiration)

	require.NoError(t, err)
	assert.Equal(t, "2026-01-16", expiration.ExpirationDate)
}
