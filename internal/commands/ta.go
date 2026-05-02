// Package commands provides CLI command builders for schwab-agent.
package commands

import (
	"context"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/major/schwab-agent/internal/client"
	"github.com/major/schwab-agent/internal/models"
	"github.com/major/schwab-agent/internal/output"
	"github.com/major/schwab-agent/internal/ta"
)

// Multi-value point types for indicators that return more than a single value per timestamp.

type macdPoint struct {
	Datetime  string  `json:"datetime"`
	MACD      float64 `json:"macd"`
	Signal    float64 `json:"signal"`
	Histogram float64 `json:"histogram"`
}

type bbandsPoint struct {
	Datetime string  `json:"datetime"`
	Upper    float64 `json:"upper"`
	Middle   float64 `json:"middle"`
	Lower    float64 `json:"lower"`
}

type stochPoint struct {
	Datetime string  `json:"datetime"`
	SlowK    float64 `json:"slowk"`
	SlowD    float64 `json:"slowd"`
}

type adxPoint struct {
	Datetime string  `json:"datetime"`
	ADX      float64 `json:"adx"`
	PlusDI   float64 `json:"plus_di"`
	MinusDI  float64 `json:"minus_di"`
}

// Indicator output types wrap values with their parameters for JSON serialization.

type macdOutput struct {
	Indicator string      `json:"indicator"`
	Symbol    string      `json:"symbol"`
	Interval  string      `json:"interval"`
	Fast      int         `json:"fast"`
	Slow      int         `json:"slow"`
	Signal    int         `json:"signal"`
	Values    []macdPoint `json:"values"`
}

type bbandsOutput struct {
	Indicator string        `json:"indicator"`
	Symbol    string        `json:"symbol"`
	Interval  string        `json:"interval"`
	Period    int           `json:"period"`
	StdDev    float64       `json:"std_dev"`
	Values    []bbandsPoint `json:"values"`
}

type stochOutput struct {
	Indicator string       `json:"indicator"`
	Symbol    string       `json:"symbol"`
	Interval  string       `json:"interval"`
	KPeriod   int          `json:"k_period"`
	SmoothK   int          `json:"smooth_k"`
	DPeriod   int          `json:"d_period"`
	Values    []stochPoint `json:"values"`
}

type adxOutput struct {
	Indicator string     `json:"indicator"`
	Symbol    string     `json:"symbol"`
	Interval  string     `json:"interval"`
	Period    int        `json:"period"`
	Values    []adxPoint `json:"values"`
}

type hvOutput struct {
	Indicator      string  `json:"indicator"`
	Symbol         string  `json:"symbol"`
	Interval       string  `json:"interval"`
	Period         int     `json:"period"`
	DailyVol       float64 `json:"daily_vol"`
	WeeklyVol      float64 `json:"weekly_vol"`
	MonthlyVol     float64 `json:"monthly_vol"`
	AnnualizedVol  float64 `json:"annualized_vol"`
	PercentileRank float64 `json:"percentile_rank"`
	Regime         string  `json:"regime"`
	MinVol         float64 `json:"min_vol"`
	MaxVol         float64 `json:"max_vol"`
	MeanVol        float64 `json:"mean_vol"`
}

type expectedMoveOutput struct {
	Indicator       string  `json:"indicator"`
	Symbol          string  `json:"symbol"`
	UnderlyingPrice float64 `json:"underlying_price"`
	Expiration      string  `json:"expiration"`
	DTE             int     `json:"dte"`
	StraddlePrice   float64 `json:"straddle_price"`
	ExpectedMove    float64 `json:"expected_move"`
	AdjustedMove    float64 `json:"adjusted_move"`
	Upper1x         float64 `json:"upper_1x"`
	Lower1x         float64 `json:"lower_1x"`
	Upper2x         float64 `json:"upper_2x"`
	Lower2x         float64 `json:"lower_2x"`
}

// taOutput is the common envelope for single-value time series indicators (SMA, EMA, RSI, etc.).
// Values use map[string]any because the value key name is the indicator name itself (dynamic).
type taOutput struct {
	Indicator string           `json:"indicator"`
	Symbol    string           `json:"symbol"`
	Interval  string           `json:"interval"`
	Period    int              `json:"period"`
	Values    []map[string]any `json:"values"`
}

