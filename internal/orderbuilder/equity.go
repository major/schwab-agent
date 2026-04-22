package orderbuilder

import "github.com/major/schwab-agent/internal/models"

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
		Session:           defaultSession(params.Session),
		Duration:          defaultDuration(params.Duration),
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
		order.Price = float64Ptr(params.Price)
	case models.OrderTypeStop:
		order.StopPrice = float64Ptr(params.StopPrice)
	case models.OrderTypeStopLimit:
		order.Price = float64Ptr(params.Price)
		order.StopPrice = float64Ptr(params.StopPrice)
	}
}

// defaultDuration applies the task default when no duration is provided.
func defaultDuration(duration models.Duration) models.Duration {
	if duration == "" {
		return models.DurationDay
	}

	return duration
}

// defaultSession applies the task default when no session is provided.
func defaultSession(session models.Session) models.Session {
	if session == "" {
		return models.SessionNormal
	}

	return session
}

// float64Ptr returns a pointer to the provided float64 value.
func float64Ptr(value float64) *float64 {
	return &value
}
