package orderbuilder

import (
	"time"

	"github.com/major/schwab-agent/internal/models"
)

// VerticalParams holds parameters for building a vertical spread order.
type VerticalParams struct {
	Underlying  string
	Expiration  time.Time
	LongStrike  float64 // Strike price of the option being bought
	ShortStrike float64 // Strike price of the option being sold
	PutCall     models.PutCall
	Open        bool // true = opening position, false = closing
	Quantity    float64
	Price       float64 // Net debit/credit amount (always positive)
	Duration    models.Duration
	Session     models.Session
}

// BuildVerticalOrder constructs an OrderRequest for a two-leg vertical spread.
//
// The order type (NET_DEBIT or NET_CREDIT) is auto-determined from the strike
// relationship and whether the position is opening or closing:
//
//	Call spreads:
//	  long < short -> debit to open (bull call), credit to close
//	  long > short -> credit to open (bear call), debit to close
//
//	Put spreads:
//	  long > short -> debit to open (bear put), credit to close
//	  long < short -> credit to open (bull put), debit to close
func BuildVerticalOrder(params *VerticalParams) (*models.OrderRequest, error) {
	longInstruction, shortInstruction := spreadInstructions(params.Open)
	orderType := verticalOrderType(params.LongStrike, params.ShortStrike, params.PutCall, params.Open)
	complexType := models.ComplexOrderStrategyTypeVertical

	order := &models.OrderRequest{
		Session:                  defaultSession(params.Session),
		Duration:                 defaultDuration(params.Duration),
		OrderType:                orderType,
		ComplexOrderStrategyType: &complexType,
		OrderStrategyType:        models.OrderStrategyTypeSingle,
		Price:                    float64Ptr(params.Price),
		OrderLegCollection: []models.OrderLegCollection{
			buildOptionLeg(&optionLegParams{
				Underlying: params.Underlying, Expiration: params.Expiration,
				Strike: params.LongStrike, PutCall: params.PutCall,
				Instruction: longInstruction, Quantity: params.Quantity,
			}),
			buildOptionLeg(&optionLegParams{
				Underlying: params.Underlying, Expiration: params.Expiration,
				Strike: params.ShortStrike, PutCall: params.PutCall,
				Instruction: shortInstruction, Quantity: params.Quantity,
			}),
		},
	}

	return order, nil
}

// verticalOrderType determines whether a vertical spread is a net debit or net
// credit based on the strike relationship and position direction.
//
// For calls: buying the lower strike costs more (debit when opening).
// For puts: buying the higher strike costs more (debit when opening).
// Closing reverses the direction.
func verticalOrderType(longStrike, shortStrike float64, putCall models.PutCall, isOpen bool) models.OrderType {
	var isDebitWhenOpening bool
	if putCall == models.PutCallCall {
		isDebitWhenOpening = longStrike < shortStrike
	} else {
		isDebitWhenOpening = longStrike > shortStrike
	}

	if isOpen == isDebitWhenOpening {
		return models.OrderTypeNetDebit
	}

	return models.OrderTypeNetCredit
}

// spreadInstructions returns the buy/sell instructions for the long and short
// legs based on whether the position is being opened or closed.
func spreadInstructions(isOpen bool) (longInstruction, shortInstruction models.Instruction) {
	if isOpen {
		return models.InstructionBuyToOpen, models.InstructionSellToOpen
	}

	return models.InstructionSellToClose, models.InstructionBuyToClose
}

// IronCondorParams holds parameters for building an iron condor order.
// An iron condor combines a bull put spread (credit) and a bear call spread (credit):
//
//	Leg 1: BUY put  at PutLongStrike   (lowest, protection)
//	Leg 2: SELL put  at PutShortStrike  (higher, premium)
//	Leg 3: SELL call at CallShortStrike (lower, premium)
//	Leg 4: BUY call  at CallLongStrike  (highest, protection)
type IronCondorParams struct {
	Underlying      string
	Expiration      time.Time
	PutLongStrike   float64 // Lowest strike: put being bought (protection)
	PutShortStrike  float64 // Put being sold (premium)
	CallShortStrike float64 // Call being sold (premium)
	CallLongStrike  float64 // Highest strike: call being bought (protection)
	Open            bool    // true = opening position, false = closing
	Quantity        float64
	Price           float64 // Net credit/debit amount (always positive)
	Duration        models.Duration
	Session         models.Session
}

