package ta

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/major/schwab-agent/internal/apperr"
)

func TestIntervalToHistoryParams(t *testing.T) {
	tests := []struct {
		name            string
		interval        string
		requiredCandles int
		wantPT          string
		wantP           string
		wantFT          string
		wantF           string
		wantErr         bool
		errMsg          string
	}{
		// Default lookback (requiredCandles=0 or small enough for minimum period)
		{
			name:            "daily default",
			interval:        "daily",
			requiredCandles: 0,
			wantPT:          "year",
			wantP:           "1",
			wantFT:          "daily",
			wantF:           "1",
		},
		{
			name:            "weekly default",
			interval:        "weekly",
			requiredCandles: 0,
			wantPT:          "year",
			wantP:           "1",
			wantFT:          "weekly",
			wantF:           "1",
		},
		{
			name:            "1min default",
			interval:        "1min",
			requiredCandles: 0,
			wantPT:          "day",
			wantP:           "1",
			wantFT:          "minute",
			wantF:           "1",
		},
		{
			name:            "5min default",
			interval:        "5min",
			requiredCandles: 0,
			wantPT:          "day",
			wantP:           "1",
			wantFT:          "minute",
			wantF:           "5",
		},
		{
			name:            "15min default",
			interval:        "15min",
			requiredCandles: 0,
			wantPT:          "day",
			wantP:           "1",
			wantFT:          "minute",
			wantF:           "15",
		},
		{
			name:            "30min default",
			interval:        "30min",
			requiredCandles: 0,
			wantPT:          "day",
			wantP:           "1",
			wantFT:          "minute",
			wantF:           "30",
		},

		// Period scaling for daily: 600 candles / 252 days per year = 3 years
		{
			name:            "daily scaled for SMA 200",
			interval:        "daily",
			requiredCandles: 600,
			wantPT:          "year",
			wantP:           "3",
			wantFT:          "daily",
			wantF:           "1",
		},
		{
			name:            "daily stays 1 year below buffered boundary",
			interval:        "daily",
			requiredCandles: 242,
			wantPT:          "year",
			wantP:           "1",
			wantFT:          "daily",
			wantF:           "1",
		},
		{
			name:            "daily uses 2 years at buffered boundary",
			interval:        "daily",
			requiredCandles: 243,
			wantPT:          "year",
			wantP:           "2",
			wantFT:          "daily",
			wantF:           "1",
		},
		// 252 candles needs a 2-year request because Schwab can return too few daily
		// candles for a one-calendar-year lookback around holidays/current sessions.
		{
			name:            "daily uses 2 years at trading-year requirement",
			interval:        "daily",
			requiredCandles: 252,
			wantPT:          "year",
			wantP:           "2",
			wantFT:          "daily",
			wantF:           "1",
		},
		// 253 candles spills into year 2
		{
			name:            "daily scales to 2 years at boundary",
			interval:        "daily",
			requiredCandles: 253,
			wantPT:          "year",
			wantP:           "2",
			wantFT:          "daily",
			wantF:           "1",
		},
		// 4 years is not a documented API value; rounds up to 5
		{
			name:            "daily rounds 4 years up to 5",
			interval:        "daily",
			requiredCandles: 800,
			wantPT:          "year",
			wantP:           "5",
			wantFT:          "daily",
			wantF:           "1",
		},
		// Exactly at the API maximum (20 years * 252 = 5040)
		{
			name:            "daily at max valid period",
			interval:        "daily",
			requiredCandles: 5040,
			wantPT:          "year",
			wantP:           "20",
			wantFT:          "daily",
			wantF:           "1",
		},

		// Period scaling for weekly: 600 candles / 52 weeks = 12, rounds to 15
		{
			name:            "weekly rounds 12 years up to 15",
			interval:        "weekly",
			requiredCandles: 600,
			wantPT:          "year",
			wantP:           "15",
			wantFT:          "weekly",
			wantF:           "1",
		},
		// 52 candles fits in 1 year
		{
			name:            "weekly stays 1 year when sufficient",
			interval:        "weekly",
			requiredCandles: 52,
			wantPT:          "year",
			wantP:           "1",
			wantFT:          "weekly",
			wantF:           "1",
		},

		// Period scaling for intraday: 390 1-min candles per day
		{
			name:            "1min scales to 2 days",
			interval:        "1min",
			requiredCandles: 500,
			wantPT:          "day",
			wantP:           "2",
			wantFT:          "minute",
			wantF:           "1",
		},
		// 5min: 78 candles per day, 200 candles needs 3 days
		{
			name:            "5min scales to 3 days",
			interval:        "5min",
			requiredCandles: 200,
			wantPT:          "day",
			wantP:           "3",
			wantFT:          "minute",
			wantF:           "5",
		},
		// 5min: 6 days is not valid, rounds to 10
		{
			name:            "5min rounds 6 days up to 10",
			interval:        "5min",
			requiredCandles: 450,
			wantPT:          "day",
			wantP:           "10",
			wantFT:          "minute",
			wantF:           "5",
		},

		// Infeasible requests (exceed API maximum capacity)
		{
			name:            "daily exceeds max capacity",
			interval:        "daily",
			requiredCandles: 999999,
			wantErr:         true,
			errMsg:          "indicator requires 999999 candles but daily interval supports at most 5040",
		},
		{
			name:            "daily exceeds max capacity by one candle",
			interval:        "daily",
			requiredCandles: 5041,
			wantErr:         true,
			errMsg:          "indicator requires 5041 candles but daily interval supports at most 5040",
		},
		{
			name:            "15min exceeds max capacity",
			interval:        "15min",
			requiredCandles: 999999,
			wantErr:         true,
			errMsg:          "indicator requires 999999 candles but 15min interval supports at most 260",
		},

		// Error cases
		{
			name:     "invalid interval",
			interval: "invalid",
			wantErr:  true,
			errMsg:   "unsupported interval",
		},
		{
			name:     "empty interval",
			interval: "",
			wantErr:  true,
			errMsg:   "unsupported interval",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			pt, p, ft, f, err := IntervalToHistoryParams(tt.interval, tt.requiredCandles)

			// Assert
			if tt.wantErr {
				require.Error(t, err)
				var valErr *apperr.ValidationError
				require.True(t, assert.ErrorAs(t, err, &valErr), "error should be ValidationError")
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantPT, pt)
				assert.Equal(t, tt.wantP, p)
				assert.Equal(t, tt.wantFT, ft)
				assert.Equal(t, tt.wantF, f)
			}
		})
	}
}