// multiTAOutput is used when SMA/EMA/RSI are requested for more than one
// period in a single command. The values are keyed as indicator_period (for
// example, sma_21) so agents can compare crossovers without making multiple
// CLI calls and then hand-merging separate JSON envelopes.
type multiTAOutput struct {
	Indicator string           `json:"indicator"`
	Symbol    string           `json:"symbol"`
	Interval  string           `json:"interval"`
	Periods   []int            `json:"periods"`
	Values    []map[string]any `json:"values"`
}

// simpleTAConfig defines a closes-only technical indicator command.
// SMA, EMA, and RSI share the same pipeline: fetch candles, extract closes,
// compute indicator, write output. Only the parameters differ.
type simpleTAConfig struct {
	name      string
	usage     string
	usageText string
	// defaultPeriod is the indicator's default --period flag value.
	defaultPeriod int
	// multiplier controls how many candles to fetch relative to the requested
	// period. EMA and RSI need 3x for convergence stability; SMA needs 1x.
	multiplier int
	// compute is the indicator function: ta.SMA, ta.EMA, or ta.RSI.
	compute func(closes []float64, period int) ([]float64, error)
}

func maxPeriod(periods []int) int {
	largest := periods[0]
	for _, period := range periods[1:] {
		if period > largest {
			largest = period
		}
	}

	return largest
}

// NewTACmd returns the Cobra command for technical analysis indicators.
func NewTACmd(c *client.Ref, w io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "ta",
		Short:   "Technical analysis indicators",
		GroupID: "market-data",
		RunE:    requireSubcommand,
	}

	cmd.AddCommand(
		makeCobraSimpleTACommand(simpleTAConfig{
			name:  "sma",
			usage: "Simple Moving Average",
			usageText: `schwab-agent ta sma AAPL
	schwab-agent ta sma AAPL --period 50 --interval daily --points 10
	schwab-agent ta sma AAPL --period 21,50,200 --points 1`,
			defaultPeriod: 20,
			multiplier:    1,
			compute:       ta.SMA,
		}, c, w),
		makeCobraSimpleTACommand(simpleTAConfig{
			name:  "ema",
			usage: "Exponential Moving Average",
			usageText: `schwab-agent ta ema AAPL
	schwab-agent ta ema AAPL --period 50 --interval daily --points 10
	schwab-agent ta ema AAPL --period 12 --period 26 --points 5`,
			defaultPeriod: 20,
			multiplier:    3,
			compute:       ta.EMA,
		}, c, w),
		makeCobraSimpleTACommand(simpleTAConfig{
			name:  "rsi",
			usage: "Relative Strength Index",
			usageText: `schwab-agent ta rsi AAPL
	schwab-agent ta rsi AAPL --period 14 --interval daily --points 10`,
			defaultPeriod: 14,
			multiplier:    3,
			compute:       ta.RSI,
		}, c, w),
		cobraTAMACDCommand(c, w),
		cobraTAATRCommand(c, w),
		cobraTABBandsCommand(c, w),
		cobraTAStochCommand(c, w),
		cobraTAADXCommand(c, w),
		cobraTAVWAPCommand(c, w),
		cobraTAHVCommand(c, w),
		cobraTAExpectedMoveCommand(c, w),
	)

	return cmd
}

// cobraParseTAPeriods returns the requested indicator windows from a Cobra command.
// Cobra's IntSlice flag accepts both repeated flags and comma-separated values,
// which lets a user ask for "21, 50, and 200 day moving averages" in a single
// command while keeping the old single --period form working unchanged.
func cobraParseTAPeriods(cmd *cobra.Command) ([]int, error) {
	periods := flagIntSlice(cmd, "period")
	if len(periods) == 0 {
		return nil, newValidationError("at least one period is required")
	}

	seen := make(map[int]struct{}, len(periods))
	for _, period := range periods {
		if period <= 0 {
			return nil, newValidationError("period must be greater than 0")
		}

		if _, ok := seen[period]; ok {
			return nil, newValidationError(fmt.Sprintf("duplicate period: %d", period))
		}
		seen[period] = struct{}{}
	}

	return periods, nil
}