// BuildIronCondorOrder constructs an OrderRequest for a four-leg iron condor.
//
// Opening is always NET_CREDIT (collecting premium on both spreads).
// Closing is always NET_DEBIT (buying back the position).
func BuildIronCondorOrder(params *IronCondorParams) (*models.OrderRequest, error) {
	longInstruction, shortInstruction := spreadInstructions(params.Open)
	complexType := models.ComplexOrderStrategyTypeIronCondor

	var orderType models.OrderType
	if params.Open {
		orderType = models.OrderTypeNetCredit
	} else {
		orderType = models.OrderTypeNetDebit
	}

	order := &models.OrderRequest{
		Session:                  defaultSession(params.Session),
		Duration:                 defaultDuration(params.Duration),
		OrderType:                orderType,
		ComplexOrderStrategyType: &complexType,
		OrderStrategyType:        models.OrderStrategyTypeSingle,
		Price:                    float64Ptr(params.Price),
		OrderLegCollection: []models.OrderLegCollection{
			buildOptionLeg(&optionLegParams{
				Underlying: params.Underlying, Expiration: params.Expiration,
				Strike: params.PutLongStrike, PutCall: models.PutCallPut,
				Instruction: longInstruction, Quantity: params.Quantity,
			}),
			buildOptionLeg(&optionLegParams{
				Underlying: params.Underlying, Expiration: params.Expiration,
				Strike: params.PutShortStrike, PutCall: models.PutCallPut,
				Instruction: shortInstruction, Quantity: params.Quantity,
			}),
			buildOptionLeg(&optionLegParams{
				Underlying: params.Underlying, Expiration: params.Expiration,
				Strike: params.CallShortStrike, PutCall: models.PutCallCall,
				Instruction: shortInstruction, Quantity: params.Quantity,
			}),
			buildOptionLeg(&optionLegParams{
				Underlying: params.Underlying, Expiration: params.Expiration,
				Strike: params.CallLongStrike, PutCall: models.PutCallCall,
				Instruction: longInstruction, Quantity: params.Quantity,
			}),
		},
	}

	return order, nil
}

// StraddleParams holds parameters for building a straddle order.
// A straddle buys (or sells) both a call and a put at the same strike.
type StraddleParams struct {
	Underlying string
	Expiration time.Time
	Strike     float64 // Single strike shared by both call and put legs
	Buy        bool    // true = buying the straddle (long), false = selling (short)
	Open       bool    // true = opening position, false = closing
	Quantity   float64
	Price      float64 // Net debit/credit amount (always positive)
	Duration   models.Duration
	Session    models.Session
}

// BuildStraddleOrder constructs an OrderRequest for a two-leg straddle.
//
// Both legs share the same strike and expiration but differ in put/call type.
// The order type is determined by the buy/sell direction:
//
//	Buy  -> NET_DEBIT  (paying to acquire both options)
//	Sell -> NET_CREDIT (collecting premium on both options)
func BuildStraddleOrder(params *StraddleParams) (*models.OrderRequest, error) {
	instruction := symmetricInstruction(params.Buy, params.Open)
	orderType := symmetricOrderType(params.Buy)
	complexType := models.ComplexOrderStrategyTypeStraddle

	order := &models.OrderRequest{
		Session:                  defaultSession(params.Session),
		Duration:                 defaultDuration(params.Duration),
		OrderType:                orderType,
		ComplexOrderStrategyType: &complexType,
		OrderStrategyType:        models.OrderStrategyTypeSingle,
		Price:                    float64Ptr(params.Price),
		OrderLegCollection: []models.OrderLegCollection{
			buildOptionLeg(&optionLegParams{
				Underlying: params.Underlying, Expiration: params.Expiration,
				Strike: params.Strike, PutCall: models.PutCallCall,
				Instruction: instruction, Quantity: params.Quantity,
			}),
			buildOptionLeg(&optionLegParams{
				Underlying: params.Underlying, Expiration: params.Expiration,
				Strike: params.Strike, PutCall: models.PutCallPut,
				Instruction: instruction, Quantity: params.Quantity,
			}),
		},
	}

	return order, nil
}

// StrangleParams holds parameters for building a strangle order.
// A strangle buys (or sells) a call and a put at different strikes.
type StrangleParams struct {
	Underlying string
	Expiration time.Time
	CallStrike float64 // Strike price for the call leg
	PutStrike  float64 // Strike price for the put leg
	Buy        bool    // true = buying the strangle (long), false = selling (short)
	Open       bool    // true = opening position, false = closing
	Quantity   float64
	Price      float64 // Net debit/credit amount (always positive)
	Duration   models.Duration
	Session    models.Session
}

