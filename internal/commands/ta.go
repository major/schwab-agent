// Package commands provides CLI command builders for schwab-agent.
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

	"github.com/leodido/structcli"
	"github.com/spf13/cobra"

	"github.com/major/schwab-agent/internal/apperr"
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
	Values     []map[string]any    `json:"values"`
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

// writeTASymbolResults preserves the historical single-symbol TA response shape
// while letting batched commands return the same symbol-keyed map pattern used by
// quote get. Schwab's price-history API is single-symbol, so batching still makes
// one API call per symbol, but agents only need one CLI invocation and can receive
// partial data when a single ticker fails.
func writeTASymbolResults(w io.Writer, symbols []string, compute func(symbol string) (any, error)) error {
	if len(symbols) == 1 {
		data, err := compute(symbols[0])
		if err != nil {
			return err
		}
		return output.WriteSuccess(w, data, output.NewMetadata())
	}

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
// (cfg.defaultPeriod) and structcli tags require static defaults.
type simpleTAOpts struct {
	Period   []int
	Interval taInterval `flag:"interval" flagdescr:"Data interval (daily, weekly, 1min, 5min, 15min, 30min)" default:"daily" flaggroup:"data"`
	Points   int        `flag:"points" flagdescr:"Number of output points (default 1; 0 = all)" default:"1" flaggroup:"data"`
}

// Attach implements structcli.Options interface.
func (o *simpleTAOpts) Attach(_ *cobra.Command) error { return nil }

// Validate implements structcli.Validatable. Called automatically during
// Unmarshal() after flag decoding. Checks that all requested periods are
// positive and unique.
func (o *simpleTAOpts) Validate(_ context.Context) []error {
	if len(o.Period) == 0 {
		return []error{fmt.Errorf("at least one period is required")}
	}

	var errs []error
	seen := make(map[int]struct{}, len(o.Period))
	for _, period := range o.Period {
		if period <= 0 {
			errs = append(errs, fmt.Errorf("period must be greater than 0"))
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
	Fast     int        `flag:"fast" flagdescr:"Fast EMA period" default:"12" flaggroup:"indicator"`
	Slow     int        `flag:"slow" flagdescr:"Slow EMA period" default:"26" flaggroup:"indicator"`
	Signal   int        `flag:"signal" flagdescr:"Signal EMA period" default:"9" flaggroup:"indicator"`
	Interval taInterval `flag:"interval" flagdescr:"Data interval (daily, weekly, 1min, 5min, 15min, 30min)" default:"daily" flaggroup:"data"`
	Points   int        `flag:"points" flagdescr:"Number of output points (default 1; 0 = all)" default:"1" flaggroup:"data"`
}

// Attach implements structcli.Options interface.
func (o *macdOpts) Attach(_ *cobra.Command) error { return nil }

// atrOpts holds CLI flags for the ATR subcommand.
type atrOpts struct {
	Period   int        `flag:"period" flagdescr:"Indicator period" default:"14" flaggroup:"indicator"`
	Interval taInterval `flag:"interval" flagdescr:"Data interval (daily, weekly, 1min, 5min, 15min, 30min)" default:"daily" flaggroup:"data"`
	Points   int        `flag:"points" flagdescr:"Number of output points (default 1; 0 = all)" default:"1" flaggroup:"data"`
}

// Attach implements structcli.Options interface.
func (o *atrOpts) Attach(_ *cobra.Command) error { return nil }

// bbandsOpts holds CLI flags for the Bollinger Bands subcommand.
type bbandsOpts struct {
	Period   int        `flag:"period" flagdescr:"BBands period" default:"20" flaggroup:"indicator"`
	StdDev   float64    `flag:"std-dev" flagdescr:"Standard deviations" default:"2.0" flaggroup:"indicator"`
	Interval taInterval `flag:"interval" flagdescr:"Data interval (daily, weekly, 1min, 5min, 15min, 30min)" default:"daily" flaggroup:"data"`
	Points   int        `flag:"points" flagdescr:"Number of output points (default 1; 0 = all)" default:"1" flaggroup:"data"`
}

// Attach implements structcli.Options interface.
func (o *bbandsOpts) Attach(_ *cobra.Command) error { return nil }

// stochOpts holds CLI flags for the Stochastic Oscillator subcommand.
type stochOpts struct {
	KPeriod  int        `flag:"k-period" flagdescr:"Fast %K lookback period" default:"14" flaggroup:"indicator"`
	SmoothK  int        `flag:"smooth-k" flagdescr:"Slow %K smoothing period" default:"3" flaggroup:"indicator"`
	DPeriod  int        `flag:"d-period" flagdescr:"Slow %D period" default:"3" flaggroup:"indicator"`
	Interval taInterval `flag:"interval" flagdescr:"Data interval (daily, weekly, 1min, 5min, 15min, 30min)" default:"daily" flaggroup:"data"`
	Points   int        `flag:"points" flagdescr:"Number of output points (default 1; 0 = all)" default:"1" flaggroup:"data"`
}

// Attach implements structcli.Options interface.
func (o *stochOpts) Attach(_ *cobra.Command) error { return nil }

// adxOpts holds CLI flags for the ADX subcommand.
type adxOpts struct {
	Period   int        `flag:"period" flagdescr:"Indicator period" default:"14" flaggroup:"indicator"`
	Interval taInterval `flag:"interval" flagdescr:"Data interval (daily, weekly, 1min, 5min, 15min, 30min)" default:"daily" flaggroup:"data"`
	Points   int        `flag:"points" flagdescr:"Number of output points (default 1; 0 = all)" default:"1" flaggroup:"data"`
}

// Attach implements structcli.Options interface.
func (o *adxOpts) Attach(_ *cobra.Command) error { return nil }

// vwapOpts holds CLI flags for the VWAP subcommand.
type vwapOpts struct {
	Interval taInterval `flag:"interval" flagdescr:"Data interval (daily, weekly, 1min, 5min, 15min, 30min)" default:"daily" flaggroup:"data"`
	Points   int        `flag:"points" flagdescr:"Number of output points (default 1; 0 = all)" default:"1" flaggroup:"data"`
}

// Attach implements structcli.Options interface.
func (o *vwapOpts) Attach(_ *cobra.Command) error { return nil }

// hvOpts holds CLI flags for the Historical Volatility subcommand.
type hvOpts struct {
	Period   int        `flag:"period" flagdescr:"Indicator period" default:"20" flaggroup:"indicator"`
	Interval taInterval `flag:"interval" flagdescr:"Data interval (daily, weekly, 1min, 5min, 15min, 30min)" default:"daily" flaggroup:"data"`
}

// Attach implements structcli.Options interface.
func (o *hvOpts) Attach(_ *cobra.Command) error { return nil }

// expectedMoveOpts holds CLI flags for the Expected Move subcommand.
type expectedMoveOpts struct {
	DTE int `flag:"dte" flagdescr:"Target days to expiration" default:"30" flaggroup:"indicator"`
}

// Attach implements structcli.Options interface.
func (o *expectedMoveOpts) Attach(_ *cobra.Command) error { return nil }

// dashboardOpts holds CLI flags for the opinionated TA dashboard command.
type dashboardOpts struct {
	Interval taInterval `flag:"interval" flagdescr:"Data interval (daily, weekly, 1min, 5min, 15min, 30min)" default:"daily" flaggroup:"data"`
	Points   int        `flag:"points" flagdescr:"Number of output points (default 1; 0 = all)" default:"1" flaggroup:"data"`
}

// Attach implements structcli.Options interface.
func (o *dashboardOpts) Attach(_ *cobra.Command) error { return nil }

// NewTACmd returns the Cobra command for technical analysis indicators.
func NewTACmd(c *client.Ref, w io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ta",
		Short: "Technical analysis indicators",
		Long: `Compute technical analysis indicators from Schwab price history. All indicators
are calculated locally after fetching candles. Use --interval to select data
frequency (daily, weekly, 1min, 5min, 15min, 30min). Time-series commands
default --points to 1 for token-efficient latest-value output; use --points 0
when you need the full computed series.`,
		GroupID: "market-data",
		RunE:    requireSubcommand,
	}
	cmd.SetFlagErrorFunc(suggestSubcommands)

	cmd.AddCommand(
		cobraTADashboardCommand(c, w),
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
			if err := structcli.Unmarshal(cmd, opts); err != nil {
				return err
			}

			return writeTASymbolResults(w, args, func(symbol string) (any, error) {
				return computeSimpleTAOutput(cmd.Context(), c, cfg, opts, symbol)
			})
		},
	}

	// Period uses IntSliceVar because its default varies per indicator (cfg.defaultPeriod)
	// and structcli struct tags require static defaults.
	// Register BEFORE Define so structcli's schema generator handles it correctly.
	cmd.Flags().IntSliceVar(&opts.Period, "period", []int{cfg.defaultPeriod}, "Indicator period (repeatable or comma-separated)")

	// Fix DefValue format: pflag uses "[20]" but structcli JSON Schema expects "20" (comma-separated).
	// Without this, structcli's schema generator tries to parse "[20]" as a JSON number, which fails.
	if f := cmd.Flags().Lookup("period"); f != nil {
		f.DefValue = strconv.Itoa(cfg.defaultPeriod)
	}

	if err := structcli.Define(cmd, opts, structcli.WithExclusions("period")); err != nil {
		panic(err)
	}
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
		values, err := cfg.compute(closes, p)
		if err != nil {
			return nil, err
		}
		valuesByPeriod[p] = values
	}

	if len(periods) == 1 {
		return buildTAOutput(cfg.name, symbol, interval, periods[0], opts.Points, timestamps, valuesByPeriod[periods[0]])
	}

	return buildMultiTAOutput(cfg.name, symbol, interval, periods, opts.Points, timestamps, valuesByPeriod), nil
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
			if err := structcli.Unmarshal(cmd, opts); err != nil {
				return err
			}

			return writeTASymbolResults(w, args, func(symbol string) (any, error) {
				return buildTADashboard(cmd.Context(), c, symbol, string(opts.Interval), opts.Points)
			})
		},
	}

	if err := structcli.Define(cmd, opts); err != nil {
		panic(err)
	}
	cmd.SetFlagErrorFunc(normalizeFlagValidationErrorFunc)

	return cmd
}

