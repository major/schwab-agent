package client

import (
	"context"
	"fmt"

	"github.com/major/schwab-agent/internal/apperr"
	"github.com/major/schwab-agent/internal/models"
)

// SearchInstruments searches for instruments by symbol and projection type.
func (c *Client) SearchInstruments(ctx context.Context, symbol, projection string) ([]models.Instrument, error) {
	params := map[string]string{
		"symbol":     symbol,
		"projection": projection,
	}
	var result models.InstrumentResponse
	err := c.doGet(ctx, "/marketdata/v1/instruments", params, &result)
	if err != nil {
		return nil, err
	}
	return result.Instruments, nil
}

// GetInstrument retrieves a single instrument by CUSIP.
// The Schwab API returns {"instruments": [...]} even for single-CUSIP lookups,
// so we parse as InstrumentResponse and extract the first element.
func (c *Client) GetInstrument(ctx context.Context, cusip string) (*models.Instrument, error) {
	path := fmt.Sprintf("/marketdata/v1/instruments/%s", cusip)
	var result models.InstrumentResponse
	err := c.doGet(ctx, path, nil, &result)
	if err != nil {
		return nil, err
	}
	if len(result.Instruments) == 0 {
		return nil, apperr.NewSymbolNotFoundError(
			fmt.Sprintf("instrument not found for CUSIP %s", cusip), nil,
		)
	}
	return &result.Instruments[0], nil
}
