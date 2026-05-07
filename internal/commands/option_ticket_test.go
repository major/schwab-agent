package commands

import (
	"bytes"
	"testing"
	"time"

	"github.com/major/schwab-go/schwab/marketdata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/major/schwab-agent/internal/apperr"
	"github.com/major/schwab-agent/internal/client"
	"github.com/major/schwab-agent/internal/models"
)

func TestOptionParentRequiresSubcommand(t *testing.T) {
	cmd := NewOptionCmd(&client.Ref{}, &bytes.Buffer{})

	_, err := runTestCommand(t, cmd)

	require.Error(t, err)
	var validationErr *apperr.ValidationError
	require.ErrorAs(t, err, &validationErr)
	assert.Contains(t, err.Error(), "requires a subcommand")
}

func TestOptionTicketChainParams(t *testing.T) {
	expiration := time.Date(2026, 6, 19, 0, 0, 0, 0, time.UTC)

	params := optionTicketChainParams("AAPL", expiration, models.PutCallCall, 200)

	assert.Equal(t, "AAPL", params.Symbol)
	assert.Equal(t, marketdata.OptionChainContractType(models.PutCallCall), params.ContractType)
	assert.Equal(t, marketdata.OptionChainStrategySingle, params.Strategy)
	assert.Equal(t, "2026-06-19", params.FromDate)
	assert.Equal(t, "2026-06-19", params.ToDate)
	assert.True(t, params.IncludeUnderlyingQuote)
	assert.InEpsilon(t, 200.0, params.Strike, 0.0001)
}

func TestMatchingTicketContracts(t *testing.T) {
	chain := &models.OptionChain{
		CallExpDateMap: map[string]map[string][]*models.OptionContract{
			"2026-06-19:42": {
				"200.0": {optionContractForTest("AAPL 260619C00200000", "CALL", 200)},
				"bad":   {optionContractForTest("AAPL 260619C00200001", "CALL", 200)},
			},
			"2026-07-17:70": {
				"200.0": {optionContractForTest("AAPL 260717C00200000", "CALL", 200)},
			},
		},
		PutExpDateMap: map[string]map[string][]*models.OptionContract{
			"2026-06-19:42": {
				"200.0": {optionContractForTest("AAPL 260619P00200000", "PUT", 200)},
			},
		},
	}
	expiration := time.Date(2026, 6, 19, 0, 0, 0, 0, time.UTC)

	calls := matchingTicketContracts(chain, expiration, models.PutCallCall, 200)
	puts := matchingTicketContracts(chain, expiration, models.PutCallPut, 200)

	require.Len(t, calls, 2)
	callSymbols := []string{stringValue(calls[0].Symbol), stringValue(calls[1].Symbol)}
	assert.ElementsMatch(t, []string{"AAPL 260619C00200000", "AAPL 260619C00200001"}, callSymbols)
	require.Len(t, puts, 1)
	assert.Equal(t, "AAPL 260619P00200000", stringValue(puts[0].Symbol))
	assert.Nil(t, matchingTicketContracts(nil, expiration, models.PutCallCall, 200))
}

func TestOptionTicketValueHelpers(t *testing.T) {
	text := "value"
	price := 12.5
	flag := true
	count := 7

	assert.Empty(t, contractSymbol(nil))
	assert.Equal(t, "value", stringValue(&text))
	assert.Empty(t, stringValue(nil))
	assert.InEpsilon(t, 12.5, floatValue(&price), 0.0001)
	assert.Zero(t, floatValue(nil))
	assert.True(t, boolValue(&flag))
	assert.False(t, boolValue(nil))
	assert.Equal(t, 7, intValue(&count))
	assert.Zero(t, intValue(nil))
}

func optionContractForTest(symbol, putCall string, strike float64) *models.OptionContract {
	return &models.OptionContract{
		Symbol:      &symbol,
		PutCall:     &putCall,
		StrikePrice: &strike,
	}
}
