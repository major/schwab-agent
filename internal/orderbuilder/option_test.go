package orderbuilder

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/major/schwab-agent/internal/apperr"
	"github.com/major/schwab-agent/internal/models"
)

// TestBuildOCCSymbol verifies OCC symbol formatting for representative contracts.
func TestBuildOCCSymbol(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		underlying string
		expiration time.Time
		strike     float64
		putCall    string
		want       string
	}{
		{
			name:       "aapl call",
			underlying: "AAPL",
			expiration: time.Date(2025, time.June, 20, 0, 0, 0, 0, time.UTC),
			strike:     200,
			putCall:    string(models.PutCallCall),
			want:       "AAPL  250620C00200000",
		},
		{
			name:       "spy put with decimal strike",
			underlying: "SPY",
			expiration: time.Date(2025, time.December, 19, 0, 0, 0, 0, time.UTC),
			strike:     450.50,
			putCall:    string(models.PutCallPut),
			want:       "SPY   251219P00450500",
		},
		{
			name:       "googl call",
			underlying: "GOOGL",
			expiration: time.Date(2025, time.January, 17, 0, 0, 0, 0, time.UTC),
			strike:     150,
			putCall:    string(models.PutCallCall),
			want:       "GOOGL 250117C00150000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, BuildOCCSymbol(tt.underlying, tt.expiration, tt.strike, tt.putCall))
		})
	}
}

// TestParseOCCSymbol verifies parsing OCC symbols back into components.
func TestParseOCCSymbol(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		symbol     string
		underlying string
		putCall    string
		strike     float64
		wantYear   int
		wantMonth  time.Month
		wantDay    int
	}{
		{
			name:       "aapl call",
			symbol:     "AAPL  250620C00200000",
			underlying: "AAPL",
			putCall:    "CALL",
			strike:     200.0,
			wantYear:   2025,
			wantMonth:  time.June,
			wantDay:    20,
		},
		{
			name:       "spy put with decimal strike",
			symbol:     "SPY   251219P00450500",
			underlying: "SPY",
			putCall:    "PUT",
			strike:     450.5,
			wantYear:   2025,
			wantMonth:  time.December,
			wantDay:    19,
		},
		{
			name:       "googl call",
			symbol:     "GOOGL 250117C00150000",
			underlying: "GOOGL",
			putCall:    "CALL",
			strike:     150.0,
			wantYear:   2025,
			wantMonth:  time.January,
			wantDay:    17,
		},
		{
			name:       "six char underlying",
			symbol:     "ABCDEF260315P01000000",
			underlying: "ABCDEF",
			putCall:    "PUT",
			strike:     1000.0,
			wantYear:   2026,
			wantMonth:  time.March,
			wantDay:    15,
		},
		{
			name:       "fractional strike with three decimal places",
			symbol:     "X     250620C00001500",
			underlying: "X",
			putCall:    "CALL",
			strike:     1.5,
			wantYear:   2025,
			wantMonth:  time.June,
			wantDay:    20,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseOCCSymbol(tt.symbol)
			require.NoError(t, err)
			assert.Equal(t, tt.underlying, result.Underlying)
			assert.Equal(t, tt.putCall, result.PutCall)
			assert.Equal(t, tt.strike, result.Strike)
			assert.Equal(t, tt.symbol, result.Symbol)
			assert.Equal(t, tt.wantYear, result.Expiration.Year())
			assert.Equal(t, tt.wantMonth, result.Expiration.Month())
			assert.Equal(t, tt.wantDay, result.Expiration.Day())
		})
	}
}

// TestParseOCCSymbolErrors verifies that invalid OCC symbols produce errors.
func TestParseOCCSymbolErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		symbol  string
		wantMsg string
	}{
		{
			name:    "too short",
			symbol:  "AAPL",
			wantMsg: "must be 21 characters",
		},
		{
			name:    "too long",
			symbol:  "AAPL  250620C002000001",
			wantMsg: "must be 21 characters",
		},
		{
			name:    "empty underlying",
			symbol:  "      250620C00200000",
			wantMsg: "empty underlying",
		},
		{
			name:    "invalid put/call indicator",
			symbol:  "AAPL  250620X00200000",
			wantMsg: "invalid put/call indicator",
		},
		{
			name:    "invalid expiration date",
			symbol:  "AAPL  259920C00200000",
			wantMsg: "invalid expiration date",
		},
		{
			name:    "non-numeric strike",
			symbol:  "AAPL  250620C0020000A",
			wantMsg: "invalid strike price",
		},
		{
			name:    "zero strike",
			symbol:  "AAPL  250620C00000000",
			wantMsg: "strike price must be positive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseOCCSymbol(tt.symbol)
			require.Error(t, err)
			assert.Nil(t, result)
			assert.Contains(t, err.Error(), tt.wantMsg)
		})
	}
}

