package client

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"

	"github.com/major/schwab-go/schwab/marketdata"

	"github.com/major/schwab-agent/internal/models"
)

// ExpirationChain represents the response from an expiration chain request.
type ExpirationChain struct {
	ExpirationList []ExpirationDate `json:"expirationList,omitempty"`
}

// ExpirationDate represents a single expiration date entry.
type ExpirationDate struct {
	ExpirationDate string `json:"expirationDate"`
}

// UnmarshalJSON accepts both the historical Schwab response field
// (expirationDate) and the schwab-go v0.1.x field (expiration). The command
// output keeps expirationDate for compatibility with existing agents.
func (e *ExpirationDate) UnmarshalJSON(data []byte) error {
	var raw struct {
		ExpirationDate string `json:"expirationDate"`
		Expiration     string `json:"expiration"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	e.ExpirationDate = raw.ExpirationDate
	if e.ExpirationDate == "" {
		e.ExpirationDate = raw.Expiration
	}
	return nil
}

// OptionChain retrieves the option chain for a symbol with schwab-go parameter
// types while decoding into the project model that preserves Schwab's observed
// option-chain JSON names. This bridges a schwab-go v0.1.x response-model gap
// without reintroducing the old stringly typed ChainParams API.
func (c *Client) OptionChain(
	ctx context.Context,
	params *marketdata.OptionChainParams,
) (*models.OptionChain, error) {
	qp := optionChainQueryParams(params)
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

func optionChainQueryParams(params *marketdata.OptionChainParams) map[string]string {
	if params == nil {
		return nil
	}

	q := make(url.Values)
	q.Set(queryParamSymbol, params.Symbol)
	setOptionalQueryString(q, "contractType", string(params.ContractType))
	setOptionalQueryInt(q, "strikeCount", params.StrikeCount)
	setOptionalQueryBool(q, "includeUnderlyingQuote", params.IncludeUnderlyingQuote)
	setOptionalQueryString(q, "strategy", string(params.Strategy))
	setOptionalQueryFloat(q, "interval", params.Interval)
	setOptionalQueryFloat(q, "strike", params.Strike)
	setOptionalQueryString(q, "range", string(params.Range))
	setOptionalQueryString(q, "fromDate", params.FromDate)
	setOptionalQueryString(q, "toDate", params.ToDate)
	setOptionalQueryFloat(q, "volatility", params.Volatility)
	setOptionalQueryFloat(q, "underlyingPrice", params.UnderlyingPrice)
	setOptionalQueryFloat(q, "interestRate", params.InterestRate)
	setOptionalQueryInt(q, "daysToExpiration", params.DaysToExpiration)
	setOptionalQueryString(q, "expMonth", string(params.ExpMonth))
	setOptionalQueryString(q, "optionType", string(params.OptionType))
	setOptionalQueryString(q, "entitlement", string(params.Entitlement))
	return flattenQueryValues(q)
}

func setOptionalQueryString(q url.Values, name, value string) {
	if value != "" {
		q.Set(name, value)
	}
}

func setOptionalQueryInt(q url.Values, name string, value int) {
	if value != 0 {
		q.Set(name, strconv.Itoa(value))
	}
}

func setOptionalQueryBool(q url.Values, name string, value bool) {
	if value {
		q.Set(name, strconv.FormatBool(value))
	}
}

func setOptionalQueryFloat(q url.Values, name string, value float64) {
	// marketdata.OptionChainParams follows schwab-go's zero-value convention:
	// zero means the optional query parameter is unset. Command parsing rejects
	// explicit zero values so users do not accidentally supply a flag that gets
	// omitted here.
	if value != 0 {
		q.Set(name, strconv.FormatFloat(value, 'f', -1, 64))
	}
}

func flattenQueryValues(q url.Values) map[string]string {
	params := make(map[string]string, len(q))
	for key, values := range q {
		if len(values) == 0 {
			continue
		}
		params[key] = values[0]
	}
	return params
}
