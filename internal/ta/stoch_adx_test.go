package ta

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/major/schwab-agent/internal/apperr"
)

func TestStochastic_ValidRange(t *testing.T) {
	// Arrange: 50 bars of OHLC
	n := 50
	highs := make([]float64, n)
	lows := make([]float64, n)
	closes := make([]float64, n)
	for i := range highs {
		closes[i] = 100.0 + float64(i)*0.5
		highs[i] = closes[i] + 2.0
		lows[i] = closes[i] - 2.0
	}

	// Act: kPeriod=14, smoothK=3, dPeriod=3
	slowK, slowD, err := Stochastic(highs, lows, closes, 14, 3, 3)

	// Assert
	require.NoError(t, err)
	assert.NotEmpty(t, slowK)
	assert.Equal(t, len(slowK), len(slowD), "slowK and slowD must have same length")
	for _, v := range slowK {
		assert.GreaterOrEqual(t, v, 0.0)
		assert.LessOrEqual(t, v, 100.0)
	}
	for _, v := range slowD {
		assert.GreaterOrEqual(t, v, 0.0)
		assert.LessOrEqual(t, v, 100.0)
	}
}

func TestStochastic_MismatchedLengths(t *testing.T) {
	highs := make([]float64, 10)
	lows := make([]float64, 8)
	closes := make([]float64, 10)
	_, _, err := Stochastic(highs, lows, closes, 14, 3, 3)
	require.Error(t, err)
	var valErr *apperr.ValidationError
	assert.ErrorAs(t, err, &valErr)
}

func TestStochastic_InvalidPeriod(t *testing.T) {
	highs := make([]float64, 50)
	lows := make([]float64, 50)
	closes := make([]float64, 50)
	_, _, err := Stochastic(highs, lows, closes, 0, 3, 3)
	require.Error(t, err)
	var valErr *apperr.ValidationError
	assert.ErrorAs(t, err, &valErr)
}

func TestADX_ThreeAlignedSlices(t *testing.T) {
	// Arrange: 100 bars of OHLC with realistic up/down movement
	// ADX needs directional movement to produce non-zero values
	n := 100
	highs := make([]float64, n)
	lows := make([]float64, n)
	closes := make([]float64, n)
	for i := range highs {
		// Create zigzag pattern with both up and down moves
		base := 100.0 + float64(i%20)*0.5 // Oscillates every 20 bars
		closes[i] = base
		if i%20 < 10 {
			// Uptrend: higher highs and higher lows
			highs[i] = base + 2.0
			lows[i] = base - 0.5
		} else {
			// Downtrend: lower highs and lower lows
			highs[i] = base + 0.5
			lows[i] = base - 2.0
		}
	}

	// Act
	adx, plusDI, minusDI, err := ADX(highs, lows, closes, 14)

	// Assert
	require.NoError(t, err)
	assert.NotEmpty(t, adx)
	assert.Equal(t, len(adx), len(plusDI), "adx and plusDI must have same length")
	assert.Equal(t, len(adx), len(minusDI), "adx and minusDI must have same length")
	for _, v := range adx {
		assert.GreaterOrEqual(t, v, 0.0)
		assert.LessOrEqual(t, v, 100.0)
	}
	for _, v := range plusDI {
		assert.GreaterOrEqual(t, v, 0.0)
		assert.LessOrEqual(t, v, 100.0)
	}
	for _, v := range minusDI {
		assert.GreaterOrEqual(t, v, 0.0)
		assert.LessOrEqual(t, v, 100.0)
	}
}

func TestADX_InvalidPeriod(t *testing.T) {
	highs := make([]float64, 50)
	lows := make([]float64, 50)
	closes := make([]float64, 50)
	_, _, _, err := ADX(highs, lows, closes, 0)
	require.Error(t, err)
	var valErr *apperr.ValidationError
	assert.ErrorAs(t, err, &valErr)
}

func TestADX_MismatchedLengths(t *testing.T) {
	highs := make([]float64, 10)
	lows := make([]float64, 8)
	closes := make([]float64, 10)
	_, _, _, err := ADX(highs, lows, closes, 14)
	require.Error(t, err)
	var valErr *apperr.ValidationError
	assert.ErrorAs(t, err, &valErr)
}
