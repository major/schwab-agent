package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// porcelainCmdNames extracts command names from a slice of cobra commands.
func porcelainCmdNames(cmds []*cobra.Command) []string {
	names := make([]string, len(cmds))
	for i, cmd := range cmds {
		names[i] = cmd.Name()
	}
	return names
}

// TestPorcelainSingleLegBuild verifies all four single-leg porcelain build
// commands produce correct order JSON with the right hardcoded direction.
func TestPorcelainSingleLegBuild(t *testing.T) {
	t.Parallel()

	tests := []struct {
		subcmd          string
		wantInstruction string
		wantPutCall     string
	}{
		{subcmd: "long-call", wantInstruction: "BUY_TO_OPEN", wantPutCall: "CALL"},
		{subcmd: "long-put", wantInstruction: "BUY_TO_OPEN", wantPutCall: "PUT"},
		{subcmd: "cash-secured-put", wantInstruction: "SELL_TO_OPEN", wantPutCall: "PUT"},
		{subcmd: "naked-call", wantInstruction: "SELL_TO_OPEN", wantPutCall: "CALL"},
	}

	for _, tt := range tests {
		t.Run(tt.subcmd, func(t *testing.T) {
			t.Parallel()

			// Arrange
			buf := &bytes.Buffer{}
			cmd := newOrderBuildCmd(buf)

			// Act
			_, err := runTestCommand(t, cmd,
				tt.subcmd,
				"--underlying", "AAPL",
				"--expiration", testFutureExpDate(),
				"--strike", "200",
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
			assert.Equal(t, tt.wantInstruction, leg["instruction"])

			instrument := leg["instrument"].(map[string]any)
			assert.Equal(t, "OPTION", instrument["assetType"])
			assert.Equal(t, "AAPL", instrument["underlyingSymbol"])
			assert.Equal(t, tt.wantPutCall, instrument["putCall"])
		})
	}
}

// TestPorcelainSingleLegMarketDefault verifies single-leg porcelain defaults
// to MARKET when no --type is specified.
func TestPorcelainSingleLegMarketDefault(t *testing.T) {
	t.Parallel()

	// Arrange
	buf := &bytes.Buffer{}
	cmd := newOrderBuildCmd(buf)

	// Act - use the first single-leg config to test default order type.
	configs := singleLegConfigs()
	_, err := runTestCommand(t, cmd,
		configs[0].name,
		"--underlying", "AAPL",
		"--expiration", testFutureExpDate(),
		"--strike", "200",
		"--quantity", "1",
	)

	// Assert
	require.NoError(t, err)

	var order map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &order))
	assert.Equal(t, "MARKET", order["orderType"])
}

// TestPorcelainVerticalBuild verifies all four vertical spread porcelain build
// commands produce correct order JSON with the right strike mapping.
func TestPorcelainVerticalBuild(t *testing.T) {
	t.Parallel()

	tests := []struct {
		subcmd        string
		wantOrderType string
	}{
		{subcmd: "put-credit-spread", wantOrderType: "NET_CREDIT"},
		{subcmd: "call-credit-spread", wantOrderType: "NET_CREDIT"},
		{subcmd: "put-debit-spread", wantOrderType: "NET_DEBIT"},
		{subcmd: "call-debit-spread", wantOrderType: "NET_DEBIT"},
	}

	for _, tt := range tests {
		t.Run(tt.subcmd, func(t *testing.T) {
			t.Parallel()

			// Arrange
			buf := &bytes.Buffer{}
			cmd := newOrderBuildCmd(buf)

			// Act - high=14, low=12
			_, err := runTestCommand(t, cmd,
				tt.subcmd,
				"--underlying", "F",
				"--expiration", testFutureExpDate(),
				"--high-strike", "14",
				"--low-strike", "12",
				"--quantity", "1",
				"--price", "0.50",
			)

			// Assert
			require.NoError(t, err)

			var order map[string]any
			require.NoError(t, json.Unmarshal(buf.Bytes(), &order))

			assert.Equal(t, tt.wantOrderType, order["orderType"])
			assert.Equal(t, "VERTICAL", order["complexOrderStrategyType"])

			legs, ok := order["orderLegCollection"].([]any)
			require.True(t, ok)
			require.Len(t, legs, 2, "vertical spread should have 2 legs")

			// Long leg is BUY_TO_OPEN, short leg is SELL_TO_OPEN.
			longLeg := legs[0].(map[string]any)
			assert.Equal(t, "BUY_TO_OPEN", longLeg["instruction"])

			shortLeg := legs[1].(map[string]any)
			assert.Equal(t, "SELL_TO_OPEN", shortLeg["instruction"])
		})
	}
}

// TestPorcelainVerticalOptsValidate verifies the porcelainVerticalOpts.Validate
// method catches high <= low strike.
func TestPorcelainVerticalOptsValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		high    float64
		low     float64
		wantErr bool
	}{
		{name: "valid", high: 14, low: 12, wantErr: false},
		{name: "equal strikes", high: 12, low: 12, wantErr: true},
		{name: "reversed strikes", high: 10, low: 14, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			opts := &porcelainVerticalOpts{
				HighStrike: tt.high,
				LowStrike:  tt.low,
			}
			errs := opts.Validate(context.Background())

			if tt.wantErr {
				require.Len(t, errs, 1)
				assert.Contains(t, errs[0].Error(), "high-strike")
			} else {
				require.Empty(t, errs)
			}
		})
	}
}

