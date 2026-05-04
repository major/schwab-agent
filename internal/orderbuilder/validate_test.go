package orderbuilder

import (
	"testing"
	"time"

	"github.com/major/schwab-agent/internal/apperr"
	"github.com/major/schwab-agent/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestValidateEquityOrderRequiresPriceForLimit verifies fix-suggesting limit validation.
func TestValidateEquityOrderRequiresPriceForLimit(t *testing.T) {
	t.Parallel()

	err := ValidateEquityOrder(&EquityParams{
		Symbol:    "AAPL",
		Quantity:  10,
		OrderType: models.OrderTypeLimit,
	})

	assertValidationError(t, err, "LIMIT order requires a price", "Add `--price <amount>` to specify the limit price")
}

// TestValidateEquityOrderRequiresStopPriceForStop verifies fix-suggesting stop validation.
func TestValidateEquityOrderRequiresStopPriceForStop(t *testing.T) {
	t.Parallel()

	err := ValidateEquityOrder(&EquityParams{
		Symbol:    "AAPL",
		Quantity:  10,
		OrderType: models.OrderTypeStop,
	})

	assertValidationError(t, err, "STOP order requires a stop price", "Add `--stop-price <amount>` to specify the stop price")
}

// TestValidateEquityOrderRejectsNonPositiveQuantity verifies quantity validation.
func TestValidateEquityOrderRejectsNonPositiveQuantity(t *testing.T) {
	t.Parallel()

	err := ValidateEquityOrder(&EquityParams{
		Symbol:    "AAPL",
		Quantity:  0,
		OrderType: models.OrderTypeMarket,
	})

	assertValidationError(t, err, "quantity must be greater than zero", "Add `--quantity <number>` with a positive value")
}

// TestValidateOptionOrderRejectsPastExpiration verifies option expiration validation.
func TestValidateOptionOrderRejectsPastExpiration(t *testing.T) {
	t.Parallel()

	err := ValidateOptionOrder(&OptionParams{
		Underlying: "AAPL",
		Expiration: time.Now().UTC().Add(-24 * time.Hour),
		Strike:     150,
		Quantity:   1,
	})

	assertValidationError(t, err, "option expiration date is in the past", "Use a future expiration date with `--expiration YYYY-MM-DD`")
}

// TestValidateOCOOrderRequiresSymbol verifies symbol validation.
func TestValidateOCOOrderRequiresSymbol(t *testing.T) {
	t.Parallel()

	err := ValidateOCOOrder(&OCOParams{
		Action:   models.InstructionSell,
		Quantity: 10,
		StopLoss: 140,
	})

	assertValidationError(t, err, "symbol is required", "Add `--symbol <TICKER>` to specify the stock symbol")
}

// TestValidateOCOOrderRequiresQuantity verifies quantity validation.
func TestValidateOCOOrderRequiresQuantity(t *testing.T) {
	t.Parallel()

	err := ValidateOCOOrder(&OCOParams{
		Symbol:   "AAPL",
		Action:   models.InstructionSell,
		Quantity: 0,
		StopLoss: 140,
	})

	assertValidationError(t, err, "quantity must be greater than zero", "Add `--quantity <number>` with a positive value")
}

// TestValidateOCOOrderRequiresAtLeastOneExit verifies that at least one exit is needed.
func TestValidateOCOOrderRequiresAtLeastOneExit(t *testing.T) {
	t.Parallel()

	err := ValidateOCOOrder(&OCOParams{
		Symbol:   "AAPL",
		Action:   models.InstructionSell,
		Quantity: 10,
	})

	assertValidationError(t, err, "OCO order requires at least one exit condition", "Add `--take-profit <amount>` and/or `--stop-loss <amount>`")
}

// TestValidateOCOOrderAcceptsStopLossOnly verifies stop-loss-only is valid.
func TestValidateOCOOrderAcceptsStopLossOnly(t *testing.T) {
	t.Parallel()

	err := ValidateOCOOrder(&OCOParams{
		Symbol:   "AAPL",
		Action:   models.InstructionSell,
		Quantity: 10,
		StopLoss: 140,
	})

	require.NoError(t, err)
}

// TestValidateOCOOrderAcceptsTakeProfitOnly verifies take-profit-only is valid.
func TestValidateOCOOrderAcceptsTakeProfitOnly(t *testing.T) {
	t.Parallel()

	err := ValidateOCOOrder(&OCOParams{
		Symbol:     "AAPL",
		Action:     models.InstructionSell,
		Quantity:   10,
		TakeProfit: 160,
	})

	require.NoError(t, err)
}

// TestValidateOCOOrderSellRejectsTakeProfitBelowStopLoss verifies price relationship
// for SELL exits (closing a long position).
func TestValidateOCOOrderSellRejectsTakeProfitBelowStopLoss(t *testing.T) {
	t.Parallel()

	err := ValidateOCOOrder(&OCOParams{
		Symbol:     "AAPL",
		Action:     models.InstructionSell,
		Quantity:   10,
		TakeProfit: 130,
		StopLoss:   140,
	})

	assertValidationError(t, err,
		"take-profit must be above stop-loss for a SELL OCO",
		"The take-profit target should be higher than the stop-loss level",
	)
}

// TestValidateOCOOrderSellRejectsEqualPrices verifies SELL rejects equal take-profit and stop-loss.
func TestValidateOCOOrderSellRejectsEqualPrices(t *testing.T) {
	t.Parallel()

	err := ValidateOCOOrder(&OCOParams{
		Symbol:     "AAPL",
		Action:     models.InstructionSell,
		Quantity:   10,
		TakeProfit: 140,
		StopLoss:   140,
	})

	assertValidationError(t, err,
		"take-profit must be above stop-loss for a SELL OCO",
		"The take-profit target should be higher than the stop-loss level",
	)
}

// TestValidateOCOOrderBuyRejectsTakeProfitAboveStopLoss verifies price relationship
// for BUY exits (closing a short position).
func TestValidateOCOOrderBuyRejectsTakeProfitAboveStopLoss(t *testing.T) {
	t.Parallel()

	err := ValidateOCOOrder(&OCOParams{
		Symbol:     "TSLA",
		Action:     models.InstructionBuy,
		Quantity:   10,
		TakeProfit: 200,
		StopLoss:   150,
	})

	assertValidationError(t, err,
		"take-profit must be below stop-loss for a BUY OCO",
		"The take-profit target should be lower than the stop-loss level",
	)
}

// TestValidateOCOOrderBuyAcceptsValidPrices verifies correct BUY price relationship passes.
func TestValidateOCOOrderBuyAcceptsValidPrices(t *testing.T) {
	t.Parallel()

	err := ValidateOCOOrder(&OCOParams{
		Symbol:     "TSLA",
		Action:     models.InstructionBuy,
		Quantity:   10,
		TakeProfit: 100,
		StopLoss:   200,
	})

	require.NoError(t, err)
}

// TestValidateBracketOrderRequiresAtLeastOneExit verifies that at least one exit is needed.
func TestValidateBracketOrderRequiresAtLeastOneExit(t *testing.T) {
	t.Parallel()

	err := ValidateBracketOrder(&BracketParams{
		Symbol:    "AAPL",
		Action:    models.InstructionBuy,
		Quantity:  10,
		OrderType: models.OrderTypeMarket,
	})

	assertValidationError(t, err, "bracket order requires at least one exit condition", "Add `--take-profit <amount>` and/or `--stop-loss <amount>`")
}

// TestValidateBracketOrderAcceptsStopLossOnly verifies stop-loss-only is valid.
func TestValidateBracketOrderAcceptsStopLossOnly(t *testing.T) {
	t.Parallel()

	err := ValidateBracketOrder(&BracketParams{
		Symbol:    "AAPL",
		Action:    models.InstructionBuy,
		Quantity:  10,
		OrderType: models.OrderTypeMarket,
		StopLoss:  140,
	})

	require.NoError(t, err)
}

// TestValidateBracketOrderAcceptsTakeProfitOnly verifies take-profit-only is valid.
func TestValidateBracketOrderAcceptsTakeProfitOnly(t *testing.T) {
	t.Parallel()

	err := ValidateBracketOrder(&BracketParams{
		Symbol:     "AAPL",
		Action:     models.InstructionBuy,
		Quantity:   10,
		OrderType:  models.OrderTypeMarket,
		TakeProfit: 160,
	})

	require.NoError(t, err)
}

// TestValidateVerticalOrderRequiresUnderlying verifies underlying symbol validation.
func TestValidateVerticalOrderRequiresUnderlying(t *testing.T) {
	t.Parallel()

	err := ValidateVerticalOrder(&VerticalParams{
		Expiration:  time.Now().UTC().Add(30 * 24 * time.Hour),
		LongStrike:  12,
		ShortStrike: 14,
		Quantity:    1,
		Price:       0.50,
	})

	assertValidationError(t, err, "underlying symbol is required", "Add `--underlying <TICKER>` to specify the underlying stock")
}

// TestValidateButterflyOrderRejectsUnorderedStrikes verifies butterfly strike ordering.
func TestValidateButterflyOrderRejectsUnorderedStrikes(t *testing.T) {
	t.Parallel()

	err := ValidateButterflyOrder(&ButterflyParams{
		Underlying: "F", Expiration: time.Now().UTC().Add(30 * 24 * time.Hour),
		LowerStrike: 12, MiddleStrike: 10, UpperStrike: 14, PutCall: models.PutCallCall,
		Buy: true, Open: true, Quantity: 1, Price: 0.50,
	})

	assertValidationError(t, err, "butterfly strikes must be ordered lower < middle < upper", "Use three increasing strikes with `--lower-strike`, `--middle-strike`, and `--upper-strike`")
}

// TestValidateButterflyOrderRejectsUnbalancedWings keeps the balanced strategy
// from accepting shapes that Schwab represents with UNBALANCED_BUTTERFLY.
func TestValidateButterflyOrderRejectsUnbalancedWings(t *testing.T) {
	t.Parallel()

	err := ValidateButterflyOrder(&ButterflyParams{
		Underlying: "F", Expiration: time.Now().UTC().Add(30 * 24 * time.Hour),
		LowerStrike: 10, MiddleStrike: 12, UpperStrike: 15, PutCall: models.PutCallCall,
		Buy: true, Open: true, Quantity: 1, Price: 0.50,
	})

	assertValidationError(t, err, "butterfly wings must be balanced", "Use equal distances from `--middle-strike` to `--lower-strike` and `--upper-strike`; unbalanced butterflies are a separate strategy type")
}

// TestValidateCondorOrderRejectsUnorderedStrikes verifies condor strike ordering.
func TestValidateCondorOrderRejectsUnorderedStrikes(t *testing.T) {
	t.Parallel()

	err := ValidateCondorOrder(&CondorParams{
		Underlying: "F", Expiration: time.Now().UTC().Add(30 * 24 * time.Hour),
		LowerStrike: 10, LowerMiddleStrike: 14, UpperMiddleStrike: 12, UpperStrike: 16, PutCall: models.PutCallPut,
		Buy: true, Open: true, Quantity: 1, Price: 0.50,
	})

	assertValidationError(t, err, "condor strikes must be ordered lower < lower-middle < upper-middle < upper", "Use four increasing strikes for the condor wings and middle strikes")
}

// TestValidateCondorOrderRejectsUnbalancedWings keeps the balanced strategy
// distinct from Schwab's UNBALANCED_CONDOR enum value.
func TestValidateCondorOrderRejectsUnbalancedWings(t *testing.T) {
	t.Parallel()

	err := ValidateCondorOrder(&CondorParams{
		Underlying: "F", Expiration: time.Now().UTC().Add(30 * 24 * time.Hour),
		LowerStrike: 10, LowerMiddleStrike: 12, UpperMiddleStrike: 14, UpperStrike: 17, PutCall: models.PutCallPut,
		Buy: true, Open: true, Quantity: 1, Price: 0.50,
	})

	assertValidationError(t, err, "condor wings must be balanced", "Use equal wing widths between lower/lower-middle and upper-middle/upper; unbalanced condors are a separate strategy type")
}

// TestValidateBackRatioOrderRejectsInvalidRatio verifies back-ratio quantity relationship.
func TestValidateBackRatioOrderRejectsInvalidRatio(t *testing.T) {
	t.Parallel()

	err := ValidateBackRatioOrder(&BackRatioParams{
		Underlying: "F", Expiration: time.Now().UTC().Add(30 * 24 * time.Hour),
		ShortStrike: 12, LongStrike: 14, PutCall: models.PutCallCall,
		Open: true, Quantity: 1, LongRatio: 1, Price: 0.20,
	})

	assertValidationError(t, err, "long ratio must be greater than one", "Use `--long-ratio 2` for the standard one-by-two back-ratio")
}

// TestValidateBackRatioOrderRejectsFractionalRatio prevents fractional option legs.
func TestValidateBackRatioOrderRejectsFractionalRatio(t *testing.T) {
	t.Parallel()

	err := ValidateBackRatioOrder(&BackRatioParams{
		Underlying: "F", Expiration: time.Now().UTC().Add(30 * 24 * time.Hour),
		ShortStrike: 12, LongStrike: 14, PutCall: models.PutCallCall,
		Open: true, Quantity: 1, LongRatio: 1.5, Price: 0.20,
	})

	assertValidationError(t, err, "long ratio must be a whole number", "Use an integer value like `--long-ratio 2`")
}

// TestValidateBackRatioOrderRejectsWrongCallDirection verifies the conventional
// call back-ratio shape: sell the lower strike and buy more contracts higher.
func TestValidateBackRatioOrderRejectsWrongCallDirection(t *testing.T) {
	t.Parallel()

	err := ValidateBackRatioOrder(&BackRatioParams{
		Underlying: "F", Expiration: time.Now().UTC().Add(30 * 24 * time.Hour),
		ShortStrike: 14, LongStrike: 12, PutCall: models.PutCallCall,
		Open: true, Quantity: 1, LongRatio: 2, Price: 0.20,
	})

	assertValidationError(t, err, "call back-ratio long strike must be above short strike", "For call back-ratios, sell the lower strike and buy more contracts at the higher strike")
}

// TestValidateBackRatioOrderRejectsWrongPutDirection verifies the conventional
// put back-ratio shape: sell the higher strike and buy more contracts lower.
func TestValidateBackRatioOrderRejectsWrongPutDirection(t *testing.T) {
	t.Parallel()

	err := ValidateBackRatioOrder(&BackRatioParams{
		Underlying: "F", Expiration: time.Now().UTC().Add(30 * 24 * time.Hour),
		ShortStrike: 12, LongStrike: 14, PutCall: models.PutCallPut,
		Open: true, Quantity: 1, LongRatio: 2, Price: 0.20,
	})

	assertValidationError(t, err, "put back-ratio long strike must be below short strike", "For put back-ratios, sell the higher strike and buy more contracts at the lower strike")
}

// TestValidateVerticalRollOrderRejectsEqualOpenStrikes verifies rolled verticals remain spreads.
func TestValidateVerticalRollOrderRejectsEqualOpenStrikes(t *testing.T) {
	t.Parallel()

	err := ValidateVerticalRollOrder(&VerticalRollParams{
		Underlying: "F", CloseExpiration: time.Now().UTC().Add(30 * 24 * time.Hour), OpenExpiration: time.Now().UTC().Add(60 * 24 * time.Hour),
		CloseLongStrike: 12, CloseShortStrike: 14, OpenLongStrike: 15, OpenShortStrike: 15,
		PutCall: models.PutCallCall, Quantity: 1, Price: 0.25,
	})

	assertValidationError(t, err, "open long and short strikes must be different", "Use different values for `--open-long-strike` and `--open-short-strike`")
}

// TestValidateVerticalRollOrderAcceptsSameExpirationStrikeRoll verifies users can
// roll a vertical up or down within the same expiration cycle.
func TestValidateVerticalRollOrderAcceptsSameExpirationStrikeRoll(t *testing.T) {
	t.Parallel()

	expiration := time.Now().UTC().Add(30 * 24 * time.Hour)

	err := ValidateVerticalRollOrder(&VerticalRollParams{
		Underlying: "F", CloseExpiration: expiration, OpenExpiration: expiration,
		CloseLongStrike: 12, CloseShortStrike: 14, OpenLongStrike: 13, OpenShortStrike: 15,
		PutCall: models.PutCallCall, Quantity: 1, Price: 0.25,
	})

	require.NoError(t, err)
}

// TestValidateVerticalRollOrderRejectsEarlierOpenExpiration verifies rolls never
// open a new spread before the spread being closed.
func TestValidateVerticalRollOrderRejectsEarlierOpenExpiration(t *testing.T) {
	t.Parallel()

	closeExpiration := time.Now().UTC().Add(60 * 24 * time.Hour)
	openExpiration := time.Now().UTC().Add(30 * 24 * time.Hour)

	err := ValidateVerticalRollOrder(&VerticalRollParams{
		Underlying: "F", CloseExpiration: closeExpiration, OpenExpiration: openExpiration,
		CloseLongStrike: 12, CloseShortStrike: 14, OpenLongStrike: 13, OpenShortStrike: 15,
		PutCall: models.PutCallCall, Quantity: 1, Price: 0.25,
	})

	assertValidationError(t, err, "open expiration must not be before close expiration", "Use `--open-expiration` on or after `--close-expiration`")
}

// TestValidateVerticalRollOrderRejectsUnequalWidths preserves the distinction
// between balanced VERTICAL_ROLL and Schwab's UNBALANCED_VERTICAL_ROLL enum.
func TestValidateVerticalRollOrderRejectsUnequalWidths(t *testing.T) {
	t.Parallel()

	err := ValidateVerticalRollOrder(&VerticalRollParams{
		Underlying: "F", CloseExpiration: time.Now().UTC().Add(30 * 24 * time.Hour), OpenExpiration: time.Now().UTC().Add(60 * 24 * time.Hour),
		CloseLongStrike: 12, CloseShortStrike: 14, OpenLongStrike: 13, OpenShortStrike: 16,
		PutCall: models.PutCallCall, Quantity: 1, Price: 0.25,
	})

	assertValidationError(t, err, "vertical roll widths must match", "Use equal close and open spread widths for `VERTICAL_ROLL`; unequal widths are a separate unbalanced strategy type")
}

// TestValidateVerticalRollOrderRejectsNoopRoll catches close/open legs that would
// close and reopen the same vertical without changing strikes or expiration.
func TestValidateVerticalRollOrderRejectsNoopRoll(t *testing.T) {
	t.Parallel()

	expiration := time.Now().UTC().Add(30 * 24 * time.Hour)

	err := ValidateVerticalRollOrder(&VerticalRollParams{
		Underlying: "F", CloseExpiration: expiration, OpenExpiration: expiration,
		CloseLongStrike: 12, CloseShortStrike: 14, OpenLongStrike: 12, OpenShortStrike: 14,
		PutCall: models.PutCallCall, Quantity: 1, Price: 0.25,
	})

	assertValidationError(t, err, "vertical roll must change strikes or expiration", "Use different open strikes or a later `--open-expiration` so the roll changes the position")
}

// TestValidateDoubleDiagonalOrderRejectsOverlappingStrikes verifies put/call sides do not overlap.
func TestValidateDoubleDiagonalOrderRejectsOverlappingStrikes(t *testing.T) {
	t.Parallel()

	err := ValidateDoubleDiagonalOrder(&DoubleDiagonalParams{
		Underlying: "F", NearExpiration: time.Now().UTC().Add(30 * 24 * time.Hour), FarExpiration: time.Now().UTC().Add(60 * 24 * time.Hour),
		PutFarStrike: 9, PutNearStrike: 15, CallNearStrike: 14, CallFarStrike: 16,
		Open: true, Quantity: 1, Price: 0.80,
	})

	assertValidationError(t, err, "double diagonal strikes must be ordered put-far < put-near < call-near < call-far", "Keep put strikes below call strikes so the near short legs do not overlap")
}

// TestValidateNewOptionStrategiesCommonFailures covers validation branches that
// are shared by the new strategy builders. These are user-facing errors, not
// coverage-only paths, because every builder can receive malformed direct API
// input even when CLI flag constraints catch many cases earlier.
func TestValidateNewOptionStrategiesCommonFailures(t *testing.T) {
	t.Parallel()

	future := time.Now().UTC().Add(30 * 24 * time.Hour)
	later := future.Add(30 * 24 * time.Hour)
	past := time.Now().UTC().Add(-24 * time.Hour)

	testCases := []struct {
		name    string
		check   func() error
		message string
	}{
		{
			name: "butterfly missing underlying",
			check: func() error {
				return ValidateButterflyOrder(&ButterflyParams{Expiration: future, LowerStrike: 10, MiddleStrike: 12, UpperStrike: 14, PutCall: models.PutCallCall, Quantity: 1, Price: 0.50})
			},
			message: "underlying symbol is required",
		},
		{
			name: "butterfly zero quantity",
			check: func() error {
				return ValidateButterflyOrder(&ButterflyParams{Underlying: "F", Expiration: future, LowerStrike: 10, MiddleStrike: 12, UpperStrike: 14, PutCall: models.PutCallCall, Price: 0.50})
			},
			message: "quantity must be greater than zero",
		},
		{
			name: "butterfly past expiration",
			check: func() error {
				return ValidateButterflyOrder(&ButterflyParams{Underlying: "F", Expiration: past, LowerStrike: 10, MiddleStrike: 12, UpperStrike: 14, PutCall: models.PutCallCall, Quantity: 1, Price: 0.50})
			},
			message: "option expiration date is in the past",
		},
		{
			name: "butterfly negative strike",
			check: func() error {
				return ValidateButterflyOrder(&ButterflyParams{Underlying: "F", Expiration: future, LowerStrike: -10, MiddleStrike: 12, UpperStrike: 14, PutCall: models.PutCallCall, Quantity: 1, Price: 0.50})
			},
			message: "lower-strike must be greater than zero",
		},
		{
			name: "butterfly missing option type",
			check: func() error {
				return ValidateButterflyOrder(&ButterflyParams{Underlying: "F", Expiration: future, LowerStrike: 10, MiddleStrike: 12, UpperStrike: 14, Quantity: 1, Price: 0.50})
			},
			message: "option type (call or put) is required",
		},
		{
			name: "butterfly missing price",
			check: func() error {
				return ValidateButterflyOrder(&ButterflyParams{Underlying: "F", Expiration: future, LowerStrike: 10, MiddleStrike: 12, UpperStrike: 14, PutCall: models.PutCallCall, Quantity: 1})
			},
			message: "price is required for butterflies",
		},
		{
			name: "condor missing option type",
			check: func() error {
				return ValidateCondorOrder(&CondorParams{Underlying: "F", Expiration: future, LowerStrike: 10, LowerMiddleStrike: 12, UpperMiddleStrike: 14, UpperStrike: 16, Quantity: 1, Price: 0.50})
			},
			message: "option type (call or put) is required",
		},
		{
			name: "condor missing underlying",
			check: func() error {
				return ValidateCondorOrder(&CondorParams{Expiration: future, LowerStrike: 10, LowerMiddleStrike: 12, UpperMiddleStrike: 14, UpperStrike: 16, PutCall: models.PutCallPut, Quantity: 1, Price: 0.50})
			},
			message: "underlying symbol is required",
		},
		{
			name: "condor zero quantity",
			check: func() error {
				return ValidateCondorOrder(&CondorParams{Underlying: "F", Expiration: future, LowerStrike: 10, LowerMiddleStrike: 12, UpperMiddleStrike: 14, UpperStrike: 16, PutCall: models.PutCallPut, Price: 0.50})
			},
			message: "quantity must be greater than zero",
		},
		{
			name: "condor past expiration",
			check: func() error {
				return ValidateCondorOrder(&CondorParams{Underlying: "F", Expiration: past, LowerStrike: 10, LowerMiddleStrike: 12, UpperMiddleStrike: 14, UpperStrike: 16, PutCall: models.PutCallPut, Quantity: 1, Price: 0.50})
			},
			message: "option expiration date is in the past",
		},
		{
			name: "condor negative strike",
			check: func() error {
				return ValidateCondorOrder(&CondorParams{Underlying: "F", Expiration: future, LowerStrike: 10, LowerMiddleStrike: -12, UpperMiddleStrike: 14, UpperStrike: 16, PutCall: models.PutCallPut, Quantity: 1, Price: 0.50})
			},
			message: "lower-middle-strike must be greater than zero",
		},
		{
			name: "condor missing price",
			check: func() error {
				return ValidateCondorOrder(&CondorParams{Underlying: "F", Expiration: future, LowerStrike: 10, LowerMiddleStrike: 12, UpperMiddleStrike: 14, UpperStrike: 16, PutCall: models.PutCallPut, Quantity: 1})
			},
			message: "price is required for condors",
		},
		{
			name: "back-ratio equal strikes",
			check: func() error {
				return ValidateBackRatioOrder(&BackRatioParams{Underlying: "F", Expiration: future, ShortStrike: 12, LongStrike: 12, PutCall: models.PutCallCall, Quantity: 1, LongRatio: 2, Price: 0.20})
			},
			message: "short and long strikes must be different",
		},
		{
			name: "back-ratio missing underlying",
			check: func() error {
				return ValidateBackRatioOrder(&BackRatioParams{Expiration: future, ShortStrike: 12, LongStrike: 14, PutCall: models.PutCallCall, Quantity: 1, LongRatio: 2, Price: 0.20})
			},
			message: "underlying symbol is required",
		},
		{
			name: "back-ratio zero quantity",
			check: func() error {
				return ValidateBackRatioOrder(&BackRatioParams{Underlying: "F", Expiration: future, ShortStrike: 12, LongStrike: 14, PutCall: models.PutCallCall, LongRatio: 2, Price: 0.20})
			},
			message: "quantity must be greater than zero",
		},
		{
			name: "back-ratio past expiration",
			check: func() error {
				return ValidateBackRatioOrder(&BackRatioParams{Underlying: "F", Expiration: past, ShortStrike: 12, LongStrike: 14, PutCall: models.PutCallCall, Quantity: 1, LongRatio: 2, Price: 0.20})
			},
			message: "option expiration date is in the past",
		},
		{
			name: "back-ratio negative strike",
			check: func() error {
				return ValidateBackRatioOrder(&BackRatioParams{Underlying: "F", Expiration: future, ShortStrike: -12, LongStrike: 14, PutCall: models.PutCallCall, Quantity: 1, LongRatio: 2, Price: 0.20})
			},
			message: "short-strike must be greater than zero",
		},
		{
			name: "back-ratio missing option type",
			check: func() error {
				return ValidateBackRatioOrder(&BackRatioParams{Underlying: "F", Expiration: future, ShortStrike: 12, LongStrike: 14, Quantity: 1, LongRatio: 2, Price: 0.20})
			},
			message: "option type (call or put) is required",
		},
		{
			name: "back-ratio missing price",
			check: func() error {
				return ValidateBackRatioOrder(&BackRatioParams{Underlying: "F", Expiration: future, ShortStrike: 12, LongStrike: 14, PutCall: models.PutCallCall, Quantity: 1, LongRatio: 2})
			},
			message: "price is required for back-ratios",
		},
		{
			name: "vertical-roll equal close strikes",
			check: func() error {
				return ValidateVerticalRollOrder(&VerticalRollParams{Underlying: "F", CloseExpiration: future, OpenExpiration: later, CloseLongStrike: 12, CloseShortStrike: 12, OpenLongStrike: 13, OpenShortStrike: 15, PutCall: models.PutCallCall, Quantity: 1, Price: 0.25})
			},
			message: "close long and short strikes must be different",
		},
		{
			name: "vertical-roll missing underlying",
			check: func() error {
				return ValidateVerticalRollOrder(&VerticalRollParams{CloseExpiration: future, OpenExpiration: later, CloseLongStrike: 12, CloseShortStrike: 14, OpenLongStrike: 13, OpenShortStrike: 15, PutCall: models.PutCallCall, Quantity: 1, Price: 0.25})
			},
			message: "underlying symbol is required",
		},
		{
			name: "vertical-roll zero quantity",
			check: func() error {
				return ValidateVerticalRollOrder(&VerticalRollParams{Underlying: "F", CloseExpiration: future, OpenExpiration: later, CloseLongStrike: 12, CloseShortStrike: 14, OpenLongStrike: 13, OpenShortStrike: 15, PutCall: models.PutCallCall, Price: 0.25})
			},
			message: "quantity must be greater than zero",
		},
		{
			name: "vertical-roll past close expiration",
			check: func() error {
				return ValidateVerticalRollOrder(&VerticalRollParams{Underlying: "F", CloseExpiration: past, OpenExpiration: later, CloseLongStrike: 12, CloseShortStrike: 14, OpenLongStrike: 13, OpenShortStrike: 15, PutCall: models.PutCallCall, Quantity: 1, Price: 0.25})
			},
			message: "option expiration date is in the past",
		},
		{
			name: "vertical-roll negative strike",
			check: func() error {
				return ValidateVerticalRollOrder(&VerticalRollParams{Underlying: "F", CloseExpiration: future, OpenExpiration: later, CloseLongStrike: 12, CloseShortStrike: 14, OpenLongStrike: -13, OpenShortStrike: 15, PutCall: models.PutCallCall, Quantity: 1, Price: 0.25})
			},
			message: "open-long-strike must be greater than zero",
		},
		{
			name: "vertical-roll missing option type",
			check: func() error {
				return ValidateVerticalRollOrder(&VerticalRollParams{Underlying: "F", CloseExpiration: future, OpenExpiration: later, CloseLongStrike: 12, CloseShortStrike: 14, OpenLongStrike: 13, OpenShortStrike: 15, Quantity: 1, Price: 0.25})
			},
			message: "option type (call or put) is required",
		},
		{
			name: "vertical-roll missing price",
			check: func() error {
				return ValidateVerticalRollOrder(&VerticalRollParams{Underlying: "F", CloseExpiration: future, OpenExpiration: later, CloseLongStrike: 12, CloseShortStrike: 14, OpenLongStrike: 13, OpenShortStrike: 15, PutCall: models.PutCallCall, Quantity: 1})
			},
			message: "price is required for vertical rolls",
		},
		{
			name: "double-diagonal missing underlying",
			check: func() error {
				return ValidateDoubleDiagonalOrder(&DoubleDiagonalParams{NearExpiration: future, FarExpiration: later, PutFarStrike: 9, PutNearStrike: 10, CallNearStrike: 14, CallFarStrike: 15, Quantity: 1, Price: 0.80})
			},
			message: "underlying symbol is required",
		},
		{
			name: "double-diagonal zero quantity",
			check: func() error {
				return ValidateDoubleDiagonalOrder(&DoubleDiagonalParams{Underlying: "F", NearExpiration: future, FarExpiration: later, PutFarStrike: 9, PutNearStrike: 10, CallNearStrike: 14, CallFarStrike: 15, Price: 0.80})
			},
			message: "quantity must be greater than zero",
		},
		{
			name: "double-diagonal past near expiration",
			check: func() error {
				return ValidateDoubleDiagonalOrder(&DoubleDiagonalParams{Underlying: "F", NearExpiration: past, FarExpiration: later, PutFarStrike: 9, PutNearStrike: 10, CallNearStrike: 14, CallFarStrike: 15, Quantity: 1, Price: 0.80})
			},
			message: "option expiration date is in the past",
		},
		{
			name: "double-diagonal near not before far",
			check: func() error {
				return ValidateDoubleDiagonalOrder(&DoubleDiagonalParams{Underlying: "F", NearExpiration: later, FarExpiration: future, PutFarStrike: 9, PutNearStrike: 10, CallNearStrike: 14, CallFarStrike: 15, Quantity: 1, Price: 0.80})
			},
			message: "near expiration must be before far expiration",
		},
		{
			name: "double-diagonal negative strike",
			check: func() error {
				return ValidateDoubleDiagonalOrder(&DoubleDiagonalParams{Underlying: "F", NearExpiration: future, FarExpiration: later, PutFarStrike: -9, PutNearStrike: 10, CallNearStrike: 14, CallFarStrike: 15, Quantity: 1, Price: 0.80})
			},
			message: "put-far-strike must be greater than zero",
		},
		{
			name: "double-diagonal missing price",
			check: func() error {
				return ValidateDoubleDiagonalOrder(&DoubleDiagonalParams{Underlying: "F", NearExpiration: future, FarExpiration: later, PutFarStrike: 9, PutNearStrike: 10, CallNearStrike: 14, CallFarStrike: 15, Quantity: 1})
			},
			message: "price is required for double diagonals",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			err := testCase.check()

			var validationErr *apperr.ValidationError
			require.ErrorAs(t, err, &validationErr)
			assert.Equal(t, testCase.message, validationErr.Message)
		})
	}
}

