// Package commands provides CLI command builders for schwab-agent.
package commands

import (
	"context"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"

	"github.com/leodido/structcli"
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
	name    string
	usage   string
	long    string
	example string
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

// simpleTAOpts holds CLI flags for factory-generated indicator commands (SMA, EMA, RSI).
// Period is excluded from struct tags because its default varies per indicator
// (cfg.defaultPeriod) and structcli tags require static defaults.
type simpleTAOpts struct {
	Period   []int
	Interval string `flag:"interval" flagdescr:"Data interval (daily, weekly, 1min, 5min, 15min, 30min)" default:"daily"`
	Points   int    `flag:"points" flagdescr:"Number of output points (0 = all)" default:"1"`
}

// Attach implements structcli.Options interface.
func (o *simpleTAOpts) Attach(_ *cobra.Command) error { return nil }

// macdOpts holds CLI flags for the MACD subcommand.
type macdOpts struct {
	Fast     int    `flag:"fast" flagdescr:"Fast EMA period" default:"12"`
	Slow     int    `flag:"slow" flagdescr:"Slow EMA period" default:"26"`
	Signal   int    `flag:"signal" flagdescr:"Signal EMA period" default:"9"`
	Interval string `flag:"interval" flagdescr:"Data interval (daily, weekly, 1min, 5min, 15min, 30min)" default:"daily"`
	Points   int    `flag:"points" flagdescr:"Number of output points (0 = all)" default:"1"`
}

// Attach implements structcli.Options interface.
func (o *macdOpts) Attach(_ *cobra.Command) error { return nil }

// atrOpts holds CLI flags for the ATR subcommand.
type atrOpts struct {
	Period   int    `flag:"period" flagdescr:"Indicator period" default:"14"`
	Interval string `flag:"interval" flagdescr:"Data interval (daily, weekly, 1min, 5min, 15min, 30min)" default:"daily"`
	Points   int    `flag:"points" flagdescr:"Number of output points (0 = all)" default:"1"`
}

// Attach implements structcli.Options interface.
func (o *atrOpts) Attach(_ *cobra.Command) error { return nil }

// bbandsOpts holds CLI flags for the Bollinger Bands subcommand.
type bbandsOpts struct {
	Period   int     `flag:"period" flagdescr:"BBands period" default:"20"`
	StdDev   float64 `flag:"std-dev" flagdescr:"Standard deviations" default:"2.0"`
	Interval string  `flag:"interval" flagdescr:"Data interval (daily, weekly, 1min, 5min, 15min, 30min)" default:"daily"`
	Points   int     `flag:"points" flagdescr:"Number of output points (0 = all)" default:"1"`
}

// Attach implements structcli.Options interface.
func (o *bbandsOpts) Attach(_ *cobra.Command) error { return nil }

// stochOpts holds CLI flags for the Stochastic Oscillator subcommand.
type stochOpts struct {
	KPeriod  int    `flag:"k-period" flagdescr:"Fast %K lookback period" default:"14"`
	SmoothK  int    `flag:"smooth-k" flagdescr:"Slow %K smoothing period" default:"3"`
	DPeriod  int    `flag:"d-period" flagdescr:"Slow %D period" default:"3"`
	Interval string `flag:"interval" flagdescr:"Data interval (daily, weekly, 1min, 5min, 15min, 30min)" default:"daily"`
	Points   int    `flag:"points" flagdescr:"Number of output points (0 = all)" default:"1"`
}

// Attach implements structcli.Options interface.
func (o *stochOpts) Attach(_ *cobra.Command) error { return nil }

// adxOpts holds CLI flags for the ADX subcommand.
type adxOpts struct {
	Period   int    `flag:"period" flagdescr:"Indicator period" default:"14"`
	Interval string `flag:"interval" flagdescr:"Data interval (daily, weekly, 1min, 5min, 15min, 30min)" default:"daily"`
	Points   int    `flag:"points" flagdescr:"Number of output points (0 = all)" default:"1"`
}

// Attach implements structcli.Options interface.
func (o *adxOpts) Attach(_ *cobra.Command) error { return nil }

// vwapOpts holds CLI flags for the VWAP subcommand.
type vwapOpts struct {
	Interval string `flag:"interval" flagdescr:"Data interval (daily, weekly, 1min, 5min, 15min, 30min)" default:"daily"`
	Points   int    `flag:"points" flagdescr:"Number of output points (0 = all)" default:"1"`
}

// Attach implements structcli.Options interface.
func (o *vwapOpts) Attach(_ *cobra.Command) error { return nil }

// hvOpts holds CLI flags for the Historical Volatility subcommand.
type hvOpts struct {
	Period   int    `flag:"period" flagdescr:"Indicator period" default:"20"`
	Interval string `flag:"interval" flagdescr:"Data interval (daily, weekly, 1min, 5min, 15min, 30min)" default:"daily"`
}

// Attach implements structcli.Options interface.
func (o *hvOpts) Attach(_ *cobra.Command) error { return nil }

// expectedMoveOpts holds CLI flags for the Expected Move subcommand.
type expectedMoveOpts struct {
	DTE int `flag:"dte" flagdescr:"Target days to expiration" default:"30"`
}

// Attach implements structcli.Options interface.
func (o *expectedMoveOpts) Attach(_ *cobra.Command) error { return nil }

// NewTACmd returns the Cobra command for technical analysis indicators.
func NewTACmd(c *client.Ref, w io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ta",
		Short: "Technical analysis indicators",
		Long: `Compute technical analysis indicators from Schwab price history. All indicators
are calculated locally after fetching candles. Use --interval to select data
frequency (daily, weekly, 1min, 5min, 15min, 30min) and --points to limit
output to the N most recent values.`,
		GroupID: "market-data",
		RunE:    requireSubcommand,
	}
	cmd.SetFlagErrorFunc(suggestSubcommands)

	cmd.AddCommand(
		makeCobraSimpleTACommand(&simpleTAConfig{
			name:  "sma",
			usage: "Simple Moving Average",
			long: `Compute Simple Moving Average for a symbol. The --period flag sets the lookback
window (default 20). Multiple periods can be requested in one command using
comma-separated values or repeated flags, producing crossover-ready output
with keys like sma_21, sma_50, sma_200.`,
			example: `  schwab-agent ta sma AAPL
  schwab-agent ta sma AAPL --period 50 --interval daily --points 10
  schwab-agent ta sma AAPL --period 21,50,200 --points 1`,
			defaultPeriod: 20,
			multiplier:    1,
			compute:       ta.SMA,
		}, c, w),
		makeCobraSimpleTACommand(&simpleTAConfig{
			name:  "ema",
			usage: "Exponential Moving Average",
			long: `Compute Exponential Moving Average for a symbol. EMA gives more weight to
recent prices than SMA. The --period flag can be repeated or comma-separated
for multiple lookbacks in one command (e.g. --period 12,26 for crossover
analysis).`,
			example: `  schwab-agent ta ema AAPL
  schwab-agent ta ema AAPL --period 50 --interval daily --points 10
  schwab-agent ta ema AAPL --period 12,26 --points 5`,
			defaultPeriod: 20,
			multiplier:    3,
			compute:       ta.EMA,
		}, c, w),
		makeCobraSimpleTACommand(&simpleTAConfig{
			name:  "rsi",
			usage: "Relative Strength Index",
			long: `Compute Relative Strength Index for a symbol. RSI ranges 0-100: values above
70 suggest overbought conditions, below 30 suggest oversold. The --period flag
can be repeated or comma-separated for multiple lookbacks. Default period is 14.`,
			example: `  schwab-agent ta rsi AAPL
  schwab-agent ta rsi AAPL --period 14 --interval daily --points 10
  schwab-agent ta rsi AAPL --period 9 --interval 5min --points 20`,
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

// cobraParseTAPeriods returns the requested indicator windows from the option struct.
// Cobra's IntSlice flag accepts both repeated flags and comma-separated values,
// which lets a user ask for "21, 50, and 200 day moving averages" in a single
// command while keeping the old single --period form working unchanged.
func cobraParseTAPeriods(opts *simpleTAOpts) ([]int, error) {
	periods := opts.Period
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
func makeCobraSimpleTACommand(cfg *simpleTAConfig, c *client.Ref, w io.Writer) *cobra.Command {
	opts := &simpleTAOpts{}
	cmd := &cobra.Command{
		Use:     cfg.name + " SYMBOL",
		Short:   cfg.usage,
		Long:    cfg.long,
		Example: cfg.example,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := structcli.Unmarshal(cmd, opts); err != nil {
				return err
			}

			symbol := args[0]

			periods, err := cobraParseTAPeriods(opts)
			if err != nil {
				return err
			}
			period := maxPeriod(periods)

			candles, timestamps, err := fetchAndValidateCandles(cmd.Context(), c, symbol, opts.Interval, period*cfg.multiplier, cfg.name)
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
				return writeTAOutput(w, cfg.name, symbol, opts.Interval, periods[0], opts.Points, timestamps, valuesByPeriod[periods[0]])
			}

			return writeMultiTAOutput(w, cfg.name, symbol, opts.Interval, periods, opts.Points, timestamps, valuesByPeriod)
		},
	}

	// Period uses IntSliceVar because its default varies per indicator (cfg.defaultPeriod)
	// and structcli struct tags require static defaults.
	// Register BEFORE Define so structcli's schema generator handles it correctly.
	cmd.Flags().IntSliceVar(&opts.Period, "period", []int{cfg.defaultPeriod}, "Indicator period (repeatable or comma-separated)")

	if err := structcli.Define(cmd, opts, structcli.WithExclusions("period")); err != nil {
		panic(err)
	}

	return cmd
}

func cobraTAMACDCommand(c *client.Ref, w io.Writer) *cobra.Command {
	opts := &macdOpts{}
	cmd := &cobra.Command{
		Use:   "macd SYMBOL",
		Short: "Moving Average Convergence/Divergence",
		Long: `Compute MACD (Moving Average Convergence/Divergence) for a symbol. Uses
--fast, --slow, and --signal periods instead of --period. Output keys include
macd, signal, and histogram (histogram = macd - signal). Defaults: fast=12,
slow=26, signal=9.`,
		Example: `  schwab-agent ta macd AAPL
  schwab-agent ta macd AAPL --fast 12 --slow 26 --signal 9 --points 10
  schwab-agent ta macd NVDA --interval weekly --points 10`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := structcli.Unmarshal(cmd, opts); err != nil {
				return err
			}

			symbol := args[0]

			// MACD layers slow EMA + signal EMA; 2x the combined period provides convergence.
			candles, timestamps, err := fetchAndValidateCandles(cmd.Context(), c, symbol, opts.Interval, (opts.Slow+opts.Signal)*2, "macd")
			if err != nil {
				return err
			}

			closes, err := ta.ExtractClose(candles)
			if err != nil {
				return err
			}

			macdVals, signalVals, histVals, err := ta.MACD(closes, opts.Fast, opts.Slow, opts.Signal)
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

			if opts.Points > 0 && opts.Points < len(out) {
				out = out[len(out)-opts.Points:]
			}

			data := macdOutput{
				Indicator: "macd",
				Symbol:    symbol,
				Interval:  opts.Interval,
				Fast:      opts.Fast,
				Slow:      opts.Slow,
				Signal:    opts.Signal,
				Values:    out,
			}
			return output.WriteSuccess(w, data, output.NewMetadata())
		},
	}

	if err := structcli.Define(cmd, opts); err != nil {
		panic(err)
	}

	return cmd
}

func cobraTAATRCommand(c *client.Ref, w io.Writer) *cobra.Command {
	opts := &atrOpts{}
	cmd := &cobra.Command{
		Use:   "atr SYMBOL",
		Short: "Average True Range",
		Long: `Compute Average True Range for a symbol. ATR measures volatility in price
units: higher values indicate wider price swings. Uses high, low, and close
data with Wilder smoothing. Default period is 14.`,
		Example: `  schwab-agent ta atr AAPL
  schwab-agent ta atr AAPL --period 14 --interval daily --points 10`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := structcli.Unmarshal(cmd, opts); err != nil {
				return err
			}

			symbol := args[0]

			// ATR uses Wilder smoothing; 3x period provides convergence stability.
			candles, timestamps, err := fetchAndValidateCandles(cmd.Context(), c, symbol, opts.Interval, opts.Period*3, "atr")
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

			values, err := ta.ATR(highs, lows, closes, opts.Period)
			if err != nil {
				return err
			}

			return writeTAOutput(w, "atr", symbol, opts.Interval, opts.Period, opts.Points, timestamps, values)
		},
	}

	if err := structcli.Define(cmd, opts); err != nil {
		panic(err)
	}

	return cmd
}

func cobraTABBandsCommand(c *client.Ref, w io.Writer) *cobra.Command {
	opts := &bbandsOpts{}
	cmd := &cobra.Command{
		Use:   "bbands SYMBOL",
		Short: "Bollinger Bands",
		Long: `Compute Bollinger Bands for a symbol. Output keys are upper, middle, and lower
bands. Price near the upper band is relatively high, near the lower band is
relatively low. Use --std-dev to widen or narrow the bands (default 2.0).`,
		Example: `  schwab-agent ta bbands AAPL
  schwab-agent ta bbands AAPL --period 20 --std-dev 2.0 --points 10
  schwab-agent ta bbands AAPL --std-dev 1.5 --points 10`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := structcli.Unmarshal(cmd, opts); err != nil {
				return err
			}

			symbol := args[0]

			candles, timestamps, err := fetchAndValidateCandles(cmd.Context(), c, symbol, opts.Interval, opts.Period, "bbands")
			if err != nil {
				return err
			}

			closes, err := ta.ExtractClose(candles)
			if err != nil {
				return err
			}

			upper, middle, lower, err := ta.BBands(closes, opts.Period, opts.StdDev)
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

			if opts.Points > 0 && opts.Points < len(out) {
				out = out[len(out)-opts.Points:]
			}

			data := bbandsOutput{
				Indicator: "bbands",
				Symbol:    symbol,
				Interval:  opts.Interval,
				Period:    opts.Period,
				StdDev:    opts.StdDev,
				Values:    out,
			}
			return output.WriteSuccess(w, data, output.NewMetadata())
		},
	}

	if err := structcli.Define(cmd, opts); err != nil {
		panic(err)
	}

	return cmd
}

