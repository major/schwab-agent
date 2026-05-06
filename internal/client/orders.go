package client

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	pathpkg "path"
	"strconv"
	"strings"

	"github.com/major/schwab-agent/internal/apperr"
	"github.com/major/schwab-agent/internal/models"
)

// OrderListParams holds optional filter parameters for listing orders.
type OrderListParams struct {
	Statuses        []string
	FromEnteredTime string
	ToEnteredTime   string
}

// toQueryParams converts date fields to a map for doGet.
//
// The Schwab API requires fromEnteredTime and toEnteredTime as mandatory query
// parameters in ISO 8601 format. When not provided, fromEnteredTime defaults
// to 60 days ago and toEnteredTime defaults to now (matching the Python
// schwab-py client behavior).
//
// Status is intentionally omitted here because the Schwab API accepts only a
// single status value per request. Multiple statuses require separate API calls
// with merged results, handled by fetchOrders.
func (p OrderListParams) toQueryParams() map[string]string {
	params := make(map[string]string)
	params["fromEnteredTime"], params["toEnteredTime"] = defaultDateRange(p.FromEnteredTime, p.ToEnteredTime)
	return params
}

// PlaceOrderResponse contains the result of a successful order placement.
type PlaceOrderResponse struct {
	OrderID int64
}

// ReplaceOrderResponse contains the best order ID Schwab exposes after a
// successful replacement. Schwab may return the new replacement order in the
// Location header; older/proxied responses sometimes omit it, so callers can
// fall back to the original order ID they asked to replace.
type ReplaceOrderResponse struct {
	OrderID int64
}

// orderIDFromLocation extracts the trailing order ID from a Schwab Location
// header such as /trader/v1/accounts/{hash}/orders/12345.
//
// Schwab usually returns a relative path, but proxies and future API changes may
// legally produce an absolute URL, query string, or trailing slash. Parse the
// header as a URL first so a successful mutation is not reported as failed just
// because the Location format is slightly different.
func orderIDFromLocation(location string) (int64, error) {
	parsedURL, err := url.Parse(location)
	if err != nil {
		return 0, fmt.Errorf("parse Location header %q: %w", location, err)
	}

	locationPath := strings.TrimRight(parsedURL.Path, "/")
	if locationPath == "" {
		return 0, fmt.Errorf("parse order ID from Location header %q: missing order ID", location)
	}

	orderIDStr := pathpkg.Base(locationPath)
	parsedID, err := strconv.ParseInt(orderIDStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse order ID from Location header %q: %w", location, err)
	}

	return parsedID, nil
}

// fetchOrders retrieves orders from the given path, fanning out one API call
// per status when multiple statuses are requested. The Schwab API accepts only
// a single status value per request, so multiple statuses require separate
// calls with merged results.
//
// When no statuses are provided, a single unfiltered request is made. When one
// status is provided, a single filtered request is made. When multiple are
// provided, one request per status is made and results are merged, deduplicating
// by OrderID.
func (c *Client) fetchOrders(ctx context.Context, path string, params OrderListParams) ([]models.Order, error) {
	baseQuery := params.toQueryParams()

	// Zero or one status: single API call.
	if len(params.Statuses) <= 1 {
		if len(params.Statuses) == 1 {
			baseQuery["status"] = params.Statuses[0]
		}
		var result []models.Order
		if err := c.doGet(ctx, path, baseQuery, &result); err != nil {
			return nil, err
		}
		return result, nil
	}

	// Multiple statuses: fan out one call per status, merge and dedup.
	// An order can only have one status at a time, but we guard against
	// API edge cases (e.g. status transition mid-request) by deduplicating
	// on OrderID.
	//
	// Initialize as empty (not nil) so JSON serialization produces [] instead
	// of null when all batches are empty, matching the single-status behavior.
	seen := make(map[int64]bool)
	merged := make([]models.Order, 0)
	for _, status := range params.Statuses {
		query := make(map[string]string, len(baseQuery)+1)
		maps.Copy(query, baseQuery)
		query["status"] = status

		var batch []models.Order
		if err := c.doGet(ctx, path, query, &batch); err != nil {
			return nil, err
		}
		for i := range batch {
			if batch[i].OrderID != nil {
				if seen[*batch[i].OrderID] {
					continue
				}
				seen[*batch[i].OrderID] = true
			}
			merged = append(merged, batch[i])
		}
	}
	return merged, nil
}

// ListOrders retrieves orders for a specific account, filtered by the given params.
func (c *Client) ListOrders(ctx context.Context, hashValue string, params OrderListParams) ([]models.Order, error) {
	path := fmt.Sprintf("/trader/v1/accounts/%s/orders", hashValue)
	return c.fetchOrders(ctx, path, params)
}

