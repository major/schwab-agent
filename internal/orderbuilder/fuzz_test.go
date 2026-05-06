package orderbuilder

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/major/schwab-agent/internal/models"
)

// FuzzParseOCCSymbol verifies that ParseOCCSymbol never panics on arbitrary input.
func FuzzParseOCCSymbol(f *testing.F) {
	// Seed with valid symbols.
	f.Add("AAPL  250620C00200000")
	f.Add("SPY   251219P00450500")
	f.Add("GOOGL 250117C00150000")
	f.Add("X     250620C00001500")
	f.Add("ABCDEF260315P01000000")

	// Seed with known-invalid inputs that exercise error paths.
	f.Add("")
	f.Add("short")
	f.Add("AAPL  250620X00200000")  // bad put/call
	f.Add("      250620C00200000")  // empty underlying
	f.Add("AAPL  259920C00200000")  // bad date
	f.Add("AAPL  250620C0020000A")  // non-numeric strike
	f.Add("AAPL  250620C00000000")  // zero strike
	f.Add("AAPL  250620C002000001") // too long

	f.Fuzz(func(t *testing.T, input string) {
		// ParseOCCSymbol must never panic regardless of input.
		result, err := ParseOCCSymbol(input)
		if err != nil {
			return
		}

		// If parsing succeeded, verify invariants.
		assert.NotEmpty(t, result.Underlying, "parsed successfully but underlying is empty")

		assert.True(
			t,
			result.PutCall == "CALL" || result.PutCall == "PUT",
			"parsed successfully but put/call is %q",
			result.PutCall,
		)

		assert.Greater(t, result.Strike, 0.0, "parsed successfully but strike is non-positive")

		assert.False(t, result.Expiration.IsZero(), "parsed successfully but expiration is zero")

		assert.Equal(t, input, result.Symbol, "parsed Symbol field mismatch")
	})
}

// FuzzBuildParseOCCRoundTrip verifies that building an OCC symbol and parsing
// it back always produces the original components.
func FuzzBuildParseOCCRoundTrip(f *testing.F) {
	f.Add("AAPL", 2025, 6, 20, 200.0, true)
	f.Add("SPY", 2025, 12, 19, 450.5, false)
	f.Add("X", 2026, 3, 15, 25.5, true)
	f.Add("GOOGL", 2025, 1, 17, 150.0, true)
	f.Add("A", 2025, 1, 1, 0.001, false)

	f.Fuzz(func(t *testing.T, underlying string, year, month, day int, strike float64, isCall bool) {
		// Filter out inputs that can't produce valid OCC symbols.
		// Real ticker symbols are ASCII uppercase letters only.
		trimmed := strings.TrimSpace(underlying)
		if trimmed == "" || len(trimmed) > 6 {
			return
		}

		for _, r := range trimmed {
			if r < 'A' || r > 'Z' {
				return
			}
		}

		// OCC format uses YYMMDD. Go's time.Parse("060102") maps two-digit
		// years to the range 1969-2068, so years outside 2000-2068 won't
		// round-trip correctly.
		if year < 2000 || year > 2068 {
			return
		}

		if month < 1 || month > 12 || day < 1 || day > 31 {
			return
		}

		// time.Date normalizes out-of-range days (e.g. Feb 30 → Mar 2).
		// Skip if the date normalizes to a different month, since that
		// means the original month/day was invalid.
		expiration := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
		if expiration.Month() != time.Month(month) || expiration.Day() != day {
			return
		}

		// Strike must be representable as a positive 8-digit integer (strike*1000).
		// OCC format caps at 99999.999, and values below 0.001 round to zero.
		if strike < 0.001 || strike > 99999.999 {
			return
		}

		putCall := string(models.PutCallCall)
		if !isCall {
			putCall = string(models.PutCallPut)
		}

		symbol := BuildOCCSymbol(trimmed, expiration, strike, putCall)

		parsed, err := ParseOCCSymbol(symbol)
		require.NoError(t, err, "BuildOCCSymbol produced unparseable symbol %q", symbol)

		// BuildOCCSymbol right-pads to 6 chars, ParseOCCSymbol trims.
		// Compare against the trimmed input to account for this.
		assert.Equal(t, trimmed, parsed.Underlying, "underlying mismatch")

		assert.Equal(t, putCall, parsed.PutCall, "putCall mismatch")

		assert.Equal(t, year, parsed.Expiration.Year(), "year mismatch")

		assert.Equal(t, time.Month(month), parsed.Expiration.Month(), "month mismatch")

		assert.Equal(t, day, parsed.Expiration.Day(), "day mismatch")
	})
}