func buildTADashboard(ctx context.Context, c *client.Ref, symbol, interval string, points int) (dashboardOutput, error) {
	const (
		smaFastPeriod    = 21
		smaMediumPeriod  = 50
		smaLongPeriod    = 200
		rsiPeriod        = 14
		macdFastPeriod   = 12
		macdSlowPeriod   = 26
		macdSignalPeriod = 9
		atrPeriod        = 14
		bbandsPeriod     = 20
		bbandsStdDev     = 2.0
		volumePeriod     = 20
		rangePeriod      = 20
		longRangePeriod  = 252
	)

	requestedRows := 1
	if points > 0 {
		requestedRows = points
	}
	dashboardMinCandles := longRangePeriod + requestedRows - 1

	candles, timestamps, err := fetchAndValidateCandles(ctx, c, symbol, interval, dashboardMinCandles, "dashboard")
	if err != nil {
		return dashboardOutput{}, err
	}

	opens, err := ta.ExtractOpen(candles)
	if err != nil {
		return dashboardOutput{}, err
	}
	highs, err := ta.ExtractHigh(candles)
	if err != nil {
		return dashboardOutput{}, err
	}
	lows, err := ta.ExtractLow(candles)
	if err != nil {
		return dashboardOutput{}, err
	}
	closes, err := ta.ExtractClose(candles)
	if err != nil {
		return dashboardOutput{}, err
	}
	volumes, err := ta.ExtractVolume(candles)
	if err != nil {
		return dashboardOutput{}, err
	}

	sma21, err := ta.SMA(closes, smaFastPeriod)
	if err != nil {
		return dashboardOutput{}, err
	}
	sma50, err := ta.SMA(closes, smaMediumPeriod)
	if err != nil {
		return dashboardOutput{}, err
	}
	sma200, err := ta.SMA(closes, smaLongPeriod)
	if err != nil {
		return dashboardOutput{}, err
	}
	rsi14, err := ta.RSI(closes, rsiPeriod)
	if err != nil {
		return dashboardOutput{}, err
	}
	macdVals, macdSignalVals, macdHistVals, err := ta.MACD(closes, macdFastPeriod, macdSlowPeriod, macdSignalPeriod)
	if err != nil {
		return dashboardOutput{}, err
	}
	atr14, err := ta.ATR(highs, lows, closes, atrPeriod)
	if err != nil {
		return dashboardOutput{}, err
	}
	bbandsUpper, bbandsMiddle, bbandsLower, err := ta.BBands(closes, bbandsPeriod, bbandsStdDev)
	if err != nil {
		return dashboardOutput{}, err
	}
	avgVolume20, err := ta.SMA(volumes, volumePeriod)
	if err != nil {
		return dashboardOutput{}, err
	}

	// The series begins where the 252-candle range is complete. SMA 200 becomes
	// available earlier, but labeling partial windows as range_252_* would mislead
	// agents about long-range highs and lows.
	baseStart := longRangePeriod - 1
	rowCount := len(candles) - baseStart
	rows := make([]map[string]any, rowCount)
	for i := range rowCount {
		candleIndex := baseStart + i
		rows[i] = map[string]any{
			"datetime": timestamps[candleIndex],
			"open":     opens[candleIndex],
			"high":     highs[candleIndex],
			"low":      lows[candleIndex],
			"close":    closes[candleIndex],
			"volume":   volumes[candleIndex],
		}
	}

	for key, values := range map[string][]float64{
		"sma_21":         sma21,
		"sma_50":         sma50,
		"sma_200":        sma200,
		"rsi_14":         rsi14,
		"macd":           macdVals,
		"macd_signal":    macdSignalVals,
		"macd_histogram": macdHistVals,
		"atr_14":         atr14,
		"bbands_upper":   bbandsUpper,
		"bbands_middle":  bbandsMiddle,
		"bbands_lower":   bbandsLower,
		"avg_volume_20":  avgVolume20,
	} {
		if err := addTailAlignedFloat(rows, key, values); err != nil {
			return dashboardOutput{}, err
		}
	}

	for i := range rows {
		closePrice := rows[i]["close"].(float64)
		atrValue := rows[i]["atr_14"].(float64)
		avgVolume := rows[i]["avg_volume_20"].(float64)
		volume := rows[i]["volume"].(float64)
		candleIndex := baseStart + i
		rangeHigh20, rangeLow20 := highLowWindow(highs, lows, candleIndex, rangePeriod)
		rangeHigh252, rangeLow252 := highLowWindow(highs, lows, candleIndex, longRangePeriod)

		rows[i]["atr_percent"] = percentOf(atrValue, closePrice)
		rows[i]["relative_volume"] = ratio(volume, avgVolume)
		rows[i]["range_20_high"] = rangeHigh20
		rows[i]["range_20_low"] = rangeLow20
		rows[i]["range_252_high"] = rangeHigh252
		rows[i]["range_252_low"] = rangeLow252
		rows[i]["distance_from_sma_21_percent"] = percentChange(closePrice, rows[i]["sma_21"].(float64))
		rows[i]["distance_from_sma_50_percent"] = percentChange(closePrice, rows[i]["sma_50"].(float64))
		rows[i]["distance_from_sma_200_percent"] = percentChange(closePrice, rows[i]["sma_200"].(float64))
		rows[i]["distance_from_range_20_high_percent"] = percentChange(closePrice, rangeHigh20)
		rows[i]["distance_from_range_252_high_percent"] = percentChange(closePrice, rangeHigh252)
	}

	latest := cloneDashboardRow(rows[len(rows)-1])
	data := dashboardOutput{
		Indicator: "dashboard",
		Symbol:    symbol,
		Interval:  interval,
		Parameters: dashboardParameters{
			SMAPeriods:   []int{smaFastPeriod, smaMediumPeriod, smaLongPeriod},
			RSIPeriod:    rsiPeriod,
			MACDFast:     macdFastPeriod,
			MACDSlow:     macdSlowPeriod,
			MACDSignal:   macdSignalPeriod,
			ATRPeriod:    atrPeriod,
			BBandsPeriod: bbandsPeriod,
			BBandsStdDev: bbandsStdDev,
			VolumePeriod: volumePeriod,
			RangePeriod:  rangePeriod,
			LongRange:    longRangePeriod,
		},
		Latest:  latest,
		Signals: dashboardSignals(latest),
		Values:  limitDashboardRows(rows, points),
	}

	return data, nil
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

func limitDashboardRows(rows []map[string]any, points int) []map[string]any {
	if points > 0 && points < len(rows) {
		rows = rows[len(rows)-points:]
	}

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

func highLowWindow(highs, lows []float64, endIndex, window int) (highest, lowest float64) {
	start := max(0, endIndex-window+1)
	highest = highs[start]
	lowest = lows[start]
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
	return value / base * 100
}

func percentChange(value, base float64) float64 {
	if base == 0 {
		return 0
	}
	return (value - base) / base * 100
}

func ratio(value, base float64) float64 {
	if base == 0 {
		return 0
	}
	return value / base
}

func dashboardSignals(latest map[string]any) map[string]any {
	closePrice := latest["close"].(float64)
	sma21 := latest["sma_21"].(float64)
	sma50 := latest["sma_50"].(float64)
	sma200 := latest["sma_200"].(float64)
	rsi14 := latest["rsi_14"].(float64)
	macdHist := latest["macd_histogram"].(float64)
	atrPercent := latest["atr_percent"].(float64)
	relativeVolume := latest["relative_volume"].(float64)

	return map[string]any{
		"trend":                   trendSignal(closePrice, sma21, sma50, sma200),
		"momentum":                momentumSignal(rsi14, macdHist),
		"volatility":              volatilitySignal(atrPercent),
		"volume":                  volumeSignal(relativeVolume),
		"close_above_sma_21":      closePrice > sma21,
		"close_above_sma_50":      closePrice > sma50,
		"close_above_sma_200":     closePrice > sma200,
		"sma_21_above_sma_50":     sma21 > sma50,
		"sma_50_above_sma_200":    sma50 > sma200,
		"rsi_overbought":          rsi14 >= 70,
		"rsi_oversold":            rsi14 <= 30,
		"macd_histogram_positive": macdHist > 0,
	}
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
	case rsi14 >= 70:
		return "overbought"
	case rsi14 <= 30:
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
	case atrPercent >= 5:
		return "elevated"
	case atrPercent <= 2:
		return "compressed"
	default:
		return "normal"
	}
}

func volumeSignal(relativeVolume float64) string {
	switch {
	case relativeVolume >= 1.5:
		return "elevated"
	case relativeVolume <= 0.75:
		return "light"
	default:
		return "normal"
	}
}

func cobraTAMACDCommand(c *client.Ref, w io.Writer) *cobra.Command {
	opts := &macdOpts{}
	cmd := &cobra.Command{
		Use:   "macd SYMBOL [SYMBOL...]",
		Short: "Moving Average Convergence/Divergence",
		Long: `Compute MACD (Moving Average Convergence/Divergence) for a symbol. Uses
--fast, --slow, and --signal periods instead of --period. Output keys include
macd, signal, and histogram (histogram = macd - signal). Defaults: fast=12,
slow=26, signal=9.`,
		Example: `  schwab-agent ta macd AAPL
  schwab-agent ta macd AAPL --fast 12 --slow 26 --signal 9 --points 10
  schwab-agent ta macd NVDA --interval weekly --points 10`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := structcli.Unmarshal(cmd, opts); err != nil {
				return err
			}

			return writeTASymbolResults(w, args, func(symbol string) (any, error) {
				return buildMACDOutput(cmd.Context(), c, opts, symbol)
			})
		},
	}

	if err := structcli.Define(cmd, opts); err != nil {
		panic(err)
	}
	cmd.SetFlagErrorFunc(normalizeFlagValidationErrorFunc)

	return cmd
}

