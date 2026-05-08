package orderbuilder

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/major/schwab-agent/internal/models"
)

// testFutureExp returns a future expiration date one month from now.
// Using a dynamic date ensures tests don't fail as calendar dates pass.
func testFutureExp() time.Time {
	return time.Now().UTC().AddDate(0, 1, 0)
}

// TestBuildJadeLizardOrder verifies the three-leg jade lizard structure:
// short put + short call + long call (protection), always NET_CREDIT.
func TestBuildJadeLizardOrder(t *testing.T) {
	t.Parallel()

	expiration := testFutureExp()

	order, err := BuildJadeLizardOrder(&JadeLizardParams{
		Underlying:      "AAPL",
		Expiration:      expiration,
		PutStrike:       180,
		ShortCallStrike: 210,
		LongCallStrike:  220,
		Quantity:        1,
		Price:           3.50,
	})

	require.NoError(t, err)
	require.NotNil(t, order)
	assert.Equal(t, models.OrderTypeNetCredit, order.OrderType)
	assert.Equal(t, models.OrderStrategyTypeSingle, order.OrderStrategyType)
	require.NotNil(t, order.ComplexOrderStrategyType)
	assert.Equal(t, models.ComplexOrderStrategyTypeCustom, *order.ComplexOrderStrategyType)
	require.NotNil(t, order.Price)
	assert.InEpsilon(t, 3.50, *order.Price, 1e-9)
	assert.Equal(t, models.DurationDay, order.Duration)
	assert.Equal(t, models.SessionNormal, order.Session)

	// Three legs: short put, short call, long call.
	require.Len(t, order.OrderLegCollection, 3)

	// Leg 1: SELL_TO_OPEN put at 180.
	putLeg := order.OrderLegCollection[0]
	assert.Equal(t, models.InstructionSellToOpen, putLeg.Instruction)
	assert.InEpsilon(t, 1.0, putLeg.Quantity, 1e-9)
	assert.Equal(t, models.AssetTypeOption, putLeg.Instrument.AssetType)
	assert.Equal(t, BuildOCCSymbol("AAPL", expiration, 180, string(models.PutCallPut)), putLeg.Instrument.Symbol)
	require.NotNil(t, putLeg.Instrument.PutCall)
	assert.Equal(t, models.PutCallPut, *putLeg.Instrument.PutCall)

	// Leg 2: SELL_TO_OPEN call at 210.
	shortCallLeg := order.OrderLegCollection[1]
	assert.Equal(t, models.InstructionSellToOpen, shortCallLeg.Instruction)
	assert.InEpsilon(t, 1.0, shortCallLeg.Quantity, 1e-9)
	assert.Equal(t, BuildOCCSymbol("AAPL", expiration, 210, string(models.PutCallCall)), shortCallLeg.Instrument.Symbol)
	require.NotNil(t, shortCallLeg.Instrument.PutCall)
	assert.Equal(t, models.PutCallCall, *shortCallLeg.Instrument.PutCall)

	// Leg 3: BUY_TO_OPEN call at 220 (protection).
	longCallLeg := order.OrderLegCollection[2]
	assert.Equal(t, models.InstructionBuyToOpen, longCallLeg.Instruction)
	assert.InEpsilon(t, 1.0, longCallLeg.Quantity, 1e-9)
	assert.Equal(t, BuildOCCSymbol("AAPL", expiration, 220, string(models.PutCallCall)), longCallLeg.Instrument.Symbol)
	require.NotNil(t, longCallLeg.Instrument.PutCall)
	assert.Equal(t, models.PutCallCall, *longCallLeg.Instrument.PutCall)
}

// TestBuildJadeLizardOrderCustomDurationSession verifies non-default duration and session.
func TestBuildJadeLizardOrderCustomDurationSession(t *testing.T) {
	t.Parallel()

	expiration := testFutureExp()

	order, err := BuildJadeLizardOrder(&JadeLizardParams{
		Underlying:      "SPY",
		Expiration:      expiration,
		PutStrike:       400,
		ShortCallStrike: 450,
		LongCallStrike:  460,
		Quantity:        5,
		Price:           2.00,
		Duration:        models.DurationGoodTillCancel,
		Session:         models.SessionNormal,
	})

	require.NoError(t, err)
	assert.Equal(t, models.DurationGoodTillCancel, order.Duration)
	assert.Equal(t, models.SessionNormal, order.Session)
	require.Len(t, order.OrderLegCollection, 3)

	// All legs should have quantity 5.
	for _, leg := range order.OrderLegCollection {
		assert.InEpsilon(t, 5.0, leg.Quantity, 1e-9)
	}
}

// TestValidateJadeLizardOrder verifies validation catches invalid parameters.
func TestValidateJadeLizardOrder(t *testing.T) {
	t.Parallel()

	expiration := testFutureExp()
	valid := JadeLizardParams{
		Underlying:      "AAPL",
		Expiration:      expiration,
		PutStrike:       180,
		ShortCallStrike: 210,
		LongCallStrike:  220,
		Quantity:        1,
		Price:           3.50,
	}

	tests := []struct {
		name   string
		modify func(*JadeLizardParams)
		errMsg string
	}{
		{
			name:   "missing underlying",
			modify: func(p *JadeLizardParams) { p.Underlying = "" },
			errMsg: "underlying symbol is required",
		},
		{
			name:   "zero quantity",
			modify: func(p *JadeLizardParams) { p.Quantity = 0 },
			errMsg: "quantity must be greater than zero",
		},
		{
			name:   "zero put strike",
			modify: func(p *JadeLizardParams) { p.PutStrike = 0 },
			errMsg: "put-strike must be greater than zero",
		},
		{
			name:   "zero short call strike",
			modify: func(p *JadeLizardParams) { p.ShortCallStrike = 0 },
			errMsg: "short-call-strike must be greater than zero",
		},
		{
			name:   "zero long call strike",
			modify: func(p *JadeLizardParams) { p.LongCallStrike = 0 },
			errMsg: "long-call-strike must be greater than zero",
		},
		{
			name:   "put strike equals short call strike",
			modify: func(p *JadeLizardParams) { p.PutStrike = 210 },
			errMsg: "put strike must be below short call strike",
		},
		{
			name:   "put strike above short call strike",
			modify: func(p *JadeLizardParams) { p.PutStrike = 215 },
			errMsg: "put strike must be below short call strike",
		},
		{
			name:   "short call strike equals long call strike",
			modify: func(p *JadeLizardParams) { p.ShortCallStrike = 220 },
			errMsg: "short call strike must be below long call strike",
		},
		{
			name:   "short call strike above long call strike",
			modify: func(p *JadeLizardParams) { p.ShortCallStrike = 225 },
			errMsg: "short call strike must be below long call strike",
		},
		{
			name:   "zero price",
			modify: func(p *JadeLizardParams) { p.Price = 0 },
			errMsg: "price is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			params := valid
			tt.modify(&params)

			err := ValidateJadeLizardOrder(&params)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errMsg)
		})
	}

	// Happy path: valid params should pass.
	t.Run("valid params", func(t *testing.T) {
		t.Parallel()

		params := valid
		require.NoError(t, ValidateJadeLizardOrder(&params))
	})
}
