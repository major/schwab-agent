package client

import (
	"context"
	"encoding/json"
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
	raw, err := c.newTraderClient().GetAccountsRaw(ctx, accountFields(fields...))
	if err != nil {
		// schwab-go v0.4.2 exposes raw account responses, which lets it own
		// request construction while this compatibility facade preserves the
		// CLI's stable local model and 404 -> AccountNotFoundError mapping.
		if schwab.IsUnauthorized(err) {
			return nil, apperr.NewAuthExpiredError("authentication expired", err)
		}
		if schwab.IsStatusCode(err, http.StatusNotFound) {
			return nil, apperr.NewAccountNotFoundError("accounts not found", err)
		}
		return nil, schwabAPIErrorToHTTPError(err)
	}

	var result []models.Account
	if unmarshalErr := json.Unmarshal(raw, &result); unmarshalErr != nil {
		return nil, fmt.Errorf("decode accounts response: %w", unmarshalErr)
	}
	return result, nil
}

// Account retrieves a specific account by hash value.
// Pass field names (e.g. "positions") to include additional data in the response.
// The Schwab API only returns position data when explicitly requested via ?fields=positions.
// Returns AccountNotFoundError on 404.
func (c *Client) Account(ctx context.Context, hashValue string, fields ...string) (*models.Account, error) {
	raw, err := c.newTraderClient().GetAccountRaw(ctx, hashValue, accountFields(fields...))
	if err != nil {
		if schwab.IsUnauthorized(err) {
			return nil, apperr.NewAuthExpiredError("authentication expired", err)
		}
		if schwab.IsStatusCode(err, http.StatusNotFound) {
			return nil, apperr.NewAccountNotFoundError(fmt.Sprintf("account %s not found", hashValue), err)
		}
		return nil, schwabAPIErrorToHTTPError(err)
	}

	var result models.Account
	if unmarshalErr := json.Unmarshal(raw, &result); unmarshalErr != nil {
		return nil, fmt.Errorf("decode account response: %w", unmarshalErr)
	}
	return &result, nil
}

func accountFields(fields ...string) string {
	if len(fields) == 0 {
		return ""
	}
	return strings.Join(fields, ",")
}
