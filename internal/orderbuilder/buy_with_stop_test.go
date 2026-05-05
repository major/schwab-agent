package orderbuilder

import (
	"errors"
	"strings"
	"testing"

	"github.com/major/schwab-agent/internal/apperr"
	"github.com/major/schwab-agent/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestValidateBuyWithStopOrder verifies buy-with-stop input validation without
// depending on the builder. The feature intentionally supports only BUY entries
// with a required protective stop; take-profit is optional.
func TestValidateBuyWithStopOrder(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		params      BuyWithStopParams
		wantErr     bool
		wantMessage string
	}{
		{
			name: "valid limit with stop-loss below price",
			params: BuyWithStopParams{
				Symbol:    "AAPL",
				Quantity:  10,
				OrderType: models.OrderTypeLimit,
				Price:     150,
				StopLoss:  140,
			},
		},
		{
			name: "valid market with stop-loss skips price validation",
			params: BuyWithStopParams{
				Symbol:    "AAPL",
				Quantity:  10,
				OrderType: models.OrderTypeMarket,
				StopLoss:  140,
			},
		},
		{
			name: "valid limit with stop-loss and take-profit above price",
			params: BuyWithStopParams{
				Symbol:     "AAPL",
				Quantity:   10,
				OrderType:  models.OrderTypeLimit,
				Price:      150,
				StopLoss:   140,
				TakeProfit: 160,
			},
		},
		{
			name: "missing stop-loss rejected",
			params: BuyWithStopParams{
				Symbol:    "AAPL",
				Quantity:  10,
				OrderType: models.OrderTypeLimit,
				Price:     150,
			},
			wantErr:     true,
			wantMessage: "stop-loss is required",
		},
		{
			name: "stop entry type rejected",
			params: BuyWithStopParams{
				Symbol:    "AAPL",
				Quantity:  10,
				OrderType: models.OrderTypeStop,
				Price:     150,
				StopLoss:  140,
			},
			wantErr:     true,
			wantMessage: "only LIMIT and MARKET entry orders are supported",
		},
		{
			name: "stop-limit entry type rejected",
			params: BuyWithStopParams{
				Symbol:    "AAPL",
				Quantity:  10,
				OrderType: models.OrderTypeStopLimit,
				Price:     150,
				StopLoss:  140,
			},
			wantErr:     true,
			wantMessage: "only LIMIT and MARKET entry orders are supported",
		},
		{
			name: "stop-loss at entry price rejected for limit buy",
			params: BuyWithStopParams{
				Symbol:    "AAPL",
				Quantity:  10,
				OrderType: models.OrderTypeLimit,
				Price:     150,
				StopLoss:  150,
			},
			wantErr:     true,
			wantMessage: "stop-loss must be below entry price",
		},
		{
			name: "take-profit below entry price rejected for limit buy",
			params: BuyWithStopParams{
				Symbol:     "AAPL",
				Quantity:   10,
				OrderType:  models.OrderTypeLimit,
				Price:      150,
				StopLoss:   140,
				TakeProfit: 149,
			},
			wantErr:     true,
			wantMessage: "take-profit must be above entry price",
		},
		{
			name: "take-profit and stop-loss at same price rejected",
			params: BuyWithStopParams{
				Symbol:     "AAPL",
				Quantity:   10,
				OrderType:  models.OrderTypeLimit,
				Price:      150,
				StopLoss:   140,
				TakeProfit: 140,
			},
			wantErr:     true,
			wantMessage: "take-profit and stop-loss cannot be the same price",
		},
		{
			name: "market skips stop-loss to price relationship",
			params: BuyWithStopParams{
				Symbol:    "AAPL",
				Quantity:  10,
				OrderType: models.OrderTypeMarket,
				Price:     0,
				StopLoss:  200,
			},
		},
		{
			name: "market skips take-profit to price relationship",
			params: BuyWithStopParams{
				Symbol:     "AAPL",
				Quantity:   10,
				OrderType:  models.OrderTypeMarket,
				Price:      0,
				StopLoss:   200,
				TakeProfit: 100,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Act
			err := ValidateBuyWithStopOrder(&tt.params)

			// Assert
			if !tt.wantErr {
				require.NoError(t, err)
				return
			}

			require.Error(t, err)
			var validationErr *apperr.ValidationError
			require.True(t, errors.As(err, &validationErr))
			assert.Contains(t, validationErr.Error(), tt.wantMessage)
		})
	}
}

