// Package ta provides technical analysis indicator calculations.
package ta

import (
	"fmt"

	schwabErrors "github.com/major/schwab-agent/internal/errors"
)

// VWAP computes Volume Weighted Average Price.
// Returns a slice of the same length as input with cumulative VWAP values.
// Returns ValidationError if slices have mismatched lengths, are empty, or have all-zero volumes.
func VWAP(highs, lows, closes, volumes []float64) ([]float64, error) {
	// Validate input lengths
	if len(highs) == 0 || len(lows) == 0 || len(closes) == 0 || len(volumes) == 0 {
		return nil, schwabErrors.NewValidationError(
			"VWAP requires non-empty slices",
			nil,
		)
	}

	if len(highs) != len(lows) || len(highs) != len(closes) || len(highs) != len(volumes) {
		return nil, schwabErrors.NewValidationError(
			fmt.Sprintf("VWAP requires equal-length slices: got highs=%d, lows=%d, closes=%d, volumes=%d",
				len(highs), len(lows), len(closes), len(volumes)),
			nil,
		)
	}

	result := make([]float64, len(highs))
	var cumulativeTV float64
	var cumulativeVol float64

	for i := 0; i < len(highs); i++ {
		// Calculate typical price: (H + L + C) / 3
		typicalPrice := (highs[i] + lows[i] + closes[i]) / 3.0

		// Accumulate typical price * volume and volume
		cumulativeTV += typicalPrice * volumes[i]
		cumulativeVol += volumes[i]

		// Check if cumulative volume is zero (all volumes so far are zero)
		if cumulativeVol == 0 {
			return nil, schwabErrors.NewValidationError(
				"VWAP requires non-zero volume data",
				nil,
			)
		}

		// Calculate VWAP as cumulative TV / cumulative volume
		result[i] = cumulativeTV / cumulativeVol
	}

	return result, nil
}