// makeCobraSimpleTACommand builds a Cobra command for a closes-only indicator.
// The returned command fetches candles, extracts close prices, runs the
// indicator's compute function, and writes the result envelope.
func makeCobraSimpleTACommand(cfg simpleTAConfig, c *client.Ref, w io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   cfg.name + " SYMBOL",
		Short: cfg.usage,
		Long:  cfg.usageText,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			symbol := args[0]

			periods, err := cobraParseTAPeriods(cmd)
			if err != nil {
				return err
			}
			period := maxPeriod(periods)
			interval := flagString(cmd, "interval")
			points := flagInt(cmd, "points")

			candles, timestamps, err := fetchAndValidateCandles(cmd.Context(), c, symbol, interval, period*cfg.multiplier, cfg.name)
			if err != nil {
				return err
			}

			closes, err := ta.ExtractClose(candles)
			if err != nil {
				return err
			}

			valuesByPeriod := make(map[int][]float64, len(periods))
			for _, p := range periods {
				values, err := cfg.compute(closes, p)
				if err != nil {
					return err
				}
				valuesByPeriod[p] = values
			}

			if len(periods) == 1 {
				return writeTAOutput(w, cfg.name, symbol, interval, periods[0], points, timestamps, valuesByPeriod[periods[0]])
			}

			return writeMultiTAOutput(w, cfg.name, symbol, interval, periods, points, timestamps, valuesByPeriod)
		},
	}

	cmd.Flags().IntSlice("period", []int{cfg.defaultPeriod}, "Indicator period (repeatable or comma-separated)")
	cmd.Flags().String("interval", "daily", "Data interval (daily, weekly, 1min, 5min, 15min, 30min)")
	cmd.Flags().Int("points", 1, "Number of output points (0 = all)")

	return cmd
}

func cobraTAMACDCommand(c *client.Ref, w io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "macd SYMBOL",
		Short: "Moving Average Convergence/Divergence",
		Long: `schwab-agent ta macd AAPL
schwab-agent ta macd AAPL --fast 12 --slow 26 --signal 9 --interval daily --points 10`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			symbol := args[0]

			fast := flagInt(cmd, "fast")
			slow := flagInt(cmd, "slow")
			signal := flagInt(cmd, "signal")
			interval := flagString(cmd, "interval")
			points := flagInt(cmd, "points")

			// MACD layers slow EMA + signal EMA; 2x the combined period provides convergence.
			candles, timestamps, err := fetchAndValidateCandles(cmd.Context(), c, symbol, interval, (slow+signal)*2, "macd")
			if err != nil {
				return err
			}

			closes, err := ta.ExtractClose(candles)
			if err != nil {
				return err
			}

			macdVals, signalVals, histVals, err := ta.MACD(closes, fast, slow, signal)
			if err != nil {
				return err
			}

			// Align timestamps to MACD output length (shorter due to lookback)
			timestamps = timestamps[len(timestamps)-len(macdVals):]

			out := make([]macdPoint, len(macdVals))
			for i := range macdVals {
				out[i] = macdPoint{
					Datetime:  timestamps[i],
					MACD:      macdVals[i],
					Signal:    signalVals[i],
					Histogram: histVals[i],
				}
			}

			if points > 0 && points < len(out) {
				out = out[len(out)-points:]
			}

			data := macdOutput{
				Indicator: "macd",
				Symbol:    symbol,
				Interval:  interval,
				Fast:      fast,
				Slow:      slow,
				Signal:    signal,
				Values:    out,
			}
			return output.WriteSuccess(w, data, output.NewMetadata())
		},
	}

	cmd.Flags().Int("fast", 12, "Fast EMA period")
	cmd.Flags().Int("slow", 26, "Slow EMA period")
	cmd.Flags().Int("signal", 9, "Signal EMA period")
	cmd.Flags().String("interval", "daily", "Data interval (daily, weekly, 1min, 5min, 15min, 30min)")
	cmd.Flags().Int("points", 1, "Number of output points (0 = all)")

	return cmd
}