func cobraTAStochCommand(c *client.Ref, w io.Writer) *cobra.Command {
	opts := &stochOpts{}
	cmd := &cobra.Command{
		Use:   "stoch SYMBOL",
		Short: "Stochastic Oscillator",
		Long: `Compute Stochastic Oscillator for a symbol. Output keys are slowk and slowd,
both ranging 0-100. Above 80 and below 20 are common signal thresholds.
Configurable via --k-period, --smooth-k, and --d-period.`,
		Example: `  schwab-agent ta stoch AAPL
  schwab-agent ta stoch AAPL --k-period 14 --smooth-k 3 --d-period 3 --points 10`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := structcli.Unmarshal(cmd, opts); err != nil {
				return err
			}

			symbol := args[0]

			// Stochastic chains three windowed ops (raw %K, smoothed %K, %D);
			// total depth is the sum of all window sizes.
			candles, timestamps, err := fetchAndValidateCandles(cmd.Context(), c, symbol, opts.Interval, opts.KPeriod+opts.SmoothK+opts.DPeriod, "stoch")
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

			slowK, slowD, err := ta.Stochastic(highs, lows, closes, opts.KPeriod, opts.SmoothK, opts.DPeriod)
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

			if opts.Points > 0 && opts.Points < len(out) {
				out = out[len(out)-opts.Points:]
			}

			data := stochOutput{
				Indicator: "stoch",
				Symbol:    symbol,
				Interval:  opts.Interval,
				KPeriod:   opts.KPeriod,
				SmoothK:   opts.SmoothK,
				DPeriod:   opts.DPeriod,
				Values:    out,
			}
			return output.WriteSuccess(w, data, output.NewMetadata())
		},
	}

	if err := structcli.Define(cmd, opts); err != nil {
		panic(err)
	}

	return cmd
}

