package ta

import (
	"fmt"
	"time"

	"github.com/major/schwab-agent/internal/apperr"
	"github.com/major/schwab-agent/internal/models"
)

// ExtractClose extracts close prices from candles. Returns ValidationError if any Close is nil.
func ExtractClose(candles []models.Candle) ([]float64, error) {
	result := make([]float64, len(candles))
	for i, c := range candles {
		if c.Close == nil {
			return nil, apperr.NewValidationError(
				fmt.Sprintf("nil close at index %d", i),
				nil,
			)
		}
		result[i] = *c.Close
	}
	return result, nil
}

// ExtractHigh extracts high prices from candles. Returns ValidationError if any High is nil.
func ExtractHigh(candles []models.Candle) ([]float64, error) {
	result := make([]float64, len(candles))
	for i, c := range candles {
		if c.High == nil {
			return nil, apperr.NewValidationError(
				fmt.Sprintf("nil high at index %d", i),
				nil,
			)
		}
		result[i] = *c.High
	}
	return result, nil
}

// ExtractLow extracts low prices from candles. Returns ValidationError if any Low is nil.
func ExtractLow(candles []models.Candle) ([]float64, error) {
	result := make([]float64, len(candles))
	for i, c := range candles {
		if c.Low == nil {
			return nil, apperr.NewValidationError(
				fmt.Sprintf("nil low at index %d", i),
				nil,
			)
		}
		result[i] = *c.Low
	}
	return result, nil
}

// ExtractOpen extracts open prices from candles. Returns ValidationError if any Open is nil.
func ExtractOpen(candles []models.Candle) ([]float64, error) {
	result := make([]float64, len(candles))
	for i, c := range candles {
		if c.Open == nil {
			return nil, apperr.NewValidationError(
				fmt.Sprintf("nil open at index %d", i),
				nil,
			)
		}
		result[i] = *c.Open
	}
	return result, nil
}

// ExtractVolume extracts volume from candles as float64. Returns ValidationError if any Volume is nil.
func ExtractVolume(candles []models.Candle) ([]float64, error) {
	result := make([]float64, len(candles))
	for i, c := range candles {
		if c.Volume == nil {
			return nil, apperr.NewValidationError(
				fmt.Sprintf("nil volume at index %d", i),
				nil,
			)
		}
		result[i] = float64(*c.Volume)
	}
	return result, nil
}

// ExtractTimestamps extracts ISO8601 timestamps from candles.
// Uses DatetimeISO8601 if available, else formats Datetime (unix ms) as RFC3339.
func ExtractTimestamps(candles []models.Candle) []string {
	result := make([]string, len(candles))
	for i, c := range candles {
		if c.DatetimeISO8601 != nil {
			result[i] = *c.DatetimeISO8601
		} else if c.Datetime != nil {
			// Convert unix milliseconds to time.Time
			t := time.UnixMilli(*c.Datetime).UTC()
			result[i] = t.Format(time.RFC3339)
		}
	}
	return result
}

// ValidateMinCandles returns a ValidationError if len(candles) < required.
// Error message format: "sma requires at least 20 candles, got 5".
func ValidateMinCandles(candles []models.Candle, required int, indicator string) error {
	if len(candles) < required {
		return apperr.NewValidationError(
			fmt.Sprintf("%s requires at least %d candles, got %d", indicator, required, len(candles)),
			nil,
		)
	}
	return nil
}
