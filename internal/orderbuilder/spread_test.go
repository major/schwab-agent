package orderbuilder

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/major/schwab-agent/internal/models"
)

// TestBuildVerticalOrderBullCallOpen verifies a bull call spread (debit to open).
// BUY lower strike call + SELL higher strike call = NET_DEBIT.
func TestBuildVerticalOrderBullCallOpen(t *testing.T) {
	t.Parallel()

	expiration := time.Date(2026, time.June, 18, 0, 0, 0, 0, time.UTC)

	order, err := BuildVerticalOrder(&VerticalParams{
		Underlying:  "F",
		Expiration:  expiration,
		LongStrike:  12,
		ShortStrike: 14,
		PutCall:     models.PutCallCall,
		Open:        true,
		Quantity:    1,
		Price:       0.50,
	})

	// Arrange/Act assertions on order-level fields.
	require.NoError(t, err)
	require.NotNil(t, order)
	assert.Equal(t, models.OrderTypeNetDebit, order.OrderType)
	assert.Equal(t, models.OrderStrategyTypeSingle, order.OrderStrategyType)
	require.NotNil(t, order.ComplexOrderStrategyType)
	assert.Equal(t, models.ComplexOrderStrategyTypeVertical, *order.ComplexOrderStrategyType)
	require.NotNil(t, order.Price)
	assert.Equal(t, 0.50, *order.Price)
	assert.Equal(t, models.DurationDay, order.Duration)
	assert.Equal(t, models.SessionNormal, order.Session)

	// Assert two legs with correct instructions and OCC symbols.
	require.Len(t, order.OrderLegCollection, 2)

	longLeg := order.OrderLegCollection[0]
	assert.Equal(t, models.InstructionBuyToOpen, longLeg.Instruction)
	assert.Equal(t, 1.0, longLeg.Quantity)
	assert.Equal(t, models.AssetTypeOption, longLeg.Instrument.AssetType)
	assert.Equal(t, "F     260618C00012000", longLeg.Instrument.Symbol)
	require.NotNil(t, longLeg.Instrument.PutCall)
	assert.Equal(t, models.PutCallCall, *longLeg.Instrument.PutCall)
	require.NotNil(t, longLeg.Instrument.OptionStrikePrice)
	assert.Equal(t, 12.0, *longLeg.Instrument.OptionStrikePrice)

	shortLeg := order.OrderLegCollection[1]
	assert.Equal(t, models.InstructionSellToOpen, shortLeg.Instruction)
	assert.Equal(t, 1.0, shortLeg.Quantity)
	assert.Equal(t, "F     260618C00014000", shortLeg.Instrument.Symbol)
	require.NotNil(t, shortLeg.Instrument.OptionStrikePrice)
	assert.Equal(t, 14.0, *shortLeg.Instrument.OptionStrikePrice)
}

// TestBuildButterflyOrderLongOpen verifies a long butterfly uses long wings and a short body.
func TestBuildButterflyOrderLongOpen(t *testing.T) {
	t.Parallel()

	expiration := time.Date(2026, time.June, 18, 0, 0, 0, 0, time.UTC)

	order, err := BuildButterflyOrder(&ButterflyParams{
		Underlying: "F", Expiration: expiration, LowerStrike: 10, MiddleStrike: 12, UpperStrike: 14,
		PutCall: models.PutCallCall, Buy: true, Open: true, Quantity: 1, Price: 0.50,
	})

	require.NoError(t, err)
	assert.Equal(t, models.OrderTypeNetDebit, order.OrderType)
	require.NotNil(t, order.ComplexOrderStrategyType)
	assert.Equal(t, models.ComplexOrderStrategyTypeButterfly, *order.ComplexOrderStrategyType)
	require.Len(t, order.OrderLegCollection, 3)
	assert.Equal(t, models.InstructionBuyToOpen, order.OrderLegCollection[0].Instruction)
	assert.Equal(t, models.InstructionSellToOpen, order.OrderLegCollection[1].Instruction)
	assert.Equal(t, 2.0, order.OrderLegCollection[1].Quantity)
	assert.Equal(t, models.InstructionBuyToOpen, order.OrderLegCollection[2].Instruction)
}

// TestBuildCondorOrderShortClose verifies a short condor close buys the wings back.
func TestBuildCondorOrderShortClose(t *testing.T) {
	t.Parallel()

	expiration := time.Date(2026, time.June, 18, 0, 0, 0, 0, time.UTC)

	order, err := BuildCondorOrder(&CondorParams{
		Underlying:        "F",
		Expiration:        expiration,
		LowerStrike:       10,
		LowerMiddleStrike: 12,
		UpperMiddleStrike: 14,
		UpperStrike:       16,
		PutCall:           models.PutCallPut,
		Buy:               false,
		Open:              false,
		Quantity:          1,
		Price:             0.75,
	})

	require.NoError(t, err)
	assert.Equal(t, models.OrderTypeNetDebit, order.OrderType)
	require.NotNil(t, order.ComplexOrderStrategyType)
	assert.Equal(t, models.ComplexOrderStrategyTypeCondor, *order.ComplexOrderStrategyType)
	require.Len(t, order.OrderLegCollection, 4)
	assert.Equal(t, models.InstructionBuyToClose, order.OrderLegCollection[0].Instruction)
	assert.Equal(t, models.InstructionSellToClose, order.OrderLegCollection[1].Instruction)
	assert.Equal(t, models.InstructionSellToClose, order.OrderLegCollection[2].Instruction)
	assert.Equal(t, models.InstructionBuyToClose, order.OrderLegCollection[3].Instruction)
}

// TestBuildBackRatioOrderOpen verifies a one-by-two back-ratio spread.
func TestBuildBackRatioOrderOpen(t *testing.T) {
	t.Parallel()

	expiration := time.Date(2026, time.June, 18, 0, 0, 0, 0, time.UTC)

	order, err := BuildBackRatioOrder(&BackRatioParams{
		Underlying: "F", Expiration: expiration, ShortStrike: 12, LongStrike: 14,
		PutCall: models.PutCallCall, Open: true, Quantity: 1, LongRatio: 2, Credit: false, Price: 0.20,
	})

	require.NoError(t, err)
	assert.Equal(t, models.OrderTypeNetDebit, order.OrderType)
	require.NotNil(t, order.ComplexOrderStrategyType)
	assert.Equal(t, models.ComplexOrderStrategyTypeBackRatio, *order.ComplexOrderStrategyType)
	require.Len(t, order.OrderLegCollection, 2)
	assert.Equal(t, models.InstructionSellToOpen, order.OrderLegCollection[0].Instruction)
	assert.Equal(t, models.InstructionBuyToOpen, order.OrderLegCollection[1].Instruction)
	assert.Equal(t, 2.0, order.OrderLegCollection[1].Quantity)
}