func cobraTAADXCommand(c *client.Ref, w io.Writer) *cobra.Command {
	opts := &adxOpts{}
	cmd := &cobra.Command{
		Use:   "adx SYMBOL",
		Short: "Average Directional Index",
		Long: `Compute Average Directional Index for a symbol. ADX above 25 indicates a
trending market, below 20 indicates a ranging market. Output includes adx,
plus_di, and minus_di for directional bias. Default period is 14.`,
		Example: `  schwab-agent ta adx AAPL
  schwab-agent ta adx AAPL --period 14 --interval daily --points 10`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := structcli.Unmarshal(cmd, opts); err != nil {
				return err
			}

			symbol := args[0]

			// ADX double-smooths (DI pass + ADX pass); 4x period ensures both converge.
			candles, timestamps, err := fetchAndValidateCandles(cmd.Context(), c, symbol, opts.Interval, opts.Period*4, "adx")
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

			adxVals, plusDI, minusDI, err := ta.ADX(highs, lows, closes, opts.Period)
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

			if opts.Points > 0 && opts.Points < len(out) {
				out = out[len(out)-opts.Points:]
			}

			data := adxOutput{
				Indicator: "adx",
				Symbol:    symbol,
				Interval:  opts.Interval,
				Period:    opts.Period,
				Values:    out,
			}
			return output.WriteSuccess(w, data, output.NewMetadata())
		},
	}

	if err := structcli.Define(cmd, opts); err != nil {
		panic(err)
	}

	return cmd
}

