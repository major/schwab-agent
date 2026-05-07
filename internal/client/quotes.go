package client

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/major/schwab-go/schwab/marketdata"

	"github.com/major/schwab-agent/internal/apperr"
	"github.com/major/schwab-agent/internal/models"
)

// QuoteParams contains optional parameters for quote requests.
type QuoteParams struct {
	Fields     []string // Specific quote fields to return (e.g., "quote", "fundamental").
	Indicative bool     // Request indicative (non-tradeable) quotes.
}

// quoteParams builds the query parameter map from QuoteParams.
// Returns an empty map for zero-value QuoteParams.
func quoteParams(p QuoteParams) map[string]string {
	m := map[string]string{}
	if len(p.Fields) > 0 {
		m[queryParamFields] = strings.Join(p.Fields, ",")
	}
	if p.Indicative {
		m["indicative"] = "true"
	}
	return m
}

func quoteFields(p QuoteParams) string {
	if len(p.Fields) == 0 {
		return ""
	}
	return strings.Join(p.Fields, ",")
}

func quoteResponseToModels(response *marketdata.QuoteResponse) (map[string]*models.QuoteEquity, error) {
	if response == nil {
		return map[string]*models.QuoteEquity{}, nil
	}
	return adaptSchwabGoModel[map[string]*models.QuoteEquity](response)
}

// Quotes retrieves quotes for multiple symbols.
// Symbols are passed as a comma-separated query parameter.
func (c *Client) Quotes(ctx context.Context, symbols []string, p QuoteParams) (map[string]*models.QuoteEquity, error) {
	response, _, err := c.newMarketDataClient().GetQuotes(ctx, symbols, quoteFields(p), p.Indicative)
	if err != nil {
		return nil, schwabAPIErrorToHTTPError(err)
	}
	return quoteResponseToModels(response)
}

// Quote retrieves a quote for a single symbol.
// Returns SymbolNotFoundError on 404.
func (c *Client) Quote(ctx context.Context, symbol string, p QuoteParams) (*models.QuoteEquity, error) {
	// Use GetQuotes rather than schwab-go's GetQuote because the multi-symbol
	// helper preserves the indicative flag this CLI has historically accepted for
	// single-symbol requests too.
	response, _, err := c.newMarketDataClient().GetQuotes(ctx, []string{symbol}, quoteFields(p), p.Indicative)
	if err != nil {
		mappedErr := schwabAPIErrorToHTTPError(err)
		if httpErr, ok := errors.AsType[*apperr.HTTPError](mappedErr); ok && httpErr.StatusCode == http.StatusNotFound {
			return nil, apperr.NewSymbolNotFoundError(fmt.Sprintf("symbol %s not found", symbol), err)
		}
		return nil, mappedErr
	}
	result, err := quoteResponseToModels(response)
	if err != nil {
		return nil, err
	}
	q, ok := result[symbol]
	if !ok {
		return nil, apperr.NewSymbolNotFoundError(fmt.Sprintf("symbol %s not found", symbol), nil)
	}
	return q, nil
}