// TestValidateVerticalOrderRejectsEqualStrikes verifies same-strike rejection.
func TestValidateVerticalOrderRejectsEqualStrikes(t *testing.T) {
	t.Parallel()

	err := ValidateVerticalOrder(&VerticalParams{
		Underlying:  "F",
		Expiration:  time.Now().UTC().Add(30 * 24 * time.Hour),
		LongStrike:  12,
		ShortStrike: 12,
		Quantity:    1,
		Price:       0.50,
	})

	assertValidationError(t, err, "long and short strikes must be different", "Use different values for `--long-strike` and `--short-strike`")
}

// TestValidateVerticalOrderRequiresPrice verifies price validation.
func TestValidateVerticalOrderRequiresPrice(t *testing.T) {
	t.Parallel()

	err := ValidateVerticalOrder(&VerticalParams{
		Underlying:  "F",
		Expiration:  time.Now().UTC().Add(30 * 24 * time.Hour),
		LongStrike:  12,
		ShortStrike: 14,
		Quantity:    1,
	})

	assertValidationError(t, err, "price is required for vertical spreads", "Add `--price <amount>` to specify the net debit or credit")
}

// TestValidateVerticalOrderAcceptsValidParams verifies valid parameters pass.
func TestValidateVerticalOrderAcceptsValidParams(t *testing.T) {
	t.Parallel()

	err := ValidateVerticalOrder(&VerticalParams{
		Underlying:  "F",
		Expiration:  time.Now().UTC().Add(30 * 24 * time.Hour),
		LongStrike:  12,
		ShortStrike: 14,
		Quantity:    1,
		Price:       0.50,
	})

	require.NoError(t, err)
}

