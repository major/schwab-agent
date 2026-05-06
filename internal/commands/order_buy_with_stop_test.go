package commands

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBuyWithStopBuild verifies the BUY-only bracket helper builds the expected
// TRIGGER entry order and protective child exits.
func TestBuyWithStopBuild(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		args            []string
		wantOrderType   string
		wantPrice       bool
		wantOCOChildren bool
	}{
		{
			name: "limit entry with stop loss",
			args: []string{
				"buy-with-stop",
				"--symbol",
				"AAPL",
				"--quantity",
				"10",
				"--price",
				"150",
				"--stop-loss",
				"140",
			},
			wantOrderType: "LIMIT",
			wantPrice:     true,
		},
		{
			name: "market entry with stop loss",
			args: []string{
				"buy-with-stop",
				"--symbol",
				"AAPL",
				"--quantity",
				"10",
				"--type",
				"MARKET",
				"--stop-loss",
				"140",
			},
			wantOrderType: "MARKET",
		},
		{
			name: "limit entry with stop loss and take profit",
			args: []string{
				"buy-with-stop",
				"--symbol",
				"AAPL",
				"--quantity",
				"10",
				"--price",
				"150",
				"--stop-loss",
				"140",
				"--take-profit",
				"170",
			},
			wantOrderType:   "LIMIT",
			wantPrice:       true,
			wantOCOChildren: true,
		},
		{
			name: "order type alias",
			args: []string{
				"buy-with-stop",
				"--symbol",
				"AAPL",
				"--quantity",
				"10",
				"--order-type",
				"MARKET",
				"--stop-loss",
				"140",
			},
			wantOrderType: "MARKET",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			buf := &bytes.Buffer{}
			cmd := newOrderBuildCmd(buf)
			RegisterOrderFlagAliasesOnTree(cmd)

			// Act
			_, err := runTestCommand(t, cmd, tt.args...)

			// Assert
			require.NoError(t, err)

			var order map[string]any
			require.NoError(t, json.Unmarshal(buf.Bytes(), &order))

			assert.Equal(t, tt.wantOrderType, order["orderType"])
			assert.Equal(t, "TRIGGER", order["orderStrategyType"])
			assert.Equal(t, "DAY", order["duration"])
			if tt.wantPrice {
				assert.Equal(t, 150.0, order["price"])
			} else {
				assert.NotContains(t, order, "price")
			}

			legs, ok := order["orderLegCollection"].([]any)
			require.True(t, ok, "orderLegCollection should be an array")
			require.Len(t, legs, 1)

			entryLeg := legs[0].(map[string]any)
			assert.Equal(t, "BUY", entryLeg["instruction"])
			assert.Equal(t, 10.0, entryLeg["quantity"])

			children, ok := order["childOrderStrategies"].([]any)
			require.True(t, ok, "buy-with-stop should have childOrderStrategies")
			require.Len(t, children, 1)

			child := children[0].(map[string]any)
			if tt.wantOCOChildren {
				assert.Equal(t, "OCO", child["orderStrategyType"])
				exits, ok := child["childOrderStrategies"].([]any)
				require.True(t, ok, "take-profit plus stop-loss should create OCO exits")
				require.Len(t, exits, 2)
				for _, exit := range exits {
					exitOrder := exit.(map[string]any)
					assert.Equal(t, "GOOD_TILL_CANCEL", exitOrder["duration"])
				}
			} else {
				assert.Equal(t, "STOP", child["orderType"])
				assert.Equal(t, "GOOD_TILL_CANCEL", child["duration"])
			}
		})
	}
}

// TestBuyWithStopValidationErrors verifies invalid buy-with-stop inputs fail
// before any order JSON is emitted.
func TestBuyWithStopValidationErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		args    []string
		wantMsg string
	}{
		{
			name:    "missing stop loss",
			args:    []string{"buy-with-stop", "--symbol", "AAPL", "--quantity", "10", "--price", "150"},
			wantMsg: "stop-loss",
		},
		{
			name: "stop loss above price",
			args: []string{
				"buy-with-stop",
				"--symbol",
				"AAPL",
				"--quantity",
				"10",
				"--price",
				"150",
				"--stop-loss",
				"160",
			},
			wantMsg: "stop-loss must be below entry price",
		},
		{
			name: "unsupported entry type",
			args: []string{
				"buy-with-stop",
				"--symbol",
				"AAPL",
				"--quantity",
				"10",
				"--type",
				"STOP",
				"--stop-loss",
				"140",
			},
			wantMsg: "only LIMIT and MARKET entry orders are supported",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			buf := &bytes.Buffer{}
			cmd := newOrderBuildCmd(buf)
			RegisterOrderFlagAliasesOnTree(cmd)

			// Act
			_, err := runTestCommand(t, cmd, tt.args...)

			// Assert
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantMsg)
			assert.Empty(t, buf.String())
		})
	}
}
