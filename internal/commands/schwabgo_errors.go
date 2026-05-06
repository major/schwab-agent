package commands

import (
	"errors"
	"fmt"
	"net/http"

	schwab "github.com/major/schwab-go/schwab"

	"github.com/major/schwab-agent/internal/apperr"
)

// mapSchwabGoError converts a schwab-go API error to a typed apperr error.
// Non-API errors and nil are returned unchanged.
func mapSchwabGoError(err error) error {
	if err == nil {
		return nil
	}

	var apiErr *schwab.APIError
	if !errors.As(err, &apiErr) {
		return err
	}

	switch apiErr.StatusCode {
	case http.StatusUnauthorized:
		return apperr.NewAuthExpiredError(apiErr.Message, nil)
	default:
		return apperr.NewHTTPError(fmt.Sprintf("HTTP %d", apiErr.StatusCode), apiErr.StatusCode, apiErr.Body, nil)
	}
}
