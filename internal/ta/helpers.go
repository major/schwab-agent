package ta

import (
	"fmt"
	"time"

	"github.com/major/schwab-go/schwab/marketdata"

	"github.com/major/schwab-agent/internal/apperr"
)

// ExtractClose extracts close prices from candles.
func ExtractClose(candles []marketdata.Candle) ([]float64, error) {
	result := make([]float64, len(candles))
	for i, c := range candles {
		if c.Close <= 0 {
			return nil, apperr.NewValidationError(
				fmt.Sprintf("non-positive close at index %d", i),
				nil,
			)
		}
		result[i] = c.Close
	}
	return result, nil
}

// ExtractHigh extracts high prices from candles.
func ExtractHigh(candles []marketdata.Candle) ([]float64, error) {
	result := make([]float64, len(candles))
	for i, c := range candles {
		if c.High <= 0 {
			return nil, apperr.NewValidationError(
				fmt.Sprintf("non-positive high at index %d", i),
				nil,
			)
		}
		result[i] = c.High
	}
	return result, nil
}

// ExtractLow extracts low prices from candles.
func ExtractLow(candles []marketdata.Candle) ([]float64, error) {
	result := make([]float64, len(candles))
	for i, c := range candles {
		if c.Low <= 0 {
			return nil, apperr.NewValidationError(
				fmt.Sprintf("non-positive low at index %d", i),
				nil,
			)
		}
		result[i] = c.Low
	}
	return result, nil
}

// ExtractOpen extracts open prices from candles.
func ExtractOpen(candles []marketdata.Candle) ([]float64, error) {
	result := make([]float64, len(candles))
	for i, c := range candles {
		if c.Open <= 0 {
			return nil, apperr.NewValidationError(
				fmt.Sprintf("non-positive open at index %d", i),
				nil,
			)
		}
		result[i] = c.Open
	}
	return result, nil
}

// ExtractVolume extracts volume from candles as float64. Zero-volume candles are
// valid for thinly traded symbols or non-regular sessions, so preserve them and
// let downstream indicators decide how to handle all-zero volume series.
func ExtractVolume(candles []marketdata.Candle) []float64 {
	result := make([]float64, len(candles))
	for i, c := range candles {
		result[i] = float64(c.Volume)
	}
	return result
}

// ExtractTimestamps extracts ISO8601 timestamps from candles.
// Uses DatetimeISO if available, else formats Datetime (unix ms) as RFC3339.
func ExtractTimestamps(candles []marketdata.Candle) ([]string, error) {
	result := make([]string, len(candles))
	for i, c := range candles {
		switch {
		case c.DatetimeISO != "":
			result[i] = c.DatetimeISO
		case c.Datetime != 0:
			// Convert unix milliseconds to time.Time
			t := time.UnixMilli(c.Datetime).UTC()
			result[i] = t.Format(time.RFC3339)
		default:
			return nil, apperr.NewValidationError(
				fmt.Sprintf("missing timestamp at index %d", i),
				nil,
			)
		}
	}
	return result, nil
}

// ValidateMinCandles returns a ValidationError if len(candles) < required.
// Error message format: "sma requires at least 20 candles, got 5".
func ValidateMinCandles(candles []marketdata.Candle, required int, indicator string) error {
	if len(candles) < required {
		return apperr.NewValidationError(
			fmt.Sprintf("%s requires at least %d candles, got %d", indicator, required, len(candles)),
			nil,
		)
	}
	return nil
}