// TestBuildVerticalRollOrder verifies closing and opening vertical legs are ordered clearly.
func TestBuildVerticalRollOrder(t *testing.T) {
	t.Parallel()

	closeExpiration := time.Date(2026, time.June, 18, 0, 0, 0, 0, time.UTC)
	openExpiration := time.Date(2026, time.July, 17, 0, 0, 0, 0, time.UTC)

	order, err := BuildVerticalRollOrder(&VerticalRollParams{
		Underlying: "F", CloseExpiration: closeExpiration, OpenExpiration: openExpiration,
		CloseLongStrike: 12, CloseShortStrike: 14, OpenLongStrike: 13, OpenShortStrike: 15,
		PutCall: models.PutCallCall, Credit: true, Quantity: 1, Price: 0.25,
	})

	require.NoError(t, err)
	assert.Equal(t, models.OrderTypeNetCredit, order.OrderType)
	require.NotNil(t, order.ComplexOrderStrategyType)
	assert.Equal(t, models.ComplexOrderStrategyTypeVerticalRoll, *order.ComplexOrderStrategyType)
	require.Len(t, order.OrderLegCollection, 4)
	assert.Equal(t, models.InstructionSellToClose, order.OrderLegCollection[0].Instruction)
	assert.Equal(t, models.InstructionBuyToClose, order.OrderLegCollection[1].Instruction)
	assert.Equal(t, models.InstructionBuyToOpen, order.OrderLegCollection[2].Instruction)
	assert.Equal(t, models.InstructionSellToOpen, order.OrderLegCollection[3].Instruction)
}

// TestBuildDoubleDiagonalOrderOpen verifies long far legs and short near legs.
func TestBuildDoubleDiagonalOrderOpen(t *testing.T) {
	t.Parallel()

	nearExpiration := time.Date(2026, time.June, 18, 0, 0, 0, 0, time.UTC)
	farExpiration := time.Date(2026, time.July, 17, 0, 0, 0, 0, time.UTC)

	order, err := BuildDoubleDiagonalOrder(&DoubleDiagonalParams{
		Underlying: "F", NearExpiration: nearExpiration, FarExpiration: farExpiration,
		PutFarStrike: 9, PutNearStrike: 10, CallNearStrike: 14, CallFarStrike: 15,
		Open: true, Quantity: 1, Price: 0.80,
	})

	require.NoError(t, err)
	assert.Equal(t, models.OrderTypeNetDebit, order.OrderType)
	require.NotNil(t, order.ComplexOrderStrategyType)
	assert.Equal(t, models.ComplexOrderStrategyTypeDoubleDiagonal, *order.ComplexOrderStrategyType)
	require.Len(t, order.OrderLegCollection, 4)
	assert.Equal(t, models.InstructionBuyToOpen, order.OrderLegCollection[0].Instruction)
	assert.Equal(t, models.InstructionSellToOpen, order.OrderLegCollection[1].Instruction)
	assert.Equal(t, models.InstructionSellToOpen, order.OrderLegCollection[2].Instruction)
	assert.Equal(t, models.InstructionBuyToOpen, order.OrderLegCollection[3].Instruction)
}

// TestBuildButterflyOrderShortOpen verifies short butterflies use credit order
// semantics and the inverse leg directions from long butterflies.
func TestBuildButterflyOrderShortOpen(t *testing.T) {
	t.Parallel()

	expiration := time.Date(2026, time.June, 18, 0, 0, 0, 0, time.UTC)

	order, err := BuildButterflyOrder(&ButterflyParams{
		Underlying: "F", Expiration: expiration, LowerStrike: 10, MiddleStrike: 12, UpperStrike: 14,
		PutCall: models.PutCallPut, Buy: false, Open: true, Quantity: 1, Price: 0.45,
	})

	require.NoError(t, err)
	assert.Equal(t, models.OrderTypeNetCredit, order.OrderType)
	require.Len(t, order.OrderLegCollection, 3)
	assert.Equal(t, models.InstructionSellToOpen, order.OrderLegCollection[0].Instruction)
	assert.Equal(t, models.InstructionBuyToOpen, order.OrderLegCollection[1].Instruction)
	assert.Equal(t, models.InstructionSellToOpen, order.OrderLegCollection[2].Instruction)
}

// TestBuildDoubleDiagonalOrderClose verifies close-side order semantics for the
// strategy that defaults to debit on open and credit on close.
func TestBuildDoubleDiagonalOrderClose(t *testing.T) {
	t.Parallel()

	nearExpiration := time.Date(2026, time.June, 18, 0, 0, 0, 0, time.UTC)
	farExpiration := time.Date(2026, time.July, 17, 0, 0, 0, 0, time.UTC)

	order, err := BuildDoubleDiagonalOrder(&DoubleDiagonalParams{
		Underlying: "F", NearExpiration: nearExpiration, FarExpiration: farExpiration,
		PutFarStrike: 9, PutNearStrike: 10, CallNearStrike: 14, CallFarStrike: 15,
		Open: false, Quantity: 1, Price: 0.70,
	})

	require.NoError(t, err)
	assert.Equal(t, models.OrderTypeNetCredit, order.OrderType)
	require.Len(t, order.OrderLegCollection, 4)
	assert.Equal(t, models.InstructionSellToClose, order.OrderLegCollection[0].Instruction)
	assert.Equal(t, models.InstructionBuyToClose, order.OrderLegCollection[1].Instruction)
	assert.Equal(t, models.InstructionBuyToClose, order.OrderLegCollection[2].Instruction)
	assert.Equal(t, models.InstructionSellToClose, order.OrderLegCollection[3].Instruction)
}

// TestBuildVerticalOrderBearCallOpen verifies a bear call spread (credit to open).
// BUY higher strike call + SELL lower strike call = NET_CREDIT.
func TestBuildVerticalOrderBearCallOpen(t *testing.T) {
	t.Parallel()

	expiration := time.Date(2026, time.June, 18, 0, 0, 0, 0, time.UTC)

	order, err := BuildVerticalOrder(&VerticalParams{
		Underlying:  "F",
		Expiration:  expiration,
		LongStrike:  14,
		ShortStrike: 12,
		PutCall:     models.PutCallCall,
		Open:        true,
		Quantity:    1,
		Price:       0.50,
	})

	require.NoError(t, err)
	assert.Equal(t, models.OrderTypeNetCredit, order.OrderType)
}

