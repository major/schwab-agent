package orderbuilder

import (
	"cmp"

	"github.com/major/schwab-agent/internal/apperr"
	"github.com/major/schwab-agent/internal/models"
)

// EquityParams holds parameters for building an equity order.
type EquityParams struct {
	Symbol    string
	Action    models.Instruction
	Quantity  float64
	OrderType models.OrderType
	Price     float64
	StopPrice float64
	Duration  models.Duration
	Session   models.Session

	SpecialInstruction models.SpecialInstruction

	// Trailing stop fields. The stop adjusts dynamically via offset
	// rather than using a fixed StopPrice.
	StopPriceOffset    float64
	StopPriceLinkBasis models.StopPriceLinkBasis
	StopPriceLinkType  models.StopPriceLinkType
	StopType           models.StopType
	ActivationPrice    float64

	// Routing and price link fields (optional, pass-through to Schwab API).
	Destination    models.RequestedDestination
	PriceLinkBasis models.PriceLinkBasis
	PriceLinkType  models.PriceLinkType
}

// BuildEquityOrder constructs an OrderRequest for an equity order.
func BuildEquityOrder(params *EquityParams) (*models.OrderRequest, error) {
	order := &models.OrderRequest{
		Session:           cmp.Or(params.Session, models.SessionNormal),
		Duration:          cmp.Or(params.Duration, models.DurationDay),
		OrderType:         params.OrderType,
		OrderStrategyType: models.OrderStrategyTypeSingle,
		OrderLegCollection: []models.OrderLegCollection{
			{
				Instruction: params.Action,
				Quantity:    params.Quantity,
				Instrument: models.OrderInstrument{
					AssetType: models.AssetTypeEquity,
					Symbol:    params.Symbol,
				},
			},
		},
	}

	applyEquityPriceFields(order, params)

	if params.SpecialInstruction != "" {
		order.SpecialInstruction = ptr(params.SpecialInstruction)
	}

	if params.Destination != "" {
		order.RequestedDestination = ptr(params.Destination)
	}

	// Market-style orders have no price to link against, so reject price-link fields early.
	if (params.OrderType == models.OrderTypeMarket || params.OrderType == models.OrderTypeMarketOnClose) &&
		(params.PriceLinkBasis != "" || params.PriceLinkType != "") {
		return nil, apperr.NewValidationError(
			"price-link-basis and price-link-type are not allowed on market orders",
			nil,
		)
	}

	// Only set price link fields when explicitly provided by the user.
	// Trailing stop limit orders derive PriceLinkBasis/PriceLinkType from
	// the stop price link values in applyEquityPriceFields - these explicit
	// fields are set afterwards and will override those derived values.
	if params.PriceLinkBasis != "" {
		order.PriceLinkBasis = ptr(params.PriceLinkBasis)
	}

	if params.PriceLinkType != "" {
		order.PriceLinkType = ptr(params.PriceLinkType)
	}

	return order, nil
}

// applyEquityPriceFields sets the order price fields required by the selected order type.
func applyEquityPriceFields(order *models.OrderRequest, params *EquityParams) {
	switch params.OrderType {
	case models.OrderTypeLimit:
		order.Price = ptr(params.Price)
	case models.OrderTypeStop:
		order.StopPrice = ptr(params.StopPrice)
	case models.OrderTypeStopLimit:
		order.Price = ptr(params.Price)
		order.StopPrice = ptr(params.StopPrice)
	case models.OrderTypeTrailingStop:
		applyTrailingStopFields(order, params)
	case models.OrderTypeTrailingStopLimit:
		applyTrailingStopFields(order, params)

		// Schwab API uses priceOffset (not price) for trailing stop limit orders.
		// The offset defines the limit price distance from the triggered stop.
		// priceLinkBasis/priceLinkType default to the stop price link values.
		order.PriceOffset = ptr(params.Price)
		order.PriceLinkBasis = ptr(models.PriceLinkBasis(params.StopPriceLinkBasis))
		order.PriceLinkType = ptr(models.PriceLinkType(params.StopPriceLinkType))
	case models.OrderTypeLimitOnClose:
		// LOC orders require a limit price, similar to LIMIT orders
		order.Price = ptr(params.Price)
		// MOC (MARKET_ON_CLOSE) orders don't require any price fields
	}
}

// applyTrailingStopFields sets the trailing stop-specific fields shared by
// TRAILING_STOP and TRAILING_STOP_LIMIT order types.
func applyTrailingStopFields(order *models.OrderRequest, params *EquityParams) {
	order.StopPriceOffset = ptr(params.StopPriceOffset)
	order.StopPriceLinkBasis = ptr(params.StopPriceLinkBasis)
	order.StopPriceLinkType = ptr(params.StopPriceLinkType)
	order.StopType = ptr(params.StopType)

	if params.ActivationPrice > 0 {
		order.ActivationPrice = ptr(params.ActivationPrice)
	}
}

// ptr returns a pointer to the provided value.
func ptr[T any](v T) *T {
	return &v
}
