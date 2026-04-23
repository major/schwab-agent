package client

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"strings"

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
		m["fields"] = strings.Join(p.Fields, ",")
	}
	if p.Indicative {
		m["indicative"] = "true"
	}
	return m
}

// Quotes retrieves quotes for multiple symbols.
// Symbols are passed as a comma-separated query parameter.
func (c *Client) Quotes(ctx context.Context, symbols []string, p QuoteParams) (map[string]*models.QuoteEquity, error) {
	params := map[string]string{"symbols": strings.Join(symbols, ",")}
	maps.Copy(params, quoteParams(p))
	var result map[string]*models.QuoteEquity
	err := c.doGet(ctx, "/marketdata/v1/quotes", params, &result)
	if err != nil {
		return nil, err
	}
	// The Schwab API includes an "errors" key in the response map for
	// invalid symbols. Remove it so only real quote entries remain.
	delete(result, "errors")
	return result, nil
}

// Quote retrieves a quote for a single symbol.
// Returns SymbolNotFoundError on 404.
func (c *Client) Quote(ctx context.Context, symbol string, p QuoteParams) (*models.QuoteEquity, error) {
	path := fmt.Sprintf("/marketdata/v1/%s/quotes", symbol)
	var result map[string]*models.QuoteEquity
	err := c.doGet(ctx, path, quoteParams(p), &result)
	if err != nil {
		// Convert 404 HTTPError to SymbolNotFoundError
		var httpErr *apperr.HTTPError
		if errors.As(err, &httpErr) && httpErr.StatusCode == http.StatusNotFound {
			return nil, apperr.NewSymbolNotFoundError(fmt.Sprintf("symbol %s not found", symbol), err)
		}
		return nil, err
	}
	q, ok := result[symbol]
	if !ok {
		return nil, apperr.NewSymbolNotFoundError(fmt.Sprintf("symbol %s not found", symbol), nil)
	}
	return q, nil
}
