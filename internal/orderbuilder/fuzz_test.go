package orderbuilder

import (
	"strings"
	"testing"
	"time"

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
	f.Add("AAPL  250620X00200000") // bad put/call
	f.Add("      250620C00200000") // empty underlying
	f.Add("AAPL  259920C00200000") // bad date
	f.Add("AAPL  250620C0020000A") // non-numeric strike
	f.Add("AAPL  250620C00000000") // zero strike
	f.Add("AAPL  250620C002000001") // too long

	f.Fuzz(func(t *testing.T, input string) {
		// ParseOCCSymbol must never panic regardless of input.
		result, err := ParseOCCSymbol(input)
		if err != nil {
			return
		}

		// If parsing succeeded, verify invariants.
		if result.Underlying == "" {
			t.Error("parsed successfully but underlying is empty")
		}

		if result.PutCall != "CALL" && result.PutCall != "PUT" {
			t.Errorf("parsed successfully but put/call is %q", result.PutCall)
		}

		if result.Strike <= 0 {
			t.Errorf("parsed successfully but strike is %f", result.Strike)
		}

		if result.Expiration.IsZero() {
			t.Error("parsed successfully but expiration is zero")
		}

		if result.Symbol != input {
			t.Errorf("parsed successfully but Symbol field is %q, want %q", result.Symbol, input)
		}
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
		if len(trimmed) == 0 || len(trimmed) > 6 {
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
		if err != nil {
			t.Fatalf("BuildOCCSymbol produced unparseable symbol %q: %v", symbol, err)
		}

		// BuildOCCSymbol right-pads to 6 chars, ParseOCCSymbol trims.
		// Compare against the trimmed input to account for this.
		if parsed.Underlying != trimmed {
			t.Errorf("underlying: got %q, want %q", parsed.Underlying, trimmed)
		}

		if parsed.PutCall != putCall {
			t.Errorf("putCall: got %q, want %q", parsed.PutCall, putCall)
		}

		if parsed.Expiration.Year() != year {
			t.Errorf("year: got %d, want %d", parsed.Expiration.Year(), year)
		}

		if parsed.Expiration.Month() != time.Month(month) {
			t.Errorf("month: got %v, want %v", parsed.Expiration.Month(), time.Month(month))
		}

		if parsed.Expiration.Day() != day {
			t.Errorf("day: got %d, want %d", parsed.Expiration.Day(), day)
		}
	})
}
