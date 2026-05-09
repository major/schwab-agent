package commands

import (
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"math"
	"strconv"
	"strings"

	"github.com/major/schwab-go/schwab/marketdata"
	"github.com/spf13/cobra"

	"github.com/major/schwab-agent/internal/apperr"
	"github.com/major/schwab-agent/internal/client"
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

type expectedMoveRange struct {
	Lower float64 `json:"lower"`
	Upper float64 `json:"upper"`
}

type expectedMoveOutput struct {
	Indicator        string                       `json:"indicator"`
	Symbol           string                       `json:"symbol"`
	UnderlyingPrice  float64                      `json:"underlying_price"`
	Expiration       string                       `json:"expiration"`
	DTE              int                          `json:"dte"`
	StraddlePrice    float64                      `json:"straddle_price"`
	ExpectedMove     float64                      `json:"expected_move"`
	ExpectedMovePct  float64                      `json:"expected_move_pct"`
	AdjustedMove     float64                      `json:"adjusted_move"`
	AdjustmentMethod string                       `json:"adjustment_method"`
	CurrentIV        float64                      `json:"current_iv,omitempty"`
	Ranges           map[string]expectedMoveRange `json:"ranges"`
	Upper1x          float64                      `json:"upper_1x"`
	Lower1x          float64                      `json:"lower_1x"`
	Upper2x          float64                      `json:"upper_2x"`
	Lower2x          float64                      `json:"lower_2x"`
}

type dashboardParameters struct {
	SMAPeriods   []int   `json:"sma_periods"`
	RSIPeriod    int     `json:"rsi_period"`
	MACDFast     int     `json:"macd_fast"`
	MACDSlow     int     `json:"macd_slow"`
	MACDSignal   int     `json:"macd_signal"`
	ATRPeriod    int     `json:"atr_period"`
	BBandsPeriod int     `json:"bbands_period"`
	BBandsStdDev float64 `json:"bbands_std_dev"`
	VolumePeriod int     `json:"volume_period"`
	RangePeriod  int     `json:"range_period"`
	LongRange    int     `json:"long_range"`
}

type dashboardOutput struct {
	Indicator  string              `json:"indicator"`
	Symbol     string              `json:"symbol"`
	Interval   string              `json:"interval"`
	Parameters dashboardParameters `json:"parameters"`
	Latest     map[string]any      `json:"latest"`
	Signals    map[string]any      `json:"signals"`
	Values     []map[string]any    `json:"values,omitempty"`
}

type dashboardInputs struct {
	Candles    []marketdata.Candle
	Timestamps []string
	Opens      []float64
	Highs      []float64
	Lows       []float64
	Closes     []float64
	Volumes    []float64
}

type dashboardSeries struct {
	SMA21       []float64
	SMA50       []float64
	SMA200      []float64
	RSI14       []float64
	MACD        []float64
	MACDSignal  []float64
	MACDHist    []float64
	ATR14       []float64
	BBandsUpper []float64
	BBandsMid   []float64
	BBandsLower []float64
	AvgVolume20 []float64
}

const (
	dashboardSMAFastPeriod       = 21
	dashboardSMAMediumPeriod     = 50
	dashboardSMALongPeriod       = 200
	dashboardRSIPeriod           = 14
	dashboardMACDFastPeriod      = 12
	dashboardMACDSlowPeriod      = 26
	dashboardMACDSignalPeriod    = 9
	dashboardATRPeriod           = 14
	dashboardBBandsPeriod        = 20
	dashboardBBandsStdDev        = 2.0
	dashboardVolumePeriod        = 20
	dashboardRangePeriod         = 20
	dashboardLongRangePeriod     = 252
	simpleTADefaultPeriod        = 20
	rsiDefaultPeriod             = 14
	smaConvergenceMultiplier     = 1
	typicalConvergenceMultiplier = 3

	rsiOverboughtThreshold        = 70.0
	rsiOversoldThreshold          = 30.0
	atrElevatedPercentThreshold   = 5.0
	atrCompressedPercentThreshold = 2.0
	relativeVolumeElevatedRatio   = 1.5
	relativeVolumeLightRatio      = 0.75
	macdConvergenceMultiplier     = 2
	atrConvergenceMultiplier      = 3
	adxConvergenceMultiplier      = 4
	vwapMinimumCandles            = 20
	hvWarmupCandles               = 21
	percentMultiplier             = 100.0
	expectedMoveExpirationParts   = 2
	quoteMidpointDivisor          = 2.0
	roundingEpsilon               = 1e-9
)

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

// taCommandConfig describes the Cobra metadata and typed builder shared by TA
// commands that accept one or more symbols and write one result per symbol.
type taCommandConfig[O any, R any] struct {
	name    string
	short   string
	long    string
	example string
	newOpts func() O
	build   func(context.Context, *client.Ref, O, string) (R, error)
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

// writeTASymbolResults writes a stable symbol-keyed object for every TA command,
// including single-symbol requests. Schwab's price-history API is single-symbol,
// so batching still makes one API call per symbol, but agents can always parse
// data[SYMBOL] without branching on request cardinality.
func writeTASymbolResults(w io.Writer, symbols []string, compute func(symbol string) (any, error)) error {
	data := make(map[string]any, len(symbols))
	partialErrors := make([]string, 0)
	joinedErrors := make([]error, 0)
	for _, symbol := range symbols {
		result, err := compute(symbol)
		if err != nil {
			partialErrors = append(partialErrors, fmt.Sprintf("%s: %v", symbol, err))
			joinedErrors = append(joinedErrors, fmt.Errorf("%s: %w", symbol, err))
			continue
		}
		data[symbol] = result
	}

	if len(data) == 0 {
		return errors.Join(joinedErrors...)
	}
	if len(partialErrors) > 0 {
		meta := output.NewMetadata()
		meta.Requested = len(symbols)
		meta.Returned = len(data)
		return output.WritePartial(w, data, partialErrors, meta)
	}

	return output.WriteSuccess(w, data, output.NewMetadata())
}

// simpleTAOpts holds CLI flags for factory-generated indicator commands (SMA, EMA, RSI).
// Period is excluded from struct tags because its default varies per indicator
// (cfg.defaultPeriod) and struct tags require static defaults.
type simpleTAOpts struct {
	Period   []int
	Interval taInterval `flag:"interval" flagdescr:"Data interval (daily, weekly, 1min, 5min, 15min, 30min)" default:"daily" flaggroup:"data"`
	Points   int        `flag:"points"   flagdescr:"Number of output points (default 1; 0 = all)"            default:"1"     flaggroup:"data"`
}

// Validate is called by validateCobraOptions after Cobra decodes bound flags.
// Checks that all requested periods are positive and unique.
func (o *simpleTAOpts) Validate(_ context.Context) []error {
	if len(o.Period) == 0 {
		return []error{errors.New("at least one period is required")}
	}

	var errs []error
	seen := make(map[int]struct{}, len(o.Period))
	for _, period := range o.Period {
		if period <= 0 {
			errs = append(errs, errors.New("period must be greater than 0"))
		}

		if _, ok := seen[period]; ok {
			errs = append(errs, fmt.Errorf("duplicate period: %d", period))
		}
		seen[period] = struct{}{}
	}

	if len(errs) > 0 {
		return errs
	}

	return nil
}

// macdOpts holds CLI flags for the MACD subcommand.
type macdOpts struct {
	Fast     int        `flag:"fast"     flagdescr:"Fast EMA period"                                         default:"12"    flaggroup:"indicator"`
	Slow     int        `flag:"slow"     flagdescr:"Slow EMA period"                                         default:"26"    flaggroup:"indicator"`
	Signal   int        `flag:"signal"   flagdescr:"Signal EMA period"                                       default:"9"     flaggroup:"indicator"`
	Interval taInterval `flag:"interval" flagdescr:"Data interval (daily, weekly, 1min, 5min, 15min, 30min)" default:"daily" flaggroup:"data"`
	Points   int        `flag:"points"   flagdescr:"Number of output points (default 1; 0 = all)"            default:"1"     flaggroup:"data"`
}

// atrOpts holds CLI flags for the ATR subcommand.
type atrOpts struct {
	Period   int        `flag:"period"   flagdescr:"Indicator period"                                        default:"14"    flaggroup:"indicator"`
	Interval taInterval `flag:"interval" flagdescr:"Data interval (daily, weekly, 1min, 5min, 15min, 30min)" default:"daily" flaggroup:"data"`
	Points   int        `flag:"points"   flagdescr:"Number of output points (default 1; 0 = all)"            default:"1"     flaggroup:"data"`
}

// bbandsOpts holds CLI flags for the Bollinger Bands subcommand.
type bbandsOpts struct {
	Period   int        `flag:"period"   flagdescr:"BBands period"                                           default:"20"    flaggroup:"indicator"`
	StdDev   float64    `flag:"std-dev"  flagdescr:"Standard deviations"                                     default:"2.0"   flaggroup:"indicator"`
	Interval taInterval `flag:"interval" flagdescr:"Data interval (daily, weekly, 1min, 5min, 15min, 30min)" default:"daily" flaggroup:"data"`
	Points   int        `flag:"points"   flagdescr:"Number of output points (default 1; 0 = all)"            default:"1"     flaggroup:"data"`
}

// stochOpts holds CLI flags for the Stochastic Oscillator subcommand.
type stochOpts struct {
	KPeriod  int        `flag:"k-period" flagdescr:"Fast %K lookback period"                                 default:"14"    flaggroup:"indicator"`
	SmoothK  int        `flag:"smooth-k" flagdescr:"Slow %K smoothing period"                                default:"3"     flaggroup:"indicator"`
	DPeriod  int        `flag:"d-period" flagdescr:"Slow %D period"                                          default:"3"     flaggroup:"indicator"`
	Interval taInterval `flag:"interval" flagdescr:"Data interval (daily, weekly, 1min, 5min, 15min, 30min)" default:"daily" flaggroup:"data"`
	Points   int        `flag:"points"   flagdescr:"Number of output points (default 1; 0 = all)"            default:"1"     flaggroup:"data"`
}

// adxOpts holds CLI flags for the ADX subcommand.
type adxOpts struct {
	Period   int        `flag:"period"   flagdescr:"Indicator period"                                        default:"14"    flaggroup:"indicator"`
	Interval taInterval `flag:"interval" flagdescr:"Data interval (daily, weekly, 1min, 5min, 15min, 30min)" default:"daily" flaggroup:"data"`
	Points   int        `flag:"points"   flagdescr:"Number of output points (default 1; 0 = all)"            default:"1"     flaggroup:"data"`
}

// vwapOpts holds CLI flags for the VWAP subcommand.
type vwapOpts struct {
	Interval taInterval `flag:"interval" flagdescr:"Data interval (daily, weekly, 1min, 5min, 15min, 30min)" default:"daily" flaggroup:"data"`
	Points   int        `flag:"points"   flagdescr:"Number of output points (default 1; 0 = all)"            default:"1"     flaggroup:"data"`
}

// hvOpts holds CLI flags for the Historical Volatility subcommand.
type hvOpts struct {
	Period   int        `flag:"period"   flagdescr:"Indicator period"                                        default:"20"    flaggroup:"indicator"`
	Interval taInterval `flag:"interval" flagdescr:"Data interval (daily, weekly, 1min, 5min, 15min, 30min)" default:"daily" flaggroup:"data"`
}

// expectedMoveOpts holds CLI flags for the Expected Move subcommand.
type expectedMoveOpts struct {
	DTE int `flag:"dte" flagdescr:"Target days to expiration" default:"30" flaggroup:"indicator"`
}

// dashboardOpts holds CLI flags for the opinionated TA dashboard command.
type dashboardOpts struct {
	Interval taInterval `flag:"interval" flagdescr:"Data interval (daily, weekly, 1min, 5min, 15min, 30min)" default:"daily" flaggroup:"data"`
	Points   int        `flag:"points"   flagdescr:"Number of output points (default 1; 0 = all)"            default:"1"     flaggroup:"data"`
}

// NewTACmd returns the Cobra command for technical analysis indicators.
func NewTACmd(c *client.Ref, w io.Writer) *cobra.Command {
	dashboardCmd := cobraTADashboardCommand(c, w)
	cmd := &cobra.Command{
		Use:   "ta",
		Short: "Technical analysis indicators",
		Long: `Compute technical analysis indicators from Schwab price history. All indicators
are calculated locally after fetching candles. Results are always keyed by
symbol under data.<SYMBOL>, even when one symbol is requested, so agents can use
the same parser for single-symbol and batch analysis. Use --interval to select
data frequency (daily, weekly, 1min, 5min, 15min, 30min). Time-series commands
default --points to 1 for token-efficient latest-value output; use --points 0
when you need the full computed series.`,
		GroupID: groupIDMarketData,
		Args:    cobra.ArbitraryArgs,
		RunE:    defaultSubcommand(dashboardCmd),
	}
	cmd.SetFlagErrorFunc(suggestSubcommands)

	cmd.AddCommand(
		dashboardCmd,
		makeCobraSimpleTACommand(&simpleTAConfig{
			name:  indicatorNameSMA,
			usage: "Simple Moving Average",
			long: `Compute Simple Moving Average for a symbol. The --period flag sets the lookback
window (default 20). Multiple periods can be requested in one command using
comma-separated values or repeated flags, producing crossover-ready output
with keys like sma_21, sma_50, sma_200.`,
			example: `  schwab-agent ta sma AAPL
  schwab-agent ta sma AAPL --period 50 --interval daily --points 10
  schwab-agent ta sma AAPL --period 21,50,200 --points 1`,
			defaultPeriod: simpleTADefaultPeriod,
			multiplier:    smaConvergenceMultiplier,
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
			defaultPeriod: simpleTADefaultPeriod,
			multiplier:    typicalConvergenceMultiplier,
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
			defaultPeriod: rsiDefaultPeriod,
			multiplier:    typicalConvergenceMultiplier,
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

// makeCobraSimpleTACommand builds a Cobra command for a closes-only indicator.
// The returned command fetches candles, extracts close prices, runs the
// indicator's compute function, and writes the result envelope.
func makeCobraSimpleTACommand(cfg *simpleTAConfig, c *client.Ref, w io.Writer) *cobra.Command {
	opts := &simpleTAOpts{}
	cmd := &cobra.Command{
		Use:     cfg.name + " SYMBOL [SYMBOL...]",
		Short:   cfg.usage,
		Long:    cfg.long,
		Example: cfg.example,
		Args:    cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Unmarshal decodes flags and runs simpleTAOpts.Validate(),
			// which rejects zero/negative and duplicate periods.
			if err := validateCobraOptions(cmd.Context(), opts); err != nil {
				return err
			}

			return writeTASymbolResults(w, args, func(symbol string) (any, error) {
				return computeSimpleTAOutput(cmd.Context(), c, cfg, opts, symbol)
			})
		},
	}

	// Period uses IntSliceVar because its default varies per indicator (cfg.defaultPeriod),
	// while the tag-driven helper can only express static defaults.
	cmd.Flags().
		IntSliceVar(&opts.Period, "period", []int{cfg.defaultPeriod}, "Indicator period (repeatable or comma-separated)")

	// Keep DefValue in the same comma-separated format users pass on the CLI.
	// pflag's default "[20]" rendering is harder to read in help and generated docs.
	if f := cmd.Flags().Lookup("period"); f != nil {
		f.DefValue = strconv.Itoa(cfg.defaultPeriod)
	}

	defineCobraFlags(cmd, opts)
	cmd.SetFlagErrorFunc(normalizeFlagValidationErrorFunc)

	return cmd
}

// makeCobraTACommand builds the repeated Cobra plumbing for TA
// commands whose real work lives in a typed per-symbol builder. Keeping the
// builders typed makes indicator-specific code easy to read while this helper
// enforces the common normalized symbol-keyed output shape.
func makeCobraTACommand[O any, R any](cfg taCommandConfig[O, R], c *client.Ref, w io.Writer) *cobra.Command {
	opts := cfg.newOpts()
	cmd := &cobra.Command{
		Use:     cfg.name + " SYMBOL [SYMBOL...]",
		Short:   cfg.short,
		Long:    cfg.long,
		Example: cfg.example,
		Args:    cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateCobraOptions(cmd.Context(), opts); err != nil {
				return err
			}

			return writeTASymbolResults(w, args, func(symbol string) (any, error) {
				return cfg.build(cmd.Context(), c, opts, symbol)
			})
		},
	}

	defineCobraFlags(cmd, opts)
	cmd.SetFlagErrorFunc(normalizeFlagValidationErrorFunc)

	return cmd
}

func computeSimpleTAOutput(
	ctx context.Context,
	c *client.Ref,
	cfg *simpleTAConfig,
	opts *simpleTAOpts,
	symbol string,
) (any, error) {
	periods := opts.Period
	period := maxPeriod(periods)
	interval := string(opts.Interval)
	candles, timestamps, err := fetchAndValidateCandles(ctx, c, symbol, interval, period*cfg.multiplier, cfg.name)
	if err != nil {
		return nil, err
	}

	closes, err := ta.ExtractClose(candles)
	if err != nil {
		return nil, err
	}

	valuesByPeriod := make(map[int][]float64, len(periods))
	for _, p := range periods {
		values, computeErr := cfg.compute(closes, p)
		if computeErr != nil {
			return nil, computeErr
		}
		valuesByPeriod[p] = values
	}

	if len(periods) == 1 {
		return buildTAOutput(
			cfg.name,
			symbol,
			interval,
			periods[0],
			opts.Points,
			timestamps,
			valuesByPeriod[periods[0]],
		)
	}

	return buildMultiTAOutput(cfg.name, symbol, interval, periods, opts.Points, timestamps, valuesByPeriod)
}

func cobraTADashboardCommand(c *client.Ref, w io.Writer) *cobra.Command {
	opts := &dashboardOpts{}
	cmd := &cobra.Command{
		Use:   "dashboard SYMBOL [SYMBOL...]",
		Short: "Opinionated TA dashboard for agent trade analysis",
		Long: `Compute an opinionated technical-analysis dashboard for a symbol with one
price-history fetch. The dashboard includes SMA 21/50/200 trend context, RSI 14,
MACD 12/26/9, ATR 14, Bollinger Bands 20/2, volume context, and 20/252-candle
high-low ranges. Signals are neutral factual labels for agents, not trading advice.`,
		Example: `  schwab-agent ta dashboard AAPL
  schwab-agent ta dashboard AAPL --points 5
  schwab-agent ta dashboard NVDA --interval weekly --points 10`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateCobraOptions(cmd.Context(), opts); err != nil {
				return err
			}

			return writeTASymbolResults(w, args, func(symbol string) (any, error) {
				return buildTADashboard(cmd.Context(), c, symbol, string(opts.Interval), opts.Points)
			})
		},
	}

	defineCobraFlags(cmd, opts)
	cmd.SetFlagErrorFunc(normalizeFlagValidationErrorFunc)

	return cmd
}

func buildTADashboard(
	ctx context.Context,
	c *client.Ref,
	symbol, interval string,
	points int,
) (dashboardOutput, error) {
	inputs, err := fetchDashboardInputs(ctx, c, symbol, interval, points)
	if err != nil {
		return dashboardOutput{}, err
	}
	series, err := buildDashboardSeries(inputs)
	if err != nil {
		return dashboardOutput{}, err
	}
	rows, err := buildDashboardRows(inputs, series)
	if err != nil {
		return dashboardOutput{}, err
	}

	latest := cloneDashboardRow(rows[len(rows)-1])
	signals, err := dashboardSignals(latest)
	if err != nil {
		return dashboardOutput{}, err
	}
	return newDashboardOutput(symbol, interval, points, latest, signals, rows), nil
}

func fetchDashboardInputs(
	ctx context.Context,
	c *client.Ref,
	symbol, interval string,
	points int,
) (dashboardInputs, error) {
	requestedRows := 1
	if points > 0 {
		requestedRows = points
	}
	minCandles := dashboardLongRangePeriod + requestedRows - 1

	candles, timestamps, err := fetchAndValidateCandles(ctx, c, symbol, interval, minCandles, "dashboard")
	if err != nil {
		return dashboardInputs{}, err
	}

	inputs := dashboardInputs{Candles: candles, Timestamps: timestamps}
	if inputs.Opens, err = ta.ExtractOpen(candles); err != nil {
		return dashboardInputs{}, err
	}
	if inputs.Highs, err = ta.ExtractHigh(candles); err != nil {
		return dashboardInputs{}, err
	}
	if inputs.Lows, err = ta.ExtractLow(candles); err != nil {
		return dashboardInputs{}, err
	}
	if inputs.Closes, err = ta.ExtractClose(candles); err != nil {
		return dashboardInputs{}, err
	}
	inputs.Volumes = ta.ExtractVolume(candles)

	return inputs, nil
}

func buildDashboardSeries(inputs dashboardInputs) (dashboardSeries, error) {
	series := dashboardSeries{}
	var err error
	if series.SMA21, err = ta.SMA(inputs.Closes, dashboardSMAFastPeriod); err != nil {
		return dashboardSeries{}, err
	}
	if series.SMA50, err = ta.SMA(inputs.Closes, dashboardSMAMediumPeriod); err != nil {
		return dashboardSeries{}, err
	}
	if series.SMA200, err = ta.SMA(inputs.Closes, dashboardSMALongPeriod); err != nil {
		return dashboardSeries{}, err
	}
	if series.RSI14, err = ta.RSI(inputs.Closes, dashboardRSIPeriod); err != nil {
		return dashboardSeries{}, err
	}
	series.MACD, series.MACDSignal, series.MACDHist, err = ta.MACD(
		inputs.Closes,
		dashboardMACDFastPeriod,
		dashboardMACDSlowPeriod,
		dashboardMACDSignalPeriod,
	)
	if err != nil {
		return dashboardSeries{}, err
	}
	if series.ATR14, err = ta.ATR(inputs.Highs, inputs.Lows, inputs.Closes, dashboardATRPeriod); err != nil {
		return dashboardSeries{}, err
	}
	series.BBandsUpper, series.BBandsMid, series.BBandsLower, err = ta.BBands(
		inputs.Closes,
		dashboardBBandsPeriod,
		dashboardBBandsStdDev,
	)
	if err != nil {
		return dashboardSeries{}, err
	}
	if series.AvgVolume20, err = ta.SMA(inputs.Volumes, dashboardVolumePeriod); err != nil {
		return dashboardSeries{}, err
	}

	return series, nil
}

func buildDashboardRows(inputs dashboardInputs, series dashboardSeries) ([]map[string]any, error) {
	baseStart := dashboardLongRangePeriod - 1
	rows := dashboardBaseRows(inputs, baseStart)
	if err := addDashboardSeries(rows, series); err != nil {
		return nil, err
	}
	if err := addDashboardDerivedFields(rows, inputs, baseStart); err != nil {
		return nil, err
	}

	return rows, nil
}

func newDashboardOutput(
	symbol, interval string,
	points int,
	latest, signals map[string]any,
	rows []map[string]any,
) dashboardOutput {
	return dashboardOutput{
		Indicator: "dashboard",
		Symbol:    symbol,
		Interval:  interval,
		Parameters: dashboardParameters{
			SMAPeriods:   []int{dashboardSMAFastPeriod, dashboardSMAMediumPeriod, dashboardSMALongPeriod},
			RSIPeriod:    dashboardRSIPeriod,
			MACDFast:     dashboardMACDFastPeriod,
			MACDSlow:     dashboardMACDSlowPeriod,
			MACDSignal:   dashboardMACDSignalPeriod,
			ATRPeriod:    dashboardATRPeriod,
			BBandsPeriod: dashboardBBandsPeriod,
			BBandsStdDev: dashboardBBandsStdDev,
			VolumePeriod: dashboardVolumePeriod,
			RangePeriod:  dashboardRangePeriod,
			LongRange:    dashboardLongRangePeriod,
		},
		Latest:  latest,
		Signals: signals,
		Values:  limitDashboardRows(rows, points),
	}
}

func dashboardBaseRows(inputs dashboardInputs, baseStart int) []map[string]any {
	rowCount := len(inputs.Candles) - baseStart
	rows := make([]map[string]any, rowCount)
	for i := range rowCount {
		candleIndex := baseStart + i
		rows[i] = map[string]any{
			dashboardKeyDatetime: inputs.Timestamps[candleIndex],
			"open":               inputs.Opens[candleIndex],
			"high":               inputs.Highs[candleIndex],
			"low":                inputs.Lows[candleIndex],
			dashboardKeyClose:    inputs.Closes[candleIndex],
			dashboardKeyVolume:   inputs.Volumes[candleIndex],
		}
	}

	return rows
}

func addDashboardSeries(rows []map[string]any, series dashboardSeries) error {
	for key, values := range map[string][]float64{
		dashboardKeySMA21:         series.SMA21,
		dashboardKeySMA50:         series.SMA50,
		dashboardKeySMA200:        series.SMA200,
		dashboardKeyRSI14:         series.RSI14,
		dashboardKeyMACD:          series.MACD,
		dashboardKeyMACDSignal:    series.MACDSignal,
		dashboardKeyMACDHistogram: series.MACDHist,
		dashboardKeyATR14:         series.ATR14,
		dashboardKeyBBandsUpper:   series.BBandsUpper,
		dashboardKeyBBandsMiddle:  series.BBandsMid,
		dashboardKeyBBandsLower:   series.BBandsLower,
		dashboardKeyAvgVolume20:   series.AvgVolume20,
	} {
		if err := addTailAlignedFloat(rows, key, values); err != nil {
			return err
		}
	}

	return nil
}

func addDashboardDerivedFields(rows []map[string]any, inputs dashboardInputs, baseStart int) error {
	for i := range rows {
		if err := addDashboardDerivedRow(rows[i], inputs, baseStart+i); err != nil {
			return err
		}
	}

	return nil
}

func addDashboardDerivedRow(row map[string]any, inputs dashboardInputs, candleIndex int) error {
	closePrice, err := dashboardRowFloat(row, dashboardKeyClose)
	if err != nil {
		return err
	}
	atrValue, err := dashboardRowFloat(row, dashboardKeyATR14)
	if err != nil {
		return err
	}
	avgVolume, err := dashboardRowFloat(row, dashboardKeyAvgVolume20)
	if err != nil {
		return err
	}
	volume, err := dashboardRowFloat(row, dashboardKeyVolume)
	if err != nil {
		return err
	}
	sma21Value, err := dashboardRowFloat(row, dashboardKeySMA21)
	if err != nil {
		return err
	}
	sma50Value, err := dashboardRowFloat(row, dashboardKeySMA50)
	if err != nil {
		return err
	}
	sma200Value, err := dashboardRowFloat(row, dashboardKeySMA200)
	if err != nil {
		return err
	}

	rangeHigh20, rangeLow20 := highLowWindow(inputs.Highs, inputs.Lows, candleIndex, dashboardRangePeriod)
	rangeHigh252, rangeLow252 := highLowWindow(inputs.Highs, inputs.Lows, candleIndex, dashboardLongRangePeriod)
	row[dashboardKeyATRPercent] = percentOf(atrValue, closePrice)
	row[dashboardKeyRelativeVolume] = ratio(volume, avgVolume)
	row[dashboardKeyRange20High] = rangeHigh20
	row[dashboardKeyRange20Low] = rangeLow20
	row[dashboardKeyRange252High] = rangeHigh252
	row[dashboardKeyRange252Low] = rangeLow252
	row[dashboardKeyDistanceFromSMA21Pct] = percentChange(closePrice, sma21Value)
	row[dashboardKeyDistanceFromSMA50Pct] = percentChange(closePrice, sma50Value)
	row[dashboardKeyDistanceFromSMA200Pct] = percentChange(closePrice, sma200Value)
	row[dashboardKeyDistanceFromRange20Pct] = percentChange(closePrice, rangeHigh20)
	row[dashboardKeyDistanceFromRange252Pct] = percentChange(closePrice, rangeHigh252)
	return nil
}

func addTailAlignedFloat(rows []map[string]any, key string, values []float64) error {
	// TA-Lib returns leading zero placeholders before an indicator warms up. The
	// ta package strips those placeholders, but an all-zero valid series, such as
	// flat-price MACD or ATR, becomes empty. Treat an empty indicator result as a
	// valid zero-valued tail so a quiet market produces JSON instead of a panic.
	if len(values) == 0 {
		for i := range rows {
			rows[i][key] = 0.0
		}
		return nil
	}

	if len(values) < len(rows) {
		return apperr.NewValidationError(
			fmt.Sprintf("dashboard %s series has %d values for %d output rows", key, len(values), len(rows)),
			nil,
		)
	}

	start := len(values) - len(rows)
	for i := range rows {
		rows[i][key] = values[start+i]
	}
	return nil
}

func tailAlignedTimestamps(timestamps []string, valueCount int, label string) ([]string, error) {
	if valueCount > len(timestamps) {
		return nil, apperr.NewValidationError(
			fmt.Sprintf("%s has %d values for %d timestamps", label, valueCount, len(timestamps)),
			nil,
		)
	}

	return timestamps[len(timestamps)-valueCount:], nil
}

func tailLimit[T any](items []T, points int) []T {
	if points > 0 && points < len(items) {
		return items[len(items)-points:]
	}

	return items
}

func limitDashboardRows(rows []map[string]any, points int) []map[string]any {
	rows = tailLimit(rows, points)

	out := make([]map[string]any, len(rows))
	for i, row := range rows {
		out[i] = cloneDashboardRow(row)
	}
	return out
}

func cloneDashboardRow(row map[string]any) map[string]any {
	clone := make(map[string]any, len(row))
	maps.Copy(clone, row)
	return clone
}

func highLowWindow(highs, lows []float64, endIndex, window int) (float64, float64) {
	start := max(0, endIndex-window+1)
	highest := highs[start]
	lowest := lows[start]
	for i := start + 1; i <= endIndex; i++ {
		if highs[i] > highest {
			highest = highs[i]
		}
		if lows[i] < lowest {
			lowest = lows[i]
		}
	}
	return highest, lowest
}

func percentOf(value, base float64) float64 {
	if base == 0 {
		return 0
	}
	return value / base * percentMultiplier
}

func percentChange(value, base float64) float64 {
	if base == 0 {
		return 0
	}
	return (value - base) / base * percentMultiplier
}

func ratio(value, base float64) float64 {
	if base == 0 {
		return 0
	}
	return value / base
}

func dashboardRowFloat(row map[string]any, key string) (float64, error) {
	value, ok := row[key]
	if !ok {
		return 0, apperr.NewValidationError(fmt.Sprintf("dashboard row missing %s", key), nil)
	}

	floatValue, ok := value.(float64)
	if !ok {
		return 0, apperr.NewValidationError(
			fmt.Sprintf("dashboard %s value has type %T, want float64", key, value),
			nil,
		)
	}

	return floatValue, nil
}

func dashboardSignals(latest map[string]any) (map[string]any, error) {
	closePrice, err := dashboardRowFloat(latest, dashboardKeyClose)
	if err != nil {
		return nil, err
	}
	sma21, err := dashboardRowFloat(latest, dashboardKeySMA21)
	if err != nil {
		return nil, err
	}
	sma50, err := dashboardRowFloat(latest, dashboardKeySMA50)
	if err != nil {
		return nil, err
	}
	sma200, err := dashboardRowFloat(latest, dashboardKeySMA200)
	if err != nil {
		return nil, err
	}
	rsi14, err := dashboardRowFloat(latest, dashboardKeyRSI14)
	if err != nil {
		return nil, err
	}
	macdHist, err := dashboardRowFloat(latest, dashboardKeyMACDHistogram)
	if err != nil {
		return nil, err
	}
	atrPercent, err := dashboardRowFloat(latest, dashboardKeyATRPercent)
	if err != nil {
		return nil, err
	}
	relativeVolume, err := dashboardRowFloat(latest, dashboardKeyRelativeVolume)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"trend":                   trendSignal(closePrice, sma21, sma50, sma200),
		"momentum":                momentumSignal(rsi14, macdHist),
		fieldVolatility:           volatilitySignal(atrPercent),
		dashboardKeyVolume:        volumeSignal(relativeVolume),
		"close_above_sma_21":      closePrice > sma21,
		"close_above_sma_50":      closePrice > sma50,
		"close_above_sma_200":     closePrice > sma200,
		"sma_21_above_sma_50":     sma21 > sma50,
		"sma_50_above_sma_200":    sma50 > sma200,
		"rsi_overbought":          rsi14 >= rsiOverboughtThreshold,
		"rsi_oversold":            rsi14 <= rsiOversoldThreshold,
		"macd_histogram_positive": macdHist > 0,
	}, nil
}

func trendSignal(closePrice, sma21, sma50, sma200 float64) string {
	switch {
	case closePrice > sma21 && sma21 > sma50 && sma50 > sma200:
		return "bullish"
	case closePrice < sma21 && sma21 < sma50 && sma50 < sma200:
		return "bearish"
	default:
		return "mixed"
	}
}

func momentumSignal(rsi14, macdHistogram float64) string {
	switch {
	case rsi14 >= rsiOverboughtThreshold:
		return "overbought"
	case rsi14 <= rsiOversoldThreshold:
		return "oversold"
	case macdHistogram > 0:
		return "bullish"
	case macdHistogram < 0:
		return "bearish"
	default:
		return "neutral"
	}
}

func volatilitySignal(atrPercent float64) string {
	switch {
	case atrPercent >= atrElevatedPercentThreshold:
		return "elevated"
	case atrPercent <= atrCompressedPercentThreshold:
		return "compressed"
	default:
		return "normal"
	}
}

func volumeSignal(relativeVolume float64) string {
	switch {
	case relativeVolume >= relativeVolumeElevatedRatio:
		return "elevated"
	case relativeVolume <= relativeVolumeLightRatio:
		return "light"
	default:
		return "normal"
	}
}

func cobraTAMACDCommand(c *client.Ref, w io.Writer) *cobra.Command {
	return makeCobraTACommand(taCommandConfig[*macdOpts, macdOutput]{
		name:  indicatorNameMACD,
		short: "Moving Average Convergence/Divergence",
		long: `Compute MACD (Moving Average Convergence/Divergence) for a symbol. Uses
--fast, --slow, and --signal periods instead of --period. Output keys include
macd, signal, and histogram (histogram = macd - signal). Defaults: fast=12,
slow=26, signal=9.`,
		example: `  schwab-agent ta macd AAPL
  schwab-agent ta macd AAPL --fast 12 --slow 26 --signal 9 --points 10
  schwab-agent ta macd NVDA --interval weekly --points 10`,
		newOpts: func() *macdOpts { return &macdOpts{} },
		build:   buildMACDOutput,
	}, c, w)
}

func buildMACDOutput(ctx context.Context, c *client.Ref, opts *macdOpts, symbol string) (macdOutput, error) {
	// MACD layers slow EMA + signal EMA; 2x the combined period provides convergence.
	interval := string(opts.Interval)
	candles, timestamps, err := fetchAndValidateCandles(
		ctx,
		c,
		symbol,
		interval,
		(opts.Slow+opts.Signal)*macdConvergenceMultiplier,
		indicatorNameMACD,
	)
	if err != nil {
		return macdOutput{}, err
	}

	closes, err := ta.ExtractClose(candles)
	if err != nil {
		return macdOutput{}, err
	}

	macdVals, signalVals, histVals, err := ta.MACD(closes, opts.Fast, opts.Slow, opts.Signal)
	if err != nil {
		return macdOutput{}, err
	}

	timestamps, err = tailAlignedTimestamps(timestamps, len(macdVals), "macd timestamp alignment")
	if err != nil {
		return macdOutput{}, err
	}
	out := make([]macdPoint, len(macdVals))
	for i := range macdVals {
		out[i] = macdPoint{
			Datetime:  timestamps[i],
			MACD:      macdVals[i],
			Signal:    signalVals[i],
			Histogram: histVals[i],
		}
	}

	out = tailLimit(out, opts.Points)

	return macdOutput{
		Indicator: indicatorNameMACD,
		Symbol:    symbol,
		Interval:  interval,
		Fast:      opts.Fast,
		Slow:      opts.Slow,
		Signal:    opts.Signal,
		Values:    out,
	}, nil
}

func cobraTAATRCommand(c *client.Ref, w io.Writer) *cobra.Command {
	return makeCobraTACommand(taCommandConfig[*atrOpts, taOutput]{
		name:  "atr",
		short: "Average True Range",
		long: `Compute Average True Range for a symbol. ATR measures volatility in price
units: higher values indicate wider price swings. Uses high, low, and close
data with Wilder smoothing. Default period is 14.`,
		example: `  schwab-agent ta atr AAPL
  schwab-agent ta atr AAPL --period 14 --interval daily --points 10`,
		newOpts: func() *atrOpts { return &atrOpts{} },
		build:   buildATROutput,
	}, c, w)
}

func buildATROutput(ctx context.Context, c *client.Ref, opts *atrOpts, symbol string) (taOutput, error) {
	// ATR uses Wilder smoothing; 3x period provides convergence stability.
	interval := string(opts.Interval)
	candles, timestamps, err := fetchAndValidateCandles(
		ctx,
		c,
		symbol,
		interval,
		opts.Period*atrConvergenceMultiplier,
		"atr",
	)
	if err != nil {
		return taOutput{}, err
	}

	highs, err := ta.ExtractHigh(candles)
	if err != nil {
		return taOutput{}, err
	}
	lows, err := ta.ExtractLow(candles)
	if err != nil {
		return taOutput{}, err
	}
	closes, err := ta.ExtractClose(candles)
	if err != nil {
		return taOutput{}, err
	}

	values, err := ta.ATR(highs, lows, closes, opts.Period)
	if err != nil {
		return taOutput{}, err
	}

	return buildTAOutput("atr", symbol, interval, opts.Period, opts.Points, timestamps, values)
}

func cobraTABBandsCommand(c *client.Ref, w io.Writer) *cobra.Command {
	return makeCobraTACommand(taCommandConfig[*bbandsOpts, bbandsOutput]{
		name:  "bbands",
		short: "Bollinger Bands",
		long: `Compute Bollinger Bands for a symbol. Output keys are upper, middle, and lower
bands. Price near the upper band is relatively high, near the lower band is
relatively low. Use --std-dev to widen or narrow the bands (default 2.0).`,
		example: `  schwab-agent ta bbands AAPL
  schwab-agent ta bbands AAPL --period 20 --std-dev 2.0 --points 10
  schwab-agent ta bbands AAPL --std-dev 1.5 --points 10`,
		newOpts: func() *bbandsOpts { return &bbandsOpts{} },
		build:   buildBBandsOutput,
	}, c, w)
}

func buildBBandsOutput(ctx context.Context, c *client.Ref, opts *bbandsOpts, symbol string) (bbandsOutput, error) {
	interval := string(opts.Interval)
	candles, timestamps, err := fetchAndValidateCandles(ctx, c, symbol, interval, opts.Period, "bbands")
	if err != nil {
		return bbandsOutput{}, err
	}

	closes, err := ta.ExtractClose(candles)
	if err != nil {
		return bbandsOutput{}, err
	}

	upper, middle, lower, err := ta.BBands(closes, opts.Period, opts.StdDev)
	if err != nil {
		return bbandsOutput{}, err
	}

	timestamps, err = tailAlignedTimestamps(timestamps, len(upper), "bbands timestamp alignment")
	if err != nil {
		return bbandsOutput{}, err
	}
	out := make([]bbandsPoint, len(upper))
	for i := range upper {
		out[i] = bbandsPoint{
			Datetime: timestamps[i],
			Upper:    upper[i],
			Middle:   middle[i],
			Lower:    lower[i],
		}
	}

	out = tailLimit(out, opts.Points)

	return bbandsOutput{
		Indicator: "bbands",
		Symbol:    symbol,
		Interval:  interval,
		Period:    opts.Period,
		StdDev:    opts.StdDev,
		Values:    out,
	}, nil
}

func cobraTAStochCommand(c *client.Ref, w io.Writer) *cobra.Command {
	return makeCobraTACommand(taCommandConfig[*stochOpts, stochOutput]{
		name:  "stoch",
		short: "Stochastic Oscillator",
		long: `Compute Stochastic Oscillator for a symbol. Output keys are slowk and slowd,
both ranging 0-100. Above 80 and below 20 are common signal thresholds.
Configurable via --k-period, --smooth-k, and --d-period.`,
		example: `  schwab-agent ta stoch AAPL
  schwab-agent ta stoch AAPL --k-period 14 --smooth-k 3 --d-period 3 --points 10`,
		newOpts: func() *stochOpts { return &stochOpts{} },
		build:   buildStochOutput,
	}, c, w)
}

func buildStochOutput(ctx context.Context, c *client.Ref, opts *stochOpts, symbol string) (stochOutput, error) {
	// Stochastic chains three windowed ops (raw %K, smoothed %K, %D);
	// total depth is the sum of all window sizes.
	interval := string(opts.Interval)
	candles, timestamps, err := fetchAndValidateCandles(
		ctx,
		c,
		symbol,
		interval,
		opts.KPeriod+opts.SmoothK+opts.DPeriod,
		"stoch",
	)
	if err != nil {
		return stochOutput{}, err
	}

	highs, err := ta.ExtractHigh(candles)
	if err != nil {
		return stochOutput{}, err
	}
	lows, err := ta.ExtractLow(candles)
	if err != nil {
		return stochOutput{}, err
	}
	closes, err := ta.ExtractClose(candles)
	if err != nil {
		return stochOutput{}, err
	}

	slowK, slowD, err := ta.Stochastic(highs, lows, closes, opts.KPeriod, opts.SmoothK, opts.DPeriod)
	if err != nil {
		return stochOutput{}, err
	}

	timestamps, err = tailAlignedTimestamps(timestamps, len(slowK), "stoch timestamp alignment")
	if err != nil {
		return stochOutput{}, err
	}
	out := make([]stochPoint, len(slowK))
	for i := range slowK {
		out[i] = stochPoint{
			Datetime: timestamps[i],
			SlowK:    slowK[i],
			SlowD:    slowD[i],
		}
	}

	out = tailLimit(out, opts.Points)

	return stochOutput{
		Indicator: "stoch",
		Symbol:    symbol,
		Interval:  interval,
		KPeriod:   opts.KPeriod,
		SmoothK:   opts.SmoothK,
		DPeriod:   opts.DPeriod,
		Values:    out,
	}, nil
}

func cobraTAADXCommand(c *client.Ref, w io.Writer) *cobra.Command {
	return makeCobraTACommand(taCommandConfig[*adxOpts, adxOutput]{
		name:  "adx",
		short: "Average Directional Index",
		long: `Compute Average Directional Index for a symbol. ADX above 25 indicates a
trending market, below 20 indicates a ranging market. Output includes adx,
plus_di, and minus_di for directional bias. Default period is 14.`,
		example: `  schwab-agent ta adx AAPL
  schwab-agent ta adx AAPL --period 14 --interval daily --points 10`,
		newOpts: func() *adxOpts { return &adxOpts{} },
		build:   buildADXOutput,
	}, c, w)
}

func buildADXOutput(ctx context.Context, c *client.Ref, opts *adxOpts, symbol string) (adxOutput, error) {
	// ADX double-smooths (DI pass + ADX pass); 4x period ensures both converge.
	interval := string(opts.Interval)
	candles, timestamps, err := fetchAndValidateCandles(
		ctx,
		c,
		symbol,
		interval,
		opts.Period*adxConvergenceMultiplier,
		"adx",
	)
	if err != nil {
		return adxOutput{}, err
	}

	highs, err := ta.ExtractHigh(candles)
	if err != nil {
		return adxOutput{}, err
	}
	lows, err := ta.ExtractLow(candles)
	if err != nil {
		return adxOutput{}, err
	}
	closes, err := ta.ExtractClose(candles)
	if err != nil {
		return adxOutput{}, err
	}

	adxVals, plusDI, minusDI, err := ta.ADX(highs, lows, closes, opts.Period)
	if err != nil {
		return adxOutput{}, err
	}

	timestamps, err = tailAlignedTimestamps(timestamps, len(adxVals), "adx timestamp alignment")
	if err != nil {
		return adxOutput{}, err
	}
	out := make([]adxPoint, len(adxVals))
	for i := range adxVals {
		out[i] = adxPoint{
			Datetime: timestamps[i],
			ADX:      adxVals[i],
			PlusDI:   plusDI[i],
			MinusDI:  minusDI[i],
		}
	}

	out = tailLimit(out, opts.Points)

	return adxOutput{
		Indicator: "adx",
		Symbol:    symbol,
		Interval:  interval,
		Period:    opts.Period,
		Values:    out,
	}, nil
}

func cobraTAVWAPCommand(c *client.Ref, w io.Writer) *cobra.Command {
	return makeCobraTACommand(taCommandConfig[*vwapOpts, taOutput]{
		name:  "vwap",
		short: "Volume Weighted Average Price",
		long: `Compute Volume Weighted Average Price for a symbol. VWAP is a cumulative
indicator with no --period flag. Primarily used for intraday analysis with
minute-level intervals. Price above VWAP suggests bullish bias, below
suggests bearish.`,
		example: `  schwab-agent ta vwap AAPL
  schwab-agent ta vwap AAPL --interval 5min --points 20`,
		newOpts: func() *vwapOpts { return &vwapOpts{} },
		build:   buildVWAPOutput,
	}, c, w)
}

func buildVWAPOutput(ctx context.Context, c *client.Ref, opts *vwapOpts, symbol string) (taOutput, error) {
	// Use a small minimum candle count. VWAP works with any count >= 1, but this
	// gives meaningful output for typical use cases.
	interval := string(opts.Interval)
	candles, timestamps, err := fetchAndValidateCandles(ctx, c, symbol, interval, vwapMinimumCandles, "vwap")
	if err != nil {
		return taOutput{}, err
	}

	highs, err := ta.ExtractHigh(candles)
	if err != nil {
		return taOutput{}, err
	}
	lows, err := ta.ExtractLow(candles)
	if err != nil {
		return taOutput{}, err
	}
	closes, err := ta.ExtractClose(candles)
	if err != nil {
		return taOutput{}, err
	}
	volumes := ta.ExtractVolume(candles)

	values, err := ta.VWAP(highs, lows, closes, volumes)
	if err != nil {
		return taOutput{}, err
	}

	// period=0 because VWAP is cumulative (no windowed period)
	return buildTAOutput("vwap", symbol, interval, 0, opts.Points, timestamps, values)
}

func cobraTAHVCommand(c *client.Ref, w io.Writer) *cobra.Command {
	return makeCobraTACommand(taCommandConfig[*hvOpts, hvOutput]{
		name:  "hv",
		short: "Historical Volatility with regime classification",
		long: `Compute Historical Volatility for a symbol. Returns a scalar summary (not
time series) with annualized volatility, percentile rank, and regime
classification (low, normal, high, extreme). Includes daily, weekly, and
monthly volatility breakdowns.`,
		example: `  schwab-agent ta hv AAPL
  schwab-agent ta hv AAPL --period 20 --interval daily`,
		newOpts: func() *hvOpts { return &hvOpts{} },
		build:   buildHVOutput,
	}, c, w)
}

func buildHVOutput(ctx context.Context, c *client.Ref, opts *hvOpts, symbol string) (hvOutput, error) {
	// Need period+1 closes for N returns, plus extra for rolling window warmup.
	// hvWarmupCandles provides a safety margin for the rolling window.
	interval := string(opts.Interval)
	candles, _, err := fetchAndValidateCandles(ctx, c, symbol, interval, opts.Period+hvWarmupCandles, "hv")
	if err != nil {
		return hvOutput{}, err
	}

	closes, err := ta.ExtractClose(candles)
	if err != nil {
		return hvOutput{}, err
	}

	result, err := ta.HistoricalVolatility(closes, opts.Period)
	if err != nil {
		return hvOutput{}, err
	}

	// HV returns a scalar summary, not a time series.
	// Do NOT use writeTAOutput (which is for time series).
	return hvOutput{
		Indicator:      "hv",
		Symbol:         symbol,
		Interval:       interval,
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
	}, nil
}

func cobraTAExpectedMoveCommand(c *client.Ref, w io.Writer) *cobra.Command {
	return makeCobraTACommand(taCommandConfig[*expectedMoveOpts, expectedMoveOutput]{
		name:  "expected-move",
		short: "Expected price move from ATM straddle pricing",
		long: `Compute expected price move from ATM straddle pricing. Fetches the option
chain, finds the nearest expiration to --dte (default 30), and prices the
ATM straddle. The raw expected move is the straddle price. The adjusted move
applies the standard 0.85x multiplier used by many options desks before
building the 1x and 2x range bounds.`,
		example: `  schwab-agent ta expected-move AAPL
  schwab-agent ta expected-move AAPL --dte 45`,
		newOpts: func() *expectedMoveOpts { return &expectedMoveOpts{} },
		build:   buildExpectedMoveOutput,
	}, c, w)
}

func buildExpectedMoveOutput(
	ctx context.Context,
	c *client.Ref,
	opts *expectedMoveOpts,
	symbol string,
) (expectedMoveOutput, error) {
	// Expected Move needs the underlying quote and near-the-money contracts in one response.
	// Keep this to a single chain call so the underlying snapshot and option prices stay aligned.
	chain, err := c.MarketData.GetOptionChain(ctx, &marketdata.OptionChainParams{
		Symbol:                 symbol,
		ContractType:           marketdata.OptionChainContractTypeAll,
		IncludeUnderlyingQuote: true,
		Range:                  marketdata.OptionChainRangeNearTheMoney,
	})
	if err != nil {
		return expectedMoveOutput{}, mapSchwabGoError(err)
	}

	if chain.Underlying == nil {
		return expectedMoveOutput{}, fmt.Errorf("no underlying data available for %s", symbol)
	}

	underlyingPrice, err := expectedMoveUnderlyingPrice(chain, symbol)
	if err != nil {
		return expectedMoveOutput{}, err
	}
	selectedExpKey, err := expectedMoveExpiration(chain, symbol, opts.DTE)
	if err != nil {
		return expectedMoveOutput{}, err
	}

	expParts := strings.Split(selectedExpKey, ":")
	expDate := expParts[0]

	callStrikes := chain.CallExpDateMap[selectedExpKey]
	putStrikes := chain.PutExpDateMap[selectedExpKey]
	if len(callStrikes) == 0 || len(putStrikes) == 0 {
		return expectedMoveOutput{}, fmt.Errorf("no options available for %s at expiration %s", symbol, expDate)
	}

	atmStrikeKey := expectedMoveATMStrike(callStrikes, underlyingPrice)
	if atmStrikeKey == "" {
		return expectedMoveOutput{}, fmt.Errorf("no strikes available for %s at expiration %s", symbol, expDate)
	}

	callContracts := callStrikes[atmStrikeKey]
	putContracts := putStrikes[atmStrikeKey]
	if len(callContracts) == 0 {
		return expectedMoveOutput{}, fmt.Errorf("no call contracts for %s at strike %s", symbol, atmStrikeKey)
	}
	if len(putContracts) == 0 {
		return expectedMoveOutput{}, fmt.Errorf("no put contracts for %s at strike %s", symbol, atmStrikeKey)
	}

	callPrice, err := contractPrice(callContracts[0], symbol, atmStrikeKey, flagCall)
	if err != nil {
		return expectedMoveOutput{}, err
	}
	putPrice, err := contractPrice(putContracts[0], symbol, atmStrikeKey, "put")
	if err != nil {
		return expectedMoveOutput{}, err
	}

	result, err := ta.ExpectedMove(underlyingPrice, callPrice, putPrice, ta.DefaultMultiplier)
	if err != nil {
		return expectedMoveOutput{}, err
	}

	actualDTE, _ := strconv.Atoi(expParts[1])
	ranges := map[string]expectedMoveRange{
		"1x": {Lower: result.Lower1x, Upper: result.Upper1x},
		"2x": {Lower: result.Lower2x, Upper: result.Upper2x},
	}
	currentIV := expectedMoveCurrentIV(callContracts[0], putContracts[0])
	adjustmentMethod := fmt.Sprintf("raw ATM straddle price multiplied by %.2fx", ta.DefaultMultiplier)

	return expectedMoveOutput{
		Indicator:        "expected-move",
		Symbol:           symbol,
		UnderlyingPrice:  round2(underlyingPrice),
		Expiration:       expDate,
		DTE:              actualDTE,
		StraddlePrice:    result.StraddlePrice,
		ExpectedMove:     result.ExpectedMove,
		ExpectedMovePct:  result.ExpectedMovePct,
		AdjustedMove:     result.AdjustedMove,
		AdjustmentMethod: adjustmentMethod,
		CurrentIV:        currentIV,
		Ranges:           ranges,
		Upper1x:          result.Upper1x,
		Lower1x:          result.Lower1x,
		Upper2x:          result.Upper2x,
		Lower2x:          result.Lower2x,
	}, nil
}

func expectedMoveCurrentIV(callContract, putContract marketdata.OptionContract) float64 {
	var vols []float64
	if callContract.Volatility > 0 {
		vols = append(vols, callContract.Volatility)
	}
	if putContract.Volatility > 0 {
		vols = append(vols, putContract.Volatility)
	}
	if len(vols) == 0 {
		return 0
	}

	total := 0.0
	for _, vol := range vols {
		total += vol
	}
	return round2(total / float64(len(vols)))
}

func round2(v float64) float64 {
	return math.Round((v+roundingEpsilon)*percentMultiplier) / percentMultiplier
}

func expectedMoveUnderlyingPrice(chain *marketdata.OptionChain, symbol string) (float64, error) {
	switch {
	case chain.Underlying.Mark > 0:
		return chain.Underlying.Mark, nil
	case chain.Underlying.Last > 0:
		return chain.Underlying.Last, nil
	case chain.UnderlyingPrice > 0:
		return chain.UnderlyingPrice, nil
	default:
		return 0, fmt.Errorf("unable to determine underlying price for %s", symbol)
	}
}

func expectedMoveExpiration(chain *marketdata.OptionChain, symbol string, targetDTE int) (string, error) {
	if len(chain.CallExpDateMap) == 0 {
		return "", fmt.Errorf("no options available for %s", symbol)
	}

	selectedExpKey := ""
	bestDiff := -1
	for expKey := range chain.CallExpDateMap {
		parts := strings.Split(expKey, ":")
		if len(parts) != expectedMoveExpirationParts {
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
		return "", fmt.Errorf("no options available for %s", symbol)
	}

	return selectedExpKey, nil
}

func expectedMoveATMStrike(callStrikes map[string][]marketdata.OptionContract, underlyingPrice float64) string {
	atmStrikeKey := ""
	bestStrikeDiff := -1.0
	for strikeKey := range callStrikes {
		strike, err := strconv.ParseFloat(strikeKey, 64)
		if err != nil {
			continue
		}

		diff := math.Abs(strike - underlyingPrice)
		if bestStrikeDiff < 0 || diff < bestStrikeDiff ||
			(diff == bestStrikeDiff && strike < mustParseFloat(atmStrikeKey)) {
			bestStrikeDiff = diff
			atmStrikeKey = strikeKey
		}
	}

	return atmStrikeKey
}

func contractPrice(contract marketdata.OptionContract, symbol, strike, putCall string) (float64, error) {
	// schwab-go models option prices as values, so an omitted mark decodes to 0.
	// Fall back to the midpoint when mark is absent/zero to preserve the old pointer-based behavior.
	if contract.MarkPrice > 0 {
		return contract.MarkPrice, nil
	}
	if contract.BidPrice > 0 && contract.AskPrice > 0 {
		return (contract.BidPrice + contract.AskPrice) / quoteMidpointDivisor, nil
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
) ([]marketdata.Candle, []string, error) {
	periodType, periodVal, freqType, freq, err := ta.IntervalToHistoryParams(interval, requiredCandles)
	if err != nil {
		return nil, nil, err
	}

	params, err := newPriceHistoryParams(priceHistoryParamsInput{
		PeriodType:    periodType,
		Period:        periodVal,
		FrequencyType: freqType,
		Frequency:     freq,
	})
	if err != nil {
		return nil, nil, err
	}

	result, err := c.MarketData.GetPriceHistory(ctx, symbol, params)
	if err != nil {
		return nil, nil, mapSchwabGoError(err)
	}

	if validateErr := ta.ValidateMinCandles(result.Candles, requiredCandles, indicator); validateErr != nil {
		return nil, nil, validateErr
	}

	timestamps, err := ta.ExtractTimestamps(result.Candles)
	if err != nil {
		return nil, nil, err
	}
	return result.Candles, timestamps, nil
}

// buildTAOutput builds the output object for a single-value time series.
// It aligns timestamps to indicator values (shorter due to lookback) and applies --points limit.
func buildTAOutput(
	indicator, symbol, interval string,
	period, points int,
	timestamps []string,
	values []float64,
) (taOutput, error) {
	// Align timestamps: indicator output is shorter than input due to lookback window.
	timestamps, err := tailAlignedTimestamps(timestamps, len(values), indicator+" timestamp alignment")
	if err != nil {
		return taOutput{}, err
	}

	out := make([]map[string]any, len(values))
	for i, v := range values {
		out[i] = map[string]any{
			dashboardKeyDatetime: timestamps[i],
			indicator:            v,
		}
	}

	// Apply --points limit (tail-slice to get most recent N entries).
	out = tailLimit(out, points)

	return taOutput{
		Indicator: indicator,
		Symbol:    symbol,
		Interval:  interval,
		Period:    period,
		Values:    out,
	}, nil
}

// buildMultiTAOutput builds a combined time series for multiple requested
// periods. Each row represents a candle timestamp and contains one value per
// period using stable keys such as sma_21, sma_50, and sma_200.
func buildMultiTAOutput(
	indicator, symbol, interval string,
	periods []int,
	points int,
	timestamps []string,
	valuesByPeriod map[int][]float64,
) (multiTAOutput, error) {
	maxValues := 0
	for _, period := range periods {
		if len(valuesByPeriod[period]) > maxValues {
			maxValues = len(valuesByPeriod[period])
		}
	}

	timestamps, err := tailAlignedTimestamps(timestamps, maxValues, indicator+" multi-period timestamp alignment")
	if err != nil {
		return multiTAOutput{}, err
	}

	out := make([]map[string]any, maxValues)
	for i := range maxValues {
		out[i] = map[string]any{dashboardKeyDatetime: timestamps[i]}
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
	out = tailLimit(out, points)

	return multiTAOutput{
		Indicator: indicator,
		Symbol:    symbol,
		Interval:  interval,
		Periods:   periods,
		Values:    out,
	}, nil
}