func cobraTAATRCommand(c *client.Ref, w io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "atr SYMBOL",
		Short: "Average True Range",
		Long: `schwab-agent ta atr AAPL
schwab-agent ta atr AAPL --period 14 --interval daily --points 10`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			symbol := args[0]

			period := flagInt(cmd, "period")
			interval := flagString(cmd, "interval")
			points := flagInt(cmd, "points")

			// ATR uses Wilder smoothing; 3x period provides convergence stability.
			candles, timestamps, err := fetchAndValidateCandles(cmd.Context(), c, symbol, interval, period*3, "atr")
			if err != nil {
				return err
			}

			highs, err := ta.ExtractHigh(candles)
			if err != nil {
				return err
			}
			lows, err := ta.ExtractLow(candles)
			if err != nil {
				return err
			}
			closes, err := ta.ExtractClose(candles)
			if err != nil {
				return err
			}

			values, err := ta.ATR(highs, lows, closes, period)
			if err != nil {
				return err
			}

			return writeTAOutput(w, "atr", symbol, interval, period, points, timestamps, values)
		},
	}

	cmd.Flags().Int("period", 14, "Indicator period")
	cmd.Flags().String("interval", "daily", "Data interval (daily, weekly, 1min, 5min, 15min, 30min)")
	cmd.Flags().Int("points", 1, "Number of output points (0 = all)")

	return cmd
}

func cobraTABBandsCommand(c *client.Ref, w io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bbands SYMBOL",
		Short: "Bollinger Bands",
		Long: `schwab-agent ta bbands AAPL
schwab-agent ta bbands AAPL --period 20 --std-dev 2.0 --interval daily --points 10`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			symbol := args[0]

			period := flagInt(cmd, "period")
			stdDev := flagFloat64(cmd, "std-dev")
			interval := flagString(cmd, "interval")
			points := flagInt(cmd, "points")

			candles, timestamps, err := fetchAndValidateCandles(cmd.Context(), c, symbol, interval, period, "bbands")
			if err != nil {
				return err
			}

			closes, err := ta.ExtractClose(candles)
			if err != nil {
				return err
			}

			upper, middle, lower, err := ta.BBands(closes, period, stdDev)
			if err != nil {
				return err
			}

			// Align timestamps to BBands output length (shorter due to lookback)
			timestamps = timestamps[len(timestamps)-len(upper):]

			out := make([]bbandsPoint, len(upper))
			for i := range upper {
				out[i] = bbandsPoint{
					Datetime: timestamps[i],
					Upper:    upper[i],
					Middle:   middle[i],
					Lower:    lower[i],
				}
			}

			if points > 0 && points < len(out) {
				out = out[len(out)-points:]
			}

			data := bbandsOutput{
				Indicator: "bbands",
				Symbol:    symbol,
				Interval:  interval,
				Period:    period,
				StdDev:    stdDev,
				Values:    out,
			}
			return output.WriteSuccess(w, data, output.NewMetadata())
		},
	}

	cmd.Flags().Int("period", 20, "BBands period")
	cmd.Flags().Float64("std-dev", 2.0, "Standard deviations")
	cmd.Flags().String("interval", "daily", "Data interval (daily, weekly, 1min, 5min, 15min, 30min)")
	cmd.Flags().Int("points", 1, "Number of output points (0 = all)")

	return cmd
}

