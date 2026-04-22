// Package commands provides CLI command builders for schwab-agent.
package commands

import (
	"context"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"

	"github.com/urfave/cli/v3"

	"github.com/major/schwab-agent/internal/client"
	"github.com/major/schwab-agent/internal/models"
	"github.com/major/schwab-agent/internal/output"
	"github.com/major/schwab-agent/internal/ta"
)

// taIndicatorFlags returns the shared flags for all TA indicator subcommands.
// RSI defaults to period 14; SMA/EMA default to 20.
func taIndicatorFlags(defaultPeriod int) []cli.Flag {
	return []cli.Flag{
		&cli.IntFlag{Name: "period", Usage: "Indicator period", Value: defaultPeriod},
		&cli.StringFlag{Name: "interval", Usage: "Data interval (daily, weekly, 1min, 5min, 15min, 30min)", Value: "daily"},
		&cli.IntFlag{Name: "points", Usage: "Number of output points (0 = all)", Value: 0},
	}
}

// TACommand returns the CLI command for technical analysis indicators.
func TACommand(c *client.Client, w io.Writer) *cli.Command {
	return &cli.Command{
		Name:  "ta",
		Usage: "Technical analysis indicators",
		Commands: []*cli.Command{
			taSMACommand(c, w),
			taEMACommand(c, w),
			taRSICommand(c, w),
			taMACDCommand(c, w),
			taATRCommand(c, w),
			taBBandsCommand(c, w),
			taStochCommand(c, w),
			taADXCommand(c, w),
			taVWAPCommand(c, w),
			taHVCommand(c, w),
			taExpectedMoveCommand(c, w),
		},
	}
}

func taSMACommand(c *client.Client, w io.Writer) *cli.Command {
	return &cli.Command{
		Name:  "sma",
		Usage: "Simple Moving Average",
		Flags: taIndicatorFlags(20),
		Action: func(ctx context.Context, cmd *cli.Command) error {
			symbol := cmd.Args().First()
			if err := requireArg(symbol, "symbol"); err != nil {
				return err
			}

			period := cmd.Int("period")
			interval := cmd.String("interval")
			points := cmd.Int("points")

			candles, timestamps, err := fetchAndValidateCandles(ctx, c, symbol, interval, period, "sma")
			if err != nil {
				return err
			}

			closes, err := ta.ExtractClose(candles)
			if err != nil {
				return err
			}

			values, err := ta.SMA(closes, period)
			if err != nil {
				return err
			}

			return writeTAOutput(w, "sma", symbol, interval, period, points, timestamps, values)
		},
	}
}

func taEMACommand(c *client.Client, w io.Writer) *cli.Command {
	return &cli.Command{
		Name:  "ema",
		Usage: "Exponential Moving Average",
		Flags: taIndicatorFlags(20),
		Action: func(ctx context.Context, cmd *cli.Command) error {
			symbol := cmd.Args().First()
			if err := requireArg(symbol, "symbol"); err != nil {
				return err
			}

			period := cmd.Int("period")
			interval := cmd.String("interval")
			points := cmd.Int("points")

			candles, timestamps, err := fetchAndValidateCandles(ctx, c, symbol, interval, period, "ema")
			if err != nil {
				return err
			}

			closes, err := ta.ExtractClose(candles)
			if err != nil {
				return err
			}

			values, err := ta.EMA(closes, period)
			if err != nil {
				return err
			}

			return writeTAOutput(w, "ema", symbol, interval, period, points, timestamps, values)
		},
	}
}

func taRSICommand(c *client.Client, w io.Writer) *cli.Command {
	return &cli.Command{
		Name:  "rsi",
		Usage: "Relative Strength Index",
		Flags: taIndicatorFlags(14),
		Action: func(ctx context.Context, cmd *cli.Command) error {
			symbol := cmd.Args().First()
			if err := requireArg(symbol, "symbol"); err != nil {
				return err
			}

			period := cmd.Int("period")
			interval := cmd.String("interval")
			points := cmd.Int("points")

			candles, timestamps, err := fetchAndValidateCandles(ctx, c, symbol, interval, period, "rsi")
			if err != nil {
				return err
			}

			closes, err := ta.ExtractClose(candles)
			if err != nil {
				return err
			}

			values, err := ta.RSI(closes, period)
			if err != nil {
				return err
			}

			return writeTAOutput(w, "rsi", symbol, interval, period, points, timestamps, values)
		},
	}
}

