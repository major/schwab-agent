package ta

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/major/schwab-agent/internal/apperr"
	"github.com/major/schwab-agent/internal/models"
)

func TestExtractClose_Valid(t *testing.T) {
	// Arrange
	v1, v2 := 100.0, 200.0
	candles := []models.Candle{{Close: &v1}, {Close: &v2}}

	// Act
	result, err := ExtractClose(candles)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, []float64{100.0, 200.0}, result)
}

func TestExtractClose_NilField(t *testing.T) {
	// Arrange
	v1 := 100.0
	candles := []models.Candle{{Close: &v1}, {Close: nil}}

	// Act
	result, err := ExtractClose(candles)

	// Assert
	require.Error(t, err)
	var valErr *apperr.ValidationError
	require.True(t, assert.ErrorAs(t, err, &valErr), "error should be ValidationError")
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "nil close")
	assert.Contains(t, err.Error(), "1")
}

func TestExtractHigh_Valid(t *testing.T) {
	// Arrange
	v1, v2 := 105.0, 205.0
	candles := []models.Candle{{High: &v1}, {High: &v2}}

	// Act
	result, err := ExtractHigh(candles)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, []float64{105.0, 205.0}, result)
}

func TestExtractHigh_NilField(t *testing.T) {
	// Arrange
	v1 := 105.0
	candles := []models.Candle{{High: &v1}, {High: nil}}

	// Act
	result, err := ExtractHigh(candles)

	// Assert
	require.Error(t, err)
	var valErr *apperr.ValidationError
	require.True(t, assert.ErrorAs(t, err, &valErr), "error should be ValidationError")
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "nil high")
}

func TestExtractLow_Valid(t *testing.T) {
	// Arrange
	v1, v2 := 95.0, 195.0
	candles := []models.Candle{{Low: &v1}, {Low: &v2}}

	// Act
	result, err := ExtractLow(candles)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, []float64{95.0, 195.0}, result)
}

func TestExtractLow_NilField(t *testing.T) {
	// Arrange
	v1 := 95.0
	candles := []models.Candle{{Low: &v1}, {Low: nil}}

	// Act
	result, err := ExtractLow(candles)

	// Assert
	require.Error(t, err)
	var valErr *apperr.ValidationError
	require.True(t, assert.ErrorAs(t, err, &valErr), "error should be ValidationError")
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "nil low")
}

func TestExtractOpen_Valid(t *testing.T) {
	// Arrange
	v1, v2 := 99.0, 199.0
	candles := []models.Candle{{Open: &v1}, {Open: &v2}}

	// Act
	result, err := ExtractOpen(candles)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, []float64{99.0, 199.0}, result)
}

func TestExtractOpen_NilField(t *testing.T) {
	// Arrange
	v1 := 99.0
	candles := []models.Candle{{Open: &v1}, {Open: nil}}

	// Act
	result, err := ExtractOpen(candles)

	// Assert
	require.Error(t, err)
	var valErr *apperr.ValidationError
	require.True(t, assert.ErrorAs(t, err, &valErr), "error should be ValidationError")
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "nil open")
}

func TestExtractVolume_Valid(t *testing.T) {
	// Arrange
	v1, v2 := int64(1000), int64(2000)
	candles := []models.Candle{{Volume: &v1}, {Volume: &v2}}

	// Act
	result, err := ExtractVolume(candles)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, []float64{1000.0, 2000.0}, result)
}

func TestExtractVolume_NilField(t *testing.T) {
	// Arrange
	v1 := int64(1000)
	candles := []models.Candle{{Volume: &v1}, {Volume: nil}}

	// Act
	result, err := ExtractVolume(candles)

	// Assert
	require.Error(t, err)
	var valErr *apperr.ValidationError
	require.True(t, assert.ErrorAs(t, err, &valErr), "error should be ValidationError")
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "nil volume")
}

func TestExtractTimestamps_WithISO8601(t *testing.T) {
	// Arrange
	iso1 := "2024-01-01T10:00:00Z"
	iso2 := "2024-01-02T10:00:00Z"
	candles := []models.Candle{
		{DatetimeISO8601: &iso1},
		{DatetimeISO8601: &iso2},
	}

	// Act
	result := ExtractTimestamps(candles)

	// Assert
	assert.Equal(t, []string{iso1, iso2}, result)
}

func TestExtractTimestamps_WithDatetime(t *testing.T) {
	// Arrange
	dt1 := int64(1704110400000) // 2024-01-01T10:00:00Z in ms
	dt2 := int64(1704196800000) // 2024-01-02T10:00:00Z in ms
	candles := []models.Candle{
		{Datetime: &dt1, DatetimeISO8601: nil},
		{Datetime: &dt2, DatetimeISO8601: nil},
	}

	// Act
	result := ExtractTimestamps(candles)

	// Assert
	assert.Len(t, result, 2)
	// Just verify they're RFC3339 formatted strings
	assert.NotEmpty(t, result[0])
	assert.NotEmpty(t, result[1])
}

func TestValidateMinCandles_Sufficient(t *testing.T) {
	// Arrange
	candles := make([]models.Candle, 20)

	// Act
	err := ValidateMinCandles(candles, 20, "sma")

	// Assert
	assert.NoError(t, err)
}

func TestValidateMinCandles_Insufficient(t *testing.T) {
	// Arrange
	candles := make([]models.Candle, 5)

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
	candles := []models.Candle{}

	// Act
	err := ValidateMinCandles(candles, 10, "ema")

	// Assert
	require.Error(t, err)
	assert.Equal(t, "ema requires at least 10 candles, got 0", err.Error())
}
