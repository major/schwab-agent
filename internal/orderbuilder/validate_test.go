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

// assertValidationError verifies message and fix suggestion for validation failures.
func assertValidationError(t *testing.T, err error, expectedMessage, expectedDetails string) {
	t.Helper()

	require.Error(t, err)

	var validationErr *apperr.ValidationError
	require.True(t, errors.As(err, &validationErr))
	assert.Equal(t, expectedMessage, validationErr.Message)
	assert.Equal(t, expectedDetails, validationErr.Details())
}
