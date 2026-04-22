package ta

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	schwabErrors "github.com/major/schwab-agent/internal/errors"
)

func TestATR_ValidValues(t *testing.T) {
	// Arrange: 30 bars of OHLC data
	n := 30
	highs := make([]float64, n)
	lows := make([]float64, n)
	closes := make([]float64, n)
	for i := range highs {
		closes[i] = 100.0 + float64(i)*0.5
		highs[i] = closes[i] + 1.0
		lows[i] = closes[i] - 1.0
	}

	// Act
	result, err := ATR(highs, lows, closes, 14)

	// Assert
	require.NoError(t, err)
	assert.NotEmpty(t, result)
	for _, v := range result {
		assert.Greater(t, v, 0.0, "ATR values should be positive")
	}
}

func TestATR_MismatchedLengths(t *testing.T) {
	highs := make([]float64, 10)
	lows := make([]float64, 8)
	closes := make([]float64, 10)
	_, err := ATR(highs, lows, closes, 14)
	require.Error(t, err)
	var valErr *schwabErrors.ValidationError
	assert.ErrorAs(t, err, &valErr)
	assert.Contains(t, err.Error(), "length")
}

func TestATR_InvalidPeriod(t *testing.T) {
	highs := make([]float64, 20)
	lows := make([]float64, 20)
	closes := make([]float64, 20)
	_, err := ATR(highs, lows, closes, 0)
	require.Error(t, err)
	var valErr *schwabErrors.ValidationError
	assert.ErrorAs(t, err, &valErr)
}

func TestBBands_CorrectOrdering(t *testing.T) {
	// Arrange: 30 close prices
	closes := make([]float64, 30)
	for i := range closes {
		closes[i] = 100.0 + float64(i)*0.1
	}

	// Act
	upper, middle, lower, err := BBands(closes, 20, 2.0)

	// Assert
	require.NoError(t, err)
	assert.NotEmpty(t, upper)
	assert.Equal(t, len(upper), len(middle), "upper and middle must have same length")
	assert.Equal(t, len(upper), len(lower), "upper and lower must have same length")
	for i := range upper {
		assert.Greater(t, upper[i], middle[i], "upper[%d] should be > middle[%d]", i, i)
		assert.Greater(t, middle[i], lower[i], "middle[%d] should be > lower[%d]", i, i)
	}
}

func TestBBands_MiddleEqualsMA(t *testing.T) {
	// Arrange: 30 close prices
	closes := make([]float64, 30)
	for i := range closes {
		closes[i] = float64(i + 1)
	}

	// Act: BBands with period=20, stdDev=2.0
	_, middle, _, err := BBands(closes, 20, 2.0)
	require.NoError(t, err)

	// Also compute SMA for comparison
	sma, err2 := SMA(closes, 20)
	require.NoError(t, err2)

	// Assert: middle band should equal SMA
	assert.Equal(t, len(sma), len(middle))
	for i := range middle {
		assert.InDelta(t, sma[i], middle[i], 1e-6, "middle[%d] should equal SMA[%d]", i, i)
	}
}

func TestBBands_InvalidStdDev(t *testing.T) {
	closes := make([]float64, 30)
	for i := range closes {
		closes[i] = float64(i + 1)
	}
	_, _, _, err := BBands(closes, 20, 0.0)
	require.Error(t, err)
	var valErr *schwabErrors.ValidationError
	assert.ErrorAs(t, err, &valErr)
}

func TestBBands_InvalidPeriod(t *testing.T) {
	closes := make([]float64, 30)
	_, _, _, err := BBands(closes, 0, 2.0)
	require.Error(t, err)
	var valErr *schwabErrors.ValidationError
	assert.ErrorAs(t, err, &valErr)
}