// TestPorcelainStraddleBuild verifies long and short straddle porcelain commands.
func TestPorcelainStraddleBuild(t *testing.T) {
	t.Parallel()

	tests := []struct {
		subcmd        string
		wantOrderType string
		wantInstr     string // both legs same instruction
	}{
		{subcmd: "long-straddle", wantOrderType: "NET_DEBIT", wantInstr: "BUY_TO_OPEN"},
		{subcmd: "short-straddle", wantOrderType: "NET_CREDIT", wantInstr: "SELL_TO_OPEN"},
	}

	for _, tt := range tests {
		t.Run(tt.subcmd, func(t *testing.T) {
			t.Parallel()

			// Arrange
			buf := &bytes.Buffer{}
			cmd := newOrderBuildCmd(buf)

			// Act
			_, err := runTestCommand(t, cmd,
				tt.subcmd,
				"--underlying", "F",
				"--expiration", testFutureExpDate(),
				"--strike", "12",
				"--quantity", "1",
				"--price", "1.50",
			)

			// Assert
			require.NoError(t, err)

			var order map[string]any
			require.NoError(t, json.Unmarshal(buf.Bytes(), &order))

			assert.Equal(t, tt.wantOrderType, order["orderType"])
			assert.Equal(t, "STRADDLE", order["complexOrderStrategyType"])

			legs, ok := order["orderLegCollection"].([]any)
			require.True(t, ok)
			require.Len(t, legs, 2, "straddle should have 2 legs")

			for i, l := range legs {
				leg := l.(map[string]any)
				assert.Equal(t, tt.wantInstr, leg["instruction"], "leg %d", i)
			}
		})
	}
}

// TestPorcelainStrangleBuild verifies long and short strangle porcelain commands.
func TestPorcelainStrangleBuild(t *testing.T) {
	t.Parallel()

	tests := []struct {
		subcmd        string
		wantOrderType string
		wantInstr     string
	}{
		{subcmd: "long-strangle", wantOrderType: "NET_DEBIT", wantInstr: "BUY_TO_OPEN"},
		{subcmd: "short-strangle", wantOrderType: "NET_CREDIT", wantInstr: "SELL_TO_OPEN"},
	}

	for _, tt := range tests {
		t.Run(tt.subcmd, func(t *testing.T) {
			t.Parallel()

			// Arrange
			buf := &bytes.Buffer{}
			cmd := newOrderBuildCmd(buf)

			// Act
			_, err := runTestCommand(t, cmd,
				tt.subcmd,
				"--underlying", "F",
				"--expiration", testFutureExpDate(),
				"--call-strike", "14",
				"--put-strike", "10",
				"--quantity", "1",
				"--price", "0.50",
			)

			// Assert
			require.NoError(t, err)

			var order map[string]any
			require.NoError(t, json.Unmarshal(buf.Bytes(), &order))

			assert.Equal(t, tt.wantOrderType, order["orderType"])
			assert.Equal(t, "STRANGLE", order["complexOrderStrategyType"])

			legs, ok := order["orderLegCollection"].([]any)
			require.True(t, ok)
			require.Len(t, legs, 2, "strangle should have 2 legs")

			for i, l := range legs {
				leg := l.(map[string]any)
				assert.Equal(t, tt.wantInstr, leg["instruction"], "leg %d", i)
			}
		})
	}
}

// TestPorcelainJadeLizardBuild verifies the jade lizard porcelain build command.
func TestPorcelainJadeLizardBuild(t *testing.T) {
	t.Parallel()

	// Arrange
	buf := &bytes.Buffer{}
	cmd := newOrderBuildCmd(buf)

	// Act
	_, err := runTestCommand(t, cmd,
		"jade-lizard",
		"--underlying", "F",
		"--expiration", testFutureExpDate(),
		"--put-strike", "10",
		"--short-call-strike", "14",
		"--long-call-strike", "16",
		"--quantity", "1",
		"--price", "1.00",
	)

	// Assert
	require.NoError(t, err)

	var order map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &order))

	assert.Equal(t, "NET_CREDIT", order["orderType"])
	assert.Equal(t, "CUSTOM", order["complexOrderStrategyType"])

	legs, ok := order["orderLegCollection"].([]any)
	require.True(t, ok)
	require.Len(t, legs, 3, "jade lizard should have 3 legs")

	// Leg 1: short put (SELL_TO_OPEN).
	putLeg := legs[0].(map[string]any)
	assert.Equal(t, "SELL_TO_OPEN", putLeg["instruction"])

	// Leg 2: short call (SELL_TO_OPEN).
	shortCallLeg := legs[1].(map[string]any)
	assert.Equal(t, "SELL_TO_OPEN", shortCallLeg["instruction"])

	// Leg 3: long call protection (BUY_TO_OPEN).
	longCallLeg := legs[2].(map[string]any)
	assert.Equal(t, "BUY_TO_OPEN", longCallLeg["instruction"])
}

// TestPorcelainCommandRegistrationConsistency verifies that all 13 porcelain
// commands are registered consistently under build, place, and preview. Uses
// command Name() method instead of hardcoded strings to avoid goconst threshold
// issues from cross-file counting.
func TestPorcelainCommandRegistrationConsistency(t *testing.T) {
	t.Parallel()

	buildCmds := orderBuildPorcelainCommands(io.Discard)
	placeCmds := orderPlacePorcelainCommands(nil, "", io.Discard)
	previewCmds := orderPreviewPorcelainCommands(nil, "", io.Discard)

	// All three groups should have the same 13 commands.
	require.Len(t, buildCmds, 13)
	require.Len(t, placeCmds, 13)
	require.Len(t, previewCmds, 13)

	// Verify all three groups have matching command names.
	buildNames := porcelainCmdNames(buildCmds)
	placeNames := porcelainCmdNames(placeCmds)
	previewNames := porcelainCmdNames(previewCmds)

	assert.ElementsMatch(t, buildNames, placeNames)
	assert.ElementsMatch(t, buildNames, previewNames)
}
