package orderbuilder

import (
	"cmp"

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
	}
}

// ptr returns a pointer to the provided value.
func ptr[T any](v T) *T {
	return &v
}
