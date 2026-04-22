package client

import (
	"context"
	"fmt"
	"strings"

	"github.com/major/schwab-agent/internal/models"
)

// Markets retrieves market hours for the specified markets.
// The Schwab API requires a comma-separated list of market names in the
// "markets" query parameter; omitting it returns 400.
func (c *Client) Markets(ctx context.Context, markets []string) (map[string]map[string]models.MarketHours, error) {
	params := map[string]string{
		"markets": strings.Join(markets, ","),
	}
	var result map[string]map[string]models.MarketHours
	err := c.doGet(ctx, "/marketdata/v1/markets", params, &result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// Market retrieves market hours for a specific market.
// The API response is doubly nested: market type -> product code -> hours.
func (c *Client) Market(ctx context.Context, market string) (map[string]map[string]models.MarketHours, error) {
	path := fmt.Sprintf("/marketdata/v1/markets/%s", market)
	var result map[string]map[string]models.MarketHours
	err := c.doGet(ctx, path, nil, &result)
	if err != nil {
		return nil, err
	}
	return result, nil
}
