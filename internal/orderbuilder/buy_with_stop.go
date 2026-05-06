// Package orderbuilder constructs Schwab order payloads.
package orderbuilder

import (
	"cmp"
	"strings"

	"github.com/major/schwab-agent/internal/models"
)

// BuyWithStopParams holds parameters for a BUY entry with protective exits.
//
// The entry is always a BUY equity order. The stop-loss is required because the
// strategy exists to make downside protection explicit at build time; the
// take-profit exit is optional and turns the child exits into an OCO group.
type BuyWithStopParams struct {
	Symbol     string
	Quantity   float64
	OrderType  models.OrderType
	Duration   models.Duration
	Session    models.Session
	Price      float64
	StopLoss   float64
	TakeProfit float64
}

// ValidateBuyWithStopOrder validates parameters for a BUY entry with a stop.
func ValidateBuyWithStopOrder(params *BuyWithStopParams) error {
	if params == nil {
		return validationError("buy-with-stop parameters are required", "Provide buy-with-stop order parameters")
	}

	if strings.TrimSpace(params.Symbol) == "" {
		return validationError("symbol is required", "Add `--symbol <TICKER>` to specify the stock symbol")
	}

	if params.Quantity <= 0 {
		return validationError("quantity must be greater than zero", "Add `--quantity <number>` with a positive value")
	}

	entryOrderType := cmp.Or(params.OrderType, models.OrderTypeLimit)
	if entryOrderType != models.OrderTypeLimit && entryOrderType != models.OrderTypeMarket {
		return validationError(
			"only LIMIT and MARKET entry orders are supported for buy-with-stop",
			"Use `--type LIMIT` with `--price`, or `--type MARKET` without an entry price",
		)
	}

	if params.StopLoss <= 0 {
		return validationError(
			"stop-loss is required",
			"Add `--stop-loss <amount>` with a positive protective stop price",
		)
	}

	if entryOrderType == models.OrderTypeLimit && params.Price <= 0 {
		return validationError(
			"LIMIT buy-with-stop order requires a price",
			"Add `--price <amount>` to specify the entry limit price",
		)
	}

	// MARKET orders should never carry a price field; reject early so the
	// builder doesn't accidentally attach one.
	if entryOrderType == models.OrderTypeMarket && params.Price > 0 {
		return validationError(
			"MARKET entry does not accept a price",
			"Remove `--price` or use `--type LIMIT` for a priced entry",
		)
	}

	// For a BUY entry, take-profit must sit above stop-loss regardless of
	// order type. Equality and inversion are both contradictory.
	if params.TakeProfit > 0 && params.TakeProfit <= params.StopLoss {
		return validationError(
			"take-profit must be above stop-loss",
			"Set `--take-profit` higher than `--stop-loss` so the exits form valid upside/downside brackets",
		)
	}

	// MARKET entries have no known entry price, so validate exit relationships
	// only when a price is present. This keeps MARKET+protective-stop usable.
	if params.Price > 0 {
		if params.StopLoss >= params.Price {
			return validationError(
				"stop-loss must be below entry price",
				"Lower `--stop-loss` so it is below the buy entry price",
			)
		}

		if params.TakeProfit > 0 && params.TakeProfit <= params.Price {
			return validationError(
				"take-profit must be above entry price",
				"Increase `--take-profit` so it is above the buy entry price",
			)
		}
	}

	return nil
}

// BuildBuyWithStopOrder constructs a TRIGGER order for buying shares with exits.
func BuildBuyWithStopOrder(params *BuyWithStopParams) (*models.OrderRequest, error) {
	if err := ValidateBuyWithStopOrder(params); err != nil {
		return nil, err
	}

	entryOrderType := cmp.Or(params.OrderType, models.OrderTypeLimit)
	entryDuration := cmp.Or(params.Duration, models.DurationDay)
	entrySession := cmp.Or(params.Session, models.SessionNormal)
	exitParams := &BracketParams{
		Symbol:     params.Symbol,
		Quantity:   params.Quantity,
		Duration:   models.DurationGoodTillCancel,
		Session:    entrySession,
		Price:      params.Price,
		StopLoss:   params.StopLoss,
		TakeProfit: params.TakeProfit,
	}

	order := &models.OrderRequest{
		Session:           entrySession,
		Duration:          entryDuration,
		OrderType:         entryOrderType,
		OrderStrategyType: models.OrderStrategyTypeTrigger,
		OrderLegCollection: []models.OrderLegCollection{
			buildEquityLeg(params.Symbol, models.InstructionBuy, params.Quantity),
		},
		ChildOrderStrategies: buildExitStrategies(exitParams, bracketExitInstruction(models.InstructionBuy)),
	}

	// Defense-in-depth: only set Price on LIMIT entries. The validator
	// already rejects MARKET+price, but guarding here prevents a future
	// code path from accidentally attaching a price to a market order.
	if entryOrderType == models.OrderTypeLimit && params.Price > 0 {
		order.Price = new(params.Price)
	}

	return order, nil
}
