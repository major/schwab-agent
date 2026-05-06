package commands

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/major/schwab-agent/internal/apperr"
)

// TestBuildOCCSymbolValid verifies that buildOCCSymbol produces correct OCC symbols for valid inputs.
func TestBuildOCCSymbolValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		underlying string
		expiration string
		strike     float64
		call       bool
		put        bool
		wantSymbol string
	}{
		{
			name:       "AAPL call",
			underlying: "AAPL",
			expiration: "2025-06-20",
			strike:     200,
			call:       true,
			put:        false,
			wantSymbol: "AAPL  250620C00200000",
		},
		{
			name:       "SPY put with decimal",
			underlying: "SPY",
			expiration: "2025-12-19",
			strike:     450.50,
			call:       false,
			put:        true,
			wantSymbol: "SPY   251219P00450500",
		},
		{
			name:       "GOOGL call",
			underlying: "GOOGL",
			expiration: "2025-01-17",
			strike:     150,
			call:       true,
			put:        false,
			wantSymbol: "GOOGL 250117C00150000",
		},
		{
			name:       "TSLA put with decimal",
			underlying: "TSLA",
			expiration: "2026-01-16",
			strike:     275.50,
			call:       false,
			put:        true,
			wantSymbol: "TSLA  260116P00275500",
		},
		{
			name:       "single char underlying",
			underlying: "X",
			expiration: "2025-06-20",
			strike:     1.5,
			call:       true,
			put:        false,
			wantSymbol: "X     250620C00001500",
		},
		{
			name:       "two char underlying",
			underlying: "GE",
			expiration: "2025-03-21",
			strike:     25.75,
			call:       false,
			put:        true,
			wantSymbol: "GE    250321P00025750",
		},
		{
			name:       "fractional strike",
			underlying: "QQQ",
			expiration: "2025-09-19",
			strike:     350.25,
			call:       true,
			put:        false,
			wantSymbol: "QQQ   250919C00350250",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			symbol, err := buildOCCSymbol(tt.underlying, tt.expiration, tt.strike, tt.call, tt.put)
			require.NoError(t, err)
			assert.Equal(t, tt.wantSymbol, symbol)
		})
	}
}

// TestBuildOCCSymbolEmptyUnderlying verifies that empty underlying returns ValidationError.
func TestBuildOCCSymbolEmptyUnderlying(t *testing.T) {
	t.Parallel()

	symbol, err := buildOCCSymbol("", "2025-06-20", 200, true, false)
	require.Error(t, err)
	assert.Empty(t, symbol)

	var valErr *apperr.ValidationError
	assert.ErrorAs(t, err, &valErr)
}

// TestBuildOCCSymbolInvalidDateFormat verifies that invalid date format returns ValidationError.
func TestBuildOCCSymbolInvalidDateFormat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		expiration string
	}{
		{"slash format", "06/20/2025"},
		{"dash format wrong order", "20-06-2025"},
		{"no dashes", "20062025"},
		{"empty", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			symbol, err := buildOCCSymbol("AAPL", tt.expiration, 200, true, false)
			require.Error(t, err)
			assert.Empty(t, symbol)

			var valErr *apperr.ValidationError
			assert.ErrorAs(t, err, &valErr)
		})
	}
}

// TestBuildOCCSymbolNegativeStrike verifies that negative strike returns ValidationError.
func TestBuildOCCSymbolNegativeStrike(t *testing.T) {
	t.Parallel()

	symbol, err := buildOCCSymbol("AAPL", "2025-06-20", -100, true, false)
	require.Error(t, err)
	assert.Empty(t, symbol)

	var valErr *apperr.ValidationError
	assert.ErrorAs(t, err, &valErr)
}

// TestBuildOCCSymbolZeroStrike verifies that zero strike returns ValidationError.
func TestBuildOCCSymbolZeroStrike(t *testing.T) {
	t.Parallel()

	symbol, err := buildOCCSymbol("AAPL", "2025-06-20", 0, true, false)
	require.Error(t, err)
	assert.Empty(t, symbol)

	var valErr *apperr.ValidationError
	assert.ErrorAs(t, err, &valErr)
}

// TestBuildOCCSymbolBothCallAndPut verifies that both call and put true returns ValidationError.
func TestBuildOCCSymbolBothCallAndPut(t *testing.T) {
	t.Parallel()

	symbol, err := buildOCCSymbol("AAPL", "2025-06-20", 200, true, true)
	require.Error(t, err)
	assert.Empty(t, symbol)

	var valErr *apperr.ValidationError
	assert.ErrorAs(t, err, &valErr)
}

// TestBuildOCCSymbolNeitherCallNorPut verifies that both call and put false returns ValidationError.
func TestBuildOCCSymbolNeitherCallNorPut(t *testing.T) {
	t.Parallel()

	symbol, err := buildOCCSymbol("AAPL", "2025-06-20", 200, false, false)
	require.Error(t, err)
	assert.Empty(t, symbol)

	var valErr *apperr.ValidationError
	assert.ErrorAs(t, err, &valErr)
}