func taMACDCommand(c *client.Client, w io.Writer) *cli.Command {
	return &cli.Command{
		Name:  "macd",
		Usage: "Moving Average Convergence/Divergence",
		Flags: []cli.Flag{
			&cli.IntFlag{Name: "fast", Usage: "Fast EMA period", Value: 12},
			&cli.IntFlag{Name: "slow", Usage: "Slow EMA period", Value: 26},
			&cli.IntFlag{Name: "signal", Usage: "Signal EMA period", Value: 9},
			&cli.StringFlag{Name: "interval", Usage: "Data interval (daily, weekly, 1min, 5min, 15min, 30min)", Value: "daily"},
			&cli.IntFlag{Name: "points", Usage: "Number of output points (0 = all)", Value: 0},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			symbol := cmd.Args().First()
			if err := requireArg(symbol, "symbol"); err != nil {
				return err
			}

			fast := cmd.Int("fast")
			slow := cmd.Int("slow")
			signal := cmd.Int("signal")
			interval := cmd.String("interval")
			points := cmd.Int("points")

			// Use slow period for data window calculation since it's the largest lookback
			candles, timestamps, err := fetchAndValidateCandles(ctx, c, symbol, interval, slow, "macd")
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

			out := make([]map[string]any, len(macdVals))
			for i := range macdVals {
				out[i] = map[string]any{
					"datetime":  timestamps[i],
					"macd":      macdVals[i],
					"signal":    signalVals[i],
					"histogram": histVals[i],
				}
			}

			if points > 0 && points < len(out) {
				out = out[len(out)-points:]
			}

			data := map[string]any{
				"indicator": "macd",
				"symbol":    symbol,
				"interval":  interval,
				"fast":      fast,
				"slow":      slow,
				"signal":    signal,
				"values":    out,
			}
			return output.WriteSuccess(w, data, output.TimestampMeta())
		},
	}
}

