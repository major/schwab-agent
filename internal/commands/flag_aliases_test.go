package commands

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/major/schwab-agent/internal/apperr"
)

func TestRegisterOrderFlagAliases(t *testing.T) {
	t.Run("registers --instruction alias when --action exists", func(t *testing.T) {
		// Arrange
		cmd := &cobra.Command{Use: "test"}
		cmd.Flags().String("action", "", "Order action")

		// Act
		registerOrderFlagAliases(cmd)

		// Assert
		instructionFlag := cmd.Flags().Lookup("instruction")
		require.NotNil(t, instructionFlag, "--instruction flag should be registered")
		assert.Equal(t, "alias for --action (order instruction)", instructionFlag.Usage)
	})

	t.Run("registers --order-type alias when --type exists", func(t *testing.T) {
		// Arrange
		cmd := &cobra.Command{Use: "test"}
		cmd.Flags().String("type", "", "Order type")

		// Act
		registerOrderFlagAliases(cmd)

		// Assert
		orderTypeFlag := cmd.Flags().Lookup("order-type")
		require.NotNil(t, orderTypeFlag, "--order-type flag should be registered")
		assert.Equal(t, "alias for --type (order type)", orderTypeFlag.Usage)
	})

	t.Run("skips --instruction alias when --action does not exist", func(t *testing.T) {
		// Arrange
		cmd := &cobra.Command{Use: "test"}

		// Act
		registerOrderFlagAliases(cmd)

		// Assert
		instructionFlag := cmd.Flags().Lookup("instruction")
		assert.Nil(t, instructionFlag, "--instruction should not be registered")
	})

	t.Run("skips --order-type alias when --type does not exist", func(t *testing.T) {
		// Arrange
		cmd := &cobra.Command{Use: "test"}

		// Act
		registerOrderFlagAliases(cmd)

		// Assert
		orderTypeFlag := cmd.Flags().Lookup("order-type")
		assert.Nil(t, orderTypeFlag, "--order-type should not be registered")
	})

	t.Run("registers both aliases when both primary flags exist", func(t *testing.T) {
		// Arrange
		cmd := &cobra.Command{Use: "test"}
		cmd.Flags().String("action", "", "Order action")
		cmd.Flags().String("type", "", "Order type")

		// Act
		registerOrderFlagAliases(cmd)

		// Assert
		instructionFlag := cmd.Flags().Lookup("instruction")
		require.NotNil(t, instructionFlag)
		orderTypeFlag := cmd.Flags().Lookup("order-type")
		require.NotNil(t, orderTypeFlag)
	})
}

func TestResolveOrderFlagAliasesViaFlags(t *testing.T) {
	t.Run("copies --instruction to --action flag before unmarshal", func(t *testing.T) {
		// Arrange
		cmd := &cobra.Command{Use: "test"}
		cmd.Flags().String("action", "", "Order action")
		registerOrderFlagAliases(cmd)
		_ = cmd.ParseFlags([]string{"--instruction", "BUY"})

		// Act
		err := resolveOrderFlagAliasesViaFlags(cmd)

		// Assert
		require.NoError(t, err)
		val, err := cmd.Flags().GetString("action")
		require.NoError(t, err)
		assert.Equal(t, "BUY", val)
	})

	t.Run("copies --order-type to --type flag before unmarshal", func(t *testing.T) {
		// Arrange
		cmd := &cobra.Command{Use: "test"}
		cmd.Flags().String("type", "", "Order type")
		registerOrderFlagAliases(cmd)
		_ = cmd.ParseFlags([]string{"--order-type", "LIMIT"})

		// Act
		err := resolveOrderFlagAliasesViaFlags(cmd)

		// Assert
		require.NoError(t, err)
		val, err := cmd.Flags().GetString("type")
		require.NoError(t, err)
		assert.Equal(t, "LIMIT", val)
	})

	t.Run("conflict: both --action and --instruction", func(t *testing.T) {
		// Arrange
		cmd := &cobra.Command{Use: "test"}
		cmd.Flags().String("action", "", "Order action")
		registerOrderFlagAliases(cmd)
		_ = cmd.ParseFlags([]string{"--action", "BUY", "--instruction", "SELL"})

		// Act
		err := resolveOrderFlagAliasesViaFlags(cmd)

		// Assert
		require.Error(t, err)
		var valErr *apperr.ValidationError
		require.ErrorAs(t, err, &valErr)
		assert.Contains(t, valErr.Error(), "--action")
		assert.Contains(t, valErr.Error(), "--instruction")
	})

	t.Run("conflict: both --type and --order-type", func(t *testing.T) {
		// Arrange
		cmd := &cobra.Command{Use: "test"}
		cmd.Flags().String("type", "", "Order type")
		registerOrderFlagAliases(cmd)
		_ = cmd.ParseFlags([]string{"--type", "LIMIT", "--order-type", "MARKET"})

		// Act
		err := resolveOrderFlagAliasesViaFlags(cmd)

		// Assert
		require.Error(t, err)
		var valErr *apperr.ValidationError
		require.ErrorAs(t, err, &valErr)
		assert.Contains(t, valErr.Error(), "--type")
		assert.Contains(t, valErr.Error(), "--order-type")
	})

	t.Run("no-op when alias flags do not exist", func(t *testing.T) {
		// Arrange - command with no alias flags registered
		cmd := &cobra.Command{Use: "test"}
		cmd.Flags().String("action", "", "Order action")
		_ = cmd.ParseFlags([]string{"--action", "BUY"})

		// Act
		err := resolveOrderFlagAliasesViaFlags(cmd)

		// Assert
		require.NoError(t, err)
	})
}

