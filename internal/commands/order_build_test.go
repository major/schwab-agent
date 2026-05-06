package commands

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/major/schwab-agent/internal/apperr"
)

// TestOrderBuildEquity verifies the equity build subcommand produces valid order JSON.
func TestOrderBuildEquity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		args          []string
		wantOrderType string
		wantAction    string
		wantQuantity  float64
		wantSymbol    string
	}{
		{
			name: "limit order",
			args: []string{
				"equity",
				"--symbol",
				"AAPL",
				"--action",
				"BUY",
				"--quantity",
				"10",
				"--type",
				"LIMIT",
				"--price",
				"200",
				"--duration",
				"DAY",
			},
			wantOrderType: "LIMIT",
			wantAction:    "BUY",
			wantQuantity:  10,
			wantSymbol:    "AAPL",
		},
		{
			name:          "market order defaults",
			args:          []string{"equity", "--symbol", "MSFT", "--action", "SELL", "--quantity", "5"},
			wantOrderType: "MARKET",
			wantAction:    "SELL",
			wantQuantity:  5,
			wantSymbol:    "MSFT",
		},
		{
			name: "stop order",
			args: []string{
				"equity",
				"--symbol",
				"TSLA",
				"--action",
				"SELL",
				"--quantity",
				"1",
				"--type",
				"STOP",
				"--stop-price",
				"145",
			},
			wantOrderType: "STOP",
			wantAction:    "SELL",
			wantQuantity:  1,
			wantSymbol:    "TSLA",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			buf := &bytes.Buffer{}
			cmd := newOrderBuildCmd(buf)

			// Act
			_, err := runTestCommand(t, cmd, tt.args...)

			// Assert
			require.NoError(t, err)

			var order map[string]any
			require.NoError(t, json.Unmarshal(buf.Bytes(), &order))

			assert.Equal(t, tt.wantOrderType, order["orderType"])
			assert.Equal(t, "SINGLE", order["orderStrategyType"])

			legs, ok := order["orderLegCollection"].([]any)
			require.True(t, ok, "orderLegCollection should be an array")
			require.Len(t, legs, 1)

			leg := legs[0].(map[string]any)
			assert.Equal(t, tt.wantAction, leg["instruction"])
			assert.Equal(t, tt.wantQuantity, leg["quantity"])

			instrument := leg["instrument"].(map[string]any)
			assert.Equal(t, tt.wantSymbol, instrument["symbol"])
			assert.Equal(t, "EQUITY", instrument["assetType"])
		})
	}
}

// TestOrderBuildOption verifies the option build subcommand produces valid option order JSON.
func TestOrderBuildOption(t *testing.T) {
	t.Parallel()

	// Arrange
	buf := &bytes.Buffer{}
	cmd := newOrderBuildCmd(buf)

	// Act - build a call option order
	_, err := runTestCommand(t, cmd,
		"option",
		"--underlying", "AAPL",
		"--expiration", "2027-12-17",
		"--strike", "200",
		"--call",
		"--action", "BUY_TO_OPEN",
		"--quantity", "1",
		"--type", "LIMIT",
		"--price", "5.00",
	)

	// Assert
	require.NoError(t, err)

	var order map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &order))

	assert.Equal(t, "LIMIT", order["orderType"])
	assert.Equal(t, "SINGLE", order["orderStrategyType"])

	legs, ok := order["orderLegCollection"].([]any)
	require.True(t, ok)
	require.Len(t, legs, 1)

	leg := legs[0].(map[string]any)
	assert.Equal(t, "BUY_TO_OPEN", leg["instruction"])
	assert.Equal(t, 1.0, leg["quantity"])

	instrument := leg["instrument"].(map[string]any)
	assert.Equal(t, "OPTION", instrument["assetType"])
	assert.Equal(t, "AAPL", instrument["underlyingSymbol"])
	assert.Equal(t, "CALL", instrument["putCall"])
	assert.Equal(t, 200.0, instrument["optionStrikePrice"])
}

