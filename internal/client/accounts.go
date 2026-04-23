package client

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/major/schwab-agent/internal/apperr"
	"github.com/major/schwab-agent/internal/models"
)

// AccountNumbers retrieves the list of account numbers and hash values.
// Returns AccountNotFoundError on 404.
func (c *Client) AccountNumbers(ctx context.Context) ([]models.AccountNumber, error) {
	var result []models.AccountNumber
	err := c.doGet(ctx, "/trader/v1/accounts/accountNumbers", nil, &result)
	if err != nil {
		// Convert 404 HTTPError to AccountNotFoundError
		var httpErr *apperr.HTTPError
		if errors.As(err, &httpErr) && httpErr.StatusCode == http.StatusNotFound {
			return nil, apperr.NewAccountNotFoundError("account numbers not found", err)
		}
		return nil, err
	}
	return result, nil
}

// Accounts retrieves all accounts.
// Pass field names (e.g. "positions") to include additional data in the response.
// The Schwab API only returns position data when explicitly requested via ?fields=positions.
// Returns AccountNotFoundError on 404.
func (c *Client) Accounts(ctx context.Context, fields ...string) ([]models.Account, error) {
	var params map[string]string
	if len(fields) > 0 {
		params = map[string]string{"fields": strings.Join(fields, ",")}
	}
	var result []models.Account
	err := c.doGet(ctx, "/trader/v1/accounts", params, &result)
	if err != nil {
		// Convert 404 HTTPError to AccountNotFoundError
		var httpErr *apperr.HTTPError
		if errors.As(err, &httpErr) && httpErr.StatusCode == http.StatusNotFound {
			return nil, apperr.NewAccountNotFoundError("accounts not found", err)
		}
		return nil, err
	}
	return result, nil
}

// Account retrieves a specific account by hash value.
// Pass field names (e.g. "positions") to include additional data in the response.
// The Schwab API only returns position data when explicitly requested via ?fields=positions.
// Returns AccountNotFoundError on 404.
func (c *Client) Account(ctx context.Context, hashValue string, fields ...string) (*models.Account, error) {
	path := fmt.Sprintf("/trader/v1/accounts/%s", hashValue)
	var params map[string]string
	if len(fields) > 0 {
		params = map[string]string{"fields": strings.Join(fields, ",")}
	}
	var result models.Account
	err := c.doGet(ctx, path, params, &result)
	if err != nil {
		// Convert 404 HTTPError to AccountNotFoundError
		var httpErr *apperr.HTTPError
		if errors.As(err, &httpErr) && httpErr.StatusCode == http.StatusNotFound {
			return nil, apperr.NewAccountNotFoundError(fmt.Sprintf("account %s not found", hashValue), err)
		}
		return nil, err
	}
	return &result, nil
}