// TestValidateIronCondorRequiresUnderlying verifies underlying is required.
func TestValidateIronCondorRequiresUnderlying(t *testing.T) {
	t.Parallel()

	err := ValidateIronCondorOrder(&IronCondorParams{
		Expiration:      time.Now().UTC().Add(30 * 24 * time.Hour),
		PutLongStrike:   8,
		PutShortStrike:  10,
		CallShortStrike: 14,
		CallLongStrike:  16,
		Quantity:        1,
		Price:           1.50,
	})

	assertValidationError(t, err, "underlying symbol is required", "Add `--underlying <TICKER>` to specify the underlying stock")
}

// TestValidateIronCondorRejectsUnorderedStrikes verifies strike ordering enforcement.
func TestValidateIronCondorRejectsUnorderedStrikes(t *testing.T) {
	t.Parallel()

	// put-long >= put-short
	err := ValidateIronCondorOrder(&IronCondorParams{
		Underlying:      "F",
		Expiration:      time.Now().UTC().Add(30 * 24 * time.Hour),
		PutLongStrike:   10,
		PutShortStrike:  10,
		CallShortStrike: 14,
		CallLongStrike:  16,
		Quantity:        1,
		Price:           1.50,
	})

	assertValidationError(t, err, "put-long-strike must be below put-short-strike",
		"The protective put (bought) must be at a lower strike than the sold put")
}