// TestBuildVerticalOrderBearPutOpen verifies a bear put spread (debit to open).
// BUY higher strike put + SELL lower strike put = NET_DEBIT.
func TestBuildVerticalOrderBearPutOpen(t *testing.T) {
	t.Parallel()

	expiration := time.Date(2026, time.June, 18, 0, 0, 0, 0, time.UTC)

	order, err := BuildVerticalOrder(&VerticalParams{
		Underlying:  "F",
		Expiration:  expiration,
		LongStrike:  14,
		ShortStrike: 12,
		PutCall:     models.PutCallPut,
		Open:        true,
		Quantity:    1,
		Price:       0.50,
	})

	require.NoError(t, err)
	assert.Equal(t, models.OrderTypeNetDebit, order.OrderType)
}

// TestBuildVerticalOrderBullPutOpen verifies a bull put spread (credit to open).
// BUY lower strike put + SELL higher strike put = NET_CREDIT.
func TestBuildVerticalOrderBullPutOpen(t *testing.T) {
	t.Parallel()

	expiration := time.Date(2026, time.June, 18, 0, 0, 0, 0, time.UTC)

	order, err := BuildVerticalOrder(&VerticalParams{
		Underlying:  "F",
		Expiration:  expiration,
		LongStrike:  12,
		ShortStrike: 14,
		PutCall:     models.PutCallPut,
		Open:        true,
		Quantity:    1,
		Price:       0.50,
	})

	require.NoError(t, err)
	assert.Equal(t, models.OrderTypeNetCredit, order.OrderType)
}

// TestBuildVerticalOrderCloseReversesOrderType verifies closing a spread flips
// the order type from debit to credit (or vice versa) and uses close instructions.
func TestBuildVerticalOrderCloseReversesOrderType(t *testing.T) {
	t.Parallel()

	expiration := time.Date(2026, time.June, 18, 0, 0, 0, 0, time.UTC)

	// A bull call spread opens as NET_DEBIT, closes as NET_CREDIT.
	order, err := BuildVerticalOrder(&VerticalParams{
		Underlying:  "F",
		Expiration:  expiration,
		LongStrike:  12,
		ShortStrike: 14,
		PutCall:     models.PutCallCall,
		Open:        false,
		Quantity:    1,
		Price:       0.30,
	})

	require.NoError(t, err)
	assert.Equal(t, models.OrderTypeNetCredit, order.OrderType)
	assert.Equal(t, models.InstructionSellToClose, order.OrderLegCollection[0].Instruction)
	assert.Equal(t, models.InstructionBuyToClose, order.OrderLegCollection[1].Instruction)
}

// TestBuildIronCondorOrderOpen verifies an opening iron condor produces 4 legs with
// correct instructions and NET_CREDIT order type.
func TestBuildIronCondorOrderOpen(t *testing.T) {
	t.Parallel()

	expiration := time.Date(2026, time.June, 18, 0, 0, 0, 0, time.UTC)

	order, err := BuildIronCondorOrder(&IronCondorParams{
		Underlying:      "F",
		Expiration:      expiration,
		PutLongStrike:   8,
		PutShortStrike:  10,
		CallShortStrike: 14,
		CallLongStrike:  16,
		Open:            true,
		Quantity:        1,
		Price:           1.50,
	})

	require.NoError(t, err)
	require.NotNil(t, order)
	assert.Equal(t, models.OrderTypeNetCredit, order.OrderType)
	assert.Equal(t, models.OrderStrategyTypeSingle, order.OrderStrategyType)
	require.NotNil(t, order.ComplexOrderStrategyType)
	assert.Equal(t, models.ComplexOrderStrategyTypeIronCondor, *order.ComplexOrderStrategyType)
	require.NotNil(t, order.Price)
	assert.Equal(t, 1.50, *order.Price)

	// Verify 4 legs in correct order.
	require.Len(t, order.OrderLegCollection, 4)

	// Leg 1: BUY put at lowest strike (protection).
	assert.Equal(t, models.InstructionBuyToOpen, order.OrderLegCollection[0].Instruction)
	require.NotNil(t, order.OrderLegCollection[0].Instrument.PutCall)
	assert.Equal(t, models.PutCallPut, *order.OrderLegCollection[0].Instrument.PutCall)
	assert.Equal(t, "F     260618P00008000", order.OrderLegCollection[0].Instrument.Symbol)

	// Leg 2: SELL put at mid-low strike (premium).
	assert.Equal(t, models.InstructionSellToOpen, order.OrderLegCollection[1].Instruction)
	require.NotNil(t, order.OrderLegCollection[1].Instrument.PutCall)
	assert.Equal(t, models.PutCallPut, *order.OrderLegCollection[1].Instrument.PutCall)
	assert.Equal(t, "F     260618P00010000", order.OrderLegCollection[1].Instrument.Symbol)

	// Leg 3: SELL call at mid-high strike (premium).
	assert.Equal(t, models.InstructionSellToOpen, order.OrderLegCollection[2].Instruction)
	require.NotNil(t, order.OrderLegCollection[2].Instrument.PutCall)
	assert.Equal(t, models.PutCallCall, *order.OrderLegCollection[2].Instrument.PutCall)
	assert.Equal(t, "F     260618C00014000", order.OrderLegCollection[2].Instrument.Symbol)

	// Leg 4: BUY call at highest strike (protection).
	assert.Equal(t, models.InstructionBuyToOpen, order.OrderLegCollection[3].Instruction)
	require.NotNil(t, order.OrderLegCollection[3].Instrument.PutCall)
	assert.Equal(t, models.PutCallCall, *order.OrderLegCollection[3].Instrument.PutCall)
	assert.Equal(t, "F     260618C00016000", order.OrderLegCollection[3].Instrument.Symbol)
}

// TestBuildIronCondorOrderClose verifies closing flips to NET_DEBIT with close instructions.
func TestBuildIronCondorOrderClose(t *testing.T) {
	t.Parallel()

	expiration := time.Date(2026, time.June, 18, 0, 0, 0, 0, time.UTC)

	order, err := BuildIronCondorOrder(&IronCondorParams{
		Underlying:      "F",
		Expiration:      expiration,
		PutLongStrike:   8,
		PutShortStrike:  10,
		CallShortStrike: 14,
		CallLongStrike:  16,
		Open:            false,
		Quantity:        1,
		Price:           0.50,
	})

	require.NoError(t, err)
	assert.Equal(t, models.OrderTypeNetDebit, order.OrderType)

	// Close: long legs SELL_TO_CLOSE, short legs BUY_TO_CLOSE.
	assert.Equal(t, models.InstructionSellToClose, order.OrderLegCollection[0].Instruction)
	assert.Equal(t, models.InstructionBuyToClose, order.OrderLegCollection[1].Instruction)
	assert.Equal(t, models.InstructionBuyToClose, order.OrderLegCollection[2].Instruction)
	assert.Equal(t, models.InstructionSellToClose, order.OrderLegCollection[3].Instruction)
}

