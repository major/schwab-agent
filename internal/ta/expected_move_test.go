package ta

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/major/schwab-agent/internal/apperr"
)

func TestExpectedMove_Correct(t *testing.T) {
	// Arrange
	underlyingPrice := 150.0
	callPrice := 5.00
	putPrice := 4.50
	multiplier := 0.85

	// Act
	result, err := ExpectedMove(underlyingPrice, callPrice, putPrice, multiplier)

	// Assert
	require.NoError(t, err)
	assert.InDelta(t, 9.50, result.StraddlePrice, 0.001)
	assert.InDelta(t, 9.50, result.ExpectedMove, 0.001)
	assert.InDelta(t, 8.075, result.AdjustedMove, 0.001)
	assert.InDelta(t, 158.075, result.Upper1x, 0.001)
	assert.InDelta(t, 141.925, result.Lower1x, 0.001)
	assert.InDelta(t, 166.15, result.Upper2x, 0.001)
	assert.InDelta(t, 133.85, result.Lower2x, 0.001)
}

func TestExpectedMove_ZeroPrices(t *testing.T) {
	// Arrange
	underlyingPrice := 100.0
	callPrice := 0.0
	putPrice := 0.0
	multiplier := 0.85

	// Act
	result, err := ExpectedMove(underlyingPrice, callPrice, putPrice, multiplier)

	// Assert
	require.NoError(t, err)
	assert.InDelta(t, 0.0, result.StraddlePrice, 0.001)
	assert.InDelta(t, 0.0, result.AdjustedMove, 0.001)
	assert.InDelta(t, 100.0, result.Upper1x, 0.001)
	assert.InDelta(t, 100.0, result.Lower1x, 0.001)
}

func TestExpectedMove_InvalidUnderlying(t *testing.T) {
	// Arrange & Act
	_, err := ExpectedMove(0, 5.0, 4.5, 0.85)

	// Assert
	require.Error(t, err)
	var valErr *apperr.ValidationError
	assert.ErrorAs(t, err, &valErr)
}

func TestExpectedMove_NegativeUnderlying(t *testing.T) {
	// Arrange & Act
	_, err := ExpectedMove(-1, 5.0, 4.5, 0.85)

	// Assert
	require.Error(t, err)
	var valErr *apperr.ValidationError
	assert.ErrorAs(t, err, &valErr)
}

func TestExpectedMove_InvalidMultiplier(t *testing.T) {
	// Arrange & Act
	_, err := ExpectedMove(150.0, 5.0, 4.5, 0)

	// Assert
	require.Error(t, err)
	var valErr *apperr.ValidationError
	assert.ErrorAs(t, err, &valErr)
}

func TestExpectedMove_MultiplierAboveOne(t *testing.T) {
	// Arrange & Act
	_, err := ExpectedMove(150.0, 5.0, 4.5, 1.5)

	// Assert
	require.Error(t, err)
	var valErr *apperr.ValidationError
	assert.ErrorAs(t, err, &valErr)
}

func TestExpectedMove_DefaultMultiplier(t *testing.T) {
	// Arrange & Act & Assert
	assert.InDelta(t, 0.85, DefaultMultiplier, 0.001)
}
