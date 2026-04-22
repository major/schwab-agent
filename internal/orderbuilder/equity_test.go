package orderbuilder

import (
	"testing"

	"github.com/major/schwab-agent/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBuildEquityOrderAppliesDefaultsAndMarketFields verifies default duration and session,
// and ensures market orders do not set price fields.
func TestBuildEquityOrderAppliesDefaultsAndMarketFields(t *testing.T) {
	order, err := BuildEquityOrder(&EquityParams{
		Symbol:    "AAPL",
		Action:    models.InstructionBuy,
		Quantity:  100,
		OrderType: models.OrderTypeMarket,
	})

	require.NoError(t, err)
	require.NotNil(t, order)
	assert.Equal(t, models.DurationDay, order.Duration)
	assert.Equal(t, models.SessionNormal, order.Session)
	assert.Equal(t, models.OrderTypeMarket, order.OrderType)
	assert.Equal(t, models.OrderStrategyTypeSingle, order.OrderStrategyType)
	assert.Nil(t, order.Price)
	assert.Nil(t, order.StopPrice)
	require.Len(t, order.OrderLegCollection, 1)
	assert.Equal(t, models.InstructionBuy, order.OrderLegCollection[0].Instruction)
	assert.Equal(t, 100.0, order.OrderLegCollection[0].Quantity)
	assert.Equal(t, models.AssetTypeEquity, order.OrderLegCollection[0].Instrument.AssetType)
	assert.Equal(t, "AAPL", order.OrderLegCollection[0].Instrument.Symbol)
}

// TestBuildEquityOrderSetsLimitPrice verifies limit orders set only the limit price.
func TestBuildEquityOrderSetsLimitPrice(t *testing.T) {
	order, err := BuildEquityOrder(&EquityParams{
		Symbol:    "AAPL",
		Action:    models.InstructionBuy,
		Quantity:  10,
		OrderType: models.OrderTypeLimit,
		Price:     212.34,
		Duration:  models.DurationGoodTillCancel,
		Session:   models.SessionAM,
	})

	require.NoError(t, err)
	require.NotNil(t, order)
	require.NotNil(t, order.Price)
	assert.Equal(t, 212.34, *order.Price)
	assert.Nil(t, order.StopPrice)
	assert.Equal(t, models.DurationGoodTillCancel, order.Duration)
	assert.Equal(t, models.SessionAM, order.Session)
}

// TestBuildEquityOrderSetsStopPrice verifies stop orders set only the stop price.
func TestBuildEquityOrderSetsStopPrice(t *testing.T) {
	order, err := BuildEquityOrder(&EquityParams{
		Symbol:    "AAPL",
		Action:    models.InstructionSell,
		Quantity:  10,
		OrderType: models.OrderTypeStop,
		StopPrice: 199.99,
	})

	require.NoError(t, err)
	require.NotNil(t, order)
	assert.Nil(t, order.Price)
	require.NotNil(t, order.StopPrice)
	assert.Equal(t, 199.99, *order.StopPrice)
}

// TestBuildEquityOrderSetsLimitAndStopPrice verifies stop limit orders set both price fields.
func TestBuildEquityOrderSetsLimitAndStopPrice(t *testing.T) {
	order, err := BuildEquityOrder(&EquityParams{
		Symbol:    "AAPL",
		Action:    models.InstructionSell,
		Quantity:  10,
		OrderType: models.OrderTypeStopLimit,
		Price:     198.50,
		StopPrice: 199.25,
	})

	require.NoError(t, err)
	require.NotNil(t, order)
	require.NotNil(t, order.Price)
	assert.Equal(t, 198.50, *order.Price)
	require.NotNil(t, order.StopPrice)
	assert.Equal(t, 199.25, *order.StopPrice)
}
