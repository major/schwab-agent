package ta

import (
	"fmt"

	"github.com/major/schwab-agent/internal/apperr"
)

// DefaultMultiplier is the default straddle price adjustment factor.
const DefaultMultiplier = 0.85

// ExpectedMoveResult contains the calculated expected move values.
type ExpectedMoveResult struct {
	StraddlePrice float64 `json:"straddle_price"`
	ExpectedMove  float64 `json:"expected_move"`
	AdjustedMove  float64 `json:"adjusted_move"`
	Upper1x       float64 `json:"upper_1x"`
	Lower1x       float64 `json:"lower_1x"`
	Upper2x       float64 `json:"upper_2x"`
	Lower2x       float64 `json:"lower_2x"`
}

// ExpectedMove calculates the expected move based on option prices and a multiplier.
// Returns ValidationError if any input is invalid.
func ExpectedMove(underlyingPrice, callPrice, putPrice, multiplier float64) (*ExpectedMoveResult, error) {
	// Validate underlyingPrice
	if underlyingPrice <= 0 {
		return nil, apperr.NewValidationError(
			fmt.Sprintf("underlyingPrice must be > 0, got %.4f", underlyingPrice),
			nil,
		)
	}

	// Validate callPrice
	if callPrice < 0 {
		return nil, apperr.NewValidationError(
			fmt.Sprintf("callPrice must be >= 0, got %.4f", callPrice),
			nil,
		)
	}

	// Validate putPrice
	if putPrice < 0 {
		return nil, apperr.NewValidationError(
			fmt.Sprintf("putPrice must be >= 0, got %.4f", putPrice),
			nil,
		)
	}

	// Validate multiplier
	if multiplier <= 0 {
		return nil, apperr.NewValidationError(
			fmt.Sprintf("multiplier must be > 0, got %.4f", multiplier),
			nil,
		)
	}

	if multiplier > 1.0 {
		return nil, apperr.NewValidationError(
			fmt.Sprintf("multiplier must be <= 1.0, got %.4f", multiplier),
			nil,
		)
	}

	// Calculate values
	straddle := callPrice + putPrice
	adjustedMove := straddle * multiplier
	upper1x := underlyingPrice + adjustedMove
	lower1x := underlyingPrice - adjustedMove
	upper2x := underlyingPrice + (2 * adjustedMove)
	lower2x := underlyingPrice - (2 * adjustedMove)

	return &ExpectedMoveResult{
		StraddlePrice: straddle,
		ExpectedMove:  straddle,
		AdjustedMove:  adjustedMove,
		Upper1x:       upper1x,
		Lower1x:       lower1x,
		Upper2x:       upper2x,
		Lower2x:       lower2x,
	}, nil
}
