package ta

import (
	"fmt"

	"github.com/markcheno/go-talib"

	"github.com/major/schwab-agent/internal/apperr"
)

// MACD computes Moving Average Convergence/Divergence.
// Returns (macd, signal, histogram []float64, err error).
// All three output slices are zero-stripped and aligned to the shortest length.
// fastPeriod must be < slowPeriod. All periods must be > 0.
// len(closes) must be >= slowPeriod + signalPeriod.
func MACD(closes []float64, fastPeriod, slowPeriod, signalPeriod int) (macdLine, signalLine, histogram []float64, err error) {
	if fastPeriod <= 0 || slowPeriod <= 0 || signalPeriod <= 0 {
		return nil, nil, nil, apperr.NewValidationError(
			"macd: all periods must be > 0",
			nil,
		)
	}
	if fastPeriod >= slowPeriod {
		return nil, nil, nil, apperr.NewValidationError(
			fmt.Sprintf("macd: fast period (%d) must be less than slow period (%d)", fastPeriod, slowPeriod),
			nil,
		)
	}
	required := slowPeriod + signalPeriod
	if len(closes) < required {
		return nil, nil, nil, apperr.NewValidationError(
			fmt.Sprintf("macd requires at least %d values, got %d", required, len(closes)),
			nil,
		)
	}

	rawMACD, rawSignal, rawHistogram := talib.Macd(closes, fastPeriod, slowPeriod, signalPeriod)

	strippedMACD := StripLeadingZeros(rawMACD)
	strippedSignal := StripLeadingZeros(rawSignal)
	strippedHistogram := StripLeadingZeros(rawHistogram)

	// Align to shortest length to keep slices in sync
	minLen := min(len(strippedHistogram), min(len(strippedSignal), len(strippedMACD)))

	// Trim from the front to align (take last minLen elements)
	if len(strippedMACD) > minLen {
		strippedMACD = strippedMACD[len(strippedMACD)-minLen:]
	}
	if len(strippedSignal) > minLen {
		strippedSignal = strippedSignal[len(strippedSignal)-minLen:]
	}
	if len(strippedHistogram) > minLen {
		strippedHistogram = strippedHistogram[len(strippedHistogram)-minLen:]
	}

	return strippedMACD, strippedSignal, strippedHistogram, nil
}