// BuildStrangleOrder constructs an OrderRequest for a two-leg strangle.
//
// A strangle uses different strikes for the call and put legs (unlike a straddle
// which shares one strike). The order type follows the same buy/sell logic:
//
//	Buy  -> NET_DEBIT  (paying to acquire both options)
//	Sell -> NET_CREDIT (collecting premium on both options)
func BuildStrangleOrder(params *StrangleParams) (*models.OrderRequest, error) {
	instruction := symmetricInstruction(params.Buy, params.Open)
	orderType := symmetricOrderType(params.Buy)
	complexType := models.ComplexOrderStrategyTypeStrangle

	order := &models.OrderRequest{
		Session:                  defaultSession(params.Session),
		Duration:                 defaultDuration(params.Duration),
		OrderType:                orderType,
		ComplexOrderStrategyType: &complexType,
		OrderStrategyType:        models.OrderStrategyTypeSingle,
		Price:                    float64Ptr(params.Price),
		OrderLegCollection: []models.OrderLegCollection{
			buildOptionLeg(&optionLegParams{
				Underlying: params.Underlying, Expiration: params.Expiration,
				Strike: params.CallStrike, PutCall: models.PutCallCall,
				Instruction: instruction, Quantity: params.Quantity,
			}),
			buildOptionLeg(&optionLegParams{
				Underlying: params.Underlying, Expiration: params.Expiration,
				Strike: params.PutStrike, PutCall: models.PutCallPut,
				Instruction: instruction, Quantity: params.Quantity,
			}),
		},
	}

	return order, nil
}

// symmetricInstruction returns the instruction for strategies where both legs
// share the same direction (straddle, strangle). Combines buy/sell with open/close
// to produce the correct TO_OPEN or TO_CLOSE instruction.
func symmetricInstruction(isBuy, isOpen bool) models.Instruction {
	if isBuy {
		if isOpen {
			return models.InstructionBuyToOpen
		}

		return models.InstructionBuyToClose
	}

	if isOpen {
		return models.InstructionSellToOpen
	}

	return models.InstructionSellToClose
}

// symmetricOrderType returns NET_DEBIT for buying or NET_CREDIT for selling.
// Used by strategies where both legs move in the same direction (straddle, strangle).
func symmetricOrderType(isBuy bool) models.OrderType {
	if isBuy {
		return models.OrderTypeNetDebit
	}

	return models.OrderTypeNetCredit
}

// CoveredCallParams holds parameters for building a covered call order.
// A covered call buys shares and simultaneously sells a call against them.
// The equity quantity is derived from the option quantity (contracts * 100).
type CoveredCallParams struct {
	Underlying string
	Expiration time.Time
	Strike     float64 // Strike price of the call being sold
	Quantity   float64 // Number of option contracts (equity shares = quantity * 100)
	Price      float64 // Net debit amount (share cost minus call premium received)
	Duration   models.Duration
	Session    models.Session
}

// BuildCoveredCallOrder constructs an OrderRequest for a covered call.
//
// A covered call combines buying shares with selling a call option against them.
// The order is always NET_DEBIT since the equity purchase far exceeds the call
// premium received. Schwab treats this as a COVERED complex order strategy.
//
// Leg 1: BUY equity shares (quantity * 100)
// Leg 2: SELL_TO_OPEN call option (quantity contracts)
func BuildCoveredCallOrder(params *CoveredCallParams) (*models.OrderRequest, error) {
	complexType := models.ComplexOrderStrategyTypeCovered

	order := &models.OrderRequest{
		Session:                  defaultSession(params.Session),
		Duration:                 defaultDuration(params.Duration),
		OrderType:                models.OrderTypeNetDebit,
		ComplexOrderStrategyType: &complexType,
		OrderStrategyType:        models.OrderStrategyTypeSingle,
		Price:                    float64Ptr(params.Price),
		OrderLegCollection: []models.OrderLegCollection{
			buildEquityLeg(params.Underlying, models.InstructionBuy, params.Quantity*optionContractMultiplier),
			buildOptionLeg(&optionLegParams{
				Underlying: params.Underlying, Expiration: params.Expiration,
				Strike: params.Strike, PutCall: models.PutCallCall,
				Instruction: models.InstructionSellToOpen, Quantity: params.Quantity,
			}),
		},
	}

	return order, nil
}

// optionLegParams holds the parameters for constructing a single option leg.
// Using a struct instead of positional args prevents accidental swaps between
// the two float64 fields (Strike and Quantity).
type optionLegParams struct {
	Underlying  string
	Expiration  time.Time
	Strike      float64
	PutCall     models.PutCall
	Instruction models.Instruction
	Quantity    float64
}

// buildOptionLeg constructs a single option leg with full OCC symbol and instrument
// metadata. This helper is shared across all multi-leg option strategies.
func buildOptionLeg(p *optionLegParams) models.OrderLegCollection {
	expirationStr := p.Expiration.Format("2006-01-02")
	multiplier := optionContractMultiplier

	return models.OrderLegCollection{
		Instruction: p.Instruction,
		Quantity:    p.Quantity,
		Instrument: models.OrderInstrument{
			AssetType:            models.AssetTypeOption,
			Symbol:               BuildOCCSymbol(p.Underlying, p.Expiration, p.Strike, string(p.PutCall)),
			PutCall:              &p.PutCall,
			UnderlyingSymbol:     &p.Underlying,
			OptionMultiplier:     &multiplier,
			OptionExpirationDate: &expirationStr,
			OptionStrikePrice:    &p.Strike,
		},
	}
}