func TestCeilDiv(t *testing.T) {
	tests := []struct {
		a, b, want int
	}{
		{1, 1, 1},
		{5, 2, 3},
		{6, 3, 2},
		{7, 3, 3},
		{252, 252, 1},
		{253, 252, 2},
		{600, 252, 3},
		{0, 252, 0},
	}

	for _, tt := range tests {
		got := ceilDiv(tt.a, tt.b)
		assert.Equal(t, tt.want, got, "ceilDiv(%d, %d)", tt.a, tt.b)
	}
}

func TestNextValidPeriod(t *testing.T) {
	tests := []struct {
		n     int
		valid []int
		want  int
	}{
		{1, validYearPeriods(), 1},
		{3, validYearPeriods(), 3},
		{4, validYearPeriods(), 5},   // 4 is not valid, rounds to 5
		{6, validYearPeriods(), 10},  // 6 is not valid, rounds to 10
		{11, validYearPeriods(), 15}, // 11 is not valid, rounds to 15
		{16, validYearPeriods(), 20}, // 16 is not valid, rounds to 20
		{21, validYearPeriods(), 20}, // exceeds max, capped at 20
		{1, validDayPeriods(), 1},
		{6, validDayPeriods(), 10},  // 6 is not valid, rounds to 10
		{11, validDayPeriods(), 10}, // exceeds max, capped at 10
	}

	for _, tt := range tests {
		got := nextValidPeriod(tt.n, tt.valid)
		assert.Equal(t, tt.want, got, "nextValidPeriod(%d, %v)", tt.n, tt.valid)
	}
}

func TestMaxCandlesForInterval(t *testing.T) {
	tests := []struct {
		interval string
		want     int
	}{
		{"daily", 5040},  // 20 years * 252 trading days
		{"weekly", 1040}, // 20 years * 52 weeks
		{"1min", 3900},   // 10 days * 390 minutes
		{"5min", 780},    // 10 days * 78 bars
		{"15min", 260},   // 10 days * 26 bars
		{"30min", 130},   // 10 days * 13 bars
		{"invalid", 0},
		{"", 0},
	}

	for _, tt := range tests {
		got := maxCandlesForInterval(tt.interval)
		assert.Equal(t, tt.want, got, "maxCandlesForInterval(%q)", tt.interval)
	}
}
