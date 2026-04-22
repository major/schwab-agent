package ta

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	schwabErrors "github.com/major/schwab-agent/internal/errors"
)

func TestMACD_ThreeAlignedSlices(t *testing.T) {
	// Arrange: 50-element close price series
	closes := make([]float64, 50)
	for i := range closes {
		closes[i] = 100.0 + float64(i)*0.5
	}

	// Act
	macdLine, signal, histogram, err := MACD(closes, 12, 26, 9)

	// Assert
	require.NoError(t, err)
	assert.NotEmpty(t, macdLine)
	assert.Equal(t, len(macdLine), len(signal), "macd and signal must have same length")
	assert.Equal(t, len(macdLine), len(histogram), "macd and histogram must have same length")
	// No leading zeros
	for i, v := range macdLine {
		if i == 0 {
			assert.NotZero(t, v, "first macd value should not be zero after stripping")
		}
	}
}

func TestMACD_HistogramEquality(t *testing.T) {
	// Arrange
	closes := make([]float64, 60)
	for i := range closes {
		closes[i] = 100.0 + math.Sin(float64(i)*0.3)*5.0
	}

	// Act
	macdLine, signal, histogram, err := MACD(closes, 12, 26, 9)

	// Assert
	require.NoError(t, err)
	for i := range histogram {
		expected := macdLine[i] - signal[i]
		assert.InDelta(t, expected, histogram[i], 1e-6, "histogram[%d] should equal macd[%d] - signal[%d]", i, i, i)
	}
}

func TestMACD_RejectsFastGeSlow(t *testing.T) {
	closes := make([]float64, 50)
	for i := range closes {
		closes[i] = float64(i + 1)
	}

	// fast >= slow should fail
	_, _, _, err := MACD(closes, 26, 12, 9)
	require.Error(t, err)
	var valErr *schwabErrors.ValidationError
	assert.ErrorAs(t, err, &valErr)
	assert.Contains(t, err.Error(), "fast period")
}

func TestMACD_RejectsZeroPeriod(t *testing.T) {
	closes := make([]float64, 50)
	_, _, _, err := MACD(closes, 0, 26, 9)
	require.Error(t, err)
	var valErr *schwabErrors.ValidationError
	assert.ErrorAs(t, err, &valErr)
}

func TestMACD_InsufficientData(t *testing.T) {
	// Need at least slow+signal=35 candles for fast=12, slow=26, signal=9
	closes := make([]float64, 10)
	_, _, _, err := MACD(closes, 12, 26, 9)
	require.Error(t, err)
	var valErr *schwabErrors.ValidationError
	assert.ErrorAs(t, err, &valErr)
}