// TestParseOCCSymbolRoundTrip verifies that build -> parse produces the original components.
func TestParseOCCSymbolRoundTrip(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		underlying string
		expiration time.Time
		strike     float64
		putCall    string
	}{
		{
			name:       "aapl call round trip",
			underlying: "AAPL",
			expiration: time.Date(2025, time.June, 20, 0, 0, 0, 0, time.UTC),
			strike:     200,
			putCall:    string(models.PutCallCall),
		},
		{
			name:       "spy put decimal round trip",
			underlying: "SPY",
			expiration: time.Date(2025, time.December, 19, 0, 0, 0, 0, time.UTC),
			strike:     450.50,
			putCall:    string(models.PutCallPut),
		},
		{
			name:       "single char underlying",
			underlying: "X",
			expiration: time.Date(2026, time.March, 15, 0, 0, 0, 0, time.UTC),
			strike:     25.5,
			putCall:    string(models.PutCallCall),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			symbol := BuildOCCSymbol(tt.underlying, tt.expiration, tt.strike, tt.putCall)

			// Act
			parsed, err := ParseOCCSymbol(symbol)

			// Assert
			require.NoError(t, err)
			assert.Equal(t, tt.underlying, parsed.Underlying)
			assert.Equal(t, tt.putCall, parsed.PutCall)
			assert.Equal(t, tt.strike, parsed.Strike)
			assert.Equal(t, tt.expiration.Year(), parsed.Expiration.Year())
			assert.Equal(t, tt.expiration.Month(), parsed.Expiration.Month())
			assert.Equal(t, tt.expiration.Day(), parsed.Expiration.Day())
		})
	}
}

// TestBuildOptionOrderAppliesDefaults verifies option order defaults and OCC symbol construction.
func TestBuildOptionOrderAppliesDefaults(t *testing.T) {
	expiration := time.Date(2025, time.June, 20, 0, 0, 0, 0, time.UTC)

	order, err := BuildOptionOrder(&OptionParams{
		Underlying: "AAPL",
		Expiration: expiration,
		Strike:     200,
		PutCall:    models.PutCallCall,
		Action:     models.InstructionBuyToOpen,
		Quantity:   1,
		OrderType:  models.OrderTypeMarket,
	})

	require.NoError(t, err)
	require.NotNil(t, order)
	assert.Equal(t, models.DurationDay, order.Duration)
	assert.Equal(t, models.SessionNormal, order.Session)
	assert.Equal(t, models.OrderTypeMarket, order.OrderType)
	assert.Equal(t, models.OrderStrategyTypeSingle, order.OrderStrategyType)
	assert.Nil(t, order.Price)
	require.Len(t, order.OrderLegCollection, 1)

	leg := order.OrderLegCollection[0]
	assert.Equal(t, models.InstructionBuyToOpen, leg.Instruction)
	assert.Equal(t, 1.0, leg.Quantity)
	assert.Equal(t, models.AssetTypeOption, leg.Instrument.AssetType)
	assert.Equal(t, "AAPL  250620C00200000", leg.Instrument.Symbol)
	require.NotNil(t, leg.Instrument.PutCall)
	assert.Equal(t, models.PutCallCall, *leg.Instrument.PutCall)
	require.NotNil(t, leg.Instrument.UnderlyingSymbol)
	assert.Equal(t, "AAPL", *leg.Instrument.UnderlyingSymbol)
	require.NotNil(t, leg.Instrument.OptionExpirationDate)
	assert.Equal(t, "2025-06-20", *leg.Instrument.OptionExpirationDate)
	require.NotNil(t, leg.Instrument.OptionStrikePrice)
	assert.Equal(t, 200.0, *leg.Instrument.OptionStrikePrice)
	require.NotNil(t, leg.Instrument.OptionMultiplier)
	assert.Equal(t, 100.0, *leg.Instrument.OptionMultiplier)
}

