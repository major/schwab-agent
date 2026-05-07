package client

import (
	"context"

	"github.com/major/schwab-agent/internal/models"
)

// UserPreference retrieves the user's preferences including account settings and streamer info.
func (c *Client) UserPreference(ctx context.Context) (*models.UserPreference, error) {
	// schwab-go v0.4.2 has typed preferences, but the typed model reshapes fields
	// this CLI already exposes, including legacy offer metadata and numeric
	// streamerInfo.tokenExpTime. Keep the raw compatibility decoder until
	// schwab-go exposes raw preferences or full model parity; see
	// major/schwab-go#63.
	var result models.UserPreference
	err := c.doGet(ctx, "/trader/v1/userPreference", nil, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}