func cobraTAStochCommand(c *client.Ref, w io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stoch SYMBOL",
		Short: "Stochastic Oscillator",
		Long: `schwab-agent ta stoch AAPL
schwab-agent ta stoch AAPL --k-period 14 --smooth-k 3 --d-period 3 --interval daily --points 10`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			symbol := args[0]

			kPeriod := flagInt(cmd, "k-period")
			smoothK := flagInt(cmd, "smooth-k")
			dPeriod := flagInt(cmd, "d-period")
			interval := flagString(cmd, "interval")
			points := flagInt(cmd, "points")

			// Stochastic chains three windowed ops (raw %K, smoothed %K, %D);
			// total depth is the sum of all window sizes.
			candles, timestamps, err := fetchAndValidateCandles(cmd.Context(), c, symbol, interval, kPeriod+smoothK+dPeriod, "stoch")
			if err != nil {
				return err
			}

			highs, err := ta.ExtractHigh(candles)
			if err != nil {
				return err
			}
			lows, err := ta.ExtractLow(candles)
			if err != nil {
				return err
			}
			closes, err := ta.ExtractClose(candles)
			if err != nil {
				return err
			}

			slowK, slowD, err := ta.Stochastic(highs, lows, closes, kPeriod, smoothK, dPeriod)
			if err != nil {
				return err
			}

			// Align timestamps to stochastic output length (shorter due to lookback)
			timestamps = timestamps[len(timestamps)-len(slowK):]

			out := make([]stochPoint, len(slowK))
			for i := range slowK {
				out[i] = stochPoint{
					Datetime: timestamps[i],
					SlowK:    slowK[i],
					SlowD:    slowD[i],
				}
			}

			if points > 0 && points < len(out) {
				out = out[len(out)-points:]
			}

			data := stochOutput{
				Indicator: "stoch",
				Symbol:    symbol,
				Interval:  interval,
				KPeriod:   kPeriod,
				SmoothK:   smoothK,
				DPeriod:   dPeriod,
				Values:    out,
			}
			return output.WriteSuccess(w, data, output.NewMetadata())
		},
	}

	cmd.Flags().Int("k-period", 14, "Fast %K lookback period")
	cmd.Flags().Int("smooth-k", 3, "Slow %K smoothing period")
	cmd.Flags().Int("d-period", 3, "Slow %D period")
	cmd.Flags().String("interval", "daily", "Data interval (daily, weekly, 1min, 5min, 15min, 30min)")
	cmd.Flags().Int("points", 1, "Number of output points (0 = all)")

	return cmd
}

func cobraTAADXCommand(c *client.Ref, w io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "adx SYMBOL",
		Short: "Average Directional Index",
		Long: `schwab-agent ta adx AAPL
schwab-agent ta adx AAPL --period 14 --interval daily --points 10`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			symbol := args[0]

			period := flagInt(cmd, "period")
			interval := flagString(cmd, "interval")
			points := flagInt(cmd, "points")

			// ADX double-smooths (DI pass + ADX pass); 4x period ensures both converge.
			candles, timestamps, err := fetchAndValidateCandles(cmd.Context(), c, symbol, interval, period*4, "adx")
			if err != nil {
				return err
			}

			highs, err := ta.ExtractHigh(candles)
			if err != nil {
				return err
			}
			lows, err := ta.ExtractLow(candles)
			if err != nil {
				return err
			}
			closes, err := ta.ExtractClose(candles)
			if err != nil {
				return err
			}

			adxVals, plusDI, minusDI, err := ta.ADX(highs, lows, closes, period)
			if err != nil {
				return err
			}

			// Align timestamps to ADX output length (shorter due to lookback)
			timestamps = timestamps[len(timestamps)-len(adxVals):]

			out := make([]adxPoint, len(adxVals))
			for i := range adxVals {
				out[i] = adxPoint{
					Datetime: timestamps[i],
					ADX:      adxVals[i],
					PlusDI:   plusDI[i],
					MinusDI:  minusDI[i],
				}
			}

			if points > 0 && points < len(out) {
				out = out[len(out)-points:]
			}

			data := adxOutput{
				Indicator: "adx",
				Symbol:    symbol,
				Interval:  interval,
				Period:    period,
				Values:    out,
			}
			return output.WriteSuccess(w, data, output.NewMetadata())
		},
	}

	cmd.Flags().Int("period", 14, "Indicator period")
	cmd.Flags().String("interval", "daily", "Data interval (daily, weekly, 1min, 5min, 15min, 30min)")
	cmd.Flags().Int("points", 1, "Number of output points (0 = all)")

	return cmd
}

