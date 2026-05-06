package ta

import (
	"fmt"
	"strconv"

	"github.com/major/schwab-agent/internal/apperr"
)

const (
	intervalDaily  = "daily"
	intervalWeekly = "weekly"
	interval1Min   = "1min"
	interval5Min   = "5min"
	interval15Min  = "15min"
	interval30Min  = "30min"

	historyPeriodTypeDay  = "day"
	historyPeriodTypeYear = "year"

	historyFrequencyTypeDaily  = "daily"
	historyFrequencyTypeMinute = "minute"
	historyFrequencyTypeWeekly = "weekly"

	weeksPerYear = 52

	// dailyLookbackSafetyCandles is added before sizing daily Schwab API period
	// requests. The daily endpoint can return fewer candles than a plain 1-year
	// request suggests because of holidays, partial current sessions, and
	// symbol-level data gaps, so keep a 10-candle margin before reducing this;
	// IntervalToHistoryParams has the request-sizing rationale where this is used.
	dailyLookbackSafetyCandles = 10

	// regularSessionMinutesPerDay is the number of 1-minute candles in a regular
	// US market session (9:30-16:00 = 6.5 hours). Extended hours are excluded;
	// revisit if the API is configured to return extended-hours candles.
	regularSessionMinutesPerDay = 390

	// maxYearPeriod is the largest "year" period the Schwab API accepts.
	maxYearPeriod = 20
	// maxDayPeriod is the largest "day" period the Schwab API accepts.
	maxDayPeriod = 10
)

// validYearPeriods lists the period values the Schwab API documents for periodType "year".
// Sending values not in this list may be silently rejected or misinterpreted.
func validYearPeriods() []int {
	return []int{1, 2, 3, 5, 10, 15, 20}
}

// validDayPeriods lists the period values the Schwab API documents for periodType "day".
func validDayPeriods() []int {
	return []int{1, 2, 3, 4, 5, 10}
}

// ceilDiv returns the ceiling of a/b for positive integers.
func ceilDiv(a, b int) int {
	return (a + b - 1) / b
}

// nextValidPeriod returns the smallest documented API period value >= n.
// If n exceeds all valid periods, returns the largest valid period.
func nextValidPeriod(n int, valid []int) int {
	for _, v := range valid {
		if v >= n {
			return v
		}
	}
	return valid[len(valid)-1]
}

// maxCandlesForInterval returns the theoretical maximum number of candles
// the Schwab API can return for a given interval at the longest documented period.
// Returns 0 for unknown intervals.
func maxCandlesForInterval(interval string) int {
	switch interval {
	case intervalDaily:
		return maxYearPeriod * tradingDaysPerYear
	case intervalWeekly:
		return maxYearPeriod * weeksPerYear
	case interval1Min:
		return maxDayPeriod * regularSessionMinutesPerDay
	case interval5Min:
		return maxDayPeriod * (regularSessionMinutesPerDay / 5)
	case interval15Min:
		return maxDayPeriod * (regularSessionMinutesPerDay / 15)
	case interval30Min:
		return maxDayPeriod * (regularSessionMinutesPerDay / 30)
	default:
		return 0
	}
}

// IntervalToHistoryParams maps a human interval string to PriceHistory query params.
// requiredCandles controls how much history to fetch: the period is scaled up so the
// API returns at least that many candles. Pass 0 to use the default (minimum) lookback.
//
// Period values are rounded up to the next value in the Schwab API's documented set
// (years: 1,2,3,5,10,15,20; days: 1,2,3,4,5,10) to avoid sending unsupported values.
//
// Returns a ValidationError if requiredCandles exceeds the API's maximum capacity for
// the given interval, or if the interval is not recognized.
func IntervalToHistoryParams(
	interval string,
	requiredCandles int,
) (string, string, string, string, error) {
	// Fail early when the request exceeds the API's maximum history depth.
	// Without this check, the period gets silently capped and the post-fetch
	// validation produces a confusing "got N candles" error that blames the
	// symbol's history rather than the API limit.
	if mc := maxCandlesForInterval(interval); mc > 0 && requiredCandles > mc {
		return "", "", "", "", apperr.NewValidationError(
			fmt.Sprintf(
				"indicator requires %d candles but %s interval supports at most %d (Schwab API limit)",
				requiredCandles, interval, mc,
			),
			nil,
		)
	}

	switch interval {
	case intervalDaily:
		// A one-calendar-year daily Schwab request can return fewer than 252
		// candles depending on holidays, current market timing, and symbol-level
		// gaps. Add a small sizing buffer before rounding to documented periods so
		// callers that need a complete 252-candle trading-year window request 2
		// years, while smaller daily indicators still use the minimum 1-year fetch.
		years := max(1, ceilDiv(requiredCandles+dailyLookbackSafetyCandles, tradingDaysPerYear))
		years = nextValidPeriod(years, validYearPeriods())
		return historyPeriodTypeYear, strconv.Itoa(years), historyFrequencyTypeDaily, "1", nil
	case intervalWeekly:
		years := max(1, ceilDiv(requiredCandles, weeksPerYear))
		years = nextValidPeriod(years, validYearPeriods())
		return historyPeriodTypeYear, strconv.Itoa(years), historyFrequencyTypeWeekly, "1", nil
	case interval1Min:
		days := max(1, ceilDiv(requiredCandles, regularSessionMinutesPerDay))
		days = nextValidPeriod(days, validDayPeriods())
		return historyPeriodTypeDay, strconv.Itoa(days), historyFrequencyTypeMinute, "1", nil
	case interval5Min:
		days := max(1, ceilDiv(requiredCandles, regularSessionMinutesPerDay/5))
		days = nextValidPeriod(days, validDayPeriods())
		return historyPeriodTypeDay, strconv.Itoa(days), historyFrequencyTypeMinute, "5", nil
	case interval15Min:
		days := max(1, ceilDiv(requiredCandles, regularSessionMinutesPerDay/15))
		days = nextValidPeriod(days, validDayPeriods())
		return historyPeriodTypeDay, strconv.Itoa(days), historyFrequencyTypeMinute, "15", nil
	case interval30Min:
		days := max(1, ceilDiv(requiredCandles, regularSessionMinutesPerDay/30))
		days = nextValidPeriod(days, validDayPeriods())
		return historyPeriodTypeDay, strconv.Itoa(days), historyFrequencyTypeMinute, "30", nil
	default:
		return "", "", "", "", apperr.NewValidationError(
			"unsupported interval: "+interval,
			nil,
		)
	}
}