func TestRegisterOrderFlagAliasesIdempotent(t *testing.T) {
	// Verify calling registerOrderFlagAliases twice doesn't panic.
	// This is important for RegisterOrderFlagAliasesOnTree which walks
	// the entire tree including commands that may have pre-registered aliases.
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("action", "", "Order action")
	cmd.Flags().String("type", "", "Order type")

	// Act - call twice
	registerOrderFlagAliases(cmd)
	registerOrderFlagAliases(cmd)

	// Assert - no panic, flags exist exactly once
	assert.NotNil(t, cmd.Flags().Lookup("instruction"))
	assert.NotNil(t, cmd.Flags().Lookup("order-type"))
}

// newOrderBuildCmdWithAliases creates a build command with alias flags
// registered, mirroring the post-Setup registration in buildAppWithDeps.
func newOrderBuildCmdWithAliases(w *bytes.Buffer) *cobra.Command {
	cmd := newOrderBuildCmd(w)
	RegisterOrderFlagAliasesOnTree(cmd)

	return cmd
}

// TestFlagAliasIntegrationBuildEquity tests --instruction and --order-type
// aliases work end-to-end through the actual order build equity command.
func TestFlagAliasIntegrationBuildEquity(t *testing.T) {
	t.Parallel()

	t.Run("--instruction BUY produces same output as --action BUY", func(t *testing.T) {
		t.Parallel()

		// Arrange
		aliasOut := &bytes.Buffer{}
		aliasBuild := newOrderBuildCmdWithAliases(aliasOut)
		primaryOut := &bytes.Buffer{}
		primaryBuild := newOrderBuildCmdWithAliases(primaryOut)

		// Act - build with alias flags
		_, err := runTestCommand(t, aliasBuild, "equity", "--symbol", "AAPL",
			"--instruction", "BUY", "--quantity", "10", "--order-type", "LIMIT",
			"--price", "200", "--duration", "DAY")
		require.NoError(t, err)

		// Act - build with primary flags
		_, err = runTestCommand(t, primaryBuild, "equity", "--symbol", "AAPL",
			"--action", "BUY", "--quantity", "10", "--type", "LIMIT",
			"--price", "200", "--duration", "DAY")
		require.NoError(t, err)

		// Assert - both produce identical JSON
		assert.JSONEq(t, primaryOut.String(), aliasOut.String())
	})

	t.Run("--instruction SELL_SHORT works", func(t *testing.T) {
		t.Parallel()

		// Arrange
		buf := &bytes.Buffer{}
		cmd := newOrderBuildCmdWithAliases(buf)

		// Act
		_, err := runTestCommand(t, cmd, "equity", "--symbol", "AAPL",
			"--instruction", "SELL_SHORT", "--quantity", "10")

		// Assert
		require.NoError(t, err)
		var order map[string]any
		require.NoError(t, json.Unmarshal(buf.Bytes(), &order))
		legs := order["orderLegCollection"].([]any)
		leg := legs[0].(map[string]any)
		assert.Equal(t, "SELL_SHORT", leg["instruction"])
	})

	t.Run("--action and --instruction conflict returns error", func(t *testing.T) {
		t.Parallel()

		// Arrange
		buf := &bytes.Buffer{}
		cmd := newOrderBuildCmdWithAliases(buf)

		// Act
		_, err := runTestCommand(t, cmd, "equity", "--symbol", "AAPL",
			"--action", "BUY", "--instruction", "SELL", "--quantity", "10")

		// Assert - Cobra's MarkFlagsMutuallyExclusive catches this before RunE
		require.Error(t, err)
		assert.Contains(t, err.Error(), "action")
		assert.Contains(t, err.Error(), "instruction")
	})

	t.Run("--type and --order-type conflict returns error", func(t *testing.T) {
		t.Parallel()

		// Arrange
		buf := &bytes.Buffer{}
		cmd := newOrderBuildCmdWithAliases(buf)

		// Act
		_, err := runTestCommand(t, cmd, "equity", "--symbol", "AAPL",
			"--action", "BUY", "--quantity", "10",
			"--type", "LIMIT", "--order-type", "MARKET", "--price", "200")

		// Assert - Cobra's MarkFlagsMutuallyExclusive catches this before RunE
		require.Error(t, err)
		assert.Contains(t, err.Error(), "type")
		assert.Contains(t, err.Error(), "order-type")
	})
}