// TestBuildStraddleOrderLongOpen verifies a long straddle (buy to open, net debit).
// BUY_TO_OPEN call + BUY_TO_OPEN put at same strike = NET_DEBIT.
func TestBuildStraddleOrderLongOpen(t *testing.T) {
	t.Parallel()

	expiration := time.Date(2026, time.June, 18, 0, 0, 0, 0, time.UTC)

	order, err := BuildStraddleOrder(&StraddleParams{
		Underlying: "F",
		Expiration: expiration,
		Strike:     12,
		Buy:        true,
		Open:       true,
		Quantity:   1,
		Price:      1.50,
	})

	// Assert order-level fields.
	require.NoError(t, err)
	require.NotNil(t, order)
	assert.Equal(t, models.OrderTypeNetDebit, order.OrderType)
	assert.Equal(t, models.OrderStrategyTypeSingle, order.OrderStrategyType)
	require.NotNil(t, order.ComplexOrderStrategyType)
	assert.Equal(t, models.ComplexOrderStrategyTypeStraddle, *order.ComplexOrderStrategyType)
	require.NotNil(t, order.Price)
	assert.Equal(t, 1.50, *order.Price)
	assert.Equal(t, models.DurationDay, order.Duration)
	assert.Equal(t, models.SessionNormal, order.Session)

	// Assert two legs: call and put at the same strike, both BUY_TO_OPEN.
	require.Len(t, order.OrderLegCollection, 2)

	callLeg := order.OrderLegCollection[0]
	assert.Equal(t, models.InstructionBuyToOpen, callLeg.Instruction)
	assert.Equal(t, 1.0, callLeg.Quantity)
	assert.Equal(t, models.AssetTypeOption, callLeg.Instrument.AssetType)
	assert.Equal(t, "F     260618C00012000", callLeg.Instrument.Symbol)
	require.NotNil(t, callLeg.Instrument.PutCall)
	assert.Equal(t, models.PutCallCall, *callLeg.Instrument.PutCall)

	putLeg := order.OrderLegCollection[1]
	assert.Equal(t, models.InstructionBuyToOpen, putLeg.Instruction)
	assert.Equal(t, 1.0, putLeg.Quantity)
	assert.Equal(t, "F     260618P00012000", putLeg.Instrument.Symbol)
	require.NotNil(t, putLeg.Instrument.PutCall)
	assert.Equal(t, models.PutCallPut, *putLeg.Instrument.PutCall)
}

// TestBuildStraddleOrderShortOpen verifies a short straddle (sell to open, net credit).
func TestBuildStraddleOrderShortOpen(t *testing.T) {
	t.Parallel()

	expiration := time.Date(2026, time.June, 18, 0, 0, 0, 0, time.UTC)

	order, err := BuildStraddleOrder(&StraddleParams{
		Underlying: "F",
		Expiration: expiration,
		Strike:     12,
		Buy:        false,
		Open:       true,
		Quantity:   1,
		Price:      1.50,
	})

	require.NoError(t, err)
	assert.Equal(t, models.OrderTypeNetCredit, order.OrderType)
	assert.Equal(t, models.InstructionSellToOpen, order.OrderLegCollection[0].Instruction)
	assert.Equal(t, models.InstructionSellToOpen, order.OrderLegCollection[1].Instruction)
}

// TestBuildStraddleOrderCloseShort verifies closing a short straddle (buy to close, net debit).
func TestBuildStraddleOrderCloseShort(t *testing.T) {
	t.Parallel()

	expiration := time.Date(2026, time.June, 18, 0, 0, 0, 0, time.UTC)

	order, err := BuildStraddleOrder(&StraddleParams{
		Underlying: "F",
		Expiration: expiration,
		Strike:     12,
		Buy:        true,
		Open:       false,
		Quantity:   1,
		Price:      0.80,
	})

	require.NoError(t, err)
	assert.Equal(t, models.OrderTypeNetDebit, order.OrderType)
	assert.Equal(t, models.InstructionBuyToClose, order.OrderLegCollection[0].Instruction)
	assert.Equal(t, models.InstructionBuyToClose, order.OrderLegCollection[1].Instruction)
}

// TestBuildStraddleOrderCloseLong verifies closing a long straddle (sell to close, net credit).
func TestBuildStraddleOrderCloseLong(t *testing.T) {
	t.Parallel()

	expiration := time.Date(2026, time.June, 18, 0, 0, 0, 0, time.UTC)

	order, err := BuildStraddleOrder(&StraddleParams{
		Underlying: "F",
		Expiration: expiration,
		Strike:     12,
		Buy:        false,
		Open:       false,
		Quantity:   1,
		Price:      0.80,
	})

	require.NoError(t, err)
	assert.Equal(t, models.OrderTypeNetCredit, order.OrderType)
	assert.Equal(t, models.InstructionSellToClose, order.OrderLegCollection[0].Instruction)
	assert.Equal(t, models.InstructionSellToClose, order.OrderLegCollection[1].Instruction)
}

// TestBuildStrangleOrderLongOpen verifies a long strangle (buy to open, net debit).
// BUY_TO_OPEN call at higher strike + BUY_TO_OPEN put at lower strike = NET_DEBIT.
func TestBuildStrangleOrderLongOpen(t *testing.T) {
	t.Parallel()

	expiration := time.Date(2026, time.June, 18, 0, 0, 0, 0, time.UTC)

	order, err := BuildStrangleOrder(&StrangleParams{
		Underlying: "F",
		Expiration: expiration,
		CallStrike: 14,
		PutStrike:  10,
		Buy:        true,
		Open:       true,
		Quantity:   1,
		Price:      0.80,
	})

	// Assert order-level fields.
	require.NoError(t, err)
	require.NotNil(t, order)
	assert.Equal(t, models.OrderTypeNetDebit, order.OrderType)
	assert.Equal(t, models.OrderStrategyTypeSingle, order.OrderStrategyType)
	require.NotNil(t, order.ComplexOrderStrategyType)
	assert.Equal(t, models.ComplexOrderStrategyTypeStrangle, *order.ComplexOrderStrategyType)
	require.NotNil(t, order.Price)
	assert.Equal(t, 0.80, *order.Price)

	// Assert two legs with different strikes, both BUY_TO_OPEN.
	require.Len(t, order.OrderLegCollection, 2)

	callLeg := order.OrderLegCollection[0]
	assert.Equal(t, models.InstructionBuyToOpen, callLeg.Instruction)
	assert.Equal(t, "F     260618C00014000", callLeg.Instrument.Symbol)
	require.NotNil(t, callLeg.Instrument.PutCall)
	assert.Equal(t, models.PutCallCall, *callLeg.Instrument.PutCall)

	putLeg := order.OrderLegCollection[1]
	assert.Equal(t, models.InstructionBuyToOpen, putLeg.Instruction)
	assert.Equal(t, "F     260618P00010000", putLeg.Instrument.Symbol)
	require.NotNil(t, putLeg.Instrument.PutCall)
	assert.Equal(t, models.PutCallPut, *putLeg.Instrument.PutCall)
}

