package ta

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/major/schwab-agent/internal/apperr"
)

func TestSMA_Correct(t *testing.T) {
	// Arrange
	closes := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	// Act
	result, err := SMA(closes, 3)
	// Assert
	require.NoError(t, err)
	assert.Len(t, result, 8)
	assert.InDelta(t, 2.0, result[0], 1e-6) // (1+2+3)/3
	assert.InDelta(t, 9.0, result[7], 1e-6) // (8+9+10)/3
}

func TestSMA_InvalidPeriod(t *testing.T) {
	// Arrange & Act
	_, err := SMA([]float64{1, 2, 3}, 0)
	// Assert
	require.Error(t, err)
	var valErr *apperr.ValidationError
	assert.ErrorAs(t, err, &valErr)
}

func TestSMA_InsufficientData(t *testing.T) {
	// Arrange & Act
	_, err := SMA([]float64{1, 2, 3}, 5)
	// Assert
	require.Error(t, err)
	var valErr *apperr.ValidationError
	assert.ErrorAs(t, err, &valErr)
}

func TestEMA_Valid(t *testing.T) {
	// Arrange
	closes := make([]float64, 20)
	for i := range closes {
		closes[i] = float64(i + 1)
	}
	// Act
	result, err := EMA(closes, 5)
	// Assert
	require.NoError(t, err)
	assert.NotEmpty(t, result)
	// All returned values should be non-zero (leading zeros stripped)
	for _, v := range result {
		assert.NotZero(t, v)
	}
}

func TestEMA_InvalidPeriod(t *testing.T) {
	// Arrange & Act
	_, err := EMA([]float64{1, 2, 3}, 0)
	// Assert
	require.Error(t, err)
	var valErr *apperr.ValidationError
	assert.ErrorAs(t, err, &valErr)
}

func TestEMA_InsufficientData(t *testing.T) {
	// Arrange & Act
	_, err := EMA([]float64{1, 2, 3}, 5)
	// Assert
	require.Error(t, err)
	var valErr *apperr.ValidationError
	assert.ErrorAs(t, err, &valErr)
}

func TestRSI_ValidRange(t *testing.T) {
	// Arrange: Generate 30 alternating up/down closes
	closes := make([]float64, 30)
	for i := range closes {
		if i%2 == 0 {
			closes[i] = 100.0 + float64(i)*0.5
		} else {
			closes[i] = 100.0 - float64(i)*0.5
		}
	}
	// Act
	result, err := RSI(closes, 14)
	// Assert
	require.NoError(t, err)
	assert.NotEmpty(t, result)
	for _, v := range result {
		assert.GreaterOrEqual(t, v, 0.0)
		assert.LessOrEqual(t, v, 100.0)
	}
}

func TestRSI_InvalidPeriod(t *testing.T) {
	// Arrange
	closes := make([]float64, 20)
	// Act
	_, err := RSI(closes, 1)
	// Assert
	require.Error(t, err)
	var valErr *apperr.ValidationError
	assert.ErrorAs(t, err, &valErr)
}

func TestRSI_InsufficientData(t *testing.T) {
	// Arrange & Act
	_, err := RSI([]float64{1, 2, 3}, 5)
	// Assert
	require.Error(t, err)
	var valErr *apperr.ValidationError
	assert.ErrorAs(t, err, &valErr)
}
