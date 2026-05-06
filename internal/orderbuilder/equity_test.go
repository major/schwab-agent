package orderbuilder

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/major/schwab-agent/internal/apperr"
	"github.com/major/schwab-agent/internal/models"
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
	assert.InDelta(t, 100.0, order.OrderLegCollection[0].Quantity, 0.001)
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
	assert.InDelta(t, 212.34, *order.Price, 0.001)
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
	assert.InDelta(t, 199.99, *order.StopPrice, 0.001)
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
	assert.InDelta(t, 198.50, *order.Price, 0.001)
	require.NotNil(t, order.StopPrice)
	assert.InDelta(t, 199.25, *order.StopPrice, 0.001)
}

// TestBuildEquityOrderSetsTrailingStopFields verifies trailing stop orders set
// offset/link/stop-type fields and do not set price or stop price.
func TestBuildEquityOrderSetsTrailingStopFields(t *testing.T) {
	order, err := BuildEquityOrder(&EquityParams{
		Symbol:             "AAPL",
		Action:             models.InstructionSell,
		Quantity:           50,
		OrderType:          models.OrderTypeTrailingStop,
		StopPriceOffset:    2.50,
		StopPriceLinkBasis: models.StopPriceLinkBasisLast,
		StopPriceLinkType:  models.StopPriceLinkTypeValue,
		StopType:           models.StopTypeStandard,
	})

	require.NoError(t, err)
	require.NotNil(t, order)
	assert.Equal(t, models.OrderTypeTrailingStop, order.OrderType)

	// Trailing stop fields must be set.
	require.NotNil(t, order.StopPriceOffset)
	assert.InDelta(t, 2.50, *order.StopPriceOffset, 0.001)
	require.NotNil(t, order.StopPriceLinkBasis)
	assert.Equal(t, models.StopPriceLinkBasisLast, *order.StopPriceLinkBasis)
	require.NotNil(t, order.StopPriceLinkType)
	assert.Equal(t, models.StopPriceLinkTypeValue, *order.StopPriceLinkType)
	require.NotNil(t, order.StopType)
	assert.Equal(t, models.StopTypeStandard, *order.StopType)

	// Price and stop price must NOT be set for trailing stop.
	assert.Nil(t, order.Price)
	assert.Nil(t, order.StopPrice)

	// Activation price was not provided, so it must be nil.
	assert.Nil(t, order.ActivationPrice)
}

// TestBuildEquityOrderSetsTrailingStopWithActivationPrice verifies the optional
// activation price is set when provided.
func TestBuildEquityOrderSetsTrailingStopWithActivationPrice(t *testing.T) {
	order, err := BuildEquityOrder(&EquityParams{
		Symbol:             "TSLA",
		Action:             models.InstructionSell,
		Quantity:           25,
		OrderType:          models.OrderTypeTrailingStop,
		StopPriceOffset:    5.00,
		StopPriceLinkBasis: models.StopPriceLinkBasisBid,
		StopPriceLinkType:  models.StopPriceLinkTypePercent,
		StopType:           models.StopTypeBid,
		ActivationPrice:    150.00,
	})

	require.NoError(t, err)
	require.NotNil(t, order)
	require.NotNil(t, order.ActivationPrice)
	assert.InDelta(t, 150.00, *order.ActivationPrice, 0.001)
}

// TestBuildEquityOrderSetsTrailingStopLimitFields verifies trailing stop limit
// orders set both trailing stop fields and the limit price.
func TestBuildEquityOrderSetsTrailingStopLimitFields(t *testing.T) {
	order, err := BuildEquityOrder(&EquityParams{
		Symbol:             "AAPL",
		Action:             models.InstructionSell,
		Quantity:           10,
		OrderType:          models.OrderTypeTrailingStopLimit,
		Price:              195.00,
		StopPriceOffset:    3.00,
		StopPriceLinkBasis: models.StopPriceLinkBasisMark,
		StopPriceLinkType:  models.StopPriceLinkTypeTick,
		StopType:           models.StopTypeStandard,
	})

	require.NoError(t, err)
	require.NotNil(t, order)
	assert.Equal(t, models.OrderTypeTrailingStopLimit, order.OrderType)

	// Price offset must be set for trailing stop limit (Schwab uses priceOffset,
	// not price, for the limit component of trailing stop limit orders).
	require.NotNil(t, order.PriceOffset)
	assert.InDelta(t, 195.00, *order.PriceOffset, 0.001)
	assert.Nil(t, order.Price)

	// Trailing stop fields must be set.
	require.NotNil(t, order.StopPriceOffset)
	assert.InDelta(t, 3.00, *order.StopPriceOffset, 0.001)
	require.NotNil(t, order.StopPriceLinkBasis)
	assert.Equal(t, models.StopPriceLinkBasisMark, *order.StopPriceLinkBasis)
	require.NotNil(t, order.StopPriceLinkType)
	assert.Equal(t, models.StopPriceLinkTypeTick, *order.StopPriceLinkType)

	// Stop price must NOT be set (trailing stops use offset, not fixed price).
	assert.Nil(t, order.StopPrice)

	// Price link fields must mirror stop price link fields for trailing stop limit.
	require.NotNil(t, order.PriceLinkBasis)
	assert.Equal(t, models.PriceLinkBasis(models.StopPriceLinkBasisMark), *order.PriceLinkBasis)
	require.NotNil(t, order.PriceLinkType)
	assert.Equal(t, models.PriceLinkType(models.StopPriceLinkTypeTick), *order.PriceLinkType)
}