// TestFlagAliasIntegrationBuildOption tests aliases on the option build command.
func TestFlagAliasIntegrationBuildOption(t *testing.T) {
	t.Parallel()

	t.Run("--instruction BUY_TO_OPEN with --order-type LIMIT", func(t *testing.T) {
		t.Parallel()

		// Arrange
		buf := &bytes.Buffer{}
		cmd := newOrderBuildCmdWithAliases(buf)

		// Act
		_, err := runTestCommand(t, cmd, "option",
			"--underlying", "AAPL", "--expiration", "2026-06-19",
			"--strike", "200", "--call",
			"--instruction", "BUY_TO_OPEN", "--quantity", "1",
			"--order-type", "LIMIT", "--price", "5.00")

		// Assert
		require.NoError(t, err)
		var order map[string]any
		require.NoError(t, json.Unmarshal(buf.Bytes(), &order))
		assert.Equal(t, "LIMIT", order["orderType"])
		legs := order["orderLegCollection"].([]any)
		leg := legs[0].(map[string]any)
		assert.Equal(t, "BUY_TO_OPEN", leg["instruction"])
	})
}

// TestFlagAliasIntegrationBuildBracket tests aliases on the bracket build command.
func TestFlagAliasIntegrationBuildBracket(t *testing.T) {
	t.Parallel()

	t.Run("--instruction BUY with --order-type MARKET", func(t *testing.T) {
		t.Parallel()

		// Arrange
		buf := &bytes.Buffer{}
		cmd := newOrderBuildCmdWithAliases(buf)

		// Act
		_, err := runTestCommand(t, cmd, "bracket",
			"--symbol", "NVDA",
			"--instruction", "BUY", "--quantity", "10",
			"--order-type", "MARKET",
			"--take-profit", "150", "--stop-loss", "120")

		// Assert
		require.NoError(t, err)
		var order map[string]any
		require.NoError(t, json.Unmarshal(buf.Bytes(), &order))
		assert.Equal(t, "MARKET", order["orderType"])
	})
}

// TestFlagAliasIntegrationBuildOCO tests aliases on the OCO build command.
// OCO has --action but no --type, so only --instruction applies.
func TestFlagAliasIntegrationBuildOCO(t *testing.T) {
	t.Parallel()

	t.Run("--instruction SELL works (OCO has no --type)", func(t *testing.T) {
		t.Parallel()

		// Arrange
		buf := &bytes.Buffer{}
		cmd := newOrderBuildCmdWithAliases(buf)

		// Act
		_, err := runTestCommand(t, cmd, "oco",
			"--symbol", "AAPL",
			"--instruction", "SELL", "--quantity", "100",
			"--take-profit", "160", "--stop-loss", "140")

		// Assert
		require.NoError(t, err)
		var order map[string]any
		require.NoError(t, json.Unmarshal(buf.Bytes(), &order))
		// OCO is an OCA strategy, verify it parsed correctly
		assert.Equal(t, "OCO", order["orderStrategyType"])
	})
}

// TestFlagAliasNotAppliedToMultiLeg confirms multi-leg strategies do NOT get
// --instruction or --order-type aliases. These strategies use --open/--close
// and --buy/--sell instead.
func TestFlagAliasNotAppliedToMultiLeg(t *testing.T) {
	t.Parallel()

	strategies := []string{
		"vertical", "iron-condor", "straddle", "strangle",
		"covered-call", "collar", "calendar", "diagonal",
		"butterfly", "condor", "vertical-roll", "back-ratio",
		"double-diagonal",
	}

	for _, strategy := range strategies {
		t.Run(strategy+" has no --instruction flag", func(t *testing.T) {
			t.Parallel()

			// Arrange
			buf := &bytes.Buffer{}
			cmd := newOrderBuildCmdWithAliases(buf)

			// Act - find the subcommand and check its flags
			sub, _, err := cmd.Find([]string{strategy})
			require.NoError(t, err, "subcommand %s should exist", strategy)

			// Assert
			assert.Nil(t, sub.Flags().Lookup("instruction"),
				"%s should NOT have --instruction flag", strategy)
			assert.Nil(t, sub.Flags().Lookup("order-type"),
				"%s should NOT have --order-type flag", strategy)
		})
	}
}