// TestBuildStrangleOrderShortOpen verifies a short strangle (sell to open, net credit).
func TestBuildStrangleOrderShortOpen(t *testing.T) {
	t.Parallel()

	expiration := time.Date(2026, time.June, 18, 0, 0, 0, 0, time.UTC)

	order, err := BuildStrangleOrder(&StrangleParams{
		Underlying: "F",
		Expiration: expiration,
		CallStrike: 14,
		PutStrike:  10,
		Buy:        false,
		Open:       true,
		Quantity:   1,
		Price:      0.80,
	})

	require.NoError(t, err)
	assert.Equal(t, models.OrderTypeNetCredit, order.OrderType)
	assert.Equal(t, models.InstructionSellToOpen, order.OrderLegCollection[0].Instruction)
	assert.Equal(t, models.InstructionSellToOpen, order.OrderLegCollection[1].Instruction)
}

// TestBuildStrangleOrderCloseShort verifies closing a short strangle (buy to close, net debit).
func TestBuildStrangleOrderCloseShort(t *testing.T) {
	t.Parallel()

	expiration := time.Date(2026, time.June, 18, 0, 0, 0, 0, time.UTC)

	order, err := BuildStrangleOrder(&StrangleParams{
		Underlying: "F",
		Expiration: expiration,
		CallStrike: 14,
		PutStrike:  10,
		Buy:        true,
		Open:       false,
		Quantity:   1,
		Price:      0.50,
	})

	require.NoError(t, err)
	assert.Equal(t, models.OrderTypeNetDebit, order.OrderType)
	assert.Equal(t, models.InstructionBuyToClose, order.OrderLegCollection[0].Instruction)
	assert.Equal(t, models.InstructionBuyToClose, order.OrderLegCollection[1].Instruction)
}

// TestBuildVerticalOrderAppliesCustomDurationAndSession verifies non-default overrides.
func TestBuildVerticalOrderAppliesCustomDurationAndSession(t *testing.T) {
	t.Parallel()

	expiration := time.Date(2026, time.June, 18, 0, 0, 0, 0, time.UTC)

	order, err := BuildVerticalOrder(&VerticalParams{
		Underlying:  "SPY",
		Expiration:  expiration,
		LongStrike:  500,
		ShortStrike: 510,
		PutCall:     models.PutCallCall,
		Open:        true,
		Quantity:    5,
		Price:       2.00,
		Duration:    models.DurationGoodTillCancel,
		Session:     models.SessionPM,
	})

	require.NoError(t, err)
	assert.Equal(t, models.DurationGoodTillCancel, order.Duration)
	assert.Equal(t, models.SessionPM, order.Session)
}

// TestBuildCoveredCallOrder verifies a covered call (buy shares + sell call).
// 1 contract = 100 shares equity + 1 call option sold.
func TestBuildCoveredCallOrder(t *testing.T) {
	t.Parallel()

	expiration := time.Date(2026, time.June, 18, 0, 0, 0, 0, time.UTC)

	order, err := BuildCoveredCallOrder(&CoveredCallParams{
		Underlying: "F",
		Expiration: expiration,
		Strike:     14,
		Quantity:   1,
		Price:      12.00,
	})

	// Assert order-level fields.
	require.NoError(t, err)
	require.NotNil(t, order)
	assert.Equal(t, models.OrderTypeNetDebit, order.OrderType)
	assert.Equal(t, models.OrderStrategyTypeSingle, order.OrderStrategyType)
	require.NotNil(t, order.ComplexOrderStrategyType)
	assert.Equal(t, models.ComplexOrderStrategyTypeCovered, *order.ComplexOrderStrategyType)
	require.NotNil(t, order.Price)
	assert.Equal(t, 12.00, *order.Price)
	assert.Equal(t, models.DurationDay, order.Duration)
	assert.Equal(t, models.SessionNormal, order.Session)

	// Assert two legs: equity BUY + option SELL_TO_OPEN.
	require.Len(t, order.OrderLegCollection, 2)

	equityLeg := order.OrderLegCollection[0]
	assert.Equal(t, models.InstructionBuy, equityLeg.Instruction)
	assert.Equal(t, 100.0, equityLeg.Quantity, "1 contract = 100 shares")
	assert.Equal(t, models.AssetTypeEquity, equityLeg.Instrument.AssetType)
	assert.Equal(t, "F", equityLeg.Instrument.Symbol)

	optionLeg := order.OrderLegCollection[1]
	assert.Equal(t, models.InstructionSellToOpen, optionLeg.Instruction)
	assert.Equal(t, 1.0, optionLeg.Quantity)
	assert.Equal(t, models.AssetTypeOption, optionLeg.Instrument.AssetType)
	assert.Equal(t, "F     260618C00014000", optionLeg.Instrument.Symbol)
	require.NotNil(t, optionLeg.Instrument.PutCall)
	assert.Equal(t, models.PutCallCall, *optionLeg.Instrument.PutCall)
	require.NotNil(t, optionLeg.Instrument.OptionStrikePrice)
	assert.Equal(t, 14.0, *optionLeg.Instrument.OptionStrikePrice)
}

// TestBuildCoveredCallOrderMultipleContracts verifies quantity scaling (2 contracts = 200 shares).
func TestBuildCoveredCallOrderMultipleContracts(t *testing.T) {
	t.Parallel()

	expiration := time.Date(2026, time.June, 18, 0, 0, 0, 0, time.UTC)

	order, err := BuildCoveredCallOrder(&CoveredCallParams{
		Underlying: "F",
		Expiration: expiration,
		Strike:     14,
		Quantity:   2,
		Price:      12.00,
	})

	require.NoError(t, err)
	require.Len(t, order.OrderLegCollection, 2)
	assert.Equal(t, 200.0, order.OrderLegCollection[0].Quantity, "2 contracts = 200 shares")
	assert.Equal(t, 2.0, order.OrderLegCollection[1].Quantity)
}

// TestBuildCoveredCallOrderCustomDurationSession verifies non-default duration and session.
func TestBuildCoveredCallOrderCustomDurationSession(t *testing.T) {
	t.Parallel()

	expiration := time.Date(2026, time.June, 18, 0, 0, 0, 0, time.UTC)

	order, err := BuildCoveredCallOrder(&CoveredCallParams{
		Underlying: "F",
		Expiration: expiration,
		Strike:     14,
		Quantity:   1,
		Price:      12.00,
		Duration:   models.DurationGoodTillCancel,
		Session:    models.SessionNormal,
	})

	require.NoError(t, err)
	assert.Equal(t, models.DurationGoodTillCancel, order.Duration)
	assert.Equal(t, models.SessionNormal, order.Session)
}