// TestOrderBuildBracket verifies the bracket build subcommand produces triggered order JSON
// with child exit strategies.
func TestOrderBuildBracket(t *testing.T) {
	t.Parallel()

	// Arrange
	buf := &bytes.Buffer{}
	cmd := newOrderBuildCmd(buf)

	// Act - build a bracket with both take-profit and stop-loss
	_, err := runTestCommand(t, cmd,
		"bracket",
		"--symbol", "NVDA",
		"--action", "BUY",
		"--quantity", "10",
		"--type", "MARKET",
		"--take-profit", "150",
		"--stop-loss", "120",
	)

	// Assert
	require.NoError(t, err)

	var order map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &order))

	assert.Equal(t, "MARKET", order["orderType"])
	assert.Equal(t, "TRIGGER", order["orderStrategyType"])

	// Verify entry leg
	legs, ok := order["orderLegCollection"].([]any)
	require.True(t, ok)
	require.Len(t, legs, 1)

	entryLeg := legs[0].(map[string]any)
	assert.Equal(t, "BUY", entryLeg["instruction"])

	// Verify child order strategies exist (the OCO exit)
	children, ok := order["childOrderStrategies"].([]any)
	require.True(t, ok, "bracket should have childOrderStrategies")
	require.NotEmpty(t, children, "bracket should have at least one child strategy")
}

// TestOrderBuildVertical verifies the vertical spread build subcommand produces
// multi-leg order JSON.
func TestOrderBuildVertical(t *testing.T) {
	t.Parallel()

	// Arrange
	buf := &bytes.Buffer{}
	cmd := newOrderBuildCmd(buf)

	// Act - build a bull call spread (long lower strike, short higher strike)
	_, err := runTestCommand(t, cmd,
		"vertical",
		"--underlying", "F",
		"--expiration", "2027-06-18",
		"--long-strike", "12",
		"--short-strike", "14",
		"--call",
		"--open",
		"--quantity", "1",
		"--price", "0.50",
	)

	// Assert
	require.NoError(t, err)

	var order map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &order))

	// Bull call spread opening = NET_DEBIT
	assert.Equal(t, "NET_DEBIT", order["orderType"])
	assert.Equal(t, "SINGLE", order["orderStrategyType"])
	assert.Equal(t, "VERTICAL", order["complexOrderStrategyType"])

	// Vertical spreads have two legs
	legs, ok := order["orderLegCollection"].([]any)
	require.True(t, ok)
	require.Len(t, legs, 2, "vertical spread should have 2 legs")

	// First leg is the long (bought) leg
	longLeg := legs[0].(map[string]any)
	assert.Equal(t, "BUY_TO_OPEN", longLeg["instruction"])

	// Second leg is the short (sold) leg
	shortLeg := legs[1].(map[string]any)
	assert.Equal(t, "SELL_TO_OPEN", shortLeg["instruction"])
}

// TestOrderBuildOCO verifies the OCO build subcommand produces valid OCO JSON.
func TestOrderBuildOCO(t *testing.T) {
	t.Parallel()

	// Arrange
	buf := &bytes.Buffer{}
	cmd := newOrderBuildCmd(buf)

	// Act
	_, err := runTestCommand(t, cmd,
		"oco",
		"--symbol", "AAPL",
		"--action", "SELL",
		"--quantity", "100",
		"--take-profit", "160",
		"--stop-loss", "140",
	)

	// Assert
	require.NoError(t, err)

	var order map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &order))

	assert.Equal(t, "OCO", order["orderStrategyType"])

	// OCO with both exits should have child order strategies
	children, ok := order["childOrderStrategies"].([]any)
	require.True(t, ok, "OCO should have childOrderStrategies")
	require.Len(t, children, 2, "OCO with both exits should have 2 children")
}