// TestValidateIronCondorRejectsOverlappingPutCall verifies put-short < call-short.
func TestValidateIronCondorRejectsOverlappingPutCall(t *testing.T) {
	t.Parallel()

	err := ValidateIronCondorOrder(&IronCondorParams{
		Underlying:      "F",
		Expiration:      time.Now().UTC().Add(30 * 24 * time.Hour),
		PutLongStrike:   8,
		PutShortStrike:  14,
		CallShortStrike: 12,
		CallLongStrike:  16,
		Quantity:        1,
		Price:           1.50,
	})

	assertValidationError(t, err, "put-short-strike must be below call-short-strike",
		"The put and call short strikes define the profit zone and must not overlap")
}

// TestValidateIronCondorAcceptsValidParams verifies valid parameters pass.
func TestValidateIronCondorAcceptsValidParams(t *testing.T) {
	t.Parallel()

	err := ValidateIronCondorOrder(&IronCondorParams{
		Underlying:      "F",
		Expiration:      time.Now().UTC().Add(30 * 24 * time.Hour),
		PutLongStrike:   8,
		PutShortStrike:  10,
		CallShortStrike: 14,
		CallLongStrike:  16,
		Quantity:        1,
		Price:           1.50,
	})

	require.NoError(t, err)
}

// TestValidateStrangleRequiresUnderlying verifies underlying is required.
func TestValidateStrangleRequiresUnderlying(t *testing.T) {
	t.Parallel()

	err := ValidateStrangleOrder(&StrangleParams{
		Expiration: time.Now().UTC().Add(30 * 24 * time.Hour),
		CallStrike: 14,
		PutStrike:  10,
		Quantity:   1,
		Price:      0.80,
	})

	assertValidationError(t, err, "underlying symbol is required", "Add `--underlying <TICKER>` to specify the underlying stock")
}

// TestValidateStrangleRejectsEqualStrikes verifies same-strike rejection.
func TestValidateStrangleRejectsEqualStrikes(t *testing.T) {
	t.Parallel()

	err := ValidateStrangleOrder(&StrangleParams{
		Underlying: "F",
		Expiration: time.Now().UTC().Add(30 * 24 * time.Hour),
		CallStrike: 12,
		PutStrike:  12,
		Quantity:   1,
		Price:      0.80,
	})

	assertValidationError(t, err, "call and put strikes must be different (use straddle for same strike)",
		"Use different values for `--call-strike` and `--put-strike`")
}

// TestValidateStrangleRequiresPrice verifies price is required.
func TestValidateStrangleRequiresPrice(t *testing.T) {
	t.Parallel()

	err := ValidateStrangleOrder(&StrangleParams{
		Underlying: "F",
		Expiration: time.Now().UTC().Add(30 * 24 * time.Hour),
		CallStrike: 14,
		PutStrike:  10,
		Quantity:   1,
	})

	assertValidationError(t, err, "price is required for strangles", "Add `--price <amount>` to specify the net debit or credit")
}

// TestValidateStrangleAcceptsValidParams verifies valid parameters pass.
func TestValidateStrangleAcceptsValidParams(t *testing.T) {
	t.Parallel()

	err := ValidateStrangleOrder(&StrangleParams{
		Underlying: "F",
		Expiration: time.Now().UTC().Add(30 * 24 * time.Hour),
		CallStrike: 14,
		PutStrike:  10,
		Buy:        true,
		Open:       true,
		Quantity:   1,
		Price:      0.80,
	})

	require.NoError(t, err)
}

// TestValidateStraddleRequiresUnderlying verifies underlying is required.
func TestValidateStraddleRequiresUnderlying(t *testing.T) {
	t.Parallel()

	err := ValidateStraddleOrder(&StraddleParams{
		Expiration: time.Now().UTC().Add(30 * 24 * time.Hour),
		Strike:     12,
		Quantity:   1,
		Price:      1.50,
	})

	assertValidationError(t, err, "underlying symbol is required", "Add `--underlying <TICKER>` to specify the underlying stock")
}

// TestValidateStraddleRequiresStrike verifies strike is required.
func TestValidateStraddleRequiresStrike(t *testing.T) {
	t.Parallel()

	err := ValidateStraddleOrder(&StraddleParams{
		Underlying: "F",
		Expiration: time.Now().UTC().Add(30 * 24 * time.Hour),
		Quantity:   1,
		Price:      1.50,
	})

	assertValidationError(t, err, "strike price must be greater than zero", "Add `--strike <price>` with a positive value")
}

// TestValidateStraddleRequiresPrice verifies price is required.
func TestValidateStraddleRequiresPrice(t *testing.T) {
	t.Parallel()

	err := ValidateStraddleOrder(&StraddleParams{
		Underlying: "F",
		Expiration: time.Now().UTC().Add(30 * 24 * time.Hour),
		Strike:     12,
		Quantity:   1,
	})

	assertValidationError(t, err, "price is required for straddles", "Add `--price <amount>` to specify the net debit or credit")
}

// TestValidateStraddleAcceptsValidParams verifies valid parameters pass.
func TestValidateStraddleAcceptsValidParams(t *testing.T) {
	t.Parallel()

	err := ValidateStraddleOrder(&StraddleParams{
		Underlying: "F",
		Expiration: time.Now().UTC().Add(30 * 24 * time.Hour),
		Strike:     12,
		Buy:        true,
		Open:       true,
		Quantity:   1,
		Price:      1.50,
	})

	require.NoError(t, err)
}

// TestValidateCoveredCallOrderRequiresUnderlying verifies underlying is mandatory.
func TestValidateCoveredCallOrderRequiresUnderlying(t *testing.T) {
	t.Parallel()

	err := ValidateCoveredCallOrder(&CoveredCallParams{
		Expiration: time.Now().UTC().Add(30 * 24 * time.Hour),
		Strike:     14,
		Quantity:   1,
		Price:      12.00,
	})

	assertValidationError(t, err, "underlying symbol is required", "Add `--underlying <TICKER>` to specify the underlying stock")
}

// TestValidateCoveredCallOrderRequiresStrike verifies strike is mandatory.
func TestValidateCoveredCallOrderRequiresStrike(t *testing.T) {
	t.Parallel()

	err := ValidateCoveredCallOrder(&CoveredCallParams{
		Underlying: "F",
		Expiration: time.Now().UTC().Add(30 * 24 * time.Hour),
		Quantity:   1,
		Price:      12.00,
	})

	assertValidationError(t, err, "call strike price must be greater than zero", "Add `--strike <price>` with a positive value")
}

// TestValidateCoveredCallOrderRequiresPrice verifies price is mandatory.
func TestValidateCoveredCallOrderRequiresPrice(t *testing.T) {
	t.Parallel()

	err := ValidateCoveredCallOrder(&CoveredCallParams{
		Underlying: "F",
		Expiration: time.Now().UTC().Add(30 * 24 * time.Hour),
		Strike:     14,
		Quantity:   1,
	})

	assertValidationError(t, err, "price is required for covered calls", "Add `--price <amount>` to specify the net debit")
}

// TestValidateCoveredCallOrderAcceptsValidParams verifies a valid covered call passes.
func TestValidateCoveredCallOrderAcceptsValidParams(t *testing.T) {
	t.Parallel()

	err := ValidateCoveredCallOrder(&CoveredCallParams{
		Underlying: "F",
		Expiration: time.Now().UTC().Add(30 * 24 * time.Hour),
		Strike:     14,
		Quantity:   1,
		Price:      12.00,
	})

	require.NoError(t, err)
}

// TestValidateCollarOrderRequiresUnderlying verifies underlying is mandatory.
func TestValidateCollarOrderRequiresUnderlying(t *testing.T) {
	t.Parallel()

	err := ValidateCollarOrder(&CollarParams{
		Expiration: time.Now().UTC().Add(30 * 24 * time.Hour),
		PutStrike:  140,
		CallStrike: 160,
		Quantity:   1,
		Price:      150.00,
	})

	assertValidationError(t, err, "underlying symbol is required", "Add `--underlying <TICKER>` to specify the underlying stock")
}

// TestValidateCollarOrderRequiresPutStrike verifies put strike is mandatory.
func TestValidateCollarOrderRequiresPutStrike(t *testing.T) {
	t.Parallel()

	err := ValidateCollarOrder(&CollarParams{
		Underlying: "AAPL",
		Expiration: time.Now().UTC().Add(30 * 24 * time.Hour),
		CallStrike: 160,
		Quantity:   1,
		Price:      150.00,
	})

	assertValidationError(t, err, "put strike price must be greater than zero", "Add `--put-strike <price>` with a positive value")
}

// TestValidateCollarOrderRequiresCallStrike verifies call strike is mandatory.
func TestValidateCollarOrderRequiresCallStrike(t *testing.T) {
	t.Parallel()

	err := ValidateCollarOrder(&CollarParams{
		Underlying: "AAPL",
		Expiration: time.Now().UTC().Add(30 * 24 * time.Hour),
		PutStrike:  140,
		Quantity:   1,
		Price:      150.00,
	})

	assertValidationError(t, err, "call strike price must be greater than zero", "Add `--call-strike <price>` with a positive value")
}

// TestValidateCollarOrderRejectsInvertedStrikes verifies put strike must be below call strike.
func TestValidateCollarOrderRejectsInvertedStrikes(t *testing.T) {
	t.Parallel()

	err := ValidateCollarOrder(&CollarParams{
		Underlying: "AAPL",
		Expiration: time.Now().UTC().Add(30 * 24 * time.Hour),
		PutStrike:  160,
		CallStrike: 140,
		Quantity:   1,
		Price:      150.00,
	})

	assertValidationError(t, err,
		"put strike must be below call strike for a collar",
		"Use `--put-strike` below `--call-strike` (protective put sits below the covered call)",
	)
}

// TestValidateCollarOrderRequiresExpiration verifies expiration is mandatory.
func TestValidateCollarOrderRequiresExpiration(t *testing.T) {
	t.Parallel()

	err := ValidateCollarOrder(&CollarParams{
		Underlying: "AAPL",
		PutStrike:  140,
		CallStrike: 160,
		Quantity:   1,
		Price:      150.00,
	})

	assertValidationError(t, err, "option expiration date is in the past", "Use a future expiration date with `--expiration YYYY-MM-DD`")
}