// TestBuildCalendarOrderOpenCall verifies a long call calendar spread (open).
// BUY_TO_OPEN far + SELL_TO_OPEN near, same strike, different expirations.
func TestBuildCalendarOrderOpenCall(t *testing.T) {
	t.Parallel()

	nearExp := time.Date(2026, time.June, 19, 0, 0, 0, 0, time.UTC)
	farExp := time.Date(2026, time.July, 17, 0, 0, 0, 0, time.UTC)

	order, err := BuildCalendarOrder(&CalendarParams{
		Underlying:     "AAPL",
		NearExpiration: nearExp,
		FarExpiration:  farExp,
		Strike:         150,
		PutCall:        models.PutCallCall,
		Open:           true,
		Quantity:       1,
		Price:          2.50,
	})

	// Assert order-level fields.
	require.NoError(t, err)
	require.NotNil(t, order)
	assert.Equal(t, models.OrderTypeNetDebit, order.OrderType)
	assert.Equal(t, models.OrderStrategyTypeSingle, order.OrderStrategyType)
	require.NotNil(t, order.ComplexOrderStrategyType)
	assert.Equal(t, models.ComplexOrderStrategyTypeCalendar, *order.ComplexOrderStrategyType)
	require.NotNil(t, order.Price)
	assert.Equal(t, 2.50, *order.Price)
	assert.Equal(t, models.DurationDay, order.Duration)
	assert.Equal(t, models.SessionNormal, order.Session)

	// Assert two legs: far (bought) first, near (sold) second.
	require.Len(t, order.OrderLegCollection, 2)

	farLeg := order.OrderLegCollection[0]
	assert.Equal(t, models.InstructionBuyToOpen, farLeg.Instruction)
	assert.Equal(t, 1.0, farLeg.Quantity)
	assert.Equal(t, models.AssetTypeOption, farLeg.Instrument.AssetType)
	require.NotNil(t, farLeg.Instrument.PutCall)
	assert.Equal(t, models.PutCallCall, *farLeg.Instrument.PutCall)
	require.NotNil(t, farLeg.Instrument.OptionStrikePrice)
	assert.Equal(t, 150.0, *farLeg.Instrument.OptionStrikePrice)
	// Far leg uses the far expiration.
	assert.Contains(t, farLeg.Instrument.Symbol, "260717")

	nearLeg := order.OrderLegCollection[1]
	assert.Equal(t, models.InstructionSellToOpen, nearLeg.Instruction)
	assert.Equal(t, 1.0, nearLeg.Quantity)
	assert.Equal(t, models.AssetTypeOption, nearLeg.Instrument.AssetType)
	require.NotNil(t, nearLeg.Instrument.PutCall)
	assert.Equal(t, models.PutCallCall, *nearLeg.Instrument.PutCall)
	require.NotNil(t, nearLeg.Instrument.OptionStrikePrice)
	assert.Equal(t, 150.0, *nearLeg.Instrument.OptionStrikePrice)
	// Near leg uses the near expiration.
	assert.Contains(t, nearLeg.Instrument.Symbol, "260619")
}

// TestBuildCalendarOrderOpenPut verifies a long put calendar spread (open).
func TestBuildCalendarOrderOpenPut(t *testing.T) {
	t.Parallel()

	nearExp := time.Date(2026, time.June, 19, 0, 0, 0, 0, time.UTC)
	farExp := time.Date(2026, time.July, 17, 0, 0, 0, 0, time.UTC)

	order, err := BuildCalendarOrder(&CalendarParams{
		Underlying:     "AAPL",
		NearExpiration: nearExp,
		FarExpiration:  farExp,
		Strike:         150,
		PutCall:        models.PutCallPut,
		Open:           true,
		Quantity:       2,
		Price:          3.00,
	})

	require.NoError(t, err)
	assert.Equal(t, models.OrderTypeNetDebit, order.OrderType)
	require.Len(t, order.OrderLegCollection, 2)

	// Both legs should be PUT.
	require.NotNil(t, order.OrderLegCollection[0].Instrument.PutCall)
	assert.Equal(t, models.PutCallPut, *order.OrderLegCollection[0].Instrument.PutCall)
	require.NotNil(t, order.OrderLegCollection[1].Instrument.PutCall)
	assert.Equal(t, models.PutCallPut, *order.OrderLegCollection[1].Instrument.PutCall)

	// Quantity should match.
	assert.Equal(t, 2.0, order.OrderLegCollection[0].Quantity)
	assert.Equal(t, 2.0, order.OrderLegCollection[1].Quantity)
}

// TestBuildCalendarOrderClose verifies closing a calendar spread uses close instructions.
func TestBuildCalendarOrderClose(t *testing.T) {
	t.Parallel()

	nearExp := time.Date(2026, time.June, 19, 0, 0, 0, 0, time.UTC)
	farExp := time.Date(2026, time.July, 17, 0, 0, 0, 0, time.UTC)

	order, err := BuildCalendarOrder(&CalendarParams{
		Underlying:     "AAPL",
		NearExpiration: nearExp,
		FarExpiration:  farExp,
		Strike:         150,
		PutCall:        models.PutCallCall,
		Open:           false,
		Quantity:       1,
		Price:          1.50,
	})

	require.NoError(t, err)
	assert.Equal(t, models.OrderTypeNetCredit, order.OrderType)

	// Close: far leg SELL_TO_CLOSE, near leg BUY_TO_CLOSE.
	assert.Equal(t, models.InstructionSellToClose, order.OrderLegCollection[0].Instruction)
	assert.Equal(t, models.InstructionBuyToClose, order.OrderLegCollection[1].Instruction)
}

// TestBuildCalendarOrderCustomDurationSession verifies non-default overrides.
func TestBuildCalendarOrderCustomDurationSession(t *testing.T) {
	t.Parallel()

	nearExp := time.Date(2026, time.June, 19, 0, 0, 0, 0, time.UTC)
	farExp := time.Date(2026, time.July, 17, 0, 0, 0, 0, time.UTC)

	order, err := BuildCalendarOrder(&CalendarParams{
		Underlying:     "AAPL",
		NearExpiration: nearExp,
		FarExpiration:  farExp,
		Strike:         150,
		PutCall:        models.PutCallCall,
		Open:           true,
		Quantity:       1,
		Price:          2.50,
		Duration:       models.DurationGoodTillCancel,
		Session:        models.SessionPM,
	})

	require.NoError(t, err)
	assert.Equal(t, models.DurationGoodTillCancel, order.Duration)
	assert.Equal(t, models.SessionPM, order.Session)
}