func cobraTAVWAPCommand(c *client.Ref, w io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "vwap SYMBOL",
		Short: "Volume Weighted Average Price",
		Long: `schwab-agent ta vwap AAPL
schwab-agent ta vwap AAPL --interval 5min --points 10`,
		Args: cobra.ExactArgs(1),
		// VWAP is cumulative - no --period flag. Only --interval and --points.
		RunE: func(cmd *cobra.Command, args []string) error {
			symbol := args[0]

			interval := flagString(cmd, "interval")
			points := flagInt(cmd, "points")

			// Use 20 as minimum candle count - VWAP works with any count >= 1
			// but 20 gives meaningful output for typical use cases.
			candles, timestamps, err := fetchAndValidateCandles(cmd.Context(), c, symbol, interval, 20, "vwap")
			if err != nil {
				return err
			}

			highs, err := ta.ExtractHigh(candles)
			if err != nil {
				return err
			}
			lows, err := ta.ExtractLow(candles)
			if err != nil {
				return err
			}
			closes, err := ta.ExtractClose(candles)
			if err != nil {
				return err
			}
			volumes, err := ta.ExtractVolume(candles)
			if err != nil {
				return err
			}

			values, err := ta.VWAP(highs, lows, closes, volumes)
			if err != nil {
				return err
			}

			// period=0 because VWAP is cumulative (no windowed period)
			return writeTAOutput(w, "vwap", symbol, interval, 0, points, timestamps, values)
		},
	}

	cmd.Flags().String("interval", "daily", "Data interval (daily, weekly, 1min, 5min, 15min, 30min)")
	cmd.Flags().Int("points", 1, "Number of output points (0 = all)")

	return cmd
}

func cobraTAHVCommand(c *client.Ref, w io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hv SYMBOL",
		Short: "Historical Volatility with regime classification",
		Long: `schwab-agent ta hv AAPL
schwab-agent ta hv AAPL --period 20 --interval daily`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			symbol := args[0]

			period := flagInt(cmd, "period")
			interval := flagString(cmd, "interval")

			// Need period+1 closes for N returns, plus extra for rolling window warmup.
			// period+21 provides a safety margin for the rolling window.
			candles, _, err := fetchAndValidateCandles(cmd.Context(), c, symbol, interval, period+21, "hv")
			if err != nil {
				return err
			}

			closes, err := ta.ExtractClose(candles)
			if err != nil {
				return err
			}

			result, err := ta.HistoricalVolatility(closes, period)
			if err != nil {
				return err
			}

			// HV returns a scalar summary, not a time series.
			// Do NOT use writeTAOutput (which is for time series).
			data := hvOutput{
				Indicator:      "hv",
				Symbol:         symbol,
				Interval:       interval,
				Period:         period,
				DailyVol:       result.DailyVol,
				WeeklyVol:      result.WeeklyVol,
				MonthlyVol:     result.MonthlyVol,
				AnnualizedVol:  result.AnnualizedVol,
				PercentileRank: result.PercentileRank,
				Regime:         result.Regime,
				MinVol:         result.MinVol,
				MaxVol:         result.MaxVol,
				MeanVol:        result.MeanVol,
			}
			return output.WriteSuccess(w, data, output.NewMetadata())
		},
	}

	cmd.Flags().Int("period", 20, "Indicator period")
	cmd.Flags().String("interval", "daily", "Data interval (daily, weekly, 1min, 5min, 15min, 30min)")
	cmd.Flags().Int("points", 1, "Number of output points (0 = all)")

	return cmd
}