// TestBuildOptionOrderSetsLimitPrice verifies limit option orders set the limit price.
func TestBuildOptionOrderSetsLimitPrice(t *testing.T) {
	order, err := BuildOptionOrder(&OptionParams{
		Underlying: "SPY",
		Expiration: time.Date(2025, time.December, 19, 0, 0, 0, 0, time.UTC),
		Strike:     450.50,
		PutCall:    models.PutCallPut,
		Action:     models.InstructionSellToClose,
		Quantity:   2,
		OrderType:  models.OrderTypeLimit,
		Price:      3.25,
		Duration:   models.DurationGoodTillCancel,
		Session:    models.SessionPM,
	})

	require.NoError(t, err)
	require.NotNil(t, order)
	require.NotNil(t, order.Price)
	assert.Equal(t, 3.25, *order.Price)
	assert.Equal(t, models.DurationGoodTillCancel, order.Duration)
	assert.Equal(t, models.SessionPM, order.Session)
	assert.Equal(t, "SPY   251219P00450500", order.OrderLegCollection[0].Instrument.Symbol)
}

// TestBuildOptionOrderSetsSpecialInstruction verifies the special instruction
// field is set on the order when provided.
func TestBuildOptionOrderSetsSpecialInstruction(t *testing.T) {
	order, err := BuildOptionOrder(&OptionParams{
		Underlying:         "AAPL",
		Expiration:         time.Date(2025, time.December, 19, 0, 0, 0, 0, time.UTC),
		Strike:             200.0,
		PutCall:            models.PutCallCall,
		Action:             models.InstructionBuyToOpen,
		Quantity:           5,
		OrderType:          models.OrderTypeLimit,
		Price:              4.50,
		SpecialInstruction: models.SpecialInstructionDoNotReduce,
	})

	require.NoError(t, err)
	require.NotNil(t, order)
	require.NotNil(t, order.SpecialInstruction)
	assert.Equal(t, models.SpecialInstructionDoNotReduce, *order.SpecialInstruction)
}

// TestBuildOptionOrderOmitsSpecialInstructionWhenEmpty verifies the field is
// nil when no special instruction is provided.
func TestBuildOptionOrderOmitsSpecialInstructionWhenEmpty(t *testing.T) {
	order, err := BuildOptionOrder(&OptionParams{
		Underlying: "AAPL",
		Expiration: time.Date(2025, time.December, 19, 0, 0, 0, 0, time.UTC),
		Strike:     200.0,
		PutCall:    models.PutCallCall,
		Action:     models.InstructionBuyToOpen,
		Quantity:   5,
		OrderType:  models.OrderTypeMarket,
	})

	require.NoError(t, err)
	require.NotNil(t, order)
	assert.Nil(t, order.SpecialInstruction)
}

// TestBuildOptionOrderSetsDestination verifies destination is set when provided.
func TestBuildOptionOrderSetsDestination(t *testing.T) {
	order, err := BuildOptionOrder(&OptionParams{
		Underlying:  "AAPL",
		Expiration:  time.Date(2025, time.December, 19, 0, 0, 0, 0, time.UTC),
		Strike:      200.0,
		PutCall:     models.PutCallCall,
		Action:      models.InstructionBuyToOpen,
		Quantity:    5,
		OrderType:   models.OrderTypeLimit,
		Price:       4.50,
		Destination: models.RequestedDestinationCBOE,
	})

	require.NoError(t, err)
	require.NotNil(t, order)
	require.NotNil(t, order.RequestedDestination)
	assert.Equal(t, models.RequestedDestinationCBOE, *order.RequestedDestination)
}

