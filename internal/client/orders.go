package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	schwabErrors "github.com/major/schwab-agent/internal/errors"
	"github.com/major/schwab-agent/internal/models"
)

// OrderListParams holds optional filter parameters for listing orders.
type OrderListParams struct {
	Status          string
	FromEnteredTime string
	ToEnteredTime   string
}

// toQueryParams converts fields to a map for doGet.
//
// The Schwab API requires fromEnteredTime and toEnteredTime as mandatory query
// parameters in ISO 8601 format. When not provided, fromEnteredTime defaults
// to 60 days ago and toEnteredTime defaults to now (matching the Python
// schwab-py client behavior).
func (p OrderListParams) toQueryParams() map[string]string {
	params := make(map[string]string)
	if p.Status != "" {
		params["status"] = p.Status
	}

	now := time.Now().UTC()

	if p.FromEnteredTime != "" {
		params["fromEnteredTime"] = p.FromEnteredTime
	} else {
		// Default to 60 days ago when no start time is provided.
		params["fromEnteredTime"] = now.AddDate(0, 0, -60).Format(time.RFC3339)
	}

	if p.ToEnteredTime != "" {
		params["toEnteredTime"] = p.ToEnteredTime
	} else {
		params["toEnteredTime"] = now.Format(time.RFC3339)
	}

	return params
}

// PlaceOrderResponse contains the result of a successful order placement.
type PlaceOrderResponse struct {
	OrderID int64
}

// ListOrders retrieves orders for a specific account, filtered by the given params.
func (c *Client) ListOrders(ctx context.Context, hashValue string, params OrderListParams) ([]models.Order, error) {
	path := fmt.Sprintf("/trader/v1/accounts/%s/orders", hashValue)
	var result []models.Order
	if err := c.doGet(ctx, path, params.toQueryParams(), &result); err != nil {
		return nil, err
	}
	return result, nil
}

// AllOrders retrieves orders across all accounts, filtered by the given params.
func (c *Client) AllOrders(ctx context.Context, params OrderListParams) ([]models.Order, error) {
	var result []models.Order
	if err := c.doGet(ctx, "/trader/v1/orders", params.toQueryParams(), &result); err != nil {
		return nil, err
	}
	return result, nil
}

// GetOrder retrieves a specific order by account hash and order ID.
func (c *Client) GetOrder(ctx context.Context, hashValue string, orderID int64) (*models.Order, error) {
	path := fmt.Sprintf("/trader/v1/accounts/%s/orders/%d", hashValue, orderID)
	var result models.Order
	if err := c.doGet(ctx, path, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// PlaceOrder places a new order for the specified account.
// Extracts the order ID from the Location header on success.
// Returns OrderRejectedError on 400 or 422 responses.
func (c *Client) PlaceOrder(ctx context.Context, hashValue string, order *models.OrderRequest) (*PlaceOrderResponse, error) {
	path := fmt.Sprintf("/trader/v1/accounts/%s/orders", hashValue)

	encoded, err := json.Marshal(order)
	if err != nil {
		return nil, fmt.Errorf("marshal request body: %w", err)
	}

	fullURL := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fullURL, bytes.NewReader(encoded))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	c.logger.Debug("http request", "method", http.MethodPost, "path", path)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	// Map status codes to typed errors, checking order-rejection codes before
	// the generic 4xx fallback so callers get OrderRejectedError specifically.
	if resp.StatusCode == http.StatusUnauthorized {
		return nil, schwabErrors.NewAuthExpiredError("authentication expired", nil)
	}
	if resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusUnprocessableEntity {
		return nil, schwabErrors.NewOrderRejectedError(
			fmt.Sprintf("order rejected: %s", string(respBody)),
			nil,
		)
	}
	if resp.StatusCode >= 400 {
		return nil, schwabErrors.NewHTTPError(
			fmt.Sprintf("HTTP %d", resp.StatusCode),
			resp.StatusCode,
			string(respBody),
			nil,
		)
	}

	// Extract order ID from the Location header (e.g. /trader/v1/accounts/{hash}/orders/12345).
	location := resp.Header.Get("Location")
	if location == "" {
		return &PlaceOrderResponse{}, nil
	}

	parts := strings.Split(location, "/")
	orderIDStr := parts[len(parts)-1]
	parsedID, err := strconv.ParseInt(orderIDStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("parse order ID from Location header %q: %w", location, err)
	}

	return &PlaceOrderResponse{OrderID: parsedID}, nil
}

// PreviewOrder previews an order without placing it, returning estimated costs and validation results.
func (c *Client) PreviewOrder(ctx context.Context, hashValue string, order *models.OrderRequest) (*models.PreviewOrder, error) {
	path := fmt.Sprintf("/trader/v1/accounts/%s/previewOrder", hashValue)
	var result models.PreviewOrder
	if err := c.doPost(ctx, path, order, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ReplaceOrder replaces an existing order with a new order specification.
func (c *Client) ReplaceOrder(ctx context.Context, hashValue string, orderID int64, order *models.OrderRequest) error {
	path := fmt.Sprintf("/trader/v1/accounts/%s/orders/%d", hashValue, orderID)
	return c.doPut(ctx, path, order, nil)
}

// CancelOrder cancels an existing order.
func (c *Client) CancelOrder(ctx context.Context, hashValue string, orderID int64) error {
	path := fmt.Sprintf("/trader/v1/accounts/%s/orders/%d", hashValue, orderID)
	return c.doDelete(ctx, path, nil)
}
