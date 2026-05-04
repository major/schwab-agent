package orderbuilder

import (
	"cmp"
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
		Session:                  cmp.Or(params.Session, models.SessionNormal),
		Duration:                 cmp.Or(params.Duration, models.DurationDay),
		OrderType:                orderType,
		ComplexOrderStrategyType: &complexType,
		OrderStrategyType:        models.OrderStrategyTypeSingle,
		Price:                    new(params.Price),
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
		Session:                  cmp.Or(params.Session, models.SessionNormal),
		Duration:                 cmp.Or(params.Duration, models.DurationDay),
		OrderType:                orderType,
		ComplexOrderStrategyType: &complexType,
		OrderStrategyType:        models.OrderStrategyTypeSingle,
		Price:                    new(params.Price),
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

// ButterflyParams holds parameters for building a same-expiration butterfly.
// A long butterfly buys the two wings and sells two contracts at the body strike.
type ButterflyParams struct {
	Underlying   string
	Expiration   time.Time
	LowerStrike  float64
	MiddleStrike float64
	UpperStrike  float64
	PutCall      models.PutCall
	Buy          bool // true = long butterfly, false = short butterfly
	Open         bool // true = opening position, false = closing
	Quantity     float64
	Price        float64
	Duration     models.Duration
	Session      models.Session
}

// BuildButterflyOrder constructs an OrderRequest for a three-strike butterfly.
func BuildButterflyOrder(params *ButterflyParams) (*models.OrderRequest, error) {
	wingInstruction, bodyInstruction := directionalSpreadInstructions(params.Buy, params.Open)
	complexType := models.ComplexOrderStrategyTypeButterfly

	order := &models.OrderRequest{
		Session:                  cmp.Or(params.Session, models.SessionNormal),
		Duration:                 cmp.Or(params.Duration, models.DurationDay),
		OrderType:                directionalSpreadOrderType(params.Buy, params.Open),
		ComplexOrderStrategyType: &complexType,
		OrderStrategyType:        models.OrderStrategyTypeSingle,
		Price:                    new(params.Price),
		OrderLegCollection: []models.OrderLegCollection{
			buildOptionLeg(&optionLegParams{
				Underlying: params.Underlying, Expiration: params.Expiration,
				Strike: params.LowerStrike, PutCall: params.PutCall,
				Instruction: wingInstruction, Quantity: params.Quantity,
			}),
			buildOptionLeg(&optionLegParams{
				Underlying: params.Underlying, Expiration: params.Expiration,
				Strike: params.MiddleStrike, PutCall: params.PutCall,
				Instruction: bodyInstruction, Quantity: params.Quantity * 2,
			}),
			buildOptionLeg(&optionLegParams{
				Underlying: params.Underlying, Expiration: params.Expiration,
				Strike: params.UpperStrike, PutCall: params.PutCall,
				Instruction: wingInstruction, Quantity: params.Quantity,
			}),
		},
	}

	return order, nil
}

// CondorParams holds parameters for building a same-expiration condor.
// A long condor buys the outside wings and sells the two middle strikes.
type CondorParams struct {
	Underlying        string
	Expiration        time.Time
	LowerStrike       float64
	LowerMiddleStrike float64
	UpperMiddleStrike float64
	UpperStrike       float64
	PutCall           models.PutCall
	Buy               bool // true = long condor, false = short condor
	Open              bool // true = opening position, false = closing
	Quantity          float64
	Price             float64
	Duration          models.Duration
	Session           models.Session
}

// BuildCondorOrder constructs an OrderRequest for a four-strike condor.
func BuildCondorOrder(params *CondorParams) (*models.OrderRequest, error) {
	wingInstruction, middleInstruction := directionalSpreadInstructions(params.Buy, params.Open)
	complexType := models.ComplexOrderStrategyTypeCondor

	order := &models.OrderRequest{
		Session:                  cmp.Or(params.Session, models.SessionNormal),
		Duration:                 cmp.Or(params.Duration, models.DurationDay),
		OrderType:                directionalSpreadOrderType(params.Buy, params.Open),
		ComplexOrderStrategyType: &complexType,
		OrderStrategyType:        models.OrderStrategyTypeSingle,
		Price:                    new(params.Price),
		OrderLegCollection: []models.OrderLegCollection{
			buildOptionLeg(&optionLegParams{
				Underlying: params.Underlying, Expiration: params.Expiration,
				Strike: params.LowerStrike, PutCall: params.PutCall,
				Instruction: wingInstruction, Quantity: params.Quantity,
			}),
			buildOptionLeg(&optionLegParams{
				Underlying: params.Underlying, Expiration: params.Expiration,
				Strike: params.LowerMiddleStrike, PutCall: params.PutCall,
				Instruction: middleInstruction, Quantity: params.Quantity,
			}),
			buildOptionLeg(&optionLegParams{
				Underlying: params.Underlying, Expiration: params.Expiration,
				Strike: params.UpperMiddleStrike, PutCall: params.PutCall,
				Instruction: middleInstruction, Quantity: params.Quantity,
			}),
			buildOptionLeg(&optionLegParams{
				Underlying: params.Underlying, Expiration: params.Expiration,
				Strike: params.UpperStrike, PutCall: params.PutCall,
				Instruction: wingInstruction, Quantity: params.Quantity,
			}),
		},
	}

	return order, nil
}

// BackRatioParams holds parameters for building an option back-ratio spread.
// Quantity is the short-leg size; LongRatio controls long contracts per short.
type BackRatioParams struct {
	Underlying  string
	Expiration  time.Time
	ShortStrike float64
	LongStrike  float64
	PutCall     models.PutCall
	Open        bool
	Quantity    float64
	LongRatio   float64
	Credit      bool
	Price       float64
	Duration    models.Duration
	Session     models.Session
}

// BuildBackRatioOrder constructs an OrderRequest for a back-ratio spread.
func BuildBackRatioOrder(params *BackRatioParams) (*models.OrderRequest, error) {
	longInstruction, shortInstruction := spreadInstructions(params.Open)
	complexType := models.ComplexOrderStrategyTypeBackRatio

	orderType := models.OrderTypeNetDebit
	if params.Credit {
		orderType = models.OrderTypeNetCredit
	}

	order := &models.OrderRequest{
		Session:                  cmp.Or(params.Session, models.SessionNormal),
		Duration:                 cmp.Or(params.Duration, models.DurationDay),
		OrderType:                orderType,
		ComplexOrderStrategyType: &complexType,
		OrderStrategyType:        models.OrderStrategyTypeSingle,
		Price:                    new(params.Price),
		OrderLegCollection: []models.OrderLegCollection{
			buildOptionLeg(&optionLegParams{
				Underlying: params.Underlying, Expiration: params.Expiration,
				Strike: params.ShortStrike, PutCall: params.PutCall,
				Instruction: shortInstruction, Quantity: params.Quantity,
			}),
			buildOptionLeg(&optionLegParams{
				Underlying: params.Underlying, Expiration: params.Expiration,
				Strike: params.LongStrike, PutCall: params.PutCall,
				Instruction: longInstruction, Quantity: params.Quantity * params.LongRatio,
			}),
		},
	}

	return order, nil
}

// VerticalRollParams holds parameters for closing one vertical and opening another.
type VerticalRollParams struct {
	Underlying       string
	CloseExpiration  time.Time
	OpenExpiration   time.Time
	CloseLongStrike  float64
	CloseShortStrike float64
	OpenLongStrike   float64
	OpenShortStrike  float64
	PutCall          models.PutCall
	Credit           bool
	Quantity         float64
	Price            float64
	Duration         models.Duration
	Session          models.Session
}

// BuildVerticalRollOrder constructs an OrderRequest that closes and opens vertical spreads.
func BuildVerticalRollOrder(params *VerticalRollParams) (*models.OrderRequest, error) {
	complexType := models.ComplexOrderStrategyTypeVerticalRoll

	orderType := models.OrderTypeNetDebit
	if params.Credit {
		orderType = models.OrderTypeNetCredit
	}

	order := &models.OrderRequest{
		Session:                  cmp.Or(params.Session, models.SessionNormal),
		Duration:                 cmp.Or(params.Duration, models.DurationDay),
		OrderType:                orderType,
		ComplexOrderStrategyType: &complexType,
		OrderStrategyType:        models.OrderStrategyTypeSingle,
		Price:                    new(params.Price),
		OrderLegCollection: []models.OrderLegCollection{
			buildOptionLeg(&optionLegParams{
				Underlying: params.Underlying, Expiration: params.CloseExpiration,
				Strike: params.CloseLongStrike, PutCall: params.PutCall,
				Instruction: models.InstructionSellToClose, Quantity: params.Quantity,
			}),
			buildOptionLeg(&optionLegParams{
				Underlying: params.Underlying, Expiration: params.CloseExpiration,
				Strike: params.CloseShortStrike, PutCall: params.PutCall,
				Instruction: models.InstructionBuyToClose, Quantity: params.Quantity,
			}),
			buildOptionLeg(&optionLegParams{
				Underlying: params.Underlying, Expiration: params.OpenExpiration,
				Strike: params.OpenLongStrike, PutCall: params.PutCall,
				Instruction: models.InstructionBuyToOpen, Quantity: params.Quantity,
			}),
			buildOptionLeg(&optionLegParams{
				Underlying: params.Underlying, Expiration: params.OpenExpiration,
				Strike: params.OpenShortStrike, PutCall: params.PutCall,
				Instruction: models.InstructionSellToOpen, Quantity: params.Quantity,
			}),
		},
	}

	return order, nil
}

// DoubleDiagonalParams holds parameters for building a double diagonal spread.
type DoubleDiagonalParams struct {
	Underlying     string
	NearExpiration time.Time
	FarExpiration  time.Time
	PutFarStrike   float64
	PutNearStrike  float64
	CallNearStrike float64
	CallFarStrike  float64
	Open           bool
	Quantity       float64
	Price          float64
	Duration       models.Duration
	Session        models.Session
}

// BuildDoubleDiagonalOrder constructs an OrderRequest for a put and call diagonal pair.
func BuildDoubleDiagonalOrder(params *DoubleDiagonalParams) (*models.OrderRequest, error) {
	longInstruction, shortInstruction := spreadInstructions(params.Open)
	complexType := models.ComplexOrderStrategyTypeDoubleDiagonal

	orderType := models.OrderTypeNetDebit
	if !params.Open {
		orderType = models.OrderTypeNetCredit
	}

	order := &models.OrderRequest{
		Session:                  cmp.Or(params.Session, models.SessionNormal),
		Duration:                 cmp.Or(params.Duration, models.DurationDay),
		OrderType:                orderType,
		ComplexOrderStrategyType: &complexType,
		OrderStrategyType:        models.OrderStrategyTypeSingle,
		Price:                    new(params.Price),
		OrderLegCollection: []models.OrderLegCollection{
			buildOptionLeg(&optionLegParams{
				Underlying: params.Underlying, Expiration: params.FarExpiration,
				Strike: params.PutFarStrike, PutCall: models.PutCallPut,
				Instruction: longInstruction, Quantity: params.Quantity,
			}),
			buildOptionLeg(&optionLegParams{
				Underlying: params.Underlying, Expiration: params.NearExpiration,
				Strike: params.PutNearStrike, PutCall: models.PutCallPut,
				Instruction: shortInstruction, Quantity: params.Quantity,
			}),
			buildOptionLeg(&optionLegParams{
				Underlying: params.Underlying, Expiration: params.NearExpiration,
				Strike: params.CallNearStrike, PutCall: models.PutCallCall,
				Instruction: shortInstruction, Quantity: params.Quantity,
			}),
			buildOptionLeg(&optionLegParams{
				Underlying: params.Underlying, Expiration: params.FarExpiration,
				Strike: params.CallFarStrike, PutCall: models.PutCallCall,
				Instruction: longInstruction, Quantity: params.Quantity,
			}),
		},
	}

	return order, nil
}

// directionalSpreadInstructions returns outside and inside leg instructions for
// strategies whose structure can be bought or sold, then opened or closed.
func directionalSpreadInstructions(isBuy, isOpen bool) (outerInstruction, innerInstruction models.Instruction) {
	if isBuy {
		return spreadInstructions(isOpen)
	}

	innerInstruction, outerInstruction = spreadInstructions(isOpen)
	return outerInstruction, innerInstruction
}

// directionalSpreadOrderType maps buy/sell plus open/close into net debit/credit.
func directionalSpreadOrderType(isBuy, isOpen bool) models.OrderType {
	if isBuy == isOpen {
		return models.OrderTypeNetDebit
	}

	return models.OrderTypeNetCredit
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
		Session:                  cmp.Or(params.Session, models.SessionNormal),
		Duration:                 cmp.Or(params.Duration, models.DurationDay),
		OrderType:                orderType,
		ComplexOrderStrategyType: &complexType,
		OrderStrategyType:        models.OrderStrategyTypeSingle,
		Price:                    new(params.Price),
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
		Session:                  cmp.Or(params.Session, models.SessionNormal),
		Duration:                 cmp.Or(params.Duration, models.DurationDay),
		OrderType:                orderType,
		ComplexOrderStrategyType: &complexType,
		OrderStrategyType:        models.OrderStrategyTypeSingle,
		Price:                    new(params.Price),
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
		Session:                  cmp.Or(params.Session, models.SessionNormal),
		Duration:                 cmp.Or(params.Duration, models.DurationDay),
		OrderType:                models.OrderTypeNetDebit,
		ComplexOrderStrategyType: &complexType,
		OrderStrategyType:        models.OrderStrategyTypeSingle,
		Price:                    new(params.Price),
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

// CalendarParams holds parameters for building a calendar spread order.
// A calendar spread buys and sells the same option at the same strike but with
// different expirations. The far-dated option is bought and the near-dated
// option is sold when opening (long calendar).
type CalendarParams struct {
	Underlying     string
	NearExpiration time.Time // Near-term expiration (sold leg when opening)
	FarExpiration  time.Time // Far-term expiration (bought leg when opening)
	Strike         float64   // Strike price shared by both legs
	PutCall        models.PutCall
	Open           bool // true = opening position, false = closing
	Quantity       float64
	Price          float64 // Net debit (open) or credit (close) amount (always positive)
	Duration       models.Duration
	Session        models.Session
}

// BuildCalendarOrder constructs an OrderRequest for a two-leg calendar spread.
//
// A long calendar spread profits from time decay: the near-term option decays
// faster than the far-term option. Opening is NET_DEBIT because the
// far-dated option costs more than the near-dated one. Closing is NET_CREDIT
// because the position is being unwound.
//
//	Open:  BUY_TO_OPEN far + SELL_TO_OPEN near (NET_DEBIT)
//	Close: SELL_TO_CLOSE far + BUY_TO_CLOSE near (NET_CREDIT)
func BuildCalendarOrder(params *CalendarParams) (*models.OrderRequest, error) {
	longInstruction, shortInstruction := spreadInstructions(params.Open)
	complexType := models.ComplexOrderStrategyTypeCalendar

	orderType := models.OrderTypeNetDebit
	if !params.Open {
		orderType = models.OrderTypeNetCredit
	}

	order := &models.OrderRequest{
		Session:                  cmp.Or(params.Session, models.SessionNormal),
		Duration:                 cmp.Or(params.Duration, models.DurationDay),
		OrderType:                orderType,
		ComplexOrderStrategyType: &complexType,
		OrderStrategyType:        models.OrderStrategyTypeSingle,
		Price:                    new(params.Price),
		OrderLegCollection: []models.OrderLegCollection{
			// Far leg (bought when opening) listed first for consistency
			// with how Schwab displays calendar spreads.
			buildOptionLeg(&optionLegParams{
				Underlying: params.Underlying, Expiration: params.FarExpiration,
				Strike: params.Strike, PutCall: params.PutCall,
				Instruction: longInstruction, Quantity: params.Quantity,
			}),
			buildOptionLeg(&optionLegParams{
				Underlying: params.Underlying, Expiration: params.NearExpiration,
				Strike: params.Strike, PutCall: params.PutCall,
				Instruction: shortInstruction, Quantity: params.Quantity,
			}),
		},
	}

	return order, nil
}

// DiagonalParams holds parameters for building a diagonal spread order.
// A diagonal spread combines different strikes AND different expirations.
// Like a calendar, the far-dated option is bought and the near-dated is sold
// when opening, but the strikes differ between legs.
type DiagonalParams struct {
	Underlying     string
	NearExpiration time.Time // Near-term expiration (sold leg when opening)
	FarExpiration  time.Time // Far-term expiration (bought leg when opening)
	NearStrike     float64   // Strike price for the near-term (sold) leg
	FarStrike      float64   // Strike price for the far-term (bought) leg
	PutCall        models.PutCall
	Open           bool // true = opening position, false = closing
	Quantity       float64
	Price          float64 // Net debit (open) or credit (close) amount (always positive)
	Duration       models.Duration
	Session        models.Session
}

// BuildDiagonalOrder constructs an OrderRequest for a two-leg diagonal spread.
//
// A diagonal combines the time spread of a calendar with the strike spread of
// a vertical. Opening is NET_DEBIT because the far-dated option costs
// more than the near-dated one (like a calendar). Closing is NET_CREDIT
// because the position is being unwound.
//
//	Open:  BUY_TO_OPEN far (FarStrike) + SELL_TO_OPEN near (NearStrike) (NET_DEBIT)
//	Close: SELL_TO_CLOSE far (FarStrike) + BUY_TO_CLOSE near (NearStrike) (NET_CREDIT)
func BuildDiagonalOrder(params *DiagonalParams) (*models.OrderRequest, error) {
	longInstruction, shortInstruction := spreadInstructions(params.Open)
	complexType := models.ComplexOrderStrategyTypeDiagonal

	orderType := models.OrderTypeNetDebit
	if !params.Open {
		orderType = models.OrderTypeNetCredit
	}

	order := &models.OrderRequest{
		Session:                  cmp.Or(params.Session, models.SessionNormal),
		Duration:                 cmp.Or(params.Duration, models.DurationDay),
		OrderType:                orderType,
		ComplexOrderStrategyType: &complexType,
		OrderStrategyType:        models.OrderStrategyTypeSingle,
		Price:                    new(params.Price),
		OrderLegCollection: []models.OrderLegCollection{
			// Far leg (bought when opening) listed first.
			buildOptionLeg(&optionLegParams{
				Underlying: params.Underlying, Expiration: params.FarExpiration,
				Strike: params.FarStrike, PutCall: params.PutCall,
				Instruction: longInstruction, Quantity: params.Quantity,
			}),
			buildOptionLeg(&optionLegParams{
				Underlying: params.Underlying, Expiration: params.NearExpiration,
				Strike: params.NearStrike, PutCall: params.PutCall,
				Instruction: shortInstruction, Quantity: params.Quantity,
			}),
		},
	}

	return order, nil
}

// CollarParams holds parameters for building a collar-with-stock order.
// A collar combines buying shares, buying a protective put, and selling a
// covered call at the same expiration. The equity quantity is derived from
// the option quantity (contracts * 100), just like a covered call.
type CollarParams struct {
	Underlying string
	PutStrike  float64   // Strike price of the protective put
	CallStrike float64   // Strike price of the covered call
	Expiration time.Time // Shared expiration for both option legs
	Quantity   float64   // Number of option contracts (equity shares = quantity * 100)
	Open       bool      // true = opening position, false = closing
	Price      float64   // Net debit (open) or credit (close) amount
	Duration   models.Duration
	Session    models.Session
}

// BuildCollarOrder constructs an OrderRequest for a collar-with-stock.
//
// A collar is a covered call with downside protection: buy shares, sell a call
// for income, and buy a put for protection. All three legs use the same
// underlying and the same expiration. Opening is NET_DEBIT, closing is NET_CREDIT.
//
// Opening:
//
//	Leg 1: BUY equity shares (quantity * 100)
//	Leg 2: BUY_TO_OPEN put option (protective put)
//	Leg 3: SELL_TO_OPEN call option (covered call)
//	Order type: NET_DEBIT
//
// Closing reverses all instructions:
//
//	Leg 1: SELL equity shares
//	Leg 2: SELL_TO_CLOSE put option
//	Leg 3: BUY_TO_CLOSE call option
//	Order type: NET_CREDIT
func BuildCollarOrder(params *CollarParams) (*models.OrderRequest, error) {
	longInstruction, shortInstruction := spreadInstructions(params.Open)
	complexType := models.ComplexOrderStrategyTypeCollarWithStock

	// Equity instruction: BUY when opening, SELL when closing.
	equityInstruction := models.InstructionBuy
	if !params.Open {
		equityInstruction = models.InstructionSell
	}

	orderType := models.OrderTypeNetDebit
	if !params.Open {
		orderType = models.OrderTypeNetCredit
	}

	order := &models.OrderRequest{
		Session:                  cmp.Or(params.Session, models.SessionNormal),
		Duration:                 cmp.Or(params.Duration, models.DurationDay),
		OrderType:                orderType,
		ComplexOrderStrategyType: &complexType,
		OrderStrategyType:        models.OrderStrategyTypeSingle,
		Price:                    new(params.Price),
		OrderLegCollection: []models.OrderLegCollection{
			// Leg 1: Equity shares.
			buildEquityLeg(params.Underlying, equityInstruction, params.Quantity*optionContractMultiplier),
			// Leg 2: Protective put (same direction as equity when opening).
			buildOptionLeg(&optionLegParams{
				Underlying: params.Underlying, Expiration: params.Expiration,
				Strike: params.PutStrike, PutCall: models.PutCallPut,
				Instruction: longInstruction, Quantity: params.Quantity,
			}),
			// Leg 3: Covered call (opposite direction when opening).
			buildOptionLeg(&optionLegParams{
				Underlying: params.Underlying, Expiration: params.Expiration,
				Strike: params.CallStrike, PutCall: models.PutCallCall,
				Instruction: shortInstruction, Quantity: params.Quantity,
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