// TestValidateCollarOrderRequiresQuantity verifies quantity must be positive.
func TestValidateCollarOrderRequiresQuantity(t *testing.T) {
	t.Parallel()

	err := ValidateCollarOrder(&CollarParams{
		Underlying: "AAPL",
		Expiration: time.Now().UTC().Add(30 * 24 * time.Hour),
		PutStrike:  140,
		CallStrike: 160,
		Price:      150.00,
	})

	assertValidationError(t, err, "quantity must be greater than zero", "Add `--quantity <number>` with a positive value")
}

// TestValidateCollarOrderAcceptsValidParams verifies a valid collar passes.
func TestValidateCollarOrderAcceptsValidParams(t *testing.T) {
	t.Parallel()

	err := ValidateCollarOrder(&CollarParams{
		Underlying: "AAPL",
		Expiration: time.Now().UTC().Add(30 * 24 * time.Hour),
		PutStrike:  140,
		CallStrike: 160,
		Quantity:   1,
		Price:      150.00,
	})

	require.NoError(t, err)
}

// TestValidateCalendarOrderRequiresUnderlying verifies underlying is mandatory.
func TestValidateCalendarOrderRequiresUnderlying(t *testing.T) {
	t.Parallel()

	err := ValidateCalendarOrder(&CalendarParams{
		NearExpiration: time.Now().UTC().Add(30 * 24 * time.Hour),
		FarExpiration:  time.Now().UTC().Add(60 * 24 * time.Hour),
		Strike:         150,
		Quantity:       1,
		Price:          2.50,
	})

	assertValidationError(t, err, "underlying symbol is required", "Add `--underlying <TICKER>` to specify the underlying stock")
}

// TestValidateCalendarOrderRejectsSameExpiration verifies that both expirations
// being identical is rejected. Calendar spreads require different expirations.
func TestValidateCalendarOrderRejectsSameExpiration(t *testing.T) {
	t.Parallel()

	sameDate := time.Now().UTC().Add(30 * 24 * time.Hour)

	err := ValidateCalendarOrder(&CalendarParams{
		Underlying:     "AAPL",
		NearExpiration: sameDate,
		FarExpiration:  sameDate,
		Strike:         150,
		Quantity:       1,
		Price:          2.50,
	})

	assertValidationError(t, err,
		"near and far expirations must be different for calendar spreads",
		"Use different dates for `--near-expiration` and `--far-expiration`",
	)
}

// TestValidateCalendarOrderRejectsNearAfterFar verifies that near expiration
// must come before far expiration.
func TestValidateCalendarOrderRejectsNearAfterFar(t *testing.T) {
	t.Parallel()

	err := ValidateCalendarOrder(&CalendarParams{
		Underlying:     "AAPL",
		NearExpiration: time.Now().UTC().Add(60 * 24 * time.Hour),
		FarExpiration:  time.Now().UTC().Add(30 * 24 * time.Hour),
		Strike:         150,
		Quantity:       1,
		Price:          2.50,
	})

	assertValidationError(t, err,
		"near expiration must be before far expiration",
		"Swap `--near-expiration` and `--far-expiration` so the near date comes first",
	)
}

// TestValidateCalendarOrderRequiresStrike verifies strike is mandatory.
func TestValidateCalendarOrderRequiresStrike(t *testing.T) {
	t.Parallel()

	err := ValidateCalendarOrder(&CalendarParams{
		Underlying:     "AAPL",
		NearExpiration: time.Now().UTC().Add(30 * 24 * time.Hour),
		FarExpiration:  time.Now().UTC().Add(60 * 24 * time.Hour),
		Quantity:       1,
		Price:          2.50,
	})

	assertValidationError(t, err, "strike price must be greater than zero", "Add `--strike <price>` with a positive value")
}

// TestValidateCalendarOrderRequiresPrice verifies price is mandatory.
func TestValidateCalendarOrderRequiresPrice(t *testing.T) {
	t.Parallel()

	err := ValidateCalendarOrder(&CalendarParams{
		Underlying:     "AAPL",
		NearExpiration: time.Now().UTC().Add(30 * 24 * time.Hour),
		FarExpiration:  time.Now().UTC().Add(60 * 24 * time.Hour),
		Strike:         150,
		PutCall:        models.PutCallCall,
		Quantity:       1,
	})

	assertValidationError(t, err, "price is required for calendar spreads", "Add `--price <amount>` to specify the net debit or credit")
}

// TestValidateCalendarOrderRequiresQuantity verifies quantity is mandatory.
func TestValidateCalendarOrderRequiresQuantity(t *testing.T) {
	t.Parallel()

	err := ValidateCalendarOrder(&CalendarParams{
		Underlying:     "AAPL",
		NearExpiration: time.Now().UTC().Add(30 * 24 * time.Hour),
		FarExpiration:  time.Now().UTC().Add(60 * 24 * time.Hour),
		Strike:         150,
		Price:          2.50,
	})

	assertValidationError(t, err, "quantity must be greater than zero", "Add `--quantity <number>` with a positive value")
}

// TestValidateCalendarOrderRejectsMissingPutCall verifies PutCall is mandatory.
func TestValidateCalendarOrderRejectsMissingPutCall(t *testing.T) {
	t.Parallel()

	err := ValidateCalendarOrder(&CalendarParams{
		Underlying:     "AAPL",
		NearExpiration: time.Now().UTC().Add(30 * 24 * time.Hour),
		FarExpiration:  time.Now().UTC().Add(60 * 24 * time.Hour),
		Strike:         150,
		Quantity:       1,
		Price:          2.50,
	})

	assertValidationError(t, err, "option type (call or put) is required", "Add `--call` or `--put`")
}

// TestValidateCalendarOrderAcceptsValidParams verifies a valid calendar passes.
func TestValidateCalendarOrderAcceptsValidParams(t *testing.T) {
	t.Parallel()

	err := ValidateCalendarOrder(&CalendarParams{
		Underlying:     "AAPL",
		NearExpiration: time.Now().UTC().Add(30 * 24 * time.Hour),
		FarExpiration:  time.Now().UTC().Add(60 * 24 * time.Hour),
		Strike:         150,
		PutCall:        models.PutCallCall,
		Quantity:       1,
		Price:          2.50,
	})

	require.NoError(t, err)
}

// TestValidateDiagonalOrderRequiresUnderlying verifies underlying is mandatory.
func TestValidateDiagonalOrderRequiresUnderlying(t *testing.T) {
	t.Parallel()

	err := ValidateDiagonalOrder(&DiagonalParams{
		NearExpiration: time.Now().UTC().Add(30 * 24 * time.Hour),
		FarExpiration:  time.Now().UTC().Add(60 * 24 * time.Hour),
		NearStrike:     150,
		FarStrike:      155,
		Quantity:       1,
		Price:          3.00,
	})

	assertValidationError(t, err, "underlying symbol is required", "Add `--underlying <TICKER>` to specify the underlying stock")
}

// TestValidateDiagonalOrderRejectsSameExpiration verifies that both expirations
// being identical is rejected.
func TestValidateDiagonalOrderRejectsSameExpiration(t *testing.T) {
	t.Parallel()

	sameDate := time.Now().UTC().Add(30 * 24 * time.Hour)

	err := ValidateDiagonalOrder(&DiagonalParams{
		Underlying:     "AAPL",
		NearExpiration: sameDate,
		FarExpiration:  sameDate,
		NearStrike:     150,
		FarStrike:      155,
		Quantity:       1,
		Price:          3.00,
	})

	assertValidationError(t, err,
		"near and far expirations must be different for diagonal spreads",
		"Use different dates for `--near-expiration` and `--far-expiration`",
	)
}

// TestValidateDiagonalOrderRejectsNearAfterFar verifies that near expiration
// must come before far expiration.
func TestValidateDiagonalOrderRejectsNearAfterFar(t *testing.T) {
	t.Parallel()

	err := ValidateDiagonalOrder(&DiagonalParams{
		Underlying:     "AAPL",
		NearExpiration: time.Now().UTC().Add(60 * 24 * time.Hour),
		FarExpiration:  time.Now().UTC().Add(30 * 24 * time.Hour),
		NearStrike:     150,
		FarStrike:      155,
		Quantity:       1,
		Price:          3.00,
	})

	assertValidationError(t, err,
		"near expiration must be before far expiration",
		"Swap `--near-expiration` and `--far-expiration` so the near date comes first",
	)
}

// TestValidateDiagonalOrderRejectsSameStrike verifies that identical strikes
// are rejected. Diagonals require different strikes (use calendar for same strike).
func TestValidateDiagonalOrderRejectsSameStrike(t *testing.T) {
	t.Parallel()

	err := ValidateDiagonalOrder(&DiagonalParams{
		Underlying:     "AAPL",
		NearExpiration: time.Now().UTC().Add(30 * 24 * time.Hour),
		FarExpiration:  time.Now().UTC().Add(60 * 24 * time.Hour),
		NearStrike:     150,
		FarStrike:      150,
		Quantity:       1,
		Price:          3.00,
	})

	assertValidationError(t, err,
		"near and far strikes must be different for diagonal spreads",
		"Use different values for `--near-strike` and `--far-strike` (use calendar for same strike)",
	)
}

// TestValidateDiagonalOrderRequiresNearStrike verifies near strike is mandatory.
func TestValidateDiagonalOrderRequiresNearStrike(t *testing.T) {
	t.Parallel()

	err := ValidateDiagonalOrder(&DiagonalParams{
		Underlying:     "AAPL",
		NearExpiration: time.Now().UTC().Add(30 * 24 * time.Hour),
		FarExpiration:  time.Now().UTC().Add(60 * 24 * time.Hour),
		FarStrike:      155,
		Quantity:       1,
		Price:          3.00,
	})

	assertValidationError(t, err, "near strike price must be greater than zero", "Add `--near-strike <price>` with a positive value")
}

// TestValidateDiagonalOrderRequiresFarStrike verifies far strike is mandatory.
func TestValidateDiagonalOrderRequiresFarStrike(t *testing.T) {
	t.Parallel()

	err := ValidateDiagonalOrder(&DiagonalParams{
		Underlying:     "AAPL",
		NearExpiration: time.Now().UTC().Add(30 * 24 * time.Hour),
		FarExpiration:  time.Now().UTC().Add(60 * 24 * time.Hour),
		NearStrike:     150,
		Quantity:       1,
		Price:          3.00,
	})

	assertValidationError(t, err, "far strike price must be greater than zero", "Add `--far-strike <price>` with a positive value")
}

// TestValidateDiagonalOrderRequiresPrice verifies price is mandatory.
func TestValidateDiagonalOrderRequiresPrice(t *testing.T) {
	t.Parallel()

	err := ValidateDiagonalOrder(&DiagonalParams{
		Underlying:     "AAPL",
		NearExpiration: time.Now().UTC().Add(30 * 24 * time.Hour),
		FarExpiration:  time.Now().UTC().Add(60 * 24 * time.Hour),
		NearStrike:     150,
		FarStrike:      155,
		PutCall:        models.PutCallCall,
		Quantity:       1,
	})

	assertValidationError(t, err, "price is required for diagonal spreads", "Add `--price <amount>` to specify the net debit or credit")
}

// TestValidateDiagonalOrderRequiresQuantity verifies quantity is mandatory.
func TestValidateDiagonalOrderRequiresQuantity(t *testing.T) {
	t.Parallel()

	err := ValidateDiagonalOrder(&DiagonalParams{
		Underlying:     "AAPL",
		NearExpiration: time.Now().UTC().Add(30 * 24 * time.Hour),
		FarExpiration:  time.Now().UTC().Add(60 * 24 * time.Hour),
		NearStrike:     150,
		FarStrike:      155,
		Price:          3.00,
	})

	assertValidationError(t, err, "quantity must be greater than zero", "Add `--quantity <number>` with a positive value")
}

