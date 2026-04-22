package ta

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/major/schwab-agent/internal/apperr"
)

func TestVWAP_Correct(t *testing.T) {
	// Arrange
	highs := []float64{11, 12, 13, 14, 15}
	lows := []float64{9, 10, 11, 12, 13}
	closes := []float64{10, 11, 12, 13, 14}
	volumes := []float64{1000, 1500, 2000, 2500, 3000}

	// Act
	result, err := VWAP(highs, lows, closes, volumes)

	// Assert
	require.NoError(t, err)
	require.Len(t, result, 5)

	// Hand-calculated expected values:
	// i=0: tp=10.0, cumTV=10000, cumVol=1000, VWAP=10.0
	assert.InDelta(t, 10.0, result[0], 0.0001)

	// i=1: tp=11.0, cumTV=26500, cumVol=2500, VWAP=10.6
	assert.InDelta(t, 10.6, result[1], 0.0001)

	// i=2: tp=12.0, cumTV=50500, cumVol=4500, VWAP=11.222...
	assert.InDelta(t, 11.222222, result[2], 0.0001)

	// i=3: tp=13.0, cumTV=83000, cumVol=7000, VWAP=11.857...
	assert.InDelta(t, 11.857143, result[3], 0.0001)

	// i=4: tp=14.0, cumTV=125000, cumVol=10000, VWAP=12.5
	assert.InDelta(t, 12.5, result[4], 0.0001)
}

func TestVWAP_SingleCandle(t *testing.T) {
	// Arrange
	highs := []float64{100}
	lows := []float64{90}
	closes := []float64{95}
	volumes := []float64{1000}

	// Act
	result, err := VWAP(highs, lows, closes, volumes)

	// Assert
	require.NoError(t, err)
	require.Len(t, result, 1)
	// tp = (100 + 90 + 95) / 3 = 95.0
	// VWAP = 95.0 * 1000 / 1000 = 95.0
	assert.InDelta(t, 95.0, result[0], 0.0001)
}

func TestVWAP_AllZeroVolume(t *testing.T) {
	// Arrange
	highs := []float64{10, 11, 12}
	lows := []float64{9, 10, 11}
	closes := []float64{9.5, 10.5, 11.5}
	volumes := []float64{0, 0, 0}

	// Act
	_, err := VWAP(highs, lows, closes, volumes)

	// Assert
	require.Error(t, err)
	var valErr *apperr.ValidationError
	assert.ErrorAs(t, err, &valErr)
}

func TestVWAP_SomeZeroVolume(t *testing.T) {
	// Arrange: volumes with zeros interspersed
	highs := []float64{10, 11, 12, 13, 14}
	lows := []float64{9, 10, 11, 12, 13}
	closes := []float64{9.5, 10.5, 11.5, 12.5, 13.5}
	volumes := []float64{1000, 0, 500, 0, 800}

	// Act
	result, err := VWAP(highs, lows, closes, volumes)

	// Assert
	require.NoError(t, err)
	require.Len(t, result, 5)

	// i=0: tp=9.5, cumTV=9500, cumVol=1000, VWAP=9.5
	assert.InDelta(t, 9.5, result[0], 0.0001)

	// i=1: tp=10.5, vol=0, cumTV=9500, cumVol=1000, VWAP=9.5 (unchanged)
	assert.InDelta(t, 9.5, result[1], 0.0001)

	// i=2: tp=11.5, vol=500, cumTV=15250, cumVol=1500, VWAP=10.166...
	assert.InDelta(t, 10.166667, result[2], 0.0001)

	// i=3: tp=12.5, vol=0, cumTV=15250, cumVol=1500, VWAP=10.166... (unchanged)
	assert.InDelta(t, 10.166667, result[3], 0.0001)

	// i=4: tp=13.5, vol=800, cumTV=26050, cumVol=2300, VWAP=11.326...
	assert.InDelta(t, 11.326087, result[4], 0.0001)
}

func TestVWAP_MismatchedLengths(t *testing.T) {
	// Arrange
	highs := []float64{10, 11, 12}
	lows := []float64{9, 10}
	closes := []float64{9.5, 10.5, 11.5}
	volumes := []float64{1000, 1500, 2000}

	// Act
	_, err := VWAP(highs, lows, closes, volumes)

	// Assert
	require.Error(t, err)
	var valErr *apperr.ValidationError
	assert.ErrorAs(t, err, &valErr)
}

func TestVWAP_EmptySlices(t *testing.T) {
	// Arrange
	highs := []float64{}
	lows := []float64{}
	closes := []float64{}
	volumes := []float64{}

	// Act
	_, err := VWAP(highs, lows, closes, volumes)

	// Assert
	require.Error(t, err)
	var valErr *apperr.ValidationError
	assert.ErrorAs(t, err, &valErr)
}