// TestBuildBuyWithStopOrder verifies the Schwab TRIGGER structure produced for
// buy entries with protective exits. Exits intentionally use GTC duration while
// the entry keeps its own duration so agents can separate entry lifetime from
// post-fill protection.
func TestBuildBuyWithStopOrder(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		params            BuyWithStopParams
		wantEntryType     models.OrderType
		wantEntryDuration models.Duration
		wantSession       models.Session
		wantPrice         float64
		wantPriceSet      bool
		wantOCO           bool
	}{
		{
			name: "limit with stop-loss only",
			params: BuyWithStopParams{
				Symbol:    "AAPL",
				Quantity:  10,
				OrderType: models.OrderTypeLimit,
				Price:     150,
				StopLoss:  140,
			},
			wantEntryType:     models.OrderTypeLimit,
			wantEntryDuration: models.DurationDay,
			wantSession:       models.SessionNormal,
			wantPrice:         150,
			wantPriceSet:      true,
		},
		{
			name: "market with stop-loss only",
			params: BuyWithStopParams{
				Symbol:    "AAPL",
				Quantity:  10,
				OrderType: models.OrderTypeMarket,
				StopLoss:  140,
			},
			wantEntryType:     models.OrderTypeMarket,
			wantEntryDuration: models.DurationDay,
			wantSession:       models.SessionNormal,
		},
		{
			name: "limit with stop-loss and take-profit",
			params: BuyWithStopParams{
				Symbol:     "AAPL",
				Quantity:   10,
				OrderType:  models.OrderTypeLimit,
				Price:      150,
				StopLoss:   140,
				TakeProfit: 160,
			},
			wantEntryType:     models.OrderTypeLimit,
			wantEntryDuration: models.DurationDay,
			wantSession:       models.SessionNormal,
			wantPrice:         150,
			wantPriceSet:      true,
			wantOCO:           true,
		},
		{
			name: "market with stop-loss and take-profit",
			params: BuyWithStopParams{
				Symbol:     "AAPL",
				Quantity:   10,
				OrderType:  models.OrderTypeMarket,
				StopLoss:   140,
				TakeProfit: 160,
			},
			wantEntryType:     models.OrderTypeMarket,
			wantEntryDuration: models.DurationDay,
			wantSession:       models.SessionNormal,
			wantOCO:           true,
		},
		{
			name: "custom entry duration keeps exits GTC",
			params: BuyWithStopParams{
				Symbol:    "AAPL",
				Quantity:  10,
				OrderType: models.OrderTypeLimit,
				Duration:  models.DurationGoodTillCancel,
				Price:     150,
				StopLoss:  140,
			},
			wantEntryType:     models.OrderTypeLimit,
			wantEntryDuration: models.DurationGoodTillCancel,
			wantSession:       models.SessionNormal,
			wantPrice:         150,
			wantPriceSet:      true,
		},
		{
			name: "custom session propagates to entry and exits",
			params: BuyWithStopParams{
				Symbol:    "AAPL",
				Quantity:  10,
				OrderType: models.OrderTypeLimit,
				Session:   models.SessionAM,
				Price:     150,
				StopLoss:  140,
			},
			wantEntryType:     models.OrderTypeLimit,
			wantEntryDuration: models.DurationDay,
			wantSession:       models.SessionAM,
			wantPrice:         150,
			wantPriceSet:      true,
		},
		{
			name: "quantity propagates to entry and exit legs",
			params: BuyWithStopParams{
				Symbol:    "AAPL",
				Quantity:  3.5,
				OrderType: models.OrderTypeLimit,
				Price:     150,
				StopLoss:  140,
			},
			wantEntryType:     models.OrderTypeLimit,
			wantEntryDuration: models.DurationDay,
			wantSession:       models.SessionNormal,
			wantPrice:         150,
			wantPriceSet:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Act
			order, err := BuildBuyWithStopOrder(&tt.params)

			// Assert
			require.NoError(t, err)
			require.NotNil(t, order)
			assertBuyWithStopEntry(t, order, &tt.params, tt.wantEntryType, tt.wantEntryDuration, tt.wantSession, tt.wantPrice, tt.wantPriceSet)

			if tt.wantOCO {
				assertBuyWithStopOCOExits(t, order, &tt.params, tt.wantSession)
				return
			}

			assertBuyWithStopSingleStopExit(t, order, &tt.params, tt.wantSession)
		})
	}
}

func assertBuyWithStopEntry(
	t *testing.T,
	order *models.OrderRequest,
	params *BuyWithStopParams,
	wantEntryType models.OrderType,
	wantEntryDuration models.Duration,
	wantSession models.Session,
	wantPrice float64,
	wantPriceSet bool,
) {
	t.Helper()

	assert.Equal(t, models.OrderStrategyTypeTrigger, order.OrderStrategyType)
	assert.Equal(t, wantEntryType, order.OrderType)
	assert.Equal(t, wantEntryDuration, order.Duration)
	assert.Equal(t, wantSession, order.Session)

	if !wantPriceSet {
		assert.Nil(t, order.Price)
	} else {
		require.NotNil(t, order.Price)
		assert.Equal(t, wantPrice, *order.Price)
	}

	require.Len(t, order.OrderLegCollection, 1)
	entryLeg := order.OrderLegCollection[0]
	assert.Equal(t, models.InstructionBuy, entryLeg.Instruction)
	assert.Equal(t, params.Quantity, entryLeg.Quantity)
	assert.Equal(t, models.AssetTypeEquity, entryLeg.Instrument.AssetType)
	assert.Equal(t, params.Symbol, entryLeg.Instrument.Symbol)
}

