package commands

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/major/schwab-agent/internal/models"
)

// TestNewStrategyCobraFlagRegistration verifies the newer option strategies can
// register their tagged fields through the Cobra-native migration helper.
func TestNewStrategyCobraFlagRegistration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		opts  any
		flags []string
	}{
		{name: "butterfly", opts: &butterflyBuildOpts{}, flags: []string{"underlying", "expiration", "lower-strike", "middle-strike", "upper-strike"}},
		{name: "condor", opts: &condorBuildOpts{}, flags: []string{"underlying", "expiration", "lower-strike", "lower-middle-strike", "upper-middle-strike", "upper-strike"}},
		{name: "back-ratio", opts: &backRatioBuildOpts{}, flags: []string{"underlying", "expiration", "short-strike", "long-strike", "long-ratio"}},
		{name: "vertical-roll", opts: &verticalRollBuildOpts{}, flags: []string{"underlying", "close-expiration", "open-expiration", "close-long-strike", "open-long-strike"}},
		{name: "double-diagonal", opts: &doubleDiagonalBuildOpts{}, flags: []string{"underlying", "near-expiration", "far-expiration", "put-far-strike", "call-far-strike"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cmd := &cobra.Command{Use: tt.name}
			defineCobraFlags(cmd, tt.opts)

			for _, name := range tt.flags {
				assert.NotNil(t, cmd.Flags().Lookup(name))
			}
		})
	}
}

// TestParseNewOptionStrategyParamsHappyPaths covers parser branches that the
// end-to-end command tests do not hit, especially sell/close, put, debit, and
// credit combinations.
func TestParseNewOptionStrategyParamsHappyPaths(t *testing.T) {
	t.Parallel()

	t.Run("butterfly put sell close", func(t *testing.T) {
		t.Parallel()

		params, err := parseButterflyParams(&butterflyBuildOpts{
			Underlying: " F ", Expiration: testFutureExpDate,
			LowerStrike: 10, MiddleStrike: 12, UpperStrike: 14,
			Put: true, Sell: true, Close: true, Quantity: 1, Price: 0.45,
			Duration: models.DurationGoodTillCancel, Session: models.SessionPM,
		}, nil)

		require.NoError(t, err)
		assert.Equal(t, "F", params.Underlying)
		assert.Equal(t, models.PutCallPut, params.PutCall)
		assert.False(t, params.Buy)
		assert.False(t, params.Open)
		assert.Equal(t, models.DurationGoodTillCancel, params.Duration)
		assert.Equal(t, models.SessionPM, params.Session)
	})

	t.Run("condor call buy open", func(t *testing.T) {
		t.Parallel()

		params, err := parseCondorParams(&condorBuildOpts{
			Underlying: "F", Expiration: testFutureExpDate,
			LowerStrike: 10, LowerMiddleStrike: 12, UpperMiddleStrike: 14, UpperStrike: 16,
			Call: true, Buy: true, Open: true, Quantity: 2, Price: 0.75,
		}, nil)

		require.NoError(t, err)
		assert.Equal(t, models.PutCallCall, params.PutCall)
		assert.True(t, params.Buy)
		assert.True(t, params.Open)
		assert.Equal(t, 2.0, params.Quantity)
	})

	t.Run("back-ratio put close credit", func(t *testing.T) {
		t.Parallel()

		params, err := parseBackRatioParams(&backRatioBuildOpts{
			Underlying: "F", Expiration: testFutureExpDate,
			ShortStrike: 14, LongStrike: 12,
			Put: true, Close: true, Credit: true, Quantity: 1, LongRatio: 3, Price: 0.10,
		}, nil)

		require.NoError(t, err)
		assert.Equal(t, models.PutCallPut, params.PutCall)
		assert.False(t, params.Open)
		assert.True(t, params.Credit)
		assert.Equal(t, 3.0, params.LongRatio)
	})

	t.Run("vertical-roll put debit", func(t *testing.T) {
		t.Parallel()

		params, err := parseVerticalRollParams(&verticalRollBuildOpts{
			Underlying: "F", CloseExpiration: testFutureExpDate, OpenExpiration: testFutureExpTime.AddDate(0, 1, 0).Format("2006-01-02"),
			CloseLongStrike: 14, CloseShortStrike: 12, OpenLongStrike: 13, OpenShortStrike: 11,
			Put: true, Debit: true, Quantity: 1, Price: 0.15,
		}, nil)

		require.NoError(t, err)
		assert.Equal(t, models.PutCallPut, params.PutCall)
		assert.False(t, params.Credit)
	})

	t.Run("double-diagonal close", func(t *testing.T) {
		t.Parallel()

		params, err := parseDoubleDiagonalParams(&doubleDiagonalBuildOpts{
			Underlying: "F", NearExpiration: testFutureExpDate, FarExpiration: testFutureExpTime.AddDate(0, 1, 0).Format("2006-01-02"),
			PutFarStrike: 9, PutNearStrike: 10, CallNearStrike: 14, CallFarStrike: 15,
			Close: true, Quantity: 1, Price: 0.70,
		}, nil)

		require.NoError(t, err)
		assert.False(t, params.Open)
	})
}

