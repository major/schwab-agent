package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	pathpkg "path"
	"strconv"
	"strings"

	"github.com/major/schwab-go/schwab/trader"

	"github.com/major/schwab-agent/internal/apperr"
	"github.com/major/schwab-agent/internal/models"
)

// OrderListParams holds optional filter parameters for listing orders.
type OrderListParams struct {
	Statuses        []string
	FromEnteredTime string
	ToEnteredTime   string
}

func (p OrderListParams) toSchwabGoParams(status string) *trader.OrderListParams {
	fromEnteredTime, toEnteredTime := defaultDateRange(p.FromEnteredTime, p.ToEnteredTime)
	return &trader.OrderListParams{
		FromEnteredTime: fromEnteredTime,
		ToEnteredTime:   toEnteredTime,
		Status:          trader.OrderStatus(status),
	}
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
func (c *Client) fetchOrders(
	ctx context.Context,
	accountHash string,
	allAccounts bool,
	params OrderListParams,
) ([]models.Order, error) {
	traderClient := c.newTraderClient()
	// Zero or one status: single API call.
	if len(params.Statuses) <= 1 {
		var status string
		if len(params.Statuses) == 1 {
			status = params.Statuses[0]
		}
		batch, err := c.fetchSchwabGoOrders(
			ctx,
			traderClient,
			accountHash,
			allAccounts,
			params.toSchwabGoParams(status),
		)
		if err != nil {
			return nil, err
		}
		return adaptSchwabGoModel[[]models.Order](batch)
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
		batch, err := c.fetchSchwabGoOrders(
			ctx,
			traderClient,
			accountHash,
			allAccounts,
			params.toSchwabGoParams(status),
		)
		if err != nil {
			return nil, err
		}
		localBatch, err := adaptSchwabGoModel[[]models.Order](batch)
		if err != nil {
			return nil, err
		}
		for i := range localBatch {
			if localBatch[i].OrderID != nil {
				if seen[*localBatch[i].OrderID] {
					continue
				}
				seen[*localBatch[i].OrderID] = true
			}
			merged = append(merged, localBatch[i])
		}
	}
	return merged, nil
}

func (c *Client) fetchSchwabGoOrders(
	ctx context.Context,
	traderClient *trader.Client,
	accountHash string,
	allAccounts bool,
	params *trader.OrderListParams,
) ([]trader.Order, error) {
	if allAccounts {
		orders, err := traderClient.GetAllOrders(ctx, params)
		if err != nil {
			return nil, schwabAPIErrorToHTTPError(err)
		}
		return orders, nil
	}
	orders, err := traderClient.GetOrders(ctx, accountHash, params)
	if err != nil {
		return nil, schwabAPIErrorToHTTPError(err)
	}
	return orders, nil
}

// ListOrders retrieves orders for a specific account, filtered by the given params.
func (c *Client) ListOrders(ctx context.Context, hashValue string, params OrderListParams) ([]models.Order, error) {
	return c.fetchOrders(ctx, hashValue, false, params)
}

// AllOrders retrieves orders across all accounts, filtered by the given params.
func (c *Client) AllOrders(ctx context.Context, params OrderListParams) ([]models.Order, error) {
	return c.fetchOrders(ctx, "", true, params)
}

// GetOrder retrieves a specific order by account hash and order ID.
func (c *Client) GetOrder(ctx context.Context, hashValue string, orderID int64) (*models.Order, error) {
	order, err := c.newTraderClient().GetOrder(ctx, hashValue, orderID)
	if err != nil {
		return nil, schwabAPIErrorToHTTPError(err)
	}
	result, err := adaptSchwabGoModel[models.Order](order)
	if err != nil {
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
	// This intentionally stays on the compatibility transport until schwab-go can
	// preserve exact order request bodies and report malformed non-empty Location
	// headers. A typed trader.OrderRequest would currently drop fields such as
	// priceOffset used by trailing stop limit orders; see major/schwab-go#65.
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
	// Preview must send the same local order JSON used by place so saved preview
	// digests remain meaningful. See PlaceOrder and major/schwab-go#65.
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
	// See PlaceOrder for the exact-body and strict Location parsing constraints.
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
	if err := c.newTraderClient().CancelOrder(ctx, hashValue, orderID); err != nil {
		return schwabAPIErrorToHTTPError(err)
	}
	return nil
}
