package orderbuilder

import (
	"time"

	"github.com/major/schwab-agent/internal/models"
)

// JadeLizardParams holds parameters for building a jade lizard order.
//
// A jade lizard sells a put and a short call spread (short call + long call
// at a higher strike) for a net credit. The long call provides upside protection,
// limiting the maximum loss on the call side to the width of the call spread
// minus the total credit received.
//
// Strike ordering: PutStrike < ShortCallStrike < LongCallStrike.
type JadeLizardParams struct {
	Underlying      string
	Expiration      time.Time
	PutStrike       float64 // Strike of the short put (lowest)
	ShortCallStrike float64 // Strike of the short call (middle)
	LongCallStrike  float64 // Strike of the long call (highest, protection)
	Quantity        float64
	Price           float64 // Net credit amount received
	Duration        models.Duration
	Session         models.Session
}

// BuildJadeLizardOrder constructs an OrderRequest for a three-leg jade lizard.
//
// A jade lizard combines a short put with a bear call spread (short call + long
// call). All legs open simultaneously for a net credit. Schwab has no dedicated
// complex order strategy type for jade lizards, so CUSTOM is used.
//
//	Leg 1: SELL_TO_OPEN put at PutStrike (premium collection).
//	Leg 2: SELL_TO_OPEN call at ShortCallStrike (premium collection).
//	Leg 3: BUY_TO_OPEN call at LongCallStrike (upside protection).
func BuildJadeLizardOrder(params *JadeLizardParams) (*models.OrderRequest, error) {
	complexType := models.ComplexOrderStrategyTypeCustom

	legs := buildOptionLegs(
		&optionLegParams{
			Underlying:  params.Underlying,
			Expiration:  params.Expiration,
			Strike:      params.PutStrike,
			PutCall:     models.PutCallPut,
			Instruction: models.InstructionSellToOpen,
			Quantity:    params.Quantity,
		},
		&optionLegParams{
			Underlying:  params.Underlying,
			Expiration:  params.Expiration,
			Strike:      params.ShortCallStrike,
			PutCall:     models.PutCallCall,
			Instruction: models.InstructionSellToOpen,
			Quantity:    params.Quantity,
		},
		&optionLegParams{
			Underlying:  params.Underlying,
			Expiration:  params.Expiration,
			Strike:      params.LongCallStrike,
			PutCall:     models.PutCallCall,
			Instruction: models.InstructionBuyToOpen,
			Quantity:    params.Quantity,
		},
	)

	return buildSpreadOrder(
		params.Session, params.Duration,
		models.OrderTypeNetCredit, complexType,
		params.Price, legs,
	), nil
}