// TestBuildEquityOrderSetsSpecialInstruction verifies the special instruction
// field is set on the order when provided.
func TestBuildEquityOrderSetsSpecialInstruction(t *testing.T) {
	order, err := BuildEquityOrder(&EquityParams{
		Symbol:             "AAPL",
		Action:             models.InstructionBuy,
		Quantity:           100,
		OrderType:          models.OrderTypeLimit,
		Price:              200.00,
		SpecialInstruction: models.SpecialInstructionAllOrNone,
	})

	require.NoError(t, err)
	require.NotNil(t, order)
	require.NotNil(t, order.SpecialInstruction)
	assert.Equal(t, models.SpecialInstructionAllOrNone, *order.SpecialInstruction)
}

// TestBuildEquityOrderOmitsSpecialInstructionWhenEmpty verifies the field is
// nil when no special instruction is provided.
func TestBuildEquityOrderOmitsSpecialInstructionWhenEmpty(t *testing.T) {
	order, err := BuildEquityOrder(&EquityParams{
		Symbol:    "AAPL",
		Action:    models.InstructionBuy,
		Quantity:  100,
		OrderType: models.OrderTypeMarket,
	})

	require.NoError(t, err)
	require.NotNil(t, order)
	assert.Nil(t, order.SpecialInstruction)
}

// TestBuildEquityOrderSetsDestination verifies the destination field is set
// on the order when provided.
func TestBuildEquityOrderSetsDestination(t *testing.T) {
	order, err := BuildEquityOrder(&EquityParams{
		Symbol:      "AAPL",
		Action:      models.InstructionBuy,
		Quantity:    100,
		OrderType:   models.OrderTypeLimit,
		Price:       150.00,
		Destination: models.RequestedDestinationINET,
	})

	require.NoError(t, err)
	require.NotNil(t, order)
	require.NotNil(t, order.RequestedDestination)
	assert.Equal(t, models.RequestedDestinationINET, *order.RequestedDestination)
}

// TestBuildEquityOrderOmitsDestinationWhenEmpty verifies destination is nil
// when not provided.
func TestBuildEquityOrderOmitsDestinationWhenEmpty(t *testing.T) {
	order, err := BuildEquityOrder(&EquityParams{
		Symbol:    "AAPL",
		Action:    models.InstructionBuy,
		Quantity:  100,
		OrderType: models.OrderTypeMarket,
	})

	require.NoError(t, err)
	require.NotNil(t, order)
	assert.Nil(t, order.RequestedDestination)
}

// TestBuildEquityOrderSetsPriceLinkFields verifies price link basis and type
// fields are set on the order when provided.
func TestBuildEquityOrderSetsPriceLinkFields(t *testing.T) {
	order, err := BuildEquityOrder(&EquityParams{
		Symbol:         "AAPL",
		Action:         models.InstructionBuy,
		Quantity:       100,
		OrderType:      models.OrderTypeLimit,
		Price:          150.00,
		PriceLinkBasis: models.PriceLinkBasisLast,
		PriceLinkType:  models.PriceLinkTypeValue,
	})

	require.NoError(t, err)
	require.NotNil(t, order)
	require.NotNil(t, order.PriceLinkBasis)
	assert.Equal(t, models.PriceLinkBasisLast, *order.PriceLinkBasis)
	require.NotNil(t, order.PriceLinkType)
	assert.Equal(t, models.PriceLinkTypeValue, *order.PriceLinkType)
}

