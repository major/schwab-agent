// Package ta provides technical analysis indicator wrappers around go-talib.
package ta

import (
	"fmt"

	"github.com/markcheno/go-talib"

	"github.com/major/schwab-agent/internal/apperr"
)

// SMA computes Simple Moving Average. Strips leading zeros from output.
// Returns ValidationError if period <= 0 or len(closes) < period.
func SMA(closes []float64, period int) ([]float64, error) {
	if period <= 0 {
		return nil, apperr.NewValidationError(
			fmt.Sprintf("sma period must be > 0, got %d", period),
			nil,
		)
	}
	if len(closes) < period {
		return nil, apperr.NewValidationError(
			fmt.Sprintf("sma requires at least %d values, got %d", period, len(closes)),
			nil,
		)
	}
	result := talib.Sma(closes, period)
	return StripLeadingZeros(result), nil
}

// EMA computes Exponential Moving Average. Strips leading zeros from output.
// Returns ValidationError if period <= 0 or len(closes) < period.
func EMA(closes []float64, period int) ([]float64, error) {
	if period <= 0 {
		return nil, apperr.NewValidationError(
			fmt.Sprintf("ema period must be > 0, got %d", period),
			nil,
		)
	}
	if len(closes) < period {
		return nil, apperr.NewValidationError(
			fmt.Sprintf("ema requires at least %d values, got %d", period, len(closes)),
			nil,
		)
	}
	result := talib.Ema(closes, period)
	return StripLeadingZeros(result), nil
}

// RSI computes Relative Strength Index. Strips leading zeros from output.
// Returns ValidationError if period <= 1 or len(closes) < period+1.
func RSI(closes []float64, period int) ([]float64, error) {
	if period <= 1 {
		return nil, apperr.NewValidationError(
			fmt.Sprintf("rsi period must be > 1, got %d", period),
			nil,
		)
	}
	if len(closes) < period+1 {
		return nil, apperr.NewValidationError(
			fmt.Sprintf("rsi requires at least %d values, got %d", period+1, len(closes)),
			nil,
		)
	}
	result := talib.Rsi(closes, period)
	return StripLeadingZeros(result), nil
}
