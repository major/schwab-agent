package orderbuilder

import (
	"errors"
	"testing"

	"github.com/major/schwab-agent/internal/apperr"
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

// TestValidateFTSOrder verifies validation rules for first-triggers-second orders.
func TestValidateFTSOrder(t *testing.T) {
	t.Parallel()

	validOrder := models.OrderRequest{
		OrderType: models.OrderTypeLimit,
		OrderLegCollection: []models.OrderLegCollection{
			{
				Instruction: models.InstructionBuy,
				Quantity:    10,
				Instrument:  models.OrderInstrument{AssetType: models.AssetTypeEquity, Symbol: "AAPL"},
			},
		},
	}

	tests := []struct {
		name    string
		params  FTSParams
		wantErr string
	}{
		{
			name: "valid primary and secondary",
			params: FTSParams{
				Primary:   validOrder,
				Secondary: validOrder,
			},
		},
		{
			name: "empty primary - no order type and no legs",
			params: FTSParams{
				Primary:   models.OrderRequest{},
				Secondary: validOrder,
			},
			wantErr: "primary order is empty",
		},
		{
			name: "empty secondary - no order type and no legs",
			params: FTSParams{
				Primary:   validOrder,
				Secondary: models.OrderRequest{},
			},
			wantErr: "secondary order is empty",
		},
		{
			name: "primary is nested TRIGGER",
			params: FTSParams{
				Primary: models.OrderRequest{
					OrderType:         models.OrderTypeLimit,
					OrderStrategyType: models.OrderStrategyTypeTrigger,
					OrderLegCollection: []models.OrderLegCollection{
						{Instruction: models.InstructionBuy, Quantity: 1},
					},
				},
				Secondary: validOrder,
			},
			wantErr: "primary order must be a simple order, not a TRIGGER strategy",
		},
		{
			name: "secondary is nested OCO",
			params: FTSParams{
				Primary: validOrder,
				Secondary: models.OrderRequest{
					OrderType:         models.OrderTypeLimit,
					OrderStrategyType: models.OrderStrategyTypeOCO,
					OrderLegCollection: []models.OrderLegCollection{
						{Instruction: models.InstructionSell, Quantity: 1},
					},
				},
			},
			wantErr: "secondary order must be a simple order, not a OCO strategy",
		},
		{
			name: "primary with SINGLE strategy type is allowed",
			params: FTSParams{
				Primary: models.OrderRequest{
					OrderType:         models.OrderTypeLimit,
					OrderStrategyType: models.OrderStrategyTypeSingle,
					OrderLegCollection: []models.OrderLegCollection{
						{Instruction: models.InstructionBuy, Quantity: 5},
					},
				},
				Secondary: validOrder,
			},
		},
		{
			name: "order with only legs and no order type is valid",
			params: FTSParams{
				Primary: models.OrderRequest{
					OrderLegCollection: []models.OrderLegCollection{
						{Instruction: models.InstructionBuy, Quantity: 1},
					},
				},
				Secondary: validOrder,
			},
		},
		{
			name: "order with only order type and no legs is valid",
			params: FTSParams{
				Primary: models.OrderRequest{
					OrderType: models.OrderTypeMarket,
				},
				Secondary: validOrder,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateFTSOrder(&tt.params)

			if tt.wantErr == "" {
				require.NoError(t, err)

				return
			}

			require.Error(t, err)

			var validationErr *apperr.ValidationError
			require.True(t, errors.As(err, &validationErr), "expected ValidationError, got %T", err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

// TestBuildFTSOrderStructure verifies the TRIGGER -> SINGLE child structure.
func TestBuildFTSOrderStructure(t *testing.T) {
	t.Parallel()

	// Arrange
	primary := models.OrderRequest{
		Session:   models.SessionNormal,
		Duration:  models.DurationDay,
		OrderType: models.OrderTypeLimit,
		Price:     new(150.0),
		OrderLegCollection: []models.OrderLegCollection{
			{
				Instruction: models.InstructionBuy,
				Quantity:    100,
				Instrument:  models.OrderInstrument{AssetType: models.AssetTypeEquity, Symbol: "AAPL"},
			},
		},
	}

	secondary := models.OrderRequest{
		Session:   models.SessionNormal,
		Duration:  models.DurationDay,
		OrderType: models.OrderTypeStop,
		StopPrice: new(140.0),
		OrderLegCollection: []models.OrderLegCollection{
			{
				Instruction: models.InstructionSell,
				Quantity:    100,
				Instrument:  models.OrderInstrument{AssetType: models.AssetTypeEquity, Symbol: "AAPL"},
			},
		},
	}

	// Act
	params := FTSParams{Primary: primary, Secondary: secondary}
	order, err := BuildFTSOrder(&params)

	// Assert
	require.NoError(t, err)

	// Primary becomes TRIGGER.
	assert.Equal(t, models.OrderStrategyTypeTrigger, order.OrderStrategyType)
	assert.Equal(t, models.OrderTypeLimit, order.OrderType)
	require.NotNil(t, order.Price)
	assert.Equal(t, 150.0, *order.Price)
	require.Len(t, order.OrderLegCollection, 1)
	assert.Equal(t, models.InstructionBuy, order.OrderLegCollection[0].Instruction)
	assert.Equal(t, 100.0, order.OrderLegCollection[0].Quantity)
	assert.Equal(t, "AAPL", order.OrderLegCollection[0].Instrument.Symbol)

	// Single SINGLE child (secondary order).
	require.Len(t, order.ChildOrderStrategies, 1)

	child := order.ChildOrderStrategies[0]
	assert.Equal(t, models.OrderStrategyTypeSingle, child.OrderStrategyType)
	assert.Equal(t, models.OrderTypeStop, child.OrderType)
	require.NotNil(t, child.StopPrice)
	assert.Equal(t, 140.0, *child.StopPrice)
	require.Len(t, child.OrderLegCollection, 1)
	assert.Equal(t, models.InstructionSell, child.OrderLegCollection[0].Instruction)
	assert.Equal(t, 100.0, child.OrderLegCollection[0].Quantity)

	// No nested children.
	assert.Empty(t, child.ChildOrderStrategies)
}

// TestBuildFTSOrderPreservesOriginalFields verifies that BuildFTSOrder carries
// over all fields from the input orders (session, duration, etc.) without
// losing anything beyond the strategy type overrides.
func TestBuildFTSOrderPreservesOriginalFields(t *testing.T) {
	t.Parallel()

	params := FTSParams{
		Primary: models.OrderRequest{
			Session:   models.SessionSeamless,
			Duration:  models.DurationGoodTillCancel,
			OrderType: models.OrderTypeMarket,
			OrderLegCollection: []models.OrderLegCollection{
				{Instruction: models.InstructionBuy, Quantity: 50},
			},
		},
		Secondary: models.OrderRequest{
			Session:   models.SessionNormal,
			Duration:  models.DurationDay,
			OrderType: models.OrderTypeLimit,
			Price:     new(200.0),
			OrderLegCollection: []models.OrderLegCollection{
				{Instruction: models.InstructionSell, Quantity: 50},
			},
		},
	}
	order, err := BuildFTSOrder(&params)

	require.NoError(t, err)

	// Primary fields are preserved.
	assert.Equal(t, models.SessionSeamless, order.Session)
	assert.Equal(t, models.DurationGoodTillCancel, order.Duration)
	assert.Equal(t, models.OrderTypeMarket, order.OrderType)

	// Secondary fields are preserved on the child.
	child := order.ChildOrderStrategies[0]
	assert.Equal(t, models.SessionNormal, child.Session)
	assert.Equal(t, models.DurationDay, child.Duration)
	assert.Equal(t, models.OrderTypeLimit, child.OrderType)
	require.NotNil(t, child.Price)
	assert.Equal(t, 200.0, *child.Price)
}

// TestBuildFTSOrderRejectsInvalidParams verifies BuildFTSOrder returns
// validation errors for invalid inputs.
func TestBuildFTSOrderRejectsInvalidParams(t *testing.T) {
	t.Parallel()

	params := FTSParams{
		Primary:   models.OrderRequest{},
		Secondary: models.OrderRequest{OrderType: models.OrderTypeLimit},
	}
	_, err := BuildFTSOrder(&params)

	require.Error(t, err)

	var validationErr *apperr.ValidationError
	require.True(t, errors.As(err, &validationErr))
	assert.Contains(t, err.Error(), "primary order is empty")
}