func buildMACDOutput(ctx context.Context, c *client.Ref, opts *macdOpts, symbol string) (macdOutput, error) {
	// MACD layers slow EMA + signal EMA; 2x the combined period provides convergence.
	interval := string(opts.Interval)
	candles, timestamps, err := fetchAndValidateCandles(ctx, c, symbol, interval, (opts.Slow+opts.Signal)*2, "macd")
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

	return macdOutput{
		Indicator: "macd",
		Symbol:    symbol,
		Interval:  interval,
		Fast:      opts.Fast,
		Slow:      opts.Slow,
		Signal:    opts.Signal,
		Values:    out,
	}, nil
}

func cobraTAATRCommand(c *client.Ref, w io.Writer) *cobra.Command {
	opts := &atrOpts{}
	cmd := &cobra.Command{
		Use:   "atr SYMBOL [SYMBOL...]",
		Short: "Average True Range",
		Long: `Compute Average True Range for a symbol. ATR measures volatility in price
units: higher values indicate wider price swings. Uses high, low, and close
data with Wilder smoothing. Default period is 14.`,
		Example: `  schwab-agent ta atr AAPL
  schwab-agent ta atr AAPL --period 14 --interval daily --points 10`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := structcli.Unmarshal(cmd, opts); err != nil {
				return err
			}

			return writeTASymbolResults(w, args, func(symbol string) (any, error) {
				return buildATROutput(cmd.Context(), c, opts, symbol)
			})
		},
	}

	if err := structcli.Define(cmd, opts); err != nil {
		panic(err)
	}
	cmd.SetFlagErrorFunc(normalizeFlagValidationErrorFunc)

	return cmd
}

