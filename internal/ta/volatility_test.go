package ta

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/major/schwab-agent/internal/apperr"
)

func TestHistoricalVolatility_Correct(t *testing.T) {
	// Arrange
	closes := make([]float64, 25)
	for i := range closes {
		closes[i] = 100 * math.Pow(1.01, float64(i))
	}

	// Act
	result, err := HistoricalVolatility(closes, 20)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Greater(t, result.DailyVol, 0.0)
	assert.Greater(t, result.AnnualizedVol, 0.0)
	assert.Contains(t, []string{"very_low", "low", "normal", "elevated", "high", "extreme"}, result.Regime)
	assert.GreaterOrEqual(t, result.PercentileRank, 0.0)
	assert.LessOrEqual(t, result.PercentileRank, 100.0)
}

func TestHistoricalVolatility_ConstantPrices(t *testing.T) {
	// Arrange
	closes := make([]float64, 21)
	for i := range closes {
		closes[i] = 100.0
	}

	// Act
	result, err := HistoricalVolatility(closes, 20)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Zero(t, result.DailyVol)
	assert.Zero(t, result.AnnualizedVol)
	assert.Equal(t, "very_low", result.Regime)
	assert.Zero(t, result.MinVol)
	assert.Zero(t, result.MaxVol)
	assert.Zero(t, result.MeanVol)
	assert.Equal(t, 100.0, result.PercentileRank)
}

func TestHistoricalVolatility_InvalidPeriod(t *testing.T) {
	// Arrange & Act
	_, err := HistoricalVolatility([]float64{100, 101, 102}, 0)

	// Assert
	require.Error(t, err)
	var valErr *apperr.ValidationError
	assert.ErrorAs(t, err, &valErr)
}

func TestHistoricalVolatility_InsufficientData(t *testing.T) {
	// Arrange & Act
	_, err := HistoricalVolatility([]float64{100, 101, 102, 103, 104}, 10)

	// Assert
	require.Error(t, err)
	var valErr *apperr.ValidationError
	assert.ErrorAs(t, err, &valErr)
}

func TestHistoricalVolatility_MinimumData(t *testing.T) {
	// Arrange
	closes := []float64{100, 101, 100, 102, 101, 103}

	// Act
	result, err := HistoricalVolatility(closes, 5)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.GreaterOrEqual(t, result.DailyVol, 0.0)
	assert.GreaterOrEqual(t, result.AnnualizedVol, 0.0)
	assert.Equal(t, result.MinVol, result.MaxVol)
	assert.Equal(t, result.MaxVol, result.MeanVol)
	assert.Equal(t, 100.0, result.PercentileRank)
}

func TestClassifyRegime(t *testing.T) {
	tests := []struct {
		name     string
		value    float64
		expected string
	}{
		{name: "very low below threshold", value: 9.99, expected: "very_low"},
		{name: "low lower bound", value: 10.0, expected: "low"},
		{name: "low upper bound", value: 14.99, expected: "low"},
		{name: "normal lower bound", value: 15.0, expected: "normal"},
		{name: "normal upper bound", value: 19.99, expected: "normal"},
		{name: "elevated lower bound", value: 20.0, expected: "elevated"},
		{name: "elevated upper bound", value: 29.99, expected: "elevated"},
		{name: "high lower bound", value: 30.0, expected: "high"},
		{name: "high upper bound", value: 49.99, expected: "high"},
		{name: "extreme lower bound", value: 50.0, expected: "extreme"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, classifyRegime(test.value))
		})
	}
}

func TestStddev(t *testing.T) {
	// Arrange
	data := []float64{2, 4, 4, 4, 5, 5, 7, 9}

	// Act
	result := stddev(data)

	// Assert
	assert.InDelta(t, 2.1381, result, 0.001)
}

func TestStddev_SingleValue(t *testing.T) {
	assert.Zero(t, stddev([]float64{42}))
}

func TestPercentileRank(t *testing.T) {
	// Arrange
	series := []float64{0.01, 0.02, 0.03, 0.04, 0.05}

	// Act
	result := percentileRank(series, 0.03)

	// Assert
	assert.Equal(t, 60.0, result)
}

func TestPercentileRank_EmptySeries(t *testing.T) {
	assert.Zero(t, percentileRank(nil, 0.03))
}
