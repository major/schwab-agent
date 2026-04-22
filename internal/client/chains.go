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
	m := map[string]string{"symbol": symbol}
	if params.ContractType != "" {
		m["contractType"] = params.ContractType
	}
	if params.StrikeCount != "" {
		m["strikeCount"] = params.StrikeCount
	}
	if params.Strategy != "" {
		m["strategy"] = params.Strategy
	}
	if params.FromDate != "" {
		m["fromDate"] = params.FromDate
	}
	if params.ToDate != "" {
		m["toDate"] = params.ToDate
	}
	if params.IncludeUnderlyingQuote != "" {
		m["includeUnderlyingQuote"] = params.IncludeUnderlyingQuote
	}
	if params.Interval != "" {
		m["interval"] = params.Interval
	}
	if params.Strike != "" {
		m["strike"] = params.Strike
	}
	if params.StrikeRange != "" {
		// The Schwab API uses "range" as the query parameter name.
		m["range"] = params.StrikeRange
	}
	if params.Volatility != "" {
		m["volatility"] = params.Volatility
	}
	if params.UnderlyingPrice != "" {
		m["underlyingPrice"] = params.UnderlyingPrice
	}
	if params.InterestRate != "" {
		m["interestRate"] = params.InterestRate
	}
	if params.DaysToExpiration != "" {
		m["daysToExpiration"] = params.DaysToExpiration
	}
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
	params := map[string]string{"symbol": symbol}
	var result ExpirationChain
	err := c.doGet(ctx, "/marketdata/v1/expirationchain", params, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}