// TestBuildOptionOrderOmitsDestinationWhenEmpty verifies destination is nil when not set.
func TestBuildOptionOrderOmitsDestinationWhenEmpty(t *testing.T) {
	order, err := BuildOptionOrder(&OptionParams{
		Underlying: "AAPL",
		Expiration: time.Date(2025, time.December, 19, 0, 0, 0, 0, time.UTC),
		Strike:     200.0,
		PutCall:    models.PutCallCall,
		Action:     models.InstructionBuyToOpen,
		Quantity:   5,
		OrderType:  models.OrderTypeMarket,
	})

	require.NoError(t, err)
	require.NotNil(t, order)
	assert.Nil(t, order.RequestedDestination)
}

// TestBuildOptionOrderSetsPriceLinkFields verifies price link fields are set when provided.
func TestBuildOptionOrderSetsPriceLinkFields(t *testing.T) {
	order, err := BuildOptionOrder(&OptionParams{
		Underlying:     "AAPL",
		Expiration:     time.Date(2025, time.December, 19, 0, 0, 0, 0, time.UTC),
		Strike:         200.0,
		PutCall:        models.PutCallCall,
		Action:         models.InstructionBuyToOpen,
		Quantity:       5,
		OrderType:      models.OrderTypeLimit,
		Price:          4.50,
		PriceLinkBasis: models.PriceLinkBasisBid,
		PriceLinkType:  models.PriceLinkTypePercent,
	})

	require.NoError(t, err)
	require.NotNil(t, order)
	require.NotNil(t, order.PriceLinkBasis)
	assert.Equal(t, models.PriceLinkBasisBid, *order.PriceLinkBasis)
	require.NotNil(t, order.PriceLinkType)
	assert.Equal(t, models.PriceLinkTypePercent, *order.PriceLinkType)
}

// TestBuildOptionOrderRejectsMarketWithPriceLink verifies that market orders
// reject price-link fields since there is no price to link against.
func TestBuildOptionOrderRejectsMarketWithPriceLink(t *testing.T) {
	t.Parallel()

	order, err := BuildOptionOrder(&OptionParams{
		Underlying:     "AAPL",
		Expiration:     time.Date(2025, time.December, 19, 0, 0, 0, 0, time.UTC),
		Strike:         200.0,
		PutCall:        models.PutCallCall,
		Action:         models.InstructionBuyToOpen,
		Quantity:       5,
		OrderType:      models.OrderTypeMarket,
		PriceLinkBasis: models.PriceLinkBasisLast,
	})

	require.Nil(t, order)
	require.Error(t, err)

	validationErr, ok := errors.AsType[*apperr.ValidationError](err)
	require.True(t, ok)
	assert.Equal(t, "price-link-basis and price-link-type are not allowed on market orders", validationErr.Message)
}

func TestBuildOptionOrderRejectsMarketOnCloseWithPriceLink(t *testing.T) {
	t.Parallel()

	order, err := BuildOptionOrder(&OptionParams{
		Underlying:     "AAPL",
		Expiration:     time.Date(2025, time.December, 19, 0, 0, 0, 0, time.UTC),
		Strike:         200.0,
		PutCall:        models.PutCallCall,
		Action:         models.InstructionBuyToOpen,
		Quantity:       5,
		OrderType:      models.OrderTypeMarketOnClose,
		PriceLinkBasis: models.PriceLinkBasisLast,
	})

	require.Nil(t, order)
	require.Error(t, err)

	validationErr, ok := errors.AsType[*apperr.ValidationError](err)
	require.True(t, ok)
	assert.Equal(t, "price-link-basis and price-link-type are not allowed on market orders", validationErr.Message)
}

// TestBuildOptionOrderOmitsPriceLinkFieldsWhenEmpty verifies price link fields
// are nil when not provided.
func TestBuildOptionOrderOmitsPriceLinkFieldsWhenEmpty(t *testing.T) {
	order, err := BuildOptionOrder(&OptionParams{
		Underlying: "AAPL",
		Expiration: time.Date(2025, time.December, 19, 0, 0, 0, 0, time.UTC),
		Strike:     200.0,
		PutCall:    models.PutCallCall,
		Action:     models.InstructionBuyToOpen,
		Quantity:   5,
		OrderType:  models.OrderTypeMarket,
	})

	require.NoError(t, err)
	require.NotNil(t, order)
	assert.Nil(t, order.PriceLinkBasis)
	assert.Nil(t, order.PriceLinkType)
}