// TestBuildDiagonalOrderOpenCall verifies a long call diagonal spread (open).
// BUY_TO_OPEN far (FarStrike) + SELL_TO_OPEN near (NearStrike), different strikes and expirations.
func TestBuildDiagonalOrderOpenCall(t *testing.T) {
	t.Parallel()

	nearExp := time.Date(2026, time.June, 19, 0, 0, 0, 0, time.UTC)
	farExp := time.Date(2026, time.July, 17, 0, 0, 0, 0, time.UTC)

	order, err := BuildDiagonalOrder(&DiagonalParams{
		Underlying:     "AAPL",
		NearExpiration: nearExp,
		FarExpiration:  farExp,
		NearStrike:     150,
		FarStrike:      155,
		PutCall:        models.PutCallCall,
		Open:           true,
		Quantity:       1,
		Price:          3.00,
	})

	// Assert order-level fields.
	require.NoError(t, err)
	require.NotNil(t, order)
	assert.Equal(t, models.OrderTypeNetDebit, order.OrderType)
	assert.Equal(t, models.OrderStrategyTypeSingle, order.OrderStrategyType)
	require.NotNil(t, order.ComplexOrderStrategyType)
	assert.Equal(t, models.ComplexOrderStrategyTypeDiagonal, *order.ComplexOrderStrategyType)
	require.NotNil(t, order.Price)
	assert.Equal(t, 3.00, *order.Price)
	assert.Equal(t, models.DurationDay, order.Duration)
	assert.Equal(t, models.SessionNormal, order.Session)

	// Assert two legs: far (bought) first, near (sold) second.
	require.Len(t, order.OrderLegCollection, 2)

	farLeg := order.OrderLegCollection[0]
	assert.Equal(t, models.InstructionBuyToOpen, farLeg.Instruction)
	assert.Equal(t, 1.0, farLeg.Quantity)
	assert.Equal(t, models.AssetTypeOption, farLeg.Instrument.AssetType)
	require.NotNil(t, farLeg.Instrument.PutCall)
	assert.Equal(t, models.PutCallCall, *farLeg.Instrument.PutCall)
	require.NotNil(t, farLeg.Instrument.OptionStrikePrice)
	assert.Equal(t, 155.0, *farLeg.Instrument.OptionStrikePrice)
	assert.Contains(t, farLeg.Instrument.Symbol, "260717")

	nearLeg := order.OrderLegCollection[1]
	assert.Equal(t, models.InstructionSellToOpen, nearLeg.Instruction)
	assert.Equal(t, 1.0, nearLeg.Quantity)
	assert.Equal(t, models.AssetTypeOption, nearLeg.Instrument.AssetType)
	require.NotNil(t, nearLeg.Instrument.PutCall)
	assert.Equal(t, models.PutCallCall, *nearLeg.Instrument.PutCall)
	require.NotNil(t, nearLeg.Instrument.OptionStrikePrice)
	assert.Equal(t, 150.0, *nearLeg.Instrument.OptionStrikePrice)
	assert.Contains(t, nearLeg.Instrument.Symbol, "260619")
}

// TestBuildDiagonalOrderOpenPut verifies a long put diagonal spread (open).
func TestBuildDiagonalOrderOpenPut(t *testing.T) {
	t.Parallel()

	nearExp := time.Date(2026, time.June, 19, 0, 0, 0, 0, time.UTC)
	farExp := time.Date(2026, time.July, 17, 0, 0, 0, 0, time.UTC)

	order, err := BuildDiagonalOrder(&DiagonalParams{
		Underlying:     "AAPL",
		NearExpiration: nearExp,
		FarExpiration:  farExp,
		NearStrike:     155,
		FarStrike:      150,
		PutCall:        models.PutCallPut,
		Open:           true,
		Quantity:       3,
		Price:          4.00,
	})

	require.NoError(t, err)
	assert.Equal(t, models.OrderTypeNetDebit, order.OrderType)
	require.Len(t, order.OrderLegCollection, 2)

	// Both legs should be PUT.
	require.NotNil(t, order.OrderLegCollection[0].Instrument.PutCall)
	assert.Equal(t, models.PutCallPut, *order.OrderLegCollection[0].Instrument.PutCall)
	require.NotNil(t, order.OrderLegCollection[1].Instrument.PutCall)
	assert.Equal(t, models.PutCallPut, *order.OrderLegCollection[1].Instrument.PutCall)

	// Far leg (bought) at FarStrike, near leg (sold) at NearStrike.
	require.NotNil(t, order.OrderLegCollection[0].Instrument.OptionStrikePrice)
	assert.Equal(t, 150.0, *order.OrderLegCollection[0].Instrument.OptionStrikePrice)
	require.NotNil(t, order.OrderLegCollection[1].Instrument.OptionStrikePrice)
	assert.Equal(t, 155.0, *order.OrderLegCollection[1].Instrument.OptionStrikePrice)

	assert.Equal(t, 3.0, order.OrderLegCollection[0].Quantity)
	assert.Equal(t, 3.0, order.OrderLegCollection[1].Quantity)
}

// TestBuildDiagonalOrderClose verifies closing a diagonal spread uses close instructions.
func TestBuildDiagonalOrderClose(t *testing.T) {
	t.Parallel()

	nearExp := time.Date(2026, time.June, 19, 0, 0, 0, 0, time.UTC)
	farExp := time.Date(2026, time.July, 17, 0, 0, 0, 0, time.UTC)

	order, err := BuildDiagonalOrder(&DiagonalParams{
		Underlying:     "AAPL",
		NearExpiration: nearExp,
		FarExpiration:  farExp,
		NearStrike:     150,
		FarStrike:      155,
		PutCall:        models.PutCallCall,
		Open:           false,
		Quantity:       1,
		Price:          2.00,
	})

	require.NoError(t, err)
	assert.Equal(t, models.OrderTypeNetCredit, order.OrderType)

	// Close: far leg SELL_TO_CLOSE, near leg BUY_TO_CLOSE.
	assert.Equal(t, models.InstructionSellToClose, order.OrderLegCollection[0].Instruction)
	assert.Equal(t, models.InstructionBuyToClose, order.OrderLegCollection[1].Instruction)
}

