package orderbuilder

import (
	"errors"
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
		name          string
		params        *EquityParams
		wantErr       bool
		wantMessage   string
		wantDetails   string
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
			require.True(t, errors.As(err, &validationErr))
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
			require.True(t, errors.As(err, &validationErr))
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
			require.True(t, errors.As(err, &validationErr))
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
			require.True(t, errors.As(err, &validationErr))
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
			require.True(t, errors.As(err, &validationErr))
			assert.Equal(t, tt.wantMsg, validationErr.Message)
		})
	}
}

// TestBracketExitInstructionInvertsAction verifies the exit instruction for both directions.
func TestBracketExitInstructionInvertsAction(t *testing.T) {
	t.Parallel()

	assert.Equal(t, models.InstructionSell, bracketExitInstruction(models.InstructionBuy),
		"BUY entry should produce SELL exit")
	assert.Equal(t, models.InstructionBuy, bracketExitInstruction(models.InstructionSell),
		"SELL entry should produce BUY exit")
}

// assertValidationError verifies message and fix suggestion for validation failures.
func assertValidationError(t *testing.T, err error, expectedMessage, expectedDetails string) {
	t.Helper()

	require.Error(t, err)

	var validationErr *apperr.ValidationError
	require.True(t, errors.As(err, &validationErr))
	assert.Equal(t, expectedMessage, validationErr.Message)
	assert.Equal(t, expectedDetails, validationErr.Details())
}
