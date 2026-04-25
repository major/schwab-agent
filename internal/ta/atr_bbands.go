package ta

import (
	"fmt"

	"github.com/markcheno/go-talib"

	"github.com/major/schwab-agent/internal/apperr"
)

// ATR computes Average True Range from OHLC data.
// All three input slices must have the same length.
// Returns ValidationError if period <= 0, slices have mismatched lengths, or insufficient data.
func ATR(highs, lows, closes []float64, period int) ([]float64, error) {
	if period <= 0 {
		return nil, apperr.NewValidationError(
			fmt.Sprintf("atr period must be > 0, got %d", period),
			nil,
		)
	}
	if len(highs) != len(lows) || len(highs) != len(closes) {
		return nil, apperr.NewValidationError(
			fmt.Sprintf("atr: mismatched slice lengths: highs=%d, lows=%d, closes=%d", len(highs), len(lows), len(closes)),
			nil,
		)
	}
	if len(closes) < period+1 {
		return nil, apperr.NewValidationError(
			fmt.Sprintf("atr requires at least %d values, got %d", period+1, len(closes)),
			nil,
		)
	}
	result := talib.Atr(highs, lows, closes, period)
	return StripLeadingZeros(result), nil
}

// BBands computes Bollinger Bands (upper, middle, lower) using SMA.
// stdDev is the number of standard deviations for both bands (must be > 0).
// Returns (upper, middle, lower []float64, err error) with leading zeros stripped and lengths aligned.
func BBands(closes []float64, period int, stdDev float64) (upper, middle, lower []float64, err error) {
	if period <= 0 {
		return nil, nil, nil, apperr.NewValidationError(
			fmt.Sprintf("bbands period must be > 0, got %d", period),
			nil,
		)
	}
	if stdDev <= 0 {
		return nil, nil, nil, apperr.NewValidationError(
			fmt.Sprintf("bbands stdDev must be > 0, got %g", stdDev),
			nil,
		)
	}
	if len(closes) < period {
		return nil, nil, nil, apperr.NewValidationError(
			fmt.Sprintf("bbands requires at least %d values, got %d", period, len(closes)),
			nil,
		)
	}

	// maType=0 means SMA (hardcoded per spec - do not expose maType to users)
	rawUpper, rawMiddle, rawLower := talib.BBands(closes, period, stdDev, stdDev, 0)

	upper = StripLeadingZeros(rawUpper)
	middle = StripLeadingZeros(rawMiddle)
	lower = StripLeadingZeros(rawLower)

	// Align to shortest length
	minLen := min(len(lower), min(len(middle), len(upper)))
	if len(upper) > minLen {
		upper = upper[len(upper)-minLen:]
	}
	if len(middle) > minLen {
		middle = middle[len(middle)-minLen:]
	}
	if len(lower) > minLen {
		lower = lower[len(lower)-minLen:]
	}

	return upper, middle, lower, nil
}
