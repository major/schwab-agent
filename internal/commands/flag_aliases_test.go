package commands

import (
	"testing"

	"github.com/major/schwab-agent/internal/apperr"
	"github.com/major/schwab-agent/internal/models"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		assert.Equal(t, "Alias for --action", instructionFlag.Usage)
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
		assert.Equal(t, "Alias for --type", orderTypeFlag.Usage)
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

func TestResolveOrderFlagAliases(t *testing.T) {
	t.Run("alias-only: --instruction set, --action not set", func(t *testing.T) {
		// Arrange
		cmd := &cobra.Command{Use: "test"}
		cmd.Flags().String("action", "", "Order action")
		registerOrderFlagAliases(cmd)
		cmd.SetArgs([]string{"--instruction", "BUY"})
		_ = cmd.ParseFlags([]string{"--instruction", "BUY"})

		action := models.InstructionBuy
		orderType := models.OrderTypeMarket

		// Act
		err := resolveOrderFlagAliases(cmd, &action, &orderType)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, models.InstructionBuy, action)
	})

	t.Run("alias-only: --order-type set, --type not set", func(t *testing.T) {
		// Arrange
		cmd := &cobra.Command{Use: "test"}
		cmd.Flags().String("type", "", "Order type")
		registerOrderFlagAliases(cmd)
		cmd.SetArgs([]string{"--order-type", "LIMIT"})
		_ = cmd.ParseFlags([]string{"--order-type", "LIMIT"})

		action := models.InstructionBuy
		orderType := models.OrderTypeMarket

		// Act
		err := resolveOrderFlagAliases(cmd, &action, &orderType)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, models.OrderTypeLimit, orderType)
	})

	t.Run("primary-only: --action set, --instruction not set", func(t *testing.T) {
		// Arrange
		cmd := &cobra.Command{Use: "test"}
		cmd.Flags().String("action", "", "Order action")
		registerOrderFlagAliases(cmd)
		cmd.SetArgs([]string{"--action", "SELL"})
		_ = cmd.ParseFlags([]string{"--action", "SELL"})

		action := models.InstructionBuy
		orderType := models.OrderTypeMarket

		// Act
		err := resolveOrderFlagAliases(cmd, &action, &orderType)

		// Assert
		require.NoError(t, err)
		// Primary flag should not be modified by resolveOrderFlagAliases
		assert.Equal(t, models.InstructionBuy, action)
	})

	t.Run("primary-only: --type set, --order-type not set", func(t *testing.T) {
		// Arrange
		cmd := &cobra.Command{Use: "test"}
		cmd.Flags().String("type", "", "Order type")
		registerOrderFlagAliases(cmd)
		cmd.SetArgs([]string{"--type", "STOP"})
		_ = cmd.ParseFlags([]string{"--type", "STOP"})

		action := models.InstructionBuy
		orderType := models.OrderTypeMarket

		// Act
		err := resolveOrderFlagAliases(cmd, &action, &orderType)

		// Assert
		require.NoError(t, err)
		// Primary flag should not be modified by resolveOrderFlagAliases
		assert.Equal(t, models.OrderTypeMarket, orderType)
	})

	t.Run("conflict: both --action and --instruction set", func(t *testing.T) {
		// Arrange
		cmd := &cobra.Command{Use: "test"}
		cmd.Flags().String("action", "", "Order action")
		registerOrderFlagAliases(cmd)
		cmd.SetArgs([]string{"--action", "BUY", "--instruction", "SELL"})
		_ = cmd.ParseFlags([]string{"--action", "BUY", "--instruction", "SELL"})

		action := models.InstructionBuy
		orderType := models.OrderTypeMarket

		// Act
		err := resolveOrderFlagAliases(cmd, &action, &orderType)

		// Assert
		require.Error(t, err)
		var valErr *apperr.ValidationError
		require.ErrorAs(t, err, &valErr)
		assert.Contains(t, valErr.Error(), "--action")
		assert.Contains(t, valErr.Error(), "--instruction")
	})

	t.Run("conflict: both --type and --order-type set", func(t *testing.T) {
		// Arrange
		cmd := &cobra.Command{Use: "test"}
		cmd.Flags().String("type", "", "Order type")
		registerOrderFlagAliases(cmd)
		cmd.SetArgs([]string{"--type", "LIMIT", "--order-type", "MARKET"})
		_ = cmd.ParseFlags([]string{"--type", "LIMIT", "--order-type", "MARKET"})

		action := models.InstructionBuy
		orderType := models.OrderTypeMarket

		// Act
		err := resolveOrderFlagAliases(cmd, &action, &orderType)

		// Assert
		require.Error(t, err)
		var valErr *apperr.ValidationError
		require.ErrorAs(t, err, &valErr)
		assert.Contains(t, valErr.Error(), "--type")
		assert.Contains(t, valErr.Error(), "--order-type")
	})

	t.Run("neither flag set is fine", func(t *testing.T) {
		// Arrange
		cmd := &cobra.Command{Use: "test"}
		cmd.Flags().String("action", "", "Order action")
		cmd.Flags().String("type", "", "Order type")
		registerOrderFlagAliases(cmd)
		cmd.SetArgs([]string{})
		_ = cmd.ParseFlags([]string{})

		action := models.InstructionBuy
		orderType := models.OrderTypeMarket

		// Act
		err := resolveOrderFlagAliases(cmd, &action, &orderType)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, models.InstructionBuy, action)
		assert.Equal(t, models.OrderTypeMarket, orderType)
	})

	t.Run("nil orderType skips --order-type resolution", func(t *testing.T) {
		// Arrange
		cmd := &cobra.Command{Use: "test"}
		cmd.Flags().String("type", "", "Order type")
		registerOrderFlagAliases(cmd)
		cmd.SetArgs([]string{"--order-type", "LIMIT"})
		_ = cmd.ParseFlags([]string{"--order-type", "LIMIT"})

		action := models.InstructionBuy

		// Act
		err := resolveOrderFlagAliases(cmd, &action, nil)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, models.InstructionBuy, action)
	})

	t.Run("alias value copied to action when only alias set", func(t *testing.T) {
		// Arrange
		cmd := &cobra.Command{Use: "test"}
		cmd.Flags().String("action", "", "Order action")
		registerOrderFlagAliases(cmd)
		cmd.SetArgs([]string{"--instruction", "SELL_SHORT"})
		_ = cmd.ParseFlags([]string{"--instruction", "SELL_SHORT"})

		action := models.InstructionBuy
		orderType := models.OrderTypeMarket

		// Act
		err := resolveOrderFlagAliases(cmd, &action, &orderType)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, models.InstructionSellShort, action)
	})

	t.Run("alias value copied to orderType when only alias set", func(t *testing.T) {
		// Arrange
		cmd := &cobra.Command{Use: "test"}
		cmd.Flags().String("type", "", "Order type")
		registerOrderFlagAliases(cmd)
		cmd.SetArgs([]string{"--order-type", "STOP_LIMIT"})
		_ = cmd.ParseFlags([]string{"--order-type", "STOP_LIMIT"})

		action := models.InstructionBuy
		orderType := models.OrderTypeMarket

		// Act
		err := resolveOrderFlagAliases(cmd, &action, &orderType)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, models.OrderTypeStopLimit, orderType)
	})
}