// TestParseNewOptionStrategyParamsErrors verifies each new parser returns the
// specific validation errors users see for bad mutually exclusive flags or date
// formats.
func TestParseNewOptionStrategyParamsErrors(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		parse   func() error
		message string
	}{
		{
			name: "butterfly missing call put",
			parse: func() error {
				_, err := parseButterflyParams(&butterflyBuildOpts{Buy: true, Open: true, Expiration: testFutureExpDate}, nil)
				return err
			},
			message: "exactly one of --call or --put is required",
		},
		{
			name: "butterfly missing buy sell",
			parse: func() error {
				_, err := parseButterflyParams(&butterflyBuildOpts{Call: true, Open: true, Expiration: testFutureExpDate}, nil)
				return err
			},
			message: "exactly one of --buy or --sell is required",
		},
		{
			name: "condor missing open close",
			parse: func() error {
				_, err := parseCondorParams(&condorBuildOpts{Call: true, Buy: true, Expiration: testFutureExpDate}, nil)
				return err
			},
			message: "exactly one of --open or --close is required",
		},
		{
			name: "condor invalid expiration",
			parse: func() error {
				_, err := parseCondorParams(&condorBuildOpts{Call: true, Buy: true, Open: true, Expiration: "06/18/2026"}, nil)
				return err
			},
			message: "expiration must use YYYY-MM-DD format",
		},
		{
			name: "back-ratio missing debit credit",
			parse: func() error {
				_, err := parseBackRatioParams(&backRatioBuildOpts{Call: true, Open: true, Expiration: testFutureExpDate}, nil)
				return err
			},
			message: "exactly one of --debit or --credit is required",
		},
		{
			name: "back-ratio invalid expiration",
			parse: func() error {
				_, err := parseBackRatioParams(&backRatioBuildOpts{Call: true, Open: true, Debit: true, Expiration: "bad"}, nil)
				return err
			},
			message: "expiration must use YYYY-MM-DD format",
		},
		{
			name: "vertical-roll both debit credit",
			parse: func() error {
				_, err := parseVerticalRollParams(&verticalRollBuildOpts{Call: true, Debit: true, Credit: true}, nil)
				return err
			},
			message: "exactly one of --debit or --credit is required",
		},
		{
			name: "vertical-roll invalid close expiration",
			parse: func() error {
				_, err := parseVerticalRollParams(&verticalRollBuildOpts{Call: true, Debit: true, CloseExpiration: "bad", OpenExpiration: testFutureExpDate}, nil)
				return err
			},
			message: "close-expiration must use YYYY-MM-DD format",
		},
		{
			name: "vertical-roll invalid open expiration",
			parse: func() error {
				_, err := parseVerticalRollParams(&verticalRollBuildOpts{Call: true, Debit: true, CloseExpiration: testFutureExpDate, OpenExpiration: "bad"}, nil)
				return err
			},
			message: "open-expiration must use YYYY-MM-DD format",
		},
		{
			name: "double-diagonal missing open close",
			parse: func() error {
				_, err := parseDoubleDiagonalParams(&doubleDiagonalBuildOpts{NearExpiration: testFutureExpDate}, nil)
				return err
			},
			message: "exactly one of --open or --close is required",
		},
		{
			name: "double-diagonal invalid near expiration",
			parse: func() error {
				_, err := parseDoubleDiagonalParams(&doubleDiagonalBuildOpts{Open: true, NearExpiration: "bad", FarExpiration: testFutureExpDate}, nil)
				return err
			},
			message: "near-expiration must use YYYY-MM-DD format",
		},
		{
			name: "double-diagonal invalid far expiration",
			parse: func() error {
				_, err := parseDoubleDiagonalParams(&doubleDiagonalBuildOpts{Open: true, NearExpiration: testFutureExpDate, FarExpiration: "bad"}, nil)
				return err
			},
			message: "far-expiration must use YYYY-MM-DD format",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			err := testCase.parse()

			require.Error(t, err)
			assert.True(t, strings.Contains(err.Error(), testCase.message), "expected %q in %q", testCase.message, err.Error())
		})
	}
}