func cobraTAVWAPCommand(c *client.Ref, w io.Writer) *cobra.Command {
	opts := &vwapOpts{}
	cmd := &cobra.Command{
		Use:   "vwap SYMBOL",
		Short: "Volume Weighted Average Price",
		Long: `Compute Volume Weighted Average Price for a symbol. VWAP is a cumulative
indicator with no --period flag. Primarily used for intraday analysis with
minute-level intervals. Price above VWAP suggests bullish bias, below
suggests bearish.`,
		Example: `  schwab-agent ta vwap AAPL
  schwab-agent ta vwap AAPL --interval 5min --points 20`,
		Args: cobra.ExactArgs(1),
		// VWAP is cumulative - no --period flag. Only --interval and --points.
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := structcli.Unmarshal(cmd, opts); err != nil {
				return err
			}

			symbol := args[0]

			// Use 20 as minimum candle count - VWAP works with any count >= 1
			// but 20 gives meaningful output for typical use cases.
			candles, timestamps, err := fetchAndValidateCandles(cmd.Context(), c, symbol, opts.Interval, 20, "vwap")
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
			return writeTAOutput(w, "vwap", symbol, opts.Interval, 0, opts.Points, timestamps, values)
		},
	}

	if err := structcli.Define(cmd, opts); err != nil {
		panic(err)
	}

	return cmd
}

