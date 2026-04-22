package client

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	schwabErrors "github.com/major/schwab-agent/internal/errors"
	"github.com/major/schwab-agent/internal/models"
)

// AccountNumbers retrieves the list of account numbers and hash values.
// Returns AccountNotFoundError on 404.
func (c *Client) AccountNumbers(ctx context.Context) ([]models.AccountNumber, error) {
	var result []models.AccountNumber
	err := c.doGet(ctx, "/trader/v1/accounts/accountNumbers", nil, &result)
	if err != nil {
		// Convert 404 HTTPError to AccountNotFoundError
		var httpErr *schwabErrors.HTTPError
		if errors.As(err, &httpErr) && httpErr.StatusCode == http.StatusNotFound {
			return nil, schwabErrors.NewAccountNotFoundError("account numbers not found", err)
		}
		return nil, err
	}
	return result, nil
}

// Accounts retrieves all accounts.
// Returns AccountNotFoundError on 404.
func (c *Client) Accounts(ctx context.Context) ([]models.Account, error) {
	var result []models.Account
	err := c.doGet(ctx, "/trader/v1/accounts", nil, &result)
	if err != nil {
		// Convert 404 HTTPError to AccountNotFoundError
		var httpErr *schwabErrors.HTTPError
		if errors.As(err, &httpErr) && httpErr.StatusCode == http.StatusNotFound {
			return nil, schwabErrors.NewAccountNotFoundError("accounts not found", err)
		}
		return nil, err
	}
	return result, nil
}

// Account retrieves a specific account by hash value.
// Returns AccountNotFoundError on 404.
func (c *Client) Account(ctx context.Context, hashValue string) (*models.Account, error) {
	path := fmt.Sprintf("/trader/v1/accounts/%s", hashValue)
	var result models.Account
	err := c.doGet(ctx, path, nil, &result)
	if err != nil {
		// Convert 404 HTTPError to AccountNotFoundError
		var httpErr *schwabErrors.HTTPError
		if errors.As(err, &httpErr) && httpErr.StatusCode == http.StatusNotFound {
			return nil, schwabErrors.NewAccountNotFoundError(fmt.Sprintf("account %s not found", hashValue), err)
		}
		return nil, err
	}
	return &result, nil
}