// TestValidateDiagonalOrderRejectsMissingPutCall verifies PutCall is mandatory.
func TestValidateDiagonalOrderRejectsMissingPutCall(t *testing.T) {
	t.Parallel()

	err := ValidateDiagonalOrder(&DiagonalParams{
		Underlying:     "AAPL",
		NearExpiration: time.Now().UTC().Add(30 * 24 * time.Hour),
		FarExpiration:  time.Now().UTC().Add(60 * 24 * time.Hour),
		NearStrike:     150,
		FarStrike:      155,
		Quantity:       1,
		Price:          3.00,
	})

	assertValidationError(t, err, "option type (call or put) is required", "Add `--call` or `--put`")
}

// TestValidateDiagonalOrderAcceptsValidParams verifies a valid diagonal passes.
func TestValidateDiagonalOrderAcceptsValidParams(t *testing.T) {
	t.Parallel()

	err := ValidateDiagonalOrder(&DiagonalParams{
		Underlying:     "AAPL",
		NearExpiration: time.Now().UTC().Add(30 * 24 * time.Hour),
		FarExpiration:  time.Now().UTC().Add(60 * 24 * time.Hour),
		NearStrike:     150,
		FarStrike:      155,
		PutCall:        models.PutCallCall,
		Quantity:       1,
		Price:          3.00,
	})

	require.NoError(t, err)
}

// TestValidateOptionOrderRequiresUnderlying verifies underlying symbol validation.
func TestValidateOptionOrderRequiresUnderlying(t *testing.T) {
	t.Parallel()

	err := ValidateOptionOrder(&OptionParams{
		Expiration: time.Now().UTC().Add(30 * 24 * time.Hour),
		Strike:     150,
		Quantity:   1,
	})

	assertValidationError(t, err, "underlying symbol is required", "Add `--underlying <TICKER>` to specify the underlying stock")
}

// TestValidateOptionOrderRejectsZeroQuantity verifies quantity validation.
func TestValidateOptionOrderRejectsZeroQuantity(t *testing.T) {
	t.Parallel()

	err := ValidateOptionOrder(&OptionParams{
		Underlying: "AAPL",
		Expiration: time.Now().UTC().Add(30 * 24 * time.Hour),
		Strike:     150,
		Quantity:   0,
	})

	assertValidationError(t, err, "quantity must be greater than zero", "Add `--quantity <number>` with a positive value")
}

// TestValidateOptionOrderRejectsZeroStrike verifies strike price validation.
func TestValidateOptionOrderRejectsZeroStrike(t *testing.T) {
	t.Parallel()

	err := ValidateOptionOrder(&OptionParams{
		Underlying: "AAPL",
		Expiration: time.Now().UTC().Add(30 * 24 * time.Hour),
		Quantity:   5,
	})

	assertValidationError(t, err, "option strike price must be greater than zero", "Add `--strike <price>` with a positive value")
}

// TestValidateOptionOrderRejectsNegativeStrike verifies negative strike is rejected.
func TestValidateOptionOrderRejectsNegativeStrike(t *testing.T) {
	t.Parallel()

	err := ValidateOptionOrder(&OptionParams{
		Underlying: "AAPL",
		Expiration: time.Now().UTC().Add(30 * 24 * time.Hour),
		Strike:     -10,
		Quantity:   5,
	})

	assertValidationError(t, err, "option strike price must be greater than zero", "Add `--strike <price>` with a positive value")
}

// TestValidateCalendarOrderRejectsPastNearExpiration verifies near expiration must be in the future.
func TestValidateCalendarOrderRejectsPastNearExpiration(t *testing.T) {
	t.Parallel()

	err := ValidateCalendarOrder(&CalendarParams{
		Underlying:     "AAPL",
		NearExpiration: time.Now().UTC().Add(-24 * time.Hour),
		FarExpiration:  time.Now().UTC().Add(60 * 24 * time.Hour),
		Strike:         150,
		Quantity:       1,
		Price:          2.50,
	})

	assertValidationError(t, err, "option expiration date is in the past", "Use a future expiration date with `--expiration YYYY-MM-DD`")
}

// TestValidateCalendarOrderRejectsPastFarExpiration verifies far expiration must be in the future.
func TestValidateCalendarOrderRejectsPastFarExpiration(t *testing.T) {
	t.Parallel()

	err := ValidateCalendarOrder(&CalendarParams{
		Underlying:     "AAPL",
		NearExpiration: time.Now().UTC().Add(30 * 24 * time.Hour),
		FarExpiration:  time.Now().UTC().Add(-24 * time.Hour),
		Strike:         150,
		Quantity:       1,
		Price:          2.50,
	})

	assertValidationError(t, err, "option expiration date is in the past", "Use a future expiration date with `--expiration YYYY-MM-DD`")
}

// TestValidateDiagonalOrderRejectsPastNearExpiration verifies near expiration must be in the future.
func TestValidateDiagonalOrderRejectsPastNearExpiration(t *testing.T) {
	t.Parallel()

	err := ValidateDiagonalOrder(&DiagonalParams{
		Underlying:     "AAPL",
		NearExpiration: time.Now().UTC().Add(-24 * time.Hour),
		FarExpiration:  time.Now().UTC().Add(60 * 24 * time.Hour),
		NearStrike:     150,
		FarStrike:      155,
		Quantity:       1,
		Price:          3.00,
	})

	assertValidationError(t, err, "option expiration date is in the past", "Use a future expiration date with `--expiration YYYY-MM-DD`")
}

// TestValidateDiagonalOrderRejectsPastFarExpiration verifies far expiration must be in the future.
func TestValidateDiagonalOrderRejectsPastFarExpiration(t *testing.T) {
	t.Parallel()

	err := ValidateDiagonalOrder(&DiagonalParams{
		Underlying:     "AAPL",
		NearExpiration: time.Now().UTC().Add(30 * 24 * time.Hour),
		FarExpiration:  time.Now().UTC().Add(-24 * time.Hour),
		NearStrike:     150,
		FarStrike:      155,
		Quantity:       1,
		Price:          3.00,
	})

	assertValidationError(t, err, "option expiration date is in the past", "Use a future expiration date with `--expiration YYYY-MM-DD`")
}

// TestValidateCollarOrderRequiresPrice verifies price is mandatory for collars.
func TestValidateCollarOrderRequiresPrice(t *testing.T) {
	t.Parallel()

	err := ValidateCollarOrder(&CollarParams{
		Underlying: "AAPL",
		Expiration: time.Now().UTC().Add(30 * 24 * time.Hour),
		PutStrike:  140,
		CallStrike: 160,
		Quantity:   1,
	})

	assertValidationError(t, err, "price is required for collars", "Add `--price <amount>` to specify the net debit or credit")
}

// TestValidateCollarOrderRejectsPastExpiration verifies collar expiration must be in the future.
func TestValidateCollarOrderRejectsPastExpiration(t *testing.T) {
	t.Parallel()

	err := ValidateCollarOrder(&CollarParams{
		Underlying: "AAPL",
		Expiration: time.Now().UTC().Add(-24 * time.Hour),
		PutStrike:  140,
		CallStrike: 160,
		Quantity:   1,
		Price:      150.00,
	})

	assertValidationError(t, err, "option expiration date is in the past", "Use a future expiration date with `--expiration YYYY-MM-DD`")
}

// TestValidateOptionOrderRejectsTrailingStop verifies trailing stop types are rejected for options.
func TestValidateOptionOrderRejectsTrailingStop(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name      string
		orderType models.OrderType
	}{
		{name: "TRAILING_STOP", orderType: models.OrderTypeTrailingStop},
		{name: "TRAILING_STOP_LIMIT", orderType: models.OrderTypeTrailingStopLimit},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateOptionOrder(&OptionParams{
				Underlying: "AAPL",
				Expiration: time.Now().UTC().Add(30 * 24 * time.Hour),
				Strike:     185,
				Quantity:   5,
				Action:     models.InstructionBuyToOpen,
				OrderType:  tc.orderType,
			})

			require.Error(t, err)
			assertValidationError(t, err, "trailing stop orders are not supported for options", "Use `order build equity` for trailing stop orders")
		})
	}
}

// TestValidateOptionOrderAcceptsValidParams verifies valid option order passes.
func TestValidateOptionOrderAcceptsValidParams(t *testing.T) {
	t.Parallel()

	err := ValidateOptionOrder(&OptionParams{
		Underlying: "AAPL",
		Expiration: time.Now().UTC().Add(30 * 24 * time.Hour),
		Strike:     185,
		Quantity:   5,
		Action:     models.InstructionBuyToOpen,
		OrderType:  models.OrderTypeLimit,
		Price:      3.50,
	})

	require.NoError(t, err)
}

// TestValidateBracketBuyTakeProfitBelowEntry verifies BUY bracket rejects
// take-profit at or below the entry price.
func TestValidateBracketBuyTakeProfitBelowEntry(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name       string
		takeProfit float64
	}{
		{name: "below entry", takeProfit: 180},
		{name: "equal to entry", takeProfit: 185},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateBracketOrder(&BracketParams{
				Symbol:     "AAPL",
				Action:     models.InstructionBuy,
				Quantity:   10,
				OrderType:  models.OrderTypeLimit,
				Price:      185,
				TakeProfit: tc.takeProfit,
				StopLoss:   175,
			})

			assertValidationError(t, err,
				"take-profit price must be above the entry price for a BUY bracket",
				"Increase `--take-profit` so it is above the entry price",
			)
		})
	}
}

// TestValidateBracketBuyStopLossAboveEntry verifies BUY bracket rejects
// stop-loss at or above the entry price.
func TestValidateBracketBuyStopLossAboveEntry(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		stopLoss float64
	}{
		{name: "above entry", stopLoss: 190},
		{name: "equal to entry", stopLoss: 185},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateBracketOrder(&BracketParams{
				Symbol:     "AAPL",
				Action:     models.InstructionBuy,
				Quantity:   10,
				OrderType:  models.OrderTypeLimit,
				Price:      185,
				TakeProfit: 195,
				StopLoss:   tc.stopLoss,
			})

			assertValidationError(t, err,
				"stop-loss price must be below the entry price for a BUY bracket",
				"Lower `--stop-loss` so it is below the entry price",
			)
		})
	}
}

// TestValidateBracketBuyAcceptsValidPrices verifies BUY bracket with
// take-profit > entry > stop-loss passes.
func TestValidateBracketBuyAcceptsValidPrices(t *testing.T) {
	t.Parallel()

	err := ValidateBracketOrder(&BracketParams{
		Symbol:     "AAPL",
		Action:     models.InstructionBuy,
		Quantity:   10,
		OrderType:  models.OrderTypeLimit,
		Price:      185,
		TakeProfit: 195,
		StopLoss:   175,
	})

	require.NoError(t, err)
}

// TestValidateBracketSellTakeProfitAboveEntry verifies SELL bracket rejects
// take-profit at or above the entry price.
func TestValidateBracketSellTakeProfitAboveEntry(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name       string
		takeProfit float64
	}{
		{name: "above entry", takeProfit: 190},
		{name: "equal to entry", takeProfit: 185},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateBracketOrder(&BracketParams{
				Symbol:     "AAPL",
				Action:     models.InstructionSellShort,
				Quantity:   10,
				OrderType:  models.OrderTypeLimit,
				Price:      185,
				TakeProfit: tc.takeProfit,
				StopLoss:   195,
			})

			assertValidationError(t, err,
				"take-profit price must be below the entry price for a SELL bracket",
				"Lower `--take-profit` so it is below the entry price",
			)
		})
	}
}

