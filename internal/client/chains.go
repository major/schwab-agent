package client

import (
	"context"

	"github.com/major/schwab-agent/internal/models"
)

// ChainParams contains optional parameters for OptionChain requests.
type ChainParams struct {
	ContractType           string
	StrikeCount            string
	Strategy               string
	FromDate               string
	ToDate                 string
	IncludeUnderlyingQuote string
	Interval               string
	Strike                 string
	StrikeRange            string
	Volatility             string
	UnderlyingPrice        string
	InterestRate           string
	DaysToExpiration       string
}

// ExpirationChain represents the response from an expiration chain request.
type ExpirationChain struct {
	ExpirationList []ExpirationDate `json:"expirationList,omitempty"`
}

// ExpirationDate represents a single expiration date entry.
type ExpirationDate struct {
	ExpirationDate string `json:"expirationDate"`
}

// chainParams builds the query parameter map from ChainParams, including the symbol.
func chainParams(symbol string, params *ChainParams) map[string]string {
	m := map[string]string{queryParamSymbol: symbol}
	setParam(m, "contractType", params.ContractType)
	setParam(m, "strikeCount", params.StrikeCount)
	setParam(m, "strategy", params.Strategy)
	setParam(m, "fromDate", params.FromDate)
	setParam(m, "toDate", params.ToDate)
	setParam(m, "includeUnderlyingQuote", params.IncludeUnderlyingQuote)
	setParam(m, "interval", params.Interval)
	setParam(m, "strike", params.Strike)
	// The Schwab API uses "range" as the query parameter name.
	setParam(m, "range", params.StrikeRange)
	setParam(m, "volatility", params.Volatility)
	setParam(m, "underlyingPrice", params.UnderlyingPrice)
	setParam(m, "interestRate", params.InterestRate)
	setParam(m, "daysToExpiration", params.DaysToExpiration)
	return m
}

// OptionChain retrieves the option chain for a symbol with optional filter parameters.
func (c *Client) OptionChain(ctx context.Context, symbol string, params *ChainParams) (*models.OptionChain, error) {
	qp := chainParams(symbol, params)
	var result models.OptionChain
	err := c.doGet(ctx, "/marketdata/v1/chains", qp, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// ExpirationChainForSymbol retrieves the expiration chain for a symbol.
func (c *Client) ExpirationChainForSymbol(ctx context.Context, symbol string) (*ExpirationChain, error) {
	params := map[string]string{queryParamSymbol: symbol}
	var result ExpirationChain
	err := c.doGet(ctx, "/marketdata/v1/expirationchain", params, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}
