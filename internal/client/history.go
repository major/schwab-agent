package client

import (
	"context"

	"github.com/major/schwab-agent/internal/models"
)

// HistoryParams contains optional parameters for PriceHistory requests.
type HistoryParams struct {
	PeriodType    string
	Period        string
	FrequencyType string
	Frequency     string
	StartDate     string
	EndDate       string
}

// historyParams builds the query parameter map from HistoryParams, including the symbol.
func historyParams(symbol string, params *HistoryParams) map[string]string {
	m := map[string]string{queryParamSymbol: symbol}
	setParam(m, "periodType", params.PeriodType)
	setParam(m, "period", params.Period)
	setParam(m, "frequencyType", params.FrequencyType)
	setParam(m, "frequency", params.Frequency)
	setParam(m, "startDate", params.StartDate)
	setParam(m, "endDate", params.EndDate)
	return m
}

// PriceHistory retrieves price history candles for a symbol.
func (c *Client) PriceHistory(ctx context.Context, symbol string, params *HistoryParams) (*models.CandleList, error) {
	qp := historyParams(symbol, params)
	var result models.CandleList
	err := c.doGet(ctx, "/marketdata/v1/pricehistory", qp, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}