func cobraTAExpectedMoveCommand(c *client.Ref, w io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "expected-move SYMBOL",
		Short: "Expected price move from ATM straddle pricing",
		Long: `schwab-agent ta expected-move AAPL
schwab-agent ta expected-move AAPL --dte 45`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			symbol := args[0]

			targetDTE := flagInt(cmd, "dte")

			// Expected Move needs the underlying quote and near-the-money contracts in one response.
			// Keep this to a single chain call so the underlying snapshot and option prices stay aligned.
			chain, err := c.OptionChain(cmd.Context(), symbol, &client.ChainParams{
				ContractType:           "ALL",
				IncludeUnderlyingQuote: "true",
				StrikeRange:            "NTM",
			})
			if err != nil {
				return err
			}

			if chain.Underlying == nil {
				return fmt.Errorf("no underlying data available for %s", symbol)
			}

			var underlyingPrice float64
			switch {
			case chain.Underlying.Mark != nil:
				underlyingPrice = *chain.Underlying.Mark
			case chain.Underlying.Last != nil:
				underlyingPrice = *chain.Underlying.Last
			case chain.UnderlyingPrice != nil:
				underlyingPrice = *chain.UnderlyingPrice
			default:
				return fmt.Errorf("unable to determine underlying price for %s", symbol)
			}

			if len(chain.CallExpDateMap) == 0 {
				return fmt.Errorf("no options available for %s", symbol)
			}

			selectedExpKey := ""
			bestDiff := -1
			for expKey := range chain.CallExpDateMap {
				parts := strings.Split(expKey, ":")
				if len(parts) != 2 {
					continue
				}

				dte, err := strconv.Atoi(parts[1])
				if err != nil {
					continue
				}

				diff := int(math.Abs(float64(dte - targetDTE)))
				if bestDiff < 0 || diff < bestDiff || (diff == bestDiff && expKey < selectedExpKey) {
					bestDiff = diff
					selectedExpKey = expKey
				}
			}
			if selectedExpKey == "" {
				return fmt.Errorf("no options available for %s", symbol)
			}

			expParts := strings.Split(selectedExpKey, ":")
			expDate := expParts[0]

			callStrikes := chain.CallExpDateMap[selectedExpKey]
			putStrikes := chain.PutExpDateMap[selectedExpKey]
			if len(callStrikes) == 0 || len(putStrikes) == 0 {
				return fmt.Errorf("no options available for %s at expiration %s", symbol, expDate)
			}

			atmStrikeKey := ""
			bestStrikeDiff := -1.0
			for strikeKey := range callStrikes {
				strike, err := strconv.ParseFloat(strikeKey, 64)
				if err != nil {
					continue
				}

				diff := math.Abs(strike - underlyingPrice)
				if bestStrikeDiff < 0 || diff < bestStrikeDiff || (diff == bestStrikeDiff && strike < mustParseFloat(atmStrikeKey)) {
					bestStrikeDiff = diff
					atmStrikeKey = strikeKey
				}
			}
			if atmStrikeKey == "" {
				return fmt.Errorf("no strikes available for %s at expiration %s", symbol, expDate)
			}

			callContracts := callStrikes[atmStrikeKey]
			putContracts := putStrikes[atmStrikeKey]
			if len(callContracts) == 0 {
				return fmt.Errorf("no call contracts for %s at strike %s", symbol, atmStrikeKey)
			}
			if len(putContracts) == 0 {
				return fmt.Errorf("no put contracts for %s at strike %s", symbol, atmStrikeKey)
			}

			callPrice, err := contractPrice(callContracts[0], symbol, atmStrikeKey, "call")
			if err != nil {
				return err
			}
			putPrice, err := contractPrice(putContracts[0], symbol, atmStrikeKey, "put")
			if err != nil {
				return err
			}

			result, err := ta.ExpectedMove(underlyingPrice, callPrice, putPrice, ta.DefaultMultiplier)
			if err != nil {
				return err
			}

			actualDTE, _ := strconv.Atoi(expParts[1])
			data := expectedMoveOutput{
				Indicator:       "expected-move",
				Symbol:          symbol,
				UnderlyingPrice: underlyingPrice,
				Expiration:      expDate,
				DTE:             actualDTE,
				StraddlePrice:   result.StraddlePrice,
				ExpectedMove:    result.ExpectedMove,
				AdjustedMove:    result.AdjustedMove,
				Upper1x:         result.Upper1x,
				Lower1x:         result.Lower1x,
				Upper2x:         result.Upper2x,
				Lower2x:         result.Lower2x,
			}
			return output.WriteSuccess(w, data, output.NewMetadata())
		},
	}

	cmd.Flags().Int("dte", 30, "Target days to expiration")

	return cmd
}

func contractPrice(contract *models.OptionContract, symbol, strike, putCall string) (float64, error) {
	if contract.Mark != nil {
		return *contract.Mark, nil
	}
	if contract.Bid != nil && contract.Ask != nil {
		return (*contract.Bid + *contract.Ask) / 2, nil
	}

	return 0, fmt.Errorf("unable to determine %s price for %s at strike %s", putCall, symbol, strike)
}