func assertBuyWithStopSingleStopExit(
	t *testing.T,
	order *models.OrderRequest,
	params *BuyWithStopParams,
	wantSession models.Session,
) {
	t.Helper()

	require.Len(t, order.ChildOrderStrategies, 1)
	stopLoss := order.ChildOrderStrategies[0]
	assertBuyWithStopExitCommon(t, &stopLoss, wantSession, params.Quantity)
	assert.Equal(t, models.OrderTypeStop, stopLoss.OrderType)
	require.NotNil(t, stopLoss.StopPrice)
	assert.Equal(t, params.StopLoss, *stopLoss.StopPrice)
	assert.Empty(t, stopLoss.ChildOrderStrategies)
}

func assertBuyWithStopOCOExits(
	t *testing.T,
	order *models.OrderRequest,
	params *BuyWithStopParams,
	wantSession models.Session,
) {
	t.Helper()

	require.Len(t, order.ChildOrderStrategies, 1)
	oco := order.ChildOrderStrategies[0]
	assert.Equal(t, models.OrderStrategyTypeOCO, oco.OrderStrategyType)
	require.Len(t, oco.ChildOrderStrategies, 2)

	takeProfit := oco.ChildOrderStrategies[0]
	assertBuyWithStopExitCommon(t, &takeProfit, wantSession, params.Quantity)
	assert.Equal(t, models.OrderTypeLimit, takeProfit.OrderType)
	require.NotNil(t, takeProfit.Price)
	assert.Equal(t, params.TakeProfit, *takeProfit.Price)

	stopLoss := oco.ChildOrderStrategies[1]
	assertBuyWithStopExitCommon(t, &stopLoss, wantSession, params.Quantity)
	assert.Equal(t, models.OrderTypeStop, stopLoss.OrderType)
	require.NotNil(t, stopLoss.StopPrice)
	assert.Equal(t, params.StopLoss, *stopLoss.StopPrice)
}

func assertBuyWithStopExitCommon(t *testing.T, order *models.OrderRequest, wantSession models.Session, wantQuantity float64) {
	t.Helper()

	assert.Equal(t, models.OrderStrategyTypeSingle, order.OrderStrategyType)
	assert.Equal(t, models.DurationGoodTillCancel, order.Duration)
	assert.Equal(t, wantSession, order.Session)
	require.Len(t, order.OrderLegCollection, 1)
	assert.Equal(t, models.InstructionSell, order.OrderLegCollection[0].Instruction)
	assert.Equal(t, wantQuantity, order.OrderLegCollection[0].Quantity)
}

// TestValidateBuyWithStopOrderRejectsStopTypes is a narrow regression check for
// the evidence artifact requested by the plan. It makes the STOP/STOP_LIMIT
// rejection reason explicit instead of relying only on table-driven coverage.
func TestValidateBuyWithStopOrderRejectsStopTypes(t *testing.T) {
	t.Parallel()

	for _, orderType := range []models.OrderType{models.OrderTypeStop, models.OrderTypeStopLimit} {
		t.Run(string(orderType), func(t *testing.T) {
			t.Parallel()

			err := ValidateBuyWithStopOrder(&BuyWithStopParams{
				Symbol:    "AAPL",
				Quantity:  10,
				OrderType: orderType,
				Price:     150,
				StopLoss:  140,
			})

			require.Error(t, err)
			assert.True(t, strings.Contains(err.Error(), "only LIMIT and MARKET"))
		})
	}
}

// TestBuildBuyWithStopOrderUsesGTCExits keeps the duration split visible:
// entry orders can be DAY, while protective exits remain GTC after the fill.
func TestBuildBuyWithStopOrderUsesGTCExits(t *testing.T) {
	t.Parallel()

	order, err := BuildBuyWithStopOrder(&BuyWithStopParams{
		Symbol:     "AAPL",
		Quantity:   10,
		OrderType:  models.OrderTypeLimit,
		Duration:   models.DurationDay,
		Price:      150,
		StopLoss:   140,
		TakeProfit: 160,
	})

	require.NoError(t, err)
	require.Len(t, order.ChildOrderStrategies, 1)
	require.Len(t, order.ChildOrderStrategies[0].ChildOrderStrategies, 2)
	assert.Equal(t, models.DurationDay, order.Duration)
	assert.Equal(t, models.DurationGoodTillCancel, order.ChildOrderStrategies[0].ChildOrderStrategies[0].Duration)
	assert.Equal(t, models.DurationGoodTillCancel, order.ChildOrderStrategies[0].ChildOrderStrategies[1].Duration)
}