// TestValidateBracketSellStopLossBelowEntry verifies SELL bracket rejects
// stop-loss at or below the entry price.
func TestValidateBracketSellStopLossBelowEntry(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		stopLoss float64
	}{
		{name: "below entry", stopLoss: 180},
		{name: "equal to entry", stopLoss: 185},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateBracketOrder(&BracketParams{
				Symbol:     "AAPL",
				Action:     models.InstructionSellShort,
				Quantity:   10,
				OrderType:  models.OrderTypeLimit,
				Price:      185,
				TakeProfit: 175,
				StopLoss:   tc.stopLoss,
			})

			assertValidationError(t, err,
				"stop-loss price must be above the entry price for a SELL bracket",
				"Raise `--stop-loss` so it is above the entry price",
			)
		})
	}
}

// TestValidateBracketSellAcceptsValidPrices verifies SELL bracket with
// stop-loss > entry > take-profit passes.
func TestValidateBracketSellAcceptsValidPrices(t *testing.T) {
	t.Parallel()

	err := ValidateBracketOrder(&BracketParams{
		Symbol:     "AAPL",
		Action:     models.InstructionSellShort,
		Quantity:   10,
		OrderType:  models.OrderTypeLimit,
		Price:      185,
		TakeProfit: 175,
		StopLoss:   195,
	})

	require.NoError(t, err)
}

// TestValidateEquityOrderRequiresSymbol verifies empty symbol rejection.
func TestValidateEquityOrderRequiresSymbol(t *testing.T) {
	t.Parallel()

	err := ValidateEquityOrder(&EquityParams{
		Quantity:  10,
		OrderType: models.OrderTypeMarket,
	})

	assertValidationError(t, err, "symbol is required", "Add `--symbol <TICKER>` to specify the stock symbol")
}

// TestValidateEquityOrderRequiresBothPricesForStopLimit verifies STOP_LIMIT requires price and stop price.
func TestValidateEquityOrderRequiresBothPricesForStopLimit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		price     float64
		stopPrice float64
	}{
		{"missing price", 0, 150},
		{"missing stop price", 150, 0},
		{"missing both", 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateEquityOrder(&EquityParams{
				Symbol:    "AAPL",
				Quantity:  10,
				OrderType: models.OrderTypeStopLimit,
				Price:     tt.price,
				StopPrice: tt.stopPrice,
			})

			assertValidationError(t, err,
				"STOP_LIMIT order requires both price and stop price",
				"Add `--price <amount> --stop-price <amount>`",
			)
		})
	}
}

// TestValidateEquityOrderRequiresOffsetForTrailingStop verifies TRAILING_STOP requires stop-offset.
func TestValidateEquityOrderRequiresOffsetForTrailingStop(t *testing.T) {
	t.Parallel()

	err := ValidateEquityOrder(&EquityParams{
		Symbol:    "AAPL",
		Quantity:  10,
		OrderType: models.OrderTypeTrailingStop,
	})

	assertValidationError(t, err,
		"TRAILING_STOP order requires a stop price offset",
		"Add `--stop-offset <amount>` to specify how far the stop trails",
	)
}

// TestValidateEquityOrderRequiresOffsetAndPriceForTrailingStopLimit verifies TRAILING_STOP_LIMIT
// requires stop-offset and price.
func TestValidateEquityOrderRequiresOffsetAndPriceForTrailingStopLimit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		price           float64
		stopPriceOffset float64
		wantMsg         string
		wantDetails     string
	}{
		{
			name:        "missing offset",
			price:       195.00,
			wantMsg:     "TRAILING_STOP_LIMIT order requires a stop price offset",
			wantDetails: "Add `--stop-offset <amount>` to specify how far the stop trails",
		},
		{
			name:            "missing price",
			stopPriceOffset: 3.00,
			wantMsg:         "TRAILING_STOP_LIMIT order requires a limit price",
			wantDetails:     "Add `--price <amount>` to specify the limit price",
		},
		{
			name:        "missing both",
			wantMsg:     "TRAILING_STOP_LIMIT order requires a stop price offset",
			wantDetails: "Add `--stop-offset <amount>` to specify how far the stop trails",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateEquityOrder(&EquityParams{
				Symbol:          "AAPL",
				Quantity:        10,
				OrderType:       models.OrderTypeTrailingStopLimit,
				Price:           tt.price,
				StopPriceOffset: tt.stopPriceOffset,
			})

			assertValidationError(t, err, tt.wantMsg, tt.wantDetails)
		})
	}
}

// TestValidateEquityOrderAcceptsValidTrailingStop verifies valid trailing stop orders pass validation.
func TestValidateEquityOrderAcceptsValidTrailingStop(t *testing.T) {
	t.Parallel()

	err := ValidateEquityOrder(&EquityParams{
		Symbol:          "AAPL",
		Action:          models.InstructionSell,
		Quantity:        10,
		OrderType:       models.OrderTypeTrailingStop,
		StopPriceOffset: 2.50,
	})

	require.NoError(t, err)
}

// TestValidateEquityOrderAcceptsValidTrailingStopLimit verifies valid trailing stop limit orders pass.
func TestValidateEquityOrderAcceptsValidTrailingStopLimit(t *testing.T) {
	t.Parallel()

	err := ValidateEquityOrder(&EquityParams{
		Symbol:          "AAPL",
		Action:          models.InstructionSell,
		Quantity:        10,
		OrderType:       models.OrderTypeTrailingStopLimit,
		Price:           195.00,
		StopPriceOffset: 3.00,
	})

	require.NoError(t, err)
}

// TestValidateEquityOrderAcceptsValidMarketOrder verifies market orders pass validation.
func TestValidateEquityOrderAcceptsValidMarketOrder(t *testing.T) {
	t.Parallel()

	err := ValidateEquityOrder(&EquityParams{
		Symbol:    "AAPL",
		Action:    models.InstructionBuy,
		Quantity:  10,
		OrderType: models.OrderTypeMarket,
	})

	require.NoError(t, err)
}

// TestValidateEquityOrderMOCLOC verifies MOC and LOC order validation.
func TestValidateEquityOrderMOCLOC(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		params      *EquityParams
		wantErr     bool
		wantMessage string
		wantDetails string
	}{
		{
			name: "MOC without price passes",
			params: &EquityParams{
				Symbol:    "AAPL",
				Action:    models.InstructionBuy,
				Quantity:  10,
				OrderType: models.OrderTypeMarketOnClose,
			},
			wantErr: false,
		},
		{
			name: "MOC with price fails",
			params: &EquityParams{
				Symbol:    "AAPL",
				Action:    models.InstructionBuy,
				Quantity:  10,
				OrderType: models.OrderTypeMarketOnClose,
				Price:     150.00,
			},
			wantErr:     true,
			wantMessage: "MARKET_ON_CLOSE order does not accept a price",
			wantDetails: "Remove `--price` flag for MOC orders",
		},
		{
			name: "MOC with stop price fails",
			params: &EquityParams{
				Symbol:    "AAPL",
				Action:    models.InstructionBuy,
				Quantity:  10,
				OrderType: models.OrderTypeMarketOnClose,
				StopPrice: 145.00,
			},
			wantErr:     true,
			wantMessage: "MARKET_ON_CLOSE order does not accept a stop price",
			wantDetails: "Remove `--stop-price` flag for MOC orders",
		},
		{
			name: "LOC with price passes",
			params: &EquityParams{
				Symbol:    "AAPL",
				Action:    models.InstructionBuy,
				Quantity:  10,
				OrderType: models.OrderTypeLimitOnClose,
				Price:     150.00,
			},
			wantErr: false,
		},
		{
			name: "LOC without price fails",
			params: &EquityParams{
				Symbol:    "AAPL",
				Action:    models.InstructionBuy,
				Quantity:  10,
				OrderType: models.OrderTypeLimitOnClose,
			},
			wantErr:     true,
			wantMessage: "LIMIT_ON_CLOSE order requires a price",
			wantDetails: "Add `--price <amount>` to specify the limit price",
		},
		{
			name: "LOC with stop price fails",
			params: &EquityParams{
				Symbol:    "AAPL",
				Action:    models.InstructionBuy,
				Quantity:  10,
				OrderType: models.OrderTypeLimitOnClose,
				Price:     150.00,
				StopPrice: 145.00,
			},
			wantErr:     true,
			wantMessage: "LIMIT_ON_CLOSE order does not accept a stop price",
			wantDetails: "Remove `--stop-price` flag for LOC orders",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateEquityOrder(tt.params)
			if tt.wantErr {
				assertValidationError(t, err, tt.wantMessage, tt.wantDetails)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestValidateVerticalOrderRemainingBranches covers quantity, expiration, and individual strike checks.
func TestValidateVerticalOrderRemainingBranches(t *testing.T) {
	t.Parallel()

	futureDate := time.Now().UTC().Add(30 * 24 * time.Hour)

	tests := []struct {
		name    string
		params  VerticalParams
		wantMsg string
	}{
		{
			name: "zero quantity",
			params: VerticalParams{
				Underlying:  "F",
				Expiration:  futureDate,
				LongStrike:  12,
				ShortStrike: 14,
				Price:       0.50,
			},
			wantMsg: "quantity must be greater than zero",
		},
		{
			name: "past expiration",
			params: VerticalParams{
				Underlying:  "F",
				Expiration:  time.Now().UTC().Add(-24 * time.Hour),
				LongStrike:  12,
				ShortStrike: 14,
				Quantity:    1,
				Price:       0.50,
			},
			wantMsg: "option expiration date is in the past",
		},
		{
			name: "zero long strike",
			params: VerticalParams{
				Underlying:  "F",
				Expiration:  futureDate,
				ShortStrike: 14,
				Quantity:    1,
				Price:       0.50,
			},
			wantMsg: "long strike price must be greater than zero",
		},
		{
			name: "zero short strike",
			params: VerticalParams{
				Underlying: "F",
				Expiration: futureDate,
				LongStrike: 12,
				Quantity:   1,
				Price:      0.50,
			},
			wantMsg: "short strike price must be greater than zero",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateVerticalOrder(&tt.params)

			require.Error(t, err)
			var validationErr *apperr.ValidationError
			require.ErrorAs(t, err, &validationErr)
			assert.Equal(t, tt.wantMsg, validationErr.Message)
		})
	}
}

// TestValidateIronCondorRemainingBranches covers quantity, expiration, zero strikes,
// and the call-short >= call-long ordering check.
func TestValidateIronCondorRemainingBranches(t *testing.T) {
	t.Parallel()

	futureDate := time.Now().UTC().Add(30 * 24 * time.Hour)

	tests := []struct {
		name    string
		params  IronCondorParams
		wantMsg string
	}{
		{
			name: "zero quantity",
			params: IronCondorParams{
				Underlying:      "F",
				Expiration:      futureDate,
				PutLongStrike:   8,
				PutShortStrike:  10,
				CallShortStrike: 14,
				CallLongStrike:  16,
				Price:           1.50,
			},
			wantMsg: "quantity must be greater than zero",
		},
		{
			name: "past expiration",
			params: IronCondorParams{
				Underlying:      "F",
				Expiration:      time.Now().UTC().Add(-24 * time.Hour),
				PutLongStrike:   8,
				PutShortStrike:  10,
				CallShortStrike: 14,
				CallLongStrike:  16,
				Quantity:        1,
				Price:           1.50,
			},
			wantMsg: "option expiration date is in the past",
		},
		{
			name: "zero put-long-strike",
			params: IronCondorParams{
				Underlying:      "F",
				Expiration:      futureDate,
				PutShortStrike:  10,
				CallShortStrike: 14,
				CallLongStrike:  16,
				Quantity:        1,
				Price:           1.50,
			},
			wantMsg: "put-long-strike must be greater than zero",
		},
		{
			name: "call-short >= call-long",
			params: IronCondorParams{
				Underlying:      "F",
				Expiration:      futureDate,
				PutLongStrike:   8,
				PutShortStrike:  10,
				CallShortStrike: 16,
				CallLongStrike:  14,
				Quantity:        1,
				Price:           1.50,
			},
			wantMsg: "call-short-strike must be below call-long-strike",
		},
		{
			name: "missing price",
			params: IronCondorParams{
				Underlying:      "F",
				Expiration:      futureDate,
				PutLongStrike:   8,
				PutShortStrike:  10,
				CallShortStrike: 14,
				CallLongStrike:  16,
				Quantity:        1,
			},
			wantMsg: "price is required for iron condors",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateIronCondorOrder(&tt.params)

			require.Error(t, err)
			var validationErr *apperr.ValidationError
			require.ErrorAs(t, err, &validationErr)
			assert.Equal(t, tt.wantMsg, validationErr.Message)
		})
	}
}

// TestValidateStrangleRemainingBranches covers quantity, expiration, and individual strike checks.
func TestValidateStrangleRemainingBranches(t *testing.T) {
	t.Parallel()

	futureDate := time.Now().UTC().Add(30 * 24 * time.Hour)

	tests := []struct {
		name    string
		params  StrangleParams
		wantMsg string
	}{
		{
			name: "zero quantity",
			params: StrangleParams{
				Underlying: "F",
				Expiration: futureDate,
				CallStrike: 14,
				PutStrike:  10,
				Price:      0.80,
			},
			wantMsg: "quantity must be greater than zero",
		},
		{
			name: "past expiration",
			params: StrangleParams{
				Underlying: "F",
				Expiration: time.Now().UTC().Add(-24 * time.Hour),
				CallStrike: 14,
				PutStrike:  10,
				Quantity:   1,
				Price:      0.80,
			},
			wantMsg: "option expiration date is in the past",
		},
		{
			name: "zero call strike",
			params: StrangleParams{
				Underlying: "F",
				Expiration: futureDate,
				PutStrike:  10,
				Quantity:   1,
				Price:      0.80,
			},
			wantMsg: "call strike price must be greater than zero",
		},
		{
			name: "zero put strike",
			params: StrangleParams{
				Underlying: "F",
				Expiration: futureDate,
				CallStrike: 14,
				Quantity:   1,
				Price:      0.80,
			},
			wantMsg: "put strike price must be greater than zero",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateStrangleOrder(&tt.params)

			require.Error(t, err)
			var validationErr *apperr.ValidationError
			require.ErrorAs(t, err, &validationErr)
			assert.Equal(t, tt.wantMsg, validationErr.Message)
		})
	}
}

