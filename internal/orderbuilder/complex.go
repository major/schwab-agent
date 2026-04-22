// Package orderbuilder constructs Schwab order payloads.
package orderbuilder

import (
	"cmp"

	"github.com/major/schwab-agent/internal/models"
)

// OCOParams holds parameters for building a standalone one-cancels-other order.
// Unlike bracket orders, OCO orders have no entry leg - they attach exit
// conditions (take-profit and/or stop-loss) to an existing position.
type OCOParams struct {
	Symbol     string
	Action     models.Instruction
	Quantity   float64
	TakeProfit float64
	StopLoss   float64
	Duration   models.Duration
	Session    models.Session
}

// BracketParams holds parameters for building a bracket order.
type BracketParams struct {
	Symbol     string
	Action     models.Instruction
	Quantity   float64
	OrderType  models.OrderType
	Price      float64
	TakeProfit float64
	StopLoss   float64
	Duration   models.Duration
	Session    models.Session
}

// BuildBracketOrder constructs a triggered order with exit conditions.
//
// Supports three modes depending on which exit prices are provided:
//   - Both take-profit and stop-loss: TRIGGER → OCO → 2 SINGLE (full bracket)
//   - Stop-loss only: TRIGGER → SINGLE stop order (entry with protective stop)
//   - Take-profit only: TRIGGER → SINGLE limit order (entry with profit target)
func BuildBracketOrder(params *BracketParams) (*models.OrderRequest, error) {
	if err := ValidateBracketOrder(params); err != nil {
		return nil, err
	}

	applyBracketDefaults(params)
	exitInstruction := bracketExitInstruction(params.Action)

	order := &models.OrderRequest{
		Session:           params.Session,
		Duration:          params.Duration,
		OrderType:         params.OrderType,
		OrderStrategyType: models.OrderStrategyTypeTrigger,
		OrderLegCollection: []models.OrderLegCollection{
			buildEquityLeg(params.Symbol, params.Action, params.Quantity),
		},
		ChildOrderStrategies: buildExitStrategies(params, exitInstruction),
	}

	if params.OrderType == models.OrderTypeLimit || params.OrderType == models.OrderTypeStopLimit {
		order.Price = ptr(params.Price)
	}

	if params.OrderType == models.OrderTypeStop || params.OrderType == models.OrderTypeStopLimit {
		order.StopPrice = ptr(params.Price)
	}

	return order, nil
}

// BuildOCOOrder constructs a standalone one-cancels-other order for exiting
// an existing position. No entry leg is created.
//
// Supports three modes depending on which exit prices are provided:
//   - Both take-profit and stop-loss: OCO with 2 SINGLE children
//   - Stop-loss only: standalone SINGLE stop order
//   - Take-profit only: standalone SINGLE limit order
//
// When only one exit is provided, the result is a plain SINGLE order rather
// than an OCO wrapper. Schwab requires OCO nodes to have exactly 2 children.
func BuildOCOOrder(params *OCOParams) (*models.OrderRequest, error) {
	if err := ValidateOCOOrder(params); err != nil {
		return nil, err
	}

	applyOCODefaults(params)

	hasTakeProfit := params.TakeProfit > 0
	hasStopLoss := params.StopLoss > 0

	// Full OCO: two exit orders, whichever fills first cancels the other.
	if hasTakeProfit && hasStopLoss {
		return &models.OrderRequest{
			OrderStrategyType: models.OrderStrategyTypeOCO,
			ChildOrderStrategies: []models.OrderRequest{
				buildOCOLimitExit(params),
				buildOCOStopExit(params),
			},
		}, nil
	}

	// Single exit: no OCO wrapper needed.
	if hasStopLoss {
		order := buildOCOStopExit(params)
		return &order, nil
	}

	order := buildOCOLimitExit(params)
	return &order, nil
}

// buildOCOLimitExit constructs a LIMIT exit order at the take-profit price.
func buildOCOLimitExit(params *OCOParams) models.OrderRequest {
	return models.OrderRequest{
		Session:           params.Session,
		Duration:          params.Duration,
		OrderType:         models.OrderTypeLimit,
		OrderStrategyType: models.OrderStrategyTypeSingle,
		Price:             ptr(params.TakeProfit),
		OrderLegCollection: []models.OrderLegCollection{
			buildEquityLeg(params.Symbol, params.Action, params.Quantity),
		},
	}
}