func buildATROutput(ctx context.Context, c *client.Ref, opts *atrOpts, symbol string) (taOutput, error) {
	// ATR uses Wilder smoothing; 3x period provides convergence stability.
	interval := string(opts.Interval)
	candles, timestamps, err := fetchAndValidateCandles(ctx, c, symbol, interval, opts.Period*3, "atr")
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
	opts := &bbandsOpts{}
	cmd := &cobra.Command{
		Use:   "bbands SYMBOL [SYMBOL...]",
		Short: "Bollinger Bands",
		Long: `Compute Bollinger Bands for a symbol. Output keys are upper, middle, and lower
bands. Price near the upper band is relatively high, near the lower band is
relatively low. Use --std-dev to widen or narrow the bands (default 2.0).`,
		Example: `  schwab-agent ta bbands AAPL
  schwab-agent ta bbands AAPL --period 20 --std-dev 2.0 --points 10
  schwab-agent ta bbands AAPL --std-dev 1.5 --points 10`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := structcli.Unmarshal(cmd, opts); err != nil {
				return err
			}

			return writeTASymbolResults(w, args, func(symbol string) (any, error) {
				return buildBBandsOutput(cmd.Context(), c, opts, symbol)
			})
		},
	}

	if err := structcli.Define(cmd, opts); err != nil {
		panic(err)
	}
	cmd.SetFlagErrorFunc(normalizeFlagValidationErrorFunc)

	return cmd
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
	opts := &stochOpts{}
	cmd := &cobra.Command{
		Use:   "stoch SYMBOL [SYMBOL...]",
		Short: "Stochastic Oscillator",
		Long: `Compute Stochastic Oscillator for a symbol. Output keys are slowk and slowd,
both ranging 0-100. Above 80 and below 20 are common signal thresholds.
Configurable via --k-period, --smooth-k, and --d-period.`,
		Example: `  schwab-agent ta stoch AAPL
  schwab-agent ta stoch AAPL --k-period 14 --smooth-k 3 --d-period 3 --points 10`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := structcli.Unmarshal(cmd, opts); err != nil {
				return err
			}

			return writeTASymbolResults(w, args, func(symbol string) (any, error) {
				return buildStochOutput(cmd.Context(), c, opts, symbol)
			})
		},
	}

	if err := structcli.Define(cmd, opts); err != nil {
		panic(err)
	}
	cmd.SetFlagErrorFunc(normalizeFlagValidationErrorFunc)

	return cmd
}

