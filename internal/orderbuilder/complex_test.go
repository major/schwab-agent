package orderbuilder

import (
	"testing"

	"github.com/major/schwab-agent/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBuildOCOOrderFullOCO verifies OCO → 2 SINGLE structure with both exits.
func TestBuildOCOOrderFullOCO(t *testing.T) {
	t.Parallel()

	order, err := BuildOCOOrder(&OCOParams{
		Symbol:     "AAPL",
		Action:     models.InstructionSell,
		Quantity:   100,
		TakeProfit: 160,
		StopLoss:   140,
		Duration:   models.DurationDay,
		Session:    models.SessionNormal,
	})

	require.NoError(t, err)
	require.NotNil(t, order)

	// Top-level is an OCO node with no legs of its own.
	assert.Equal(t, models.OrderStrategyTypeOCO, order.OrderStrategyType)
	assert.Empty(t, order.OrderLegCollection)

	// OCO node should contain exactly 2 SINGLE exit orders.
	require.Len(t, order.ChildOrderStrategies, 2)

	takeProfit := order.ChildOrderStrategies[0]
	assert.Equal(t, models.OrderStrategyTypeSingle, takeProfit.OrderStrategyType)
	assert.Equal(t, models.OrderTypeLimit, takeProfit.OrderType)
	require.NotNil(t, takeProfit.Price)
	assert.Equal(t, 160.0, *takeProfit.Price)
	require.Len(t, takeProfit.OrderLegCollection, 1)
	assert.Equal(t, models.InstructionSell, takeProfit.OrderLegCollection[0].Instruction)
	assert.Equal(t, 100.0, takeProfit.OrderLegCollection[0].Quantity)

	stopLoss := order.ChildOrderStrategies[1]
	assert.Equal(t, models.OrderStrategyTypeSingle, stopLoss.OrderStrategyType)
	assert.Equal(t, models.OrderTypeStop, stopLoss.OrderType)
	require.NotNil(t, stopLoss.StopPrice)
	assert.Equal(t, 140.0, *stopLoss.StopPrice)
	require.Len(t, stopLoss.OrderLegCollection, 1)
	assert.Equal(t, models.InstructionSell, stopLoss.OrderLegCollection[0].Instruction)
	assert.Equal(t, 100.0, stopLoss.OrderLegCollection[0].Quantity)
}

// TestBuildOCOOrderStopLossOnly verifies standalone SINGLE stop structure.
func TestBuildOCOOrderStopLossOnly(t *testing.T) {
	t.Parallel()

	order, err := BuildOCOOrder(&OCOParams{
		Symbol:   "F",
		Action:   models.InstructionSell,
		Quantity: 50,
		StopLoss: 7,
		Duration: models.DurationDay,
		Session:  models.SessionNormal,
	})

	require.NoError(t, err)
	require.NotNil(t, order)

	// Single exit: no OCO wrapper, just a plain SINGLE stop order.
	assert.Equal(t, models.OrderStrategyTypeSingle, order.OrderStrategyType)
	assert.Equal(t, models.OrderTypeStop, order.OrderType)
	require.NotNil(t, order.StopPrice)
	assert.Equal(t, 7.0, *order.StopPrice)
	require.Len(t, order.OrderLegCollection, 1)
	assert.Equal(t, models.InstructionSell, order.OrderLegCollection[0].Instruction)
	assert.Equal(t, 50.0, order.OrderLegCollection[0].Quantity)
	assert.Empty(t, order.ChildOrderStrategies)
}

// TestBuildOCOOrderTakeProfitOnly verifies standalone SINGLE limit structure.
func TestBuildOCOOrderTakeProfitOnly(t *testing.T) {
	t.Parallel()

	order, err := BuildOCOOrder(&OCOParams{
		Symbol:     "F",
		Action:     models.InstructionSell,
		Quantity:   50,
		TakeProfit: 15,
		Duration:   models.DurationDay,
		Session:    models.SessionNormal,
	})

	require.NoError(t, err)
	require.NotNil(t, order)

	// Single exit: no OCO wrapper, just a plain SINGLE limit order.
	assert.Equal(t, models.OrderStrategyTypeSingle, order.OrderStrategyType)
	assert.Equal(t, models.OrderTypeLimit, order.OrderType)
	require.NotNil(t, order.Price)
	assert.Equal(t, 15.0, *order.Price)
	require.Len(t, order.OrderLegCollection, 1)
	assert.Equal(t, models.InstructionSell, order.OrderLegCollection[0].Instruction)
	assert.Equal(t, 50.0, order.OrderLegCollection[0].Quantity)
	assert.Empty(t, order.ChildOrderStrategies)
}

// TestBuildOCOOrderAppliesDefaults verifies DAY/NORMAL defaults are applied.
func TestBuildOCOOrderAppliesDefaults(t *testing.T) {
	t.Parallel()

	order, err := BuildOCOOrder(&OCOParams{
		Symbol:     "AAPL",
		Action:     models.InstructionSell,
		Quantity:   10,
		TakeProfit: 160,
		StopLoss:   140,
	})

	require.NoError(t, err)
	require.NotNil(t, order)

	// Defaults propagate to child orders.
	require.Len(t, order.ChildOrderStrategies, 2)
	assert.Equal(t, models.DurationDay, order.ChildOrderStrategies[0].Duration)
	assert.Equal(t, models.SessionNormal, order.ChildOrderStrategies[0].Session)
	assert.Equal(t, models.DurationDay, order.ChildOrderStrategies[1].Duration)
	assert.Equal(t, models.SessionNormal, order.ChildOrderStrategies[1].Session)
}

// TestBuildOCOOrderBuyToCloseShort verifies BUY exit for closing a short position.
func TestBuildOCOOrderBuyToCloseShort(t *testing.T) {
	t.Parallel()

	order, err := BuildOCOOrder(&OCOParams{
		Symbol:     "TSLA",
		Action:     models.InstructionBuy,
		Quantity:   25,
		TakeProfit: 100,
		StopLoss:   200,
		Duration:   models.DurationGoodTillCancel,
		Session:    models.SessionNormal,
	})

	require.NoError(t, err)
	require.NotNil(t, order)

	assert.Equal(t, models.OrderStrategyTypeOCO, order.OrderStrategyType)
	require.Len(t, order.ChildOrderStrategies, 2)

	// BUY to close: take-profit is below current (price dropped), stop-loss is above.
	takeProfit := order.ChildOrderStrategies[0]
	assert.Equal(t, models.InstructionBuy, takeProfit.OrderLegCollection[0].Instruction)
	require.NotNil(t, takeProfit.Price)
	assert.Equal(t, 100.0, *takeProfit.Price)

	stopLoss := order.ChildOrderStrategies[1]
	assert.Equal(t, models.InstructionBuy, stopLoss.OrderLegCollection[0].Instruction)
	require.NotNil(t, stopLoss.StopPrice)
	assert.Equal(t, 200.0, *stopLoss.StopPrice)
}

// TestBuildBracketOrderStopLossOnly verifies TRIGGER → SINGLE stop structure.
func TestBuildBracketOrderStopLossOnly(t *testing.T) {
	t.Parallel()

	order, err := BuildBracketOrder(&BracketParams{
		Symbol:    "F",
		Action:    models.InstructionBuy,
		Quantity:  1,
		OrderType: models.OrderTypeLimit,
		Price:     9,
		StopLoss:  7,
		Duration:  models.DurationDay,
		Session:   models.SessionNormal,
	})

	require.NoError(t, err)
	require.NotNil(t, order)

	// Arrange
	assert.Equal(t, models.OrderStrategyTypeTrigger, order.OrderStrategyType)
	require.NotNil(t, order.Price)
	assert.Equal(t, 9.0, *order.Price)

	// Single SINGLE child (no OCO wrapper).
	require.Len(t, order.ChildOrderStrategies, 1)

	stopLoss := order.ChildOrderStrategies[0]
	assert.Equal(t, models.OrderStrategyTypeSingle, stopLoss.OrderStrategyType)
	assert.Equal(t, models.OrderTypeStop, stopLoss.OrderType)
	require.NotNil(t, stopLoss.StopPrice)
	assert.Equal(t, 7.0, *stopLoss.StopPrice)
	require.Len(t, stopLoss.OrderLegCollection, 1)
	assert.Equal(t, models.InstructionSell, stopLoss.OrderLegCollection[0].Instruction)
	assert.Equal(t, 1.0, stopLoss.OrderLegCollection[0].Quantity)

	// No nested children (no OCO).
	assert.Empty(t, stopLoss.ChildOrderStrategies)
}

// TestBuildBracketOrderTakeProfitOnly verifies TRIGGER → SINGLE limit structure.
func TestBuildBracketOrderTakeProfitOnly(t *testing.T) {
	t.Parallel()

	order, err := BuildBracketOrder(&BracketParams{
		Symbol:     "F",
		Action:     models.InstructionBuy,
		Quantity:   1,
		OrderType:  models.OrderTypeLimit,
		Price:      9,
		TakeProfit: 15,
		Duration:   models.DurationDay,
		Session:    models.SessionNormal,
	})

	require.NoError(t, err)
	require.NotNil(t, order)

	assert.Equal(t, models.OrderStrategyTypeTrigger, order.OrderStrategyType)

	// Single SINGLE child (no OCO wrapper).
	require.Len(t, order.ChildOrderStrategies, 1)

	takeProfit := order.ChildOrderStrategies[0]
	assert.Equal(t, models.OrderStrategyTypeSingle, takeProfit.OrderStrategyType)
	assert.Equal(t, models.OrderTypeLimit, takeProfit.OrderType)
	require.NotNil(t, takeProfit.Price)
	assert.Equal(t, 15.0, *takeProfit.Price)
	require.Len(t, takeProfit.OrderLegCollection, 1)
	assert.Equal(t, models.InstructionSell, takeProfit.OrderLegCollection[0].Instruction)

	assert.Empty(t, takeProfit.ChildOrderStrategies)
}

// TestBuildBracketOrderCreatesTriggerWithOCOChildren verifies the Schwab bracket order structure.
func TestBuildBracketOrderCreatesTriggerWithOCOChildren(t *testing.T) {
	t.Parallel()

	order, err := BuildBracketOrder(&BracketParams{
		Symbol:     "AAPL",
		Action:     models.InstructionBuy,
		Quantity:   100,
		OrderType:  models.OrderTypeLimit,
		Price:      150,
		TakeProfit: 160,
		StopLoss:   140,
		Duration:   models.DurationDay,
		Session:    models.SessionNormal,
	})

	require.NoError(t, err)
	require.NotNil(t, order)

	assert.Equal(t, models.OrderStrategyTypeTrigger, order.OrderStrategyType)
	assert.Equal(t, models.OrderTypeLimit, order.OrderType)
	require.NotNil(t, order.Price)
	assert.Equal(t, 150.0, *order.Price)
	require.Len(t, order.OrderLegCollection, 1)
	assert.Equal(t, models.InstructionBuy, order.OrderLegCollection[0].Instruction)
	assert.Equal(t, 100.0, order.OrderLegCollection[0].Quantity)
	assert.Equal(t, models.AssetTypeEquity, order.OrderLegCollection[0].Instrument.AssetType)
	assert.Equal(t, "AAPL", order.OrderLegCollection[0].Instrument.Symbol)

	// Parent should have exactly 1 OCO child node.
	require.Len(t, order.ChildOrderStrategies, 1)

	ocoNode := order.ChildOrderStrategies[0]
	assert.Equal(t, models.OrderStrategyTypeOCO, ocoNode.OrderStrategyType)

	// OCO node should contain exactly 2 SINGLE exit orders.
	require.Len(t, ocoNode.ChildOrderStrategies, 2)

	takeProfit := ocoNode.ChildOrderStrategies[0]
	assert.Equal(t, models.OrderStrategyTypeSingle, takeProfit.OrderStrategyType)
	assert.Equal(t, models.OrderTypeLimit, takeProfit.OrderType)
	require.NotNil(t, takeProfit.Price)
	assert.Equal(t, 160.0, *takeProfit.Price)
	require.Len(t, takeProfit.OrderLegCollection, 1)
	assert.Equal(t, models.InstructionSell, takeProfit.OrderLegCollection[0].Instruction)
	assert.Equal(t, 100.0, takeProfit.OrderLegCollection[0].Quantity)

	stopLoss := ocoNode.ChildOrderStrategies[1]
	assert.Equal(t, models.OrderStrategyTypeSingle, stopLoss.OrderStrategyType)
	assert.Equal(t, models.OrderTypeStop, stopLoss.OrderType)
	require.NotNil(t, stopLoss.StopPrice)
	assert.Equal(t, 140.0, *stopLoss.StopPrice)
	require.Len(t, stopLoss.OrderLegCollection, 1)
	assert.Equal(t, models.InstructionSell, stopLoss.OrderLegCollection[0].Instruction)
	assert.Equal(t, 100.0, stopLoss.OrderLegCollection[0].Quantity)
}