// buildOCOStopExit constructs a STOP exit order at the stop-loss price.
func buildOCOStopExit(params *OCOParams) models.OrderRequest {
	return models.OrderRequest{
		Session:           params.Session,
		Duration:          params.Duration,
		OrderType:         models.OrderTypeStop,
		OrderStrategyType: models.OrderStrategyTypeSingle,
		StopPrice:         ptr(params.StopLoss),
		OrderLegCollection: []models.OrderLegCollection{
			buildEquityLeg(params.Symbol, params.Action, params.Quantity),
		},
	}
}

// applyOCODefaults fills in Schwab's standard session and duration defaults.
func applyOCODefaults(params *OCOParams) {
	params.Duration = cmp.Or(params.Duration, models.DurationDay)
	params.Session = cmp.Or(params.Session, models.SessionNormal)
}

// buildExitStrategies constructs the child order strategies based on which exit
// conditions are provided. Two exits use an OCO wrapper; a single exit is a
// direct SINGLE child of the TRIGGER parent.
func buildExitStrategies(params *BracketParams, exitInstruction models.Instruction) []models.OrderRequest {
	hasTakeProfit := params.TakeProfit > 0
	hasStopLoss := params.StopLoss > 0

	// Full bracket: TRIGGER → OCO → 2 SINGLE.
	// Schwab requires OCO nodes to have exactly 2 child orders.
	if hasTakeProfit && hasStopLoss {
		return []models.OrderRequest{
			{
				OrderStrategyType: models.OrderStrategyTypeOCO,
				ChildOrderStrategies: []models.OrderRequest{
					buildTakeProfitOrder(params, exitInstruction),
					buildStopLossOrder(params, exitInstruction),
				},
			},
		}
	}

	// Single exit: TRIGGER → SINGLE (no OCO wrapper needed).
	if hasStopLoss {
		return []models.OrderRequest{buildStopLossOrder(params, exitInstruction)}
	}

	return []models.OrderRequest{buildTakeProfitOrder(params, exitInstruction)}
}

// buildTakeProfitOrder constructs a LIMIT SELL exit order at the take-profit price.
func buildTakeProfitOrder(params *BracketParams, exitInstruction models.Instruction) models.OrderRequest {
	return models.OrderRequest{
		Session:           params.Session,
		Duration:          params.Duration,
		OrderType:         models.OrderTypeLimit,
		OrderStrategyType: models.OrderStrategyTypeSingle,
		Price:             ptr(params.TakeProfit),
		OrderLegCollection: []models.OrderLegCollection{
			buildEquityLeg(params.Symbol, exitInstruction, params.Quantity),
		},
	}
}

// buildStopLossOrder constructs a STOP SELL exit order at the stop-loss price.
func buildStopLossOrder(params *BracketParams, exitInstruction models.Instruction) models.OrderRequest {
	return models.OrderRequest{
		Session:           params.Session,
		Duration:          params.Duration,
		OrderType:         models.OrderTypeStop,
		OrderStrategyType: models.OrderStrategyTypeSingle,
		StopPrice:         ptr(params.StopLoss),
		OrderLegCollection: []models.OrderLegCollection{
			buildEquityLeg(params.Symbol, exitInstruction, params.Quantity),
		},
	}
}

// applyBracketDefaults fills in Schwab's standard session and duration defaults.
func applyBracketDefaults(params *BracketParams) {
	params.Duration = cmp.Or(params.Duration, models.DurationDay)
	params.Session = cmp.Or(params.Session, models.SessionNormal)
}

// bracketExitInstruction returns the closing instruction for the bracket exit legs.
func bracketExitInstruction(action models.Instruction) models.Instruction {
	if action == models.InstructionSell {
		return models.InstructionBuy
	}

	return models.InstructionSell
}

// buildEquityLeg constructs a single-leg equity order entry.
func buildEquityLeg(symbol string, instruction models.Instruction, quantity float64) models.OrderLegCollection {
	return models.OrderLegCollection{
		Instruction: instruction,
		Quantity:    quantity,
		Instrument: models.OrderInstrument{
			AssetType: models.AssetTypeEquity,
			Symbol:    symbol,
		},
	}
}