func buildStochOutput(ctx context.Context, c *client.Ref, opts *stochOpts, symbol string) (stochOutput, error) {
	// Stochastic chains three windowed ops (raw %K, smoothed %K, %D);
	// total depth is the sum of all window sizes.
	interval := string(opts.Interval)
	candles, timestamps, err := fetchAndValidateCandles(ctx, c, symbol, interval, opts.KPeriod+opts.SmoothK+opts.DPeriod, "stoch")
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
	opts := &adxOpts{}
	cmd := &cobra.Command{
		Use:   "adx SYMBOL [SYMBOL...]",
		Short: "Average Directional Index",
		Long: `Compute Average Directional Index for a symbol. ADX above 25 indicates a
trending market, below 20 indicates a ranging market. Output includes adx,
plus_di, and minus_di for directional bias. Default period is 14.`,
		Example: `  schwab-agent ta adx AAPL
  schwab-agent ta adx AAPL --period 14 --interval daily --points 10`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := structcli.Unmarshal(cmd, opts); err != nil {
				return err
			}

			return writeTASymbolResults(w, args, func(symbol string) (any, error) {
				return buildADXOutput(cmd.Context(), c, opts, symbol)
			})
		},
	}

	if err := structcli.Define(cmd, opts); err != nil {
		panic(err)
	}
	cmd.SetFlagErrorFunc(normalizeFlagValidationErrorFunc)

	return cmd
}