func taATRCommand(c *client.Client, w io.Writer) *cli.Command {
	return &cli.Command{
		Name:  "atr",
		Usage: "Average True Range",
		Flags: taIndicatorFlags(14),
		Action: func(ctx context.Context, cmd *cli.Command) error {
			symbol := cmd.Args().First()
			if err := requireArg(symbol, "symbol"); err != nil {
				return err
			}

			period := cmd.Int("period")
			interval := cmd.String("interval")
			points := cmd.Int("points")

			candles, timestamps, err := fetchAndValidateCandles(ctx, c, symbol, interval, period, "atr")
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
}

func taBBandsCommand(c *client.Client, w io.Writer) *cli.Command {
	return &cli.Command{
		Name:  "bbands",
		Usage: "Bollinger Bands",
		Flags: []cli.Flag{
			&cli.IntFlag{Name: "period", Usage: "BBands period", Value: 20},
			&cli.Float64Flag{Name: "std-dev", Usage: "Standard deviations", Value: 2.0},
			&cli.StringFlag{Name: "interval", Usage: "Data interval (daily, weekly, 1min, 5min, 15min, 30min)", Value: "daily"},
			&cli.IntFlag{Name: "points", Usage: "Number of output points (0 = all)", Value: 0},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			symbol := cmd.Args().First()
			if err := requireArg(symbol, "symbol"); err != nil {
				return err
			}

			period := cmd.Int("period")
			stdDev := cmd.Float("std-dev")
			interval := cmd.String("interval")
			points := cmd.Int("points")

			candles, timestamps, err := fetchAndValidateCandles(ctx, c, symbol, interval, period, "bbands")
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

			out := make([]map[string]any, len(upper))
			for i := range upper {
				out[i] = map[string]any{
					"datetime": timestamps[i],
					"upper":    upper[i],
					"middle":   middle[i],
					"lower":    lower[i],
				}
			}

			if points > 0 && points < len(out) {
				out = out[len(out)-points:]
			}

			data := map[string]any{
				"indicator": "bbands",
				"symbol":    symbol,
				"interval":  interval,
				"period":    period,
				"std_dev":   stdDev,
				"values":    out,
			}
			return output.WriteSuccess(w, data, output.TimestampMeta())
		},
	}
}

func taStochCommand(c *client.Client, w io.Writer) *cli.Command {
	return &cli.Command{
		Name:  "stoch",
		Usage: "Stochastic Oscillator",
		Flags: []cli.Flag{
			&cli.IntFlag{Name: "k-period", Usage: "Fast %K lookback period", Value: 14},
			&cli.IntFlag{Name: "smooth-k", Usage: "Slow %K smoothing period", Value: 3},
			&cli.IntFlag{Name: "d-period", Usage: "Slow %D period", Value: 3},
			&cli.StringFlag{Name: "interval", Usage: "Data interval (daily, weekly, 1min, 5min, 15min, 30min)", Value: "daily"},
			&cli.IntFlag{Name: "points", Usage: "Number of output points (0 = all)", Value: 0},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			symbol := cmd.Args().First()
			if err := requireArg(symbol, "symbol"); err != nil {
				return err
			}

			kPeriod := cmd.Int("k-period")
			smoothK := cmd.Int("smooth-k")
			dPeriod := cmd.Int("d-period")
			interval := cmd.String("interval")
			points := cmd.Int("points")

			candles, timestamps, err := fetchAndValidateCandles(ctx, c, symbol, interval, kPeriod, "stoch")
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

			out := make([]map[string]any, len(slowK))
			for i := range slowK {
				out[i] = map[string]any{
					"datetime": timestamps[i],
					"slowk":    slowK[i],
					"slowd":    slowD[i],
				}
			}

			if points > 0 && points < len(out) {
				out = out[len(out)-points:]
			}

			data := map[string]any{
				"indicator": "stoch",
				"symbol":    symbol,
				"interval":  interval,
				"k_period":  kPeriod,
				"smooth_k":  smoothK,
				"d_period":  dPeriod,
				"values":    out,
			}
			return output.WriteSuccess(w, data, output.TimestampMeta())
		},
	}
}

func taADXCommand(c *client.Client, w io.Writer) *cli.Command {
	return &cli.Command{
		Name:  "adx",
		Usage: "Average Directional Index",
		Flags: taIndicatorFlags(14),
		Action: func(ctx context.Context, cmd *cli.Command) error {
			symbol := cmd.Args().First()
			if err := requireArg(symbol, "symbol"); err != nil {
				return err
			}

			period := cmd.Int("period")
			interval := cmd.String("interval")
			points := cmd.Int("points")

			candles, timestamps, err := fetchAndValidateCandles(ctx, c, symbol, interval, period, "adx")
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

			out := make([]map[string]any, len(adxVals))
			for i := range adxVals {
				out[i] = map[string]any{
					"datetime": timestamps[i],
					"adx":      adxVals[i],
					"plus_di":  plusDI[i],
					"minus_di": minusDI[i],
				}
			}

			if points > 0 && points < len(out) {
				out = out[len(out)-points:]
			}

			data := map[string]any{
				"indicator": "adx",
				"symbol":    symbol,
				"interval":  interval,
				"period":    period,
				"values":    out,
			}
			return output.WriteSuccess(w, data, output.TimestampMeta())
		},
	}
}

func taVWAPCommand(c *client.Client, w io.Writer) *cli.Command {
	return &cli.Command{
		Name:  "vwap",
		Usage: "Volume Weighted Average Price",
		// VWAP is cumulative - no --period flag. Only --interval and --points.
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "interval", Usage: "Data interval (daily, weekly, 1min, 5min, 15min, 30min)", Value: "daily"},
			&cli.IntFlag{Name: "points", Usage: "Number of output points (0 = all)", Value: 0},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			symbol := cmd.Args().First()
			if err := requireArg(symbol, "symbol"); err != nil {
				return err
			}

			interval := cmd.String("interval")
			points := cmd.Int("points")

			// Use 20 as minimum candle count - VWAP works with any count >= 1
			// but 20 gives meaningful output for typical use cases.
			candles, timestamps, err := fetchAndValidateCandles(ctx, c, symbol, interval, 20, "vwap")
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
}

func taHVCommand(c *client.Client, w io.Writer) *cli.Command {
	return &cli.Command{
		Name:  "hv",
		Usage: "Historical Volatility with regime classification",
		Flags: taIndicatorFlags(20),
		Action: func(ctx context.Context, cmd *cli.Command) error {
			symbol := cmd.Args().First()
			if err := requireArg(symbol, "symbol"); err != nil {
				return err
			}

			period := cmd.Int("period")
			interval := cmd.String("interval")

			// Need period+1 closes for N returns, plus extra for rolling window warmup.
			// period+21 provides a safety margin for the rolling window.
			candles, _, err := fetchAndValidateCandles(ctx, c, symbol, interval, period+21, "hv")
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
			data := map[string]any{
				"indicator":       "hv",
				"symbol":          symbol,
				"interval":        interval,
				"period":          period,
				"daily_vol":       result.DailyVol,
				"weekly_vol":      result.WeeklyVol,
				"monthly_vol":     result.MonthlyVol,
				"annualized_vol":  result.AnnualizedVol,
				"percentile_rank": result.PercentileRank,
				"regime":          result.Regime,
				"min_vol":         result.MinVol,
				"max_vol":         result.MaxVol,
				"mean_vol":        result.MeanVol,
			}
			return output.WriteSuccess(w, data, output.TimestampMeta())
		},
	}
}

func taExpectedMoveCommand(c *client.Client, w io.Writer) *cli.Command {
	return &cli.Command{
		Name:  "expected-move",
		Usage: "Expected price move from ATM straddle pricing",
		Flags: []cli.Flag{
			&cli.IntFlag{Name: "dte", Usage: "Target days to expiration", Value: 30},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			symbol := cmd.Args().First()
			if err := requireArg(symbol, "symbol"); err != nil {
				return err
			}

			targetDTE := cmd.Int("dte")

			// Expected Move needs the underlying quote and near-the-money contracts in one response.
			// Keep this to a single chain call so the underlying snapshot and option prices stay aligned.
			chain, err := c.OptionChain(ctx, symbol, &client.ChainParams{
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
			data := map[string]any{
				"indicator":        "expected-move",
				"symbol":           symbol,
				"underlying_price": underlyingPrice,
				"expiration":       expDate,
				"dte":              actualDTE,
				"straddle_price":   result.StraddlePrice,
				"expected_move":    result.ExpectedMove,
				"adjusted_move":    result.AdjustedMove,
				"upper_1x":         result.Upper1x,
				"lower_1x":         result.Lower1x,
				"upper_2x":         result.Upper2x,
				"lower_2x":         result.Lower2x,
			}
			return output.WriteSuccess(w, data, output.TimestampMeta())
		},
	}
}

// contractPrice extracts the best available option price.
// The chain API sometimes omits mark, so fall back to the bid/ask midpoint.
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
// Returns the candle slice and pre-extracted timestamps for alignment.
func fetchAndValidateCandles(
	ctx context.Context,
	c *client.Client,
	symbol, interval string,
	period int,
	indicator string,
) ([]models.Candle, []string, error) {
	periodType, periodVal, freqType, freq, err := ta.IntervalToHistoryParams(interval)
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

	required := max(period*3, period+20)
	if err := ta.ValidateMinCandles(result.Candles, required, indicator); err != nil {
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

	data := map[string]any{
		"indicator": indicator,
		"symbol":    symbol,
		"interval":  interval,
		"period":    period,
		"values":    out,
	}
	return output.WriteSuccess(w, data, output.TimestampMeta())
}
