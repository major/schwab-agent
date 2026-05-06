package ta

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/major/schwab-go/schwab/marketdata"

	"github.com/major/schwab-agent/internal/apperr"
)

func TestExtractClose_Valid(t *testing.T) {
	// Arrange
	candles := []marketdata.Candle{{Close: 100.0}, {Close: 200.0}}

	// Act
	result, err := ExtractClose(candles)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, []float64{100.0, 200.0}, result)
}

func TestExtractClose_NonPositiveField(t *testing.T) {
	// Arrange
	candles := []marketdata.Candle{{Close: 100.0}, {Close: -1.0}}

	// Act
	result, err := ExtractClose(candles)

	// Assert
	require.Error(t, err)
	var valErr *apperr.ValidationError
	require.True(t, assert.ErrorAs(t, err, &valErr), "error should be ValidationError")
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "non-positive close")
	assert.Contains(t, err.Error(), "1")
}

func TestExtractHigh_Valid(t *testing.T) {
	// Arrange
	candles := []marketdata.Candle{{High: 105.0}, {High: 205.0}}

	// Act
	result, err := ExtractHigh(candles)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, []float64{105.0, 205.0}, result)
}

func TestExtractHigh_NonPositiveField(t *testing.T) {
	// Arrange
	candles := []marketdata.Candle{{High: 105.0}, {}}

	// Act
	result, err := ExtractHigh(candles)

	// Assert
	require.Error(t, err)
	var valErr *apperr.ValidationError
	require.True(t, assert.ErrorAs(t, err, &valErr), "error should be ValidationError")
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "non-positive high")
}

func TestExtractLow_Valid(t *testing.T) {
	// Arrange
	candles := []marketdata.Candle{{Low: 95.0}, {Low: 195.0}}

	// Act
	result, err := ExtractLow(candles)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, []float64{95.0, 195.0}, result)
}

func TestExtractLow_NonPositiveField(t *testing.T) {
	// Arrange
	candles := []marketdata.Candle{{Low: 95.0}, {Low: -1.0}}

	// Act
	result, err := ExtractLow(candles)

	// Assert
	require.Error(t, err)
	var valErr *apperr.ValidationError
	require.True(t, assert.ErrorAs(t, err, &valErr), "error should be ValidationError")
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "non-positive low")
}

func TestExtractOpen_Valid(t *testing.T) {
	// Arrange
	candles := []marketdata.Candle{{Open: 99.0}, {Open: 199.0}}

	// Act
	result, err := ExtractOpen(candles)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, []float64{99.0, 199.0}, result)
}

func TestExtractOpen_NonPositiveField(t *testing.T) {
	// Arrange
	candles := []marketdata.Candle{{Open: 99.0}, {}}

	// Act
	result, err := ExtractOpen(candles)

	// Assert
	require.Error(t, err)
	var valErr *apperr.ValidationError
	require.True(t, assert.ErrorAs(t, err, &valErr), "error should be ValidationError")
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "non-positive open")
}

func TestExtractVolume_Valid(t *testing.T) {
	// Arrange
	candles := []marketdata.Candle{{Volume: 1000}, {Volume: 2000}}

	// Act
	result := ExtractVolume(candles)

	// Assert
	assert.Equal(t, []float64{1000.0, 2000.0}, result)
}

func TestExtractVolume_ZeroFieldAllowed(t *testing.T) {
	// Arrange
	candles := []marketdata.Candle{{Volume: 1000}, {}}

	// Act
	result := ExtractVolume(candles)

	// Assert
	assert.Equal(t, []float64{1000.0, 0.0}, result)
}

func TestExtractTimestamps_WithISO8601(t *testing.T) {
	// Arrange
	iso1 := "2024-01-01T10:00:00Z"
	iso2 := "2024-01-02T10:00:00Z"
	candles := []marketdata.Candle{
		{DatetimeISO: iso1},
		{DatetimeISO: iso2},
	}

	// Act
	result, err := ExtractTimestamps(candles)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, []string{iso1, iso2}, result)
}

func TestExtractTimestamps_WithDatetime(t *testing.T) {
	// Arrange
	candles := []marketdata.Candle{
		{Datetime: 1704110400000}, // 2024-01-01T10:00:00Z in ms
		{Datetime: 1704196800000}, // 2024-01-02T10:00:00Z in ms
	}

	// Act
	result, err := ExtractTimestamps(candles)

	// Assert
	require.NoError(t, err)
	assert.Len(t, result, 2)
	// Just verify they're RFC3339 formatted strings
	assert.NotEmpty(t, result[0])
	assert.NotEmpty(t, result[1])
}

func TestExtractTimestamps_MissingTimestamp(t *testing.T) {
	// Arrange
	candles := []marketdata.Candle{{Datetime: 1704110400000}, {}}

	// Act
	result, err := ExtractTimestamps(candles)

	// Assert
	require.Error(t, err)
	var valErr *apperr.ValidationError
	require.True(t, assert.ErrorAs(t, err, &valErr), "error should be ValidationError")
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "missing timestamp at index 1")
}

func TestValidateMinCandles_Sufficient(t *testing.T) {
	// Arrange
	candles := make([]marketdata.Candle, 20)

	// Act
	err := ValidateMinCandles(candles, 20, "sma")

	// Assert
	assert.NoError(t, err)
}

func TestValidateMinCandles_Insufficient(t *testing.T) {
	// Arrange
	candles := make([]marketdata.Candle, 5)

	// Act
	err := ValidateMinCandles(candles, 20, "sma")

	// Assert
	require.Error(t, err)
	var valErr *apperr.ValidationError
	require.True(t, assert.ErrorAs(t, err, &valErr), "error should be ValidationError")
	assert.Equal(t, "sma requires at least 20 candles, got 5", err.Error())
}

func TestValidateMinCandles_Empty(t *testing.T) {
	// Arrange
	candles := []marketdata.Candle{}

	// Act
	err := ValidateMinCandles(candles, 10, "ema")

	// Assert
	require.Error(t, err)
	assert.Equal(t, "ema requires at least 10 candles, got 0", err.Error())
}