func cobraTAHVCommand(c *client.Ref, w io.Writer) *cobra.Command {
	opts := &hvOpts{}
	cmd := &cobra.Command{
		Use:   "hv SYMBOL",
		Short: "Historical Volatility with regime classification",
		Long: `Compute Historical Volatility for a symbol. Returns a scalar summary (not
time series) with annualized volatility, percentile rank, and regime
classification (low, normal, high, extreme). Includes daily, weekly, and
monthly volatility breakdowns.`,
		Example: `  schwab-agent ta hv AAPL
  schwab-agent ta hv AAPL --period 20 --interval daily`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := structcli.Unmarshal(cmd, opts); err != nil {
				return err
			}

			symbol := args[0]

			// Need period+1 closes for N returns, plus extra for rolling window warmup.
			// period+21 provides a safety margin for the rolling window.
			candles, _, err := fetchAndValidateCandles(cmd.Context(), c, symbol, opts.Interval, opts.Period+21, "hv")
			if err != nil {
				return err
			}

			closes, err := ta.ExtractClose(candles)
			if err != nil {
				return err
			}

			result, err := ta.HistoricalVolatility(closes, opts.Period)
			if err != nil {
				return err
			}

			// HV returns a scalar summary, not a time series.
			// Do NOT use writeTAOutput (which is for time series).
			data := hvOutput{
				Indicator:      "hv",
				Symbol:         symbol,
				Interval:       opts.Interval,
				Period:         opts.Period,
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

	if err := structcli.Define(cmd, opts); err != nil {
		panic(err)
	}

	return cmd
}

func cobraTAExpectedMoveCommand(c *client.Ref, w io.Writer) *cobra.Command {
	opts := &expectedMoveOpts{}
	cmd := &cobra.Command{
		Use:   "expected-move SYMBOL",
		Short: "Expected price move from ATM straddle pricing",
		Long: `Compute expected price move from ATM straddle pricing. Fetches the option
chain, finds the nearest expiration to --dte (default 30), and prices the
ATM straddle. Output includes 1x and 2x standard deviation ranges (upper
and lower bounds).`,
		Example: `  schwab-agent ta expected-move AAPL
  schwab-agent ta expected-move AAPL --dte 45`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := structcli.Unmarshal(cmd, opts); err != nil {
				return err
			}

			symbol := args[0]

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

				diff := int(math.Abs(float64(dte - opts.DTE)))
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

	if err := structcli.Define(cmd, opts); err != nil {
		panic(err)
	}

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