// AllOrders retrieves orders across all accounts, filtered by the given params.
func (c *Client) AllOrders(ctx context.Context, params OrderListParams) ([]models.Order, error) {
	return c.fetchOrders(ctx, "/trader/v1/orders", params)
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
func (c *Client) PlaceOrder(
	ctx context.Context,
	hashValue string,
	order *models.OrderRequest,
) (*PlaceOrderResponse, error) {
	path := fmt.Sprintf("/trader/v1/accounts/%s/orders", hashValue)

	encoded, err := json.Marshal(order)
	if err != nil {
		return nil, fmt.Errorf("marshal request body: %w", err)
	}

	c.logger.DebugContext(ctx, "http request", "method", http.MethodPost, "path", path)

	resp, err := c.resty.R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/json").
		SetBody(encoded).
		Execute(http.MethodPost, path)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}

	respBody := resp.Bytes()

	// Map status codes to typed errors, checking order-rejection codes before
	// the generic 4xx fallback so callers get OrderRejectedError specifically.
	if resp.StatusCode() == http.StatusUnauthorized {
		return nil, apperr.NewAuthExpiredError("authentication expired", nil)
	}
	if resp.StatusCode() == http.StatusBadRequest || resp.StatusCode() == http.StatusUnprocessableEntity {
		return nil, apperr.NewOrderRejectedError(
			fmt.Sprintf("order rejected: %s", string(respBody)),
			nil,
		)
	}
	if resp.StatusCode() >= http.StatusBadRequest {
		return nil, apperr.NewHTTPError(
			fmt.Sprintf("HTTP %d", resp.StatusCode()),
			resp.StatusCode(),
			string(respBody),
			nil,
		)
	}

	// Extract order ID from the Location header (e.g. /trader/v1/accounts/{hash}/orders/12345).
	location := resp.Header().Get("Location")
	if location == "" {
		return &PlaceOrderResponse{}, nil
	}

	parsedID, err := orderIDFromLocation(location)
	if err != nil {
		return nil, err
	}

	return &PlaceOrderResponse{OrderID: parsedID}, nil
}

// PreviewOrder previews an order without placing it, returning estimated costs and validation results.
func (c *Client) PreviewOrder(
	ctx context.Context,
	hashValue string,
	order *models.OrderRequest,
) (*models.PreviewOrder, error) {
	path := fmt.Sprintf("/trader/v1/accounts/%s/previewOrder", hashValue)
	var result models.PreviewOrder
	if err := c.doPost(ctx, path, order, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ReplaceOrder replaces an existing order with a new order specification.
func (c *Client) ReplaceOrder(
	ctx context.Context,
	hashValue string,
	orderID int64,
	order *models.OrderRequest,
) (*ReplaceOrderResponse, error) {
	path := fmt.Sprintf("/trader/v1/accounts/%s/orders/%d", hashValue, orderID)

	encoded, err := json.Marshal(order)
	if err != nil {
		return nil, fmt.Errorf("marshal request body: %w", err)
	}

	c.logger.DebugContext(ctx, "http request", "method", http.MethodPut, "path", path)

	resp, err := c.resty.R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/json").
		SetBody(encoded).
		Execute(http.MethodPut, path)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}

	respBody := resp.Bytes()
	if resp.StatusCode() == http.StatusUnauthorized {
		return nil, apperr.NewAuthExpiredError("authentication expired", nil)
	}
	if resp.StatusCode() == http.StatusBadRequest || resp.StatusCode() == http.StatusUnprocessableEntity {
		return nil, apperr.NewOrderRejectedError(
			fmt.Sprintf("order rejected: %s", string(respBody)),
			nil,
		)
	}
	if resp.StatusCode() >= http.StatusBadRequest {
		return nil, apperr.NewHTTPError(
			fmt.Sprintf("HTTP %d", resp.StatusCode()),
			resp.StatusCode(),
			string(respBody),
			nil,
		)
	}

	location := resp.Header().Get("Location")
	if location == "" {
		return &ReplaceOrderResponse{OrderID: orderID}, nil
	}

	replacementID, err := orderIDFromLocation(location)
	if err != nil {
		return nil, err
	}

	return &ReplaceOrderResponse{OrderID: replacementID}, nil
}

// CancelOrder cancels an existing order.
func (c *Client) CancelOrder(ctx context.Context, hashValue string, orderID int64) error {
	path := fmt.Sprintf("/trader/v1/accounts/%s/orders/%d", hashValue, orderID)
	return c.doDelete(ctx, path, nil)
}