// TestBuildCollarOrderOpen verifies an opening collar (buy shares + buy put + sell call).
// 1 contract = 100 shares equity + 1 protective put + 1 covered call.
func TestBuildCollarOrderOpen(t *testing.T) {
	t.Parallel()

	expiration := time.Date(2026, time.June, 19, 0, 0, 0, 0, time.UTC)

	order, err := BuildCollarOrder(&CollarParams{
		Underlying: "AAPL",
		PutStrike:  140,
		CallStrike: 160,
		Expiration: expiration,
		Quantity:   1,
		Open:       true,
		Price:      150.00,
	})

	// Assert order-level fields.
	require.NoError(t, err)
	require.NotNil(t, order)
	assert.Equal(t, models.OrderTypeNetDebit, order.OrderType)
	assert.Equal(t, models.OrderStrategyTypeSingle, order.OrderStrategyType)
	require.NotNil(t, order.ComplexOrderStrategyType)
	assert.Equal(t, models.ComplexOrderStrategyTypeCollarWithStock, *order.ComplexOrderStrategyType)
	require.NotNil(t, order.Price)
	assert.Equal(t, 150.00, *order.Price)
	assert.Equal(t, models.DurationDay, order.Duration)
	assert.Equal(t, models.SessionNormal, order.Session)

	// Assert three legs: equity + put + call.
	require.Len(t, order.OrderLegCollection, 3)

	// Leg 1: BUY equity shares (1 contract = 100 shares).
	equityLeg := order.OrderLegCollection[0]
	assert.Equal(t, models.InstructionBuy, equityLeg.Instruction)
	assert.Equal(t, 100.0, equityLeg.Quantity, "1 contract = 100 shares")
	assert.Equal(t, models.AssetTypeEquity, equityLeg.Instrument.AssetType)
	assert.Equal(t, "AAPL", equityLeg.Instrument.Symbol)

	// Leg 2: BUY_TO_OPEN put (protective put).
	putLeg := order.OrderLegCollection[1]
	assert.Equal(t, models.InstructionBuyToOpen, putLeg.Instruction)
	assert.Equal(t, 1.0, putLeg.Quantity)
	assert.Equal(t, models.AssetTypeOption, putLeg.Instrument.AssetType)
	require.NotNil(t, putLeg.Instrument.PutCall)
	assert.Equal(t, models.PutCallPut, *putLeg.Instrument.PutCall)
	require.NotNil(t, putLeg.Instrument.OptionStrikePrice)
	assert.Equal(t, 140.0, *putLeg.Instrument.OptionStrikePrice)
	assert.Contains(t, putLeg.Instrument.Symbol, "260619")
	assert.Contains(t, putLeg.Instrument.Symbol, "P00140000")

	// Leg 3: SELL_TO_OPEN call (covered call).
	callLeg := order.OrderLegCollection[2]
	assert.Equal(t, models.InstructionSellToOpen, callLeg.Instruction)
	assert.Equal(t, 1.0, callLeg.Quantity)
	assert.Equal(t, models.AssetTypeOption, callLeg.Instrument.AssetType)
	require.NotNil(t, callLeg.Instrument.PutCall)
	assert.Equal(t, models.PutCallCall, *callLeg.Instrument.PutCall)
	require.NotNil(t, callLeg.Instrument.OptionStrikePrice)
	assert.Equal(t, 160.0, *callLeg.Instrument.OptionStrikePrice)
	assert.Contains(t, callLeg.Instrument.Symbol, "260619")
	assert.Contains(t, callLeg.Instrument.Symbol, "C00160000")
}

// TestBuildCollarOrderClose verifies a closing collar reverses all instructions.
func TestBuildCollarOrderClose(t *testing.T) {
	t.Parallel()

	expiration := time.Date(2026, time.June, 19, 0, 0, 0, 0, time.UTC)

	order, err := BuildCollarOrder(&CollarParams{
		Underlying: "AAPL",
		PutStrike:  140,
		CallStrike: 160,
		Expiration: expiration,
		Quantity:   1,
		Open:       false,
		Price:      150.00,
	})

	require.NoError(t, err)
	require.Len(t, order.OrderLegCollection, 3)

	// Closing: equity SELL, put SELL_TO_CLOSE, call BUY_TO_CLOSE, NET_CREDIT.
	assert.Equal(t, models.OrderTypeNetCredit, order.OrderType)
	assert.Equal(t, models.InstructionSell, order.OrderLegCollection[0].Instruction)
	assert.Equal(t, models.InstructionSellToClose, order.OrderLegCollection[1].Instruction)
	assert.Equal(t, models.InstructionBuyToClose, order.OrderLegCollection[2].Instruction)
}

// TestBuildCollarOrderQuantityScaling verifies equity quantity = option quantity * 100.
func TestBuildCollarOrderQuantityScaling(t *testing.T) {
	t.Parallel()

	expiration := time.Date(2026, time.June, 19, 0, 0, 0, 0, time.UTC)

	order, err := BuildCollarOrder(&CollarParams{
		Underlying: "AAPL",
		PutStrike:  140,
		CallStrike: 160,
		Expiration: expiration,
		Quantity:   5,
		Open:       true,
		Price:      750.00,
	})

	require.NoError(t, err)
	require.Len(t, order.OrderLegCollection, 3)
	assert.Equal(t, 500.0, order.OrderLegCollection[0].Quantity, "5 contracts = 500 shares")
	assert.Equal(t, 5.0, order.OrderLegCollection[1].Quantity)
	assert.Equal(t, 5.0, order.OrderLegCollection[2].Quantity)
}

// TestBuildCollarOrderCustomDurationSession verifies non-default overrides.
func TestBuildCollarOrderCustomDurationSession(t *testing.T) {
	t.Parallel()

	expiration := time.Date(2026, time.June, 19, 0, 0, 0, 0, time.UTC)

	order, err := BuildCollarOrder(&CollarParams{
		Underlying: "AAPL",
		PutStrike:  140,
		CallStrike: 160,
		Expiration: expiration,
		Quantity:   1,
		Open:       true,
		Price:      150.00,
		Duration:   models.DurationGoodTillCancel,
		Session:    models.SessionPM,
	})

	require.NoError(t, err)
	assert.Equal(t, models.DurationGoodTillCancel, order.Duration)
	assert.Equal(t, models.SessionPM, order.Session)
}

// TestBuildDiagonalOrderCustomDurationSession verifies non-default overrides.
func TestBuildDiagonalOrderCustomDurationSession(t *testing.T) {
	t.Parallel()

	nearExp := time.Date(2026, time.June, 19, 0, 0, 0, 0, time.UTC)
	farExp := time.Date(2026, time.July, 17, 0, 0, 0, 0, time.UTC)

	order, err := BuildDiagonalOrder(&DiagonalParams{
		Underlying:     "SPY",
		NearExpiration: nearExp,
		FarExpiration:  farExp,
		NearStrike:     500,
		FarStrike:      505,
		PutCall:        models.PutCallCall,
		Open:           true,
		Quantity:       5,
		Price:          2.00,
		Duration:       models.DurationGoodTillCancel,
		Session:        models.SessionPM,
	})

	require.NoError(t, err)
	assert.Equal(t, models.DurationGoodTillCancel, order.Duration)
	assert.Equal(t, models.SessionPM, order.Session)
}
