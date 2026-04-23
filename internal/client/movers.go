package client

import (
	"context"
	"fmt"

	"github.com/major/schwab-agent/internal/models"
)

// MoversParams contains optional parameters for Movers requests.
type MoversParams struct {
	Sort      string
	Frequency string
}

// Movers retrieves the top movers for a given market index.
func (c *Client) Movers(ctx context.Context, index string, params MoversParams) (*models.ScreenerResponse, error) {
	path := fmt.Sprintf("/marketdata/v1/movers/%s", index)
	qp := map[string]string{}
	setParam(qp, "sort", params.Sort)
	setParam(qp, "frequency", params.Frequency)
	var result models.ScreenerResponse
	err := c.doGet(ctx, path, qp, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}