func buildADXOutput(ctx context.Context, c *client.Ref, opts *adxOpts, symbol string) (adxOutput, error) {
	// ADX double-smooths (DI pass + ADX pass); 4x period ensures both converge.
	interval := string(opts.Interval)
	candles, timestamps, err := fetchAndValidateCandles(ctx, c, symbol, interval, opts.Period*4, "adx")
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

	return adxOutput{
		Indicator: "adx",
		Symbol:    symbol,
		Interval:  interval,
		Period:    opts.Period,
		Values:    out,
	}, nil
}

func cobraTAVWAPCommand(c *client.Ref, w io.Writer) *cobra.Command {
	opts := &vwapOpts{}
	cmd := &cobra.Command{
		Use:   "vwap SYMBOL [SYMBOL...]",
		Short: "Volume Weighted Average Price",
		Long: `Compute Volume Weighted Average Price for a symbol. VWAP is a cumulative
indicator with no --period flag. Primarily used for intraday analysis with
minute-level intervals. Price above VWAP suggests bullish bias, below
suggests bearish.`,
		Example: `  schwab-agent ta vwap AAPL
  schwab-agent ta vwap AAPL --interval 5min --points 20`,
		Args: cobra.MinimumNArgs(1),
		// VWAP is cumulative - no --period flag. Only --interval and --points.
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := structcli.Unmarshal(cmd, opts); err != nil {
				return err
			}

			return writeTASymbolResults(w, args, func(symbol string) (any, error) {
				return buildVWAPOutput(cmd.Context(), c, opts, symbol)
			})
		},
	}

	if err := structcli.Define(cmd, opts); err != nil {
		panic(err)
	}
	cmd.SetFlagErrorFunc(normalizeFlagValidationErrorFunc)

	return cmd
}

