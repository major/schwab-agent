package client

import (
	"context"
	"fmt"

	"github.com/major/schwab-agent/internal/models"
)

// TransactionParams holds optional filter parameters for transaction queries.
type TransactionParams struct {
	Types     string
	StartDate string
	EndDate   string
	Symbol    string
}

// Transactions retrieves transactions for an account with optional filters.
//
// The Schwab API requires startDate and endDate as mandatory query parameters
// in ISO 8601 format (e.g. "2025-01-01T00:00:00.000Z"). When not provided,
// startDate defaults to 60 days ago and endDate defaults to now (matching the
// Python schwab-py client behavior).
//
// The types parameter filters by transaction type (e.g. "TRADE", "DIVIDEND").
// When not provided, no type filter is sent, which returns all types.
func (c *Client) Transactions(
	ctx context.Context,
	hashValue string,
	params TransactionParams,
) ([]models.Transaction, error) {
	path := fmt.Sprintf("/trader/v1/accounts/%s/transactions", hashValue)

	queryParams := make(map[string]string)
	queryParams["startDate"], queryParams["endDate"] = defaultDateRange(params.StartDate, params.EndDate)
	setParam(queryParams, "types", params.Types)
	setParam(queryParams, "symbol", params.Symbol)

	var result []models.Transaction
	err := c.doGet(ctx, path, queryParams, &result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// Transaction retrieves a specific transaction by ID.
func (c *Client) Transaction(ctx context.Context, hashValue string, txnID int64) (*models.Transaction, error) {
	path := fmt.Sprintf("/trader/v1/accounts/%s/transactions/%d", hashValue, txnID)
	var result models.Transaction
	err := c.doGet(ctx, path, nil, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}
