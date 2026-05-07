package client

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	schwab "github.com/major/schwab-go/schwab"
	"github.com/major/schwab-go/schwab/trader"

	"github.com/major/schwab-agent/internal/apperr"
	"github.com/major/schwab-agent/internal/models"
)

// AccountNumbers retrieves the list of account numbers and hash values.
// Returns AccountNotFoundError on 404.
func (c *Client) AccountNumbers(ctx context.Context) ([]models.AccountNumber, error) {
	accountNumbers, err := c.newTraderClient().GetAccountNumbers(ctx)
	if err != nil {
		// Preserve the historical internal/client error contract while letting
		// schwab-go own the request construction, authentication header, and JSON
		// decoding for this endpoint.
		if schwab.IsUnauthorized(err) {
			return nil, apperr.NewAuthExpiredError("authentication expired", err)
		}
		if schwab.IsStatusCode(err, http.StatusNotFound) {
			return nil, apperr.NewAccountNotFoundError("account numbers not found", err)
		}
		return nil, schwabAPIErrorToHTTPError(err)
	}
	return accountNumberHashesToModels(accountNumbers), nil
}

func accountNumberHashesToModels(accountNumbers []trader.AccountNumberHash) []models.AccountNumber {
	if accountNumbers == nil {
		return nil
	}
	result := make([]models.AccountNumber, 0, len(accountNumbers))
	for _, accountNumber := range accountNumbers {
		result = append(result, models.AccountNumber{
			AccountNumber: accountNumber.AccountNumber,
			HashValue:     accountNumber.HashValue,
		})
	}
	return result
}

// Accounts retrieves all accounts.
// Pass field names (e.g. "positions") to include additional data in the response.
// The Schwab API only returns position data when explicitly requested via ?fields=positions.
// Returns AccountNotFoundError on 404.
func (c *Client) Accounts(ctx context.Context, fields ...string) ([]models.Account, error) {
	var params map[string]string
	if len(fields) > 0 {
		params = map[string]string{queryParamFields: strings.Join(fields, ",")}
	}
	var result []models.Account
	err := c.doGet(ctx, "/trader/v1/accounts", params, &result)
	if err != nil {
		// Convert 404 HTTPError to AccountNotFoundError
		if httpErr, ok := errors.AsType[*apperr.HTTPError](err); ok && httpErr.StatusCode == http.StatusNotFound {
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
		params = map[string]string{queryParamFields: strings.Join(fields, ",")}
	}
	var result models.Account
	err := c.doGet(ctx, path, params, &result)
	if err != nil {
		// Convert 404 HTTPError to AccountNotFoundError
		if httpErr, ok := errors.AsType[*apperr.HTTPError](err); ok && httpErr.StatusCode == http.StatusNotFound {
			return nil, apperr.NewAccountNotFoundError(fmt.Sprintf("account %s not found", hashValue), err)
		}
		return nil, err
	}
	return &result, nil
}