func buildVWAPOutput(ctx context.Context, c *client.Ref, opts *vwapOpts, symbol string) (taOutput, error) {
	// Use 20 as minimum candle count - VWAP works with any count >= 1
	// but 20 gives meaningful output for typical use cases.
	interval := string(opts.Interval)
	candles, timestamps, err := fetchAndValidateCandles(ctx, c, symbol, interval, 20, "vwap")
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
	volumes, err := ta.ExtractVolume(candles)
	if err != nil {
		return taOutput{}, err
	}

	values, err := ta.VWAP(highs, lows, closes, volumes)
	if err != nil {
		return taOutput{}, err
	}

	// period=0 because VWAP is cumulative (no windowed period)
	return buildTAOutput("vwap", symbol, interval, 0, opts.Points, timestamps, values)
}

func cobraTAHVCommand(c *client.Ref, w io.Writer) *cobra.Command {
	opts := &hvOpts{}
	cmd := &cobra.Command{
		Use:   "hv SYMBOL [SYMBOL...]",
		Short: "Historical Volatility with regime classification",
		Long: `Compute Historical Volatility for a symbol. Returns a scalar summary (not
time series) with annualized volatility, percentile rank, and regime
classification (low, normal, high, extreme). Includes daily, weekly, and
monthly volatility breakdowns.`,
		Example: `  schwab-agent ta hv AAPL
  schwab-agent ta hv AAPL --period 20 --interval daily`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := structcli.Unmarshal(cmd, opts); err != nil {
				return err
			}

			return writeTASymbolResults(w, args, func(symbol string) (any, error) {
				return buildHVOutput(cmd.Context(), c, opts, symbol)
			})
		},
	}

	if err := structcli.Define(cmd, opts); err != nil {
		panic(err)
	}
	cmd.SetFlagErrorFunc(normalizeFlagValidationErrorFunc)

	return cmd
}

