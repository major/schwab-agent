package client

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	schwabErrors "github.com/major/schwab-agent/internal/errors"
	"github.com/major/schwab-agent/internal/models"
)

// Quotes retrieves quotes for multiple symbols.
// Symbols are passed as a comma-separated query parameter.
func (c *Client) Quotes(ctx context.Context, symbols []string) (map[string]*models.QuoteEquity, error) {
	params := map[string]string{
		"symbols": strings.Join(symbols, ","),
	}
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
func (c *Client) Quote(ctx context.Context, symbol string) (*models.QuoteEquity, error) {
	path := fmt.Sprintf("/marketdata/v1/%s/quotes", symbol)
	var result map[string]*models.QuoteEquity
	err := c.doGet(ctx, path, nil, &result)
	if err != nil {
		// Convert 404 HTTPError to SymbolNotFoundError
		var httpErr *schwabErrors.HTTPError
		if errors.As(err, &httpErr) && httpErr.StatusCode == http.StatusNotFound {
			return nil, schwabErrors.NewSymbolNotFoundError(fmt.Sprintf("symbol %s not found", symbol), err)
		}
		return nil, err
	}
	q, ok := result[symbol]
	if !ok {
		return nil, schwabErrors.NewSymbolNotFoundError(fmt.Sprintf("symbol %s not found", symbol), nil)
	}
	return q, nil
}
