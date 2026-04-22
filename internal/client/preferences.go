package client

import (
	"context"

	"github.com/major/schwab-agent/internal/models"
)

// UserPreference retrieves the user's preferences including account settings and streamer info.
func (c *Client) UserPreference(ctx context.Context) (*models.UserPreference, error) {
	var result models.UserPreference
	err := c.doGet(ctx, "/trader/v1/userPreference", nil, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}