func buildHVOutput(ctx context.Context, c *client.Ref, opts *hvOpts, symbol string) (hvOutput, error) {
	// Need period+1 closes for N returns, plus extra for rolling window warmup.
	// period+21 provides a safety margin for the rolling window.
	interval := string(opts.Interval)
	candles, _, err := fetchAndValidateCandles(ctx, c, symbol, interval, opts.Period+21, "hv")
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
	opts := &expectedMoveOpts{}
	cmd := &cobra.Command{
		Use:   "expected-move SYMBOL [SYMBOL...]",
		Short: "Expected price move from ATM straddle pricing",
		Long: `Compute expected price move from ATM straddle pricing. Fetches the option
chain, finds the nearest expiration to --dte (default 30), and prices the
ATM straddle. Output includes 1x and 2x standard deviation ranges (upper
and lower bounds).`,
		Example: `  schwab-agent ta expected-move AAPL
  schwab-agent ta expected-move AAPL --dte 45`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := structcli.Unmarshal(cmd, opts); err != nil {
				return err
			}

			return writeTASymbolResults(w, args, func(symbol string) (any, error) {
				return buildExpectedMoveOutput(cmd.Context(), c, opts, symbol)
			})
		},
	}

	if err := structcli.Define(cmd, opts); err != nil {
		panic(err)
	}

	return cmd
}

func buildExpectedMoveOutput(ctx context.Context, c *client.Ref, opts *expectedMoveOpts, symbol string) (expectedMoveOutput, error) {
	// Expected Move needs the underlying quote and near-the-money contracts in one response.
	// Keep this to a single chain call so the underlying snapshot and option prices stay aligned.
	chain, err := c.OptionChain(ctx, symbol, &client.ChainParams{
		ContractType:           "ALL",
		IncludeUnderlyingQuote: "true",
		StrikeRange:            "NTM",
	})
	if err != nil {
		return expectedMoveOutput{}, err
	}

	if chain.Underlying == nil {
		return expectedMoveOutput{}, fmt.Errorf("no underlying data available for %s", symbol)
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
		return expectedMoveOutput{}, fmt.Errorf("unable to determine underlying price for %s", symbol)
	}

	if len(chain.CallExpDateMap) == 0 {
		return expectedMoveOutput{}, fmt.Errorf("no options available for %s", symbol)
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
		return expectedMoveOutput{}, fmt.Errorf("no options available for %s", symbol)
	}

	expParts := strings.Split(selectedExpKey, ":")
	expDate := expParts[0]

	callStrikes := chain.CallExpDateMap[selectedExpKey]
	putStrikes := chain.PutExpDateMap[selectedExpKey]
	if len(callStrikes) == 0 || len(putStrikes) == 0 {
		return expectedMoveOutput{}, fmt.Errorf("no options available for %s at expiration %s", symbol, expDate)
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

	callPrice, err := contractPrice(callContracts[0], symbol, atmStrikeKey, "call")
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
	return expectedMoveOutput{
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
	}, nil
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

// buildTAOutput builds the output object for a single-value time series.
// It aligns timestamps to indicator values (shorter due to lookback) and applies --points limit.
func buildTAOutput(
	indicator, symbol, interval string,
	period, points int,
	timestamps []string,
	values []float64,
) (taOutput, error) {
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
) multiTAOutput {
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

	return multiTAOutput{
		Indicator: indicator,
		Symbol:    symbol,
		Interval:  interval,
		Periods:   periods,
		Values:    out,
	}
}