// TestValidateStraddleRemainingBranches covers quantity and expiration checks.
func TestValidateStraddleRemainingBranches(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		params  StraddleParams
		wantMsg string
	}{
		{
			name: "zero quantity",
			params: StraddleParams{
				Underlying: "F",
				Expiration: time.Now().UTC().Add(30 * 24 * time.Hour),
				Strike:     12,
				Price:      1.50,
			},
			wantMsg: "quantity must be greater than zero",
		},
		{
			name: "past expiration",
			params: StraddleParams{
				Underlying: "F",
				Expiration: time.Now().UTC().Add(-24 * time.Hour),
				Strike:     12,
				Quantity:   1,
				Price:      1.50,
			},
			wantMsg: "option expiration date is in the past",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateStraddleOrder(&tt.params)

			require.Error(t, err)
			var validationErr *apperr.ValidationError
			require.ErrorAs(t, err, &validationErr)
			assert.Equal(t, tt.wantMsg, validationErr.Message)
		})
	}
}

// TestValidateCoveredCallRemainingBranches covers quantity and expiration checks.
func TestValidateCoveredCallRemainingBranches(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		params  CoveredCallParams
		wantMsg string
	}{
		{
			name: "zero quantity",
			params: CoveredCallParams{
				Underlying: "F",
				Expiration: time.Now().UTC().Add(30 * 24 * time.Hour),
				Strike:     14,
				Price:      12.00,
			},
			wantMsg: "quantity must be greater than zero",
		},
		{
			name: "past expiration",
			params: CoveredCallParams{
				Underlying: "F",
				Expiration: time.Now().UTC().Add(-24 * time.Hour),
				Strike:     14,
				Quantity:   1,
				Price:      12.00,
			},
			wantMsg: "option expiration date is in the past",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateCoveredCallOrder(&tt.params)

			require.Error(t, err)
			var validationErr *apperr.ValidationError
			require.ErrorAs(t, err, &validationErr)
			assert.Equal(t, tt.wantMsg, validationErr.Message)
		})
	}
}

// TestValidateEquityOrderRejectsMismatchedPriceLink verifies that setting
// price-link-basis without price-link-type (or vice versa) is an error.
func TestValidateEquityOrderRejectsMismatchedPriceLink(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		priceLinkBasis models.PriceLinkBasis
		priceLinkType  models.PriceLinkType
	}{
		{
			name:           "basis without type",
			priceLinkBasis: models.PriceLinkBasisLast,
		},
		{
			name:          "type without basis",
			priceLinkType: models.PriceLinkTypeValue,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateEquityOrder(&EquityParams{
				Symbol:         "AAPL",
				Quantity:       10,
				OrderType:      models.OrderTypeLimit,
				Price:          150,
				PriceLinkBasis: tt.priceLinkBasis,
				PriceLinkType:  tt.priceLinkType,
			})

			assertValidationError(t, err,
				"price-link-basis and price-link-type must be specified together",
				"Add both `--price-link-basis <value>` and `--price-link-type <value>`, or omit both",
			)
		})
	}
}

// TestValidateEquityOrderAcceptsPairedPriceLink verifies that setting both
// price-link-basis and price-link-type together passes validation.
func TestValidateEquityOrderAcceptsPairedPriceLink(t *testing.T) {
	t.Parallel()

	err := ValidateEquityOrder(&EquityParams{
		Symbol:         "AAPL",
		Quantity:       10,
		OrderType:      models.OrderTypeLimit,
		Price:          150,
		PriceLinkBasis: models.PriceLinkBasisLast,
		PriceLinkType:  models.PriceLinkTypeValue,
	})

	require.NoError(t, err)
}

// TestValidateOptionOrderRejectsMismatchedPriceLink verifies that setting
// price-link-basis without price-link-type (or vice versa) is an error.
func TestValidateOptionOrderRejectsMismatchedPriceLink(t *testing.T) {
	t.Parallel()

	err := ValidateOptionOrder(&OptionParams{
		Underlying:     "AAPL",
		Expiration:     time.Now().UTC().Add(30 * 24 * time.Hour),
		Strike:         150,
		Quantity:       1,
		OrderType:      models.OrderTypeLimit,
		Price:          3.50,
		PriceLinkBasis: models.PriceLinkBasisBid,
	})

	assertValidationError(t, err,
		"price-link-basis and price-link-type must be specified together",
		"Add both `--price-link-basis <value>` and `--price-link-type <value>`, or omit both",
	)
}

// TestBracketExitInstructionInvertsAction verifies the exit instruction for both directions.
func TestBracketExitInstructionInvertsAction(t *testing.T) {
	t.Parallel()

	assert.Equal(t, models.InstructionSell, bracketExitInstruction(models.InstructionBuy),
		"BUY entry should produce SELL exit")
	assert.Equal(t, models.InstructionBuy, bracketExitInstruction(models.InstructionSell),
		"SELL entry should produce BUY exit")
}

// TestValidateOrderRequestAcceptsTriggerParentLegs verifies raw bracket specs remain valid.
func TestValidateOrderRequestAcceptsTriggerParentLegs(t *testing.T) {
	t.Parallel()

	order, err := BuildBracketOrder(&BracketParams{
		Symbol:     "AAPL",
		Action:     models.InstructionBuy,
		Quantity:   10,
		OrderType:  models.OrderTypeLimit,
		Price:      150,
		TakeProfit: 160,
		StopLoss:   140,
	})
	require.NoError(t, err)

	err = ValidateOrderRequest(order)

	require.NoError(t, err)
}

// TestValidateOrderRequestAcceptsFTSTriggerSpecs verifies first-triggers-second specs.
func TestValidateOrderRequestAcceptsFTSTriggerSpecs(t *testing.T) {
	t.Parallel()

	primary, err := BuildEquityOrder(&EquityParams{
		Symbol:    "AAPL",
		Action:    models.InstructionBuy,
		Quantity:  10,
		OrderType: models.OrderTypeMarket,
	})
	require.NoError(t, err)
	secondary, err := BuildEquityOrder(&EquityParams{
		Symbol:    "MSFT",
		Action:    models.InstructionBuy,
		Quantity:  5,
		OrderType: models.OrderTypeMarket,
	})
	require.NoError(t, err)
	order, err := BuildFTSOrder(&FTSParams{Primary: *primary, Secondary: *secondary})
	require.NoError(t, err)

	err = ValidateOrderRequest(&order)

	require.NoError(t, err)
}

// TestValidateOrderRequestRejectsOCOParentLegs preserves OCO container validation.
func TestValidateOrderRequestRejectsOCOParentLegs(t *testing.T) {
	t.Parallel()

	order, err := BuildBracketOrder(&BracketParams{
		Symbol:     "AAPL",
		Action:     models.InstructionBuy,
		Quantity:   10,
		OrderType:  models.OrderTypeMarket,
		TakeProfit: 160,
		StopLoss:   140,
	})
	require.NoError(t, err)
	ocoNode := order.ChildOrderStrategies[0]
	ocoNode.OrderLegCollection = order.OrderLegCollection

	err = ValidateOrderRequest(&ocoNode)

	assertValidationError(t, err, "order OCO parent must not contain direct order legs", "Move executable legs into childOrderStrategies")
}

// TestValidateOrderRequestPriceLinkPathIncludesNestedChild verifies nested error paths.
func TestValidateOrderRequestPriceLinkPathIncludesNestedChild(t *testing.T) {
	t.Parallel()

	order, err := BuildBracketOrder(&BracketParams{
		Symbol:     "AAPL",
		Action:     models.InstructionBuy,
		Quantity:   10,
		OrderType:  models.OrderTypeMarket,
		TakeProfit: 160,
		StopLoss:   140,
	})
	require.NoError(t, err)
	priceLinkBasis := models.PriceLinkBasisBid
	order.ChildOrderStrategies[0].ChildOrderStrategies[0].PriceLinkBasis = &priceLinkBasis

	err = ValidateOrderRequest(order)

	assertValidationError(t, err,
		"order.childOrderStrategies[0].childOrderStrategies[0] priceLinkBasis and priceLinkType must be specified together",
		"Set both priceLinkBasis and priceLinkType, or omit both fields",
	)
}

// assertValidationError verifies message and fix suggestion for validation failures.
func assertValidationError(t *testing.T, err error, expectedMessage, expectedDetails string) {
	t.Helper()

	require.Error(t, err)

	var validationErr *apperr.ValidationError
	require.ErrorAs(t, err, &validationErr)
	assert.Equal(t, expectedMessage, validationErr.Message)
	assert.Equal(t, expectedDetails, validationErr.Details())
}
