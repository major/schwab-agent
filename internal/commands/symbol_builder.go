package commands

import (
	"strings"
	"time"

	"github.com/major/schwab-agent/internal/apperr"
	"github.com/major/schwab-agent/internal/orderbuilder"
)

// buildOCCSymbol constructs an OCC option symbol from its components.
// It validates all inputs and returns a ValidationError for invalid data.
//
// Parameters:
//   - underlying: ticker symbol (e.g., "AAPL", "SPY")
//   - expiration: date string in "YYYY-MM-DD" format
//   - strike: strike price as float64
//   - call: true if call option
//   - put: true if put option
//
// Returns the OCC symbol string (e.g., "AAPL  250620C00200000") or ValidationError.
func buildOCCSymbol(underlying, expiration string, strike float64, call, put bool) (string, error) {
	// Validate underlying is not empty
	underlying = strings.TrimSpace(underlying)
	if underlying == "" {
		return "", apperr.NewValidationError("underlying is required", nil)
	}

	// Validate expiration date format
	expirationTime, err := time.Parse("2006-01-02", strings.TrimSpace(expiration))
	if err != nil {
		return "", apperr.NewValidationError("expiration must use YYYY-MM-DD format", err)
	}

	// Validate strike is positive
	if strike <= 0 {
		return "", apperr.NewValidationError("strike must be greater than zero", nil)
	}

	// Validate exactly one of call or put is true
	putCall, err := parsePutCall(call, put)
	if err != nil {
		return "", err
	}

	// Use the underlying orderbuilder.BuildOCCSymbol to construct the symbol
	symbol := orderbuilder.BuildOCCSymbol(underlying, expirationTime, strike, string(putCall))

	return symbol, nil
}
