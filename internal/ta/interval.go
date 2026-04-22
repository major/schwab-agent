package ta

import (
	"github.com/major/schwab-agent/internal/apperr"
)

// IntervalToHistoryParams maps a human interval string to PriceHistory query params.
// Returns (periodType, period, freqType, freq string, err error)
// Supported: daily, weekly, 1min, 5min, 15min, 30min
// Mapping:
//   daily  -> year, 1, daily, 1
//   weekly -> year, 1, weekly, 1
//   1min   -> day, 10, minute, 1
//   5min   -> day, 10, minute, 5
//   15min  -> day, 10, minute, 15
//   30min  -> day, 10, minute, 30
// Returns ValidationError for unsupported intervals.
func IntervalToHistoryParams(interval string) (periodType, period, freqType, freq string, err error) {
	switch interval {
	case "daily":
		return "year", "1", "daily", "1", nil
	case "weekly":
		return "year", "1", "weekly", "1", nil
	case "1min":
		return "day", "10", "minute", "1", nil
	case "5min":
		return "day", "10", "minute", "5", nil
	case "15min":
		return "day", "10", "minute", "15", nil
	case "30min":
		return "day", "10", "minute", "30", nil
	default:
		return "", "", "", "", apperr.NewValidationError(
			"unsupported interval: "+interval,
			nil,
		)
	}
}