// mustParseFloat parses a strike key for ATM tie-breaking.
// Returning 0 on parse failure is fine because this helper is only used after empty-string checks.
func mustParseFloat(s string) float64 {
	if s == "" {
		return 0
	}

	v, _ := strconv.ParseFloat(s, 64)
	return v
}

// fetchAndValidateCandles fetches price history and validates minimum candle count.
// requiredCandles is indicator-specific: each caller computes its own minimum based
// on warmup needs (e.g. SMA needs exactly period, EMA needs 3x for convergence,
// ADX needs 4x for double smoothing). The history lookback window is scaled to
// match. See https://github.com/major/schwab-agent/issues/12.
func fetchAndValidateCandles(
	ctx context.Context,
	c *client.Ref,
	symbol, interval string,
	requiredCandles int,
	indicator string,
) ([]models.Candle, []string, error) {
	periodType, periodVal, freqType, freq, err := ta.IntervalToHistoryParams(interval, requiredCandles)
	if err != nil {
		return nil, nil, err
	}

	params := client.HistoryParams{
		PeriodType:    periodType,
		Period:        periodVal,
		FrequencyType: freqType,
		Frequency:     freq,
	}
	result, err := c.PriceHistory(ctx, symbol, &params)
	if err != nil {
		return nil, nil, err
	}

	if err := ta.ValidateMinCandles(result.Candles, requiredCandles, indicator); err != nil {
		return nil, nil, err
	}

	timestamps := ta.ExtractTimestamps(result.Candles)
	return result.Candles, timestamps, nil
}

// writeTAOutput builds the output map and writes it as a success envelope.
// Aligns timestamps to indicator values (shorter due to lookback) and applies --points limit.
func writeTAOutput(
	w io.Writer,
	indicator, symbol, interval string,
	period, points int,
	timestamps []string,
	values []float64,
) error {
	// Align timestamps: indicator output is shorter than input due to lookback window
	timestamps = timestamps[len(timestamps)-len(values):]

	out := make([]map[string]any, len(values))
	for i, v := range values {
		out[i] = map[string]any{
			"datetime": timestamps[i],
			indicator:  v,
		}
	}

	// Apply --points limit (tail-slice to get most recent N entries)
	if points > 0 && points < len(out) {
		out = out[len(out)-points:]
	}

	data := taOutput{
		Indicator: indicator,
		Symbol:    symbol,
		Interval:  interval,
		Period:    period,
		Values:    out,
	}
	return output.WriteSuccess(w, data, output.TimestampMeta())
}

// writeMultiTAOutput builds a combined time series for multiple requested
// periods. Each row represents a candle timestamp and contains one value per
// period using stable keys such as sma_21, sma_50, and sma_200.
func writeMultiTAOutput(
	w io.Writer,
	indicator, symbol, interval string,
	periods []int,
	points int,
	timestamps []string,
	valuesByPeriod map[int][]float64,
) error {
	maxValues := 0
	for _, period := range periods {
		if len(valuesByPeriod[period]) > maxValues {
			maxValues = len(valuesByPeriod[period])
		}
	}

	start := len(timestamps) - maxValues
	out := make([]map[string]any, maxValues)
	for i := range maxValues {
		out[i] = map[string]any{"datetime": timestamps[start+i]}
	}

	for _, period := range periods {
		values := valuesByPeriod[period]
		valueStart := maxValues - len(values)
		key := fmt.Sprintf("%s_%d", indicator, period)
		for i, value := range values {
			out[valueStart+i][key] = value
		}
	}

	// Apply --points after merging so the latest timestamp can include every
	// requested period in one object, which is the common crossover use case.
	if points > 0 && points < len(out) {
		out = out[len(out)-points:]
	}

	data := multiTAOutput{
		Indicator: indicator,
		Symbol:    symbol,
		Interval:  interval,
		Periods:   periods,
		Values:    out,
	}
	return output.WriteSuccess(w, data, output.TimestampMeta())
}