// TestOrderBuildValidationErrors verifies that build subcommands return
// ValidationError for invalid inputs.
func TestOrderBuildValidationErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		args    []string
		wantMsg string
	}{
		{
			name:    "equity missing action",
			args:    []string{"equity", "--symbol", "AAPL", "--quantity", "10"},
			wantMsg: "action",
		},
		{
			name:    "equity limit without price",
			args:    []string{"equity", "--symbol", "AAPL", "--action", "BUY", "--quantity", "10", "--type", "LIMIT"},
			wantMsg: "LIMIT order requires a price",
		},
		{
			name:    "equity zero quantity",
			args:    []string{"equity", "--symbol", "AAPL", "--action", "BUY", "--quantity", "0"},
			wantMsg: "quantity must be greater than zero",
		},
		{
			name: "option missing call or put",
			args: []string{
				"option",
				"--underlying",
				"AAPL",
				"--expiration",
				"2027-12-17",
				"--strike",
				"200",
				"--action",
				"BUY_TO_OPEN",
				"--quantity",
				"1",
			},
			wantMsg: "at least one of the flags in the group [call put] is required",
		},
		{
			name:    "bracket missing exit conditions",
			args:    []string{"bracket", "--symbol", "NVDA", "--action", "BUY", "--quantity", "10"},
			wantMsg: "bracket order requires at least one exit condition",
		},
		{
			name: "vertical missing call or put",
			args: []string{
				"vertical",
				"--underlying",
				"F",
				"--expiration",
				"2027-06-18",
				"--long-strike",
				"12",
				"--short-strike",
				"14",
				"--open",
				"--quantity",
				"1",
				"--price",
				"0.50",
			},
			wantMsg: "at least one of the flags in the group [call put] is required",
		},
		{
			name:    "oco missing exit conditions",
			args:    []string{"oco", "--symbol", "AAPL", "--action", "SELL", "--quantity", "100"},
			wantMsg: "OCO order requires at least one exit condition",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			buf := &bytes.Buffer{}
			cmd := newOrderBuildCmd(buf)

			// Act
			_, err := runTestCommand(t, cmd, tt.args...)

			// Assert
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantMsg)
		})
	}
}

// TestOrderBuildValidationErrorType verifies that validation failures return
// the correct typed error from the apperr package.
func TestOrderBuildValidationErrorType(t *testing.T) {
	t.Parallel()

	// Arrange - equity build with zero quantity triggers a ValidationError
	buf := &bytes.Buffer{}
	cmd := newOrderBuildCmd(buf)

	// Act
	_, err := runTestCommand(t, cmd, "equity", "--symbol", "AAPL", "--action", "BUY", "--quantity", "0")

	// Assert
	var valErr *apperr.ValidationError
	require.ErrorAs(t, err, &valErr)
	assert.Contains(t, valErr.Error(), "quantity must be greater than zero")
}

// TestOrderBuildNoSubcommand verifies that running build without a subcommand
// returns a helpful ValidationError listing available subcommands.
func TestOrderBuildNoSubcommand(t *testing.T) {
	t.Parallel()

	// Arrange
	buf := &bytes.Buffer{}
	cmd := newOrderBuildCmd(buf)

	// Act
	_, err := runTestCommand(t, cmd)

	// Assert
	var valErr *apperr.ValidationError
	require.ErrorAs(t, err, &valErr)
	assert.Contains(t, valErr.Error(), "requires a subcommand")
}

// TestOrderBuildRawJSON verifies that build commands produce raw JSON output
// (not envelope-wrapped). The output should decode directly as an order object,
// not as {"data": ..., "metadata": ...}.
func TestOrderBuildRawJSON(t *testing.T) {
	t.Parallel()

	// Arrange
	buf := &bytes.Buffer{}
	cmd := newOrderBuildCmd(buf)

	// Act
	_, err := runTestCommand(t, cmd,
		"equity", "--symbol", "AAPL", "--action", "BUY", "--quantity", "10",
	)

	// Assert
	require.NoError(t, err)

	var order map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &order))

	// Raw JSON should have orderType at top level, not wrapped in a "data" key
	assert.Contains(t, order, "orderType", "output should be raw order JSON")
	assert.NotContains(t, order, "data", "output should not be envelope-wrapped")
	assert.NotContains(t, order, "metadata", "output should not be envelope-wrapped")
}