// TestBuildEquityOrderOmitsPriceLinkFieldsWhenEmpty verifies price link fields
// are nil when not provided.
func TestBuildEquityOrderOmitsPriceLinkFieldsWhenEmpty(t *testing.T) {
	order, err := BuildEquityOrder(&EquityParams{
		Symbol:    "AAPL",
		Action:    models.InstructionBuy,
		Quantity:  100,
		OrderType: models.OrderTypeMarket,
	})

	require.NoError(t, err)
	require.NotNil(t, order)
	assert.Nil(t, order.PriceLinkBasis)
	assert.Nil(t, order.PriceLinkType)
}

// TestBuildEquityOrderRejectsMarketWithPriceLink verifies that market orders
// reject price-link fields since there is no price to link against.
func TestBuildEquityOrderRejectsMarketWithPriceLink(t *testing.T) {
	t.Parallel()

	order, err := BuildEquityOrder(&EquityParams{
		Symbol:         "AAPL",
		Action:         models.InstructionBuy,
		Quantity:       100,
		OrderType:      models.OrderTypeMarket,
		PriceLinkBasis: models.PriceLinkBasisLast,
	})

	require.Nil(t, order)
	require.Error(t, err)

	validationErr, ok := errors.AsType[*apperr.ValidationError](err)
	require.True(t, ok)
	assert.Equal(t, "price-link-basis and price-link-type are not allowed on market orders", validationErr.Message)
}

func TestBuildEquityOrderRejectsMarketOnCloseWithPriceLink(t *testing.T) {
	t.Parallel()

	order, err := BuildEquityOrder(&EquityParams{
		Symbol:         "AAPL",
		Action:         models.InstructionBuy,
		Quantity:       100,
		OrderType:      models.OrderTypeMarketOnClose,
		PriceLinkBasis: models.PriceLinkBasisLast,
	})

	require.Nil(t, order)
	require.Error(t, err)

	validationErr, ok := errors.AsType[*apperr.ValidationError](err)
	require.True(t, ok)
	assert.Equal(t, "price-link-basis and price-link-type are not allowed on market orders", validationErr.Message)
}

// TestBuildEquityOrderSetsLimitOnClosePrice verifies LOC orders set the limit price.
func TestBuildEquityOrderSetsLimitOnClosePrice(t *testing.T) {
	order, err := BuildEquityOrder(&EquityParams{
		Symbol:    "SPY",
		Action:    models.InstructionSell,
		Quantity:  50,
		OrderType: models.OrderTypeLimitOnClose,
		Price:     450.25,
	})

	require.NoError(t, err)
	require.NotNil(t, order)
	assert.Equal(t, models.OrderTypeLimitOnClose, order.OrderType)
	require.NotNil(t, order.Price, "LOC orders must set Price")
	assert.InDelta(t, 450.25, *order.Price, 0.001)
	assert.Nil(t, order.StopPrice, "LOC orders should not set StopPrice")
}

// TestBuildEquityOrderPriceLinkDoesNotBreakTrailingStopLimit verifies that when
// explicit price link fields are NOT set, trailing stop limit orders still
// derive PriceLinkBasis and PriceLinkType from the stop price link values.
func TestBuildEquityOrderPriceLinkDoesNotBreakTrailingStopLimit(t *testing.T) {
	order, err := BuildEquityOrder(&EquityParams{
		Symbol:             "AAPL",
		Action:             models.InstructionSell,
		Quantity:           10,
		OrderType:          models.OrderTypeTrailingStopLimit,
		Price:              195.00,
		StopPriceOffset:    3.00,
		StopPriceLinkBasis: models.StopPriceLinkBasisMark,
		StopPriceLinkType:  models.StopPriceLinkTypeTick,
		StopType:           models.StopTypeStandard,
	})

	require.NoError(t, err)
	require.NotNil(t, order)

	// PriceLinkBasis/Type should still be derived from stop price link values
	// when no explicit price link fields are set.
	require.NotNil(t, order.PriceLinkBasis)
	assert.Equal(t, models.PriceLinkBasis(models.StopPriceLinkBasisMark), *order.PriceLinkBasis)
	require.NotNil(t, order.PriceLinkType)
	assert.Equal(t, models.PriceLinkType(models.StopPriceLinkTypeTick), *order.PriceLinkType)
}
