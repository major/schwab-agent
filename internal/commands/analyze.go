package commands

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/major/schwab-agent/internal/apperr"
	"github.com/major/schwab-agent/internal/client"
	"github.com/major/schwab-agent/internal/output"
)

// analyzeOpts holds CLI flags for the analyze command. Same data-frequency
// controls as ta dashboard since analyze wraps it alongside a quote fetch.
type analyzeOpts struct {
	Interval taInterval `flag:"interval" flagdescr:"Data interval (daily, weekly, 1min, 5min, 15min, 30min)" default:"daily" flaggroup:"data"`
	Points   int        `flag:"points"   flagdescr:"Number of TA output points (default 1; 0 = all)"         default:"1"     flaggroup:"data"`
}

// analyzeResult holds the combined quote and TA dashboard data for a single
// symbol. Fields use `any` to avoid tight coupling to specific response types;
// either field is nil when that data source failed.
type analyzeResult struct {
	Quote    any `json:"quote"`
	Analysis any `json:"analysis"`
}

// NewAnalyzeCmd returns a top-level command that combines a quote lookup and a
// TA dashboard into a single CLI call per symbol. This saves agents two
// round-trips (quote get + ta dashboard) when they need both market data and
// technical context for a trading decision.
func NewAnalyzeCmd(c *client.Ref, w io.Writer) *cobra.Command {
	opts := &analyzeOpts{}
	cmd := &cobra.Command{
		Use:   "analyze SYMBOL [SYMBOL...]",
		Short: "Combined quote and technical analysis for one or more symbols",
		Long: `Fetch a quote and compute an opinionated TA dashboard for each symbol in a
single CLI call. Combines "quote get" and "ta dashboard" output into per-symbol
nesting so agents get both market data and technical context without two
round-trips. Partial failures are reported per-symbol: if the quote succeeds
but TA fails, that symbol appears with its quote and a null analysis field.`,
		Example: `  schwab-agent analyze AAPL
  schwab-agent analyze AAPL NVDA
  schwab-agent analyze AAPL --interval weekly --points 5`,
		GroupID: groupIDMarketData,
		Args:    cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateCobraOptions(cmd.Context(), opts); err != nil {
				return err
			}

			interval := string(opts.Interval)
			points := opts.Points

			return writeAnalyzeResults(cmd.Context(), c, w, args, interval, points)
		},
	}

	defineCobraFlags(cmd, opts)
	cmd.SetFlagErrorFunc(normalizeFlagValidationErrorFunc)

	return cmd
}

// writeAnalyzeResults fetches quote + TA for each symbol and writes the output.
// Single-symbol success returns the result directly; multi-symbol uses the
// partial-failure pattern from writeTASymbolResults.
func writeAnalyzeResults(
	ctx context.Context,
	c *client.Ref,
	w io.Writer,
	symbols []string,
	interval string,
	points int,
) error {
	if len(symbols) == 1 {
		symbol := symbols[0]
		result, err := buildAnalyzeResult(ctx, c, symbol, interval, points)
		if err != nil {
			// Keep the single-symbol output contract aligned with the multi-symbol
			// path below. Young IPOs, thin history, or transient quote failures can
			// leave one side useful, so return a partial envelope instead of discarding
			// the successful data and turning the whole command into a hard failure.
			// Symbol-not-found stays fatal because TA can still succeed from historical
			// candles even when the quote endpoint proves the requested symbol is not
			// available as a current quote.
			var symErr *apperr.SymbolNotFoundError
			if !errors.As(err, &symErr) && hasAnalyzeData(result) {
				meta := output.NewMetadata()
				meta.Requested = 1
				meta.Returned = 1
				return output.WritePartial(
					w,
					map[string]any{symbol: result},
					[]string{fmt.Sprintf("%s: %v", symbol, err)},
					meta,
				)
			}
			return err
		}
		return output.WriteSuccess(w, map[string]any{symbol: result}, output.NewMetadata())
	}

	data := make(map[string]any, len(symbols))
	partialErrors := make([]string, 0)
	joinedErrors := make([]error, 0)

	for _, symbol := range symbols {
		result, err := buildAnalyzeResult(ctx, c, symbol, interval, points)
		if err != nil {
			// Check if result has any partial data (quote or analysis succeeded).
			// If either field is non-nil, include the partial result and report the error.
			// Only skip the symbol entirely if both fields are nil (total failure).
			if hasAnalyzeData(result) {
				data[symbol] = result
			}
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

func hasAnalyzeData(result analyzeResult) bool {
	return result.Quote != nil || result.Analysis != nil
}

// buildAnalyzeResult fetches a quote and computes the TA dashboard for a
// single symbol. Both calls happen sequentially since the TA dashboard
// requires a separate price-history fetch anyway (no shared data to reuse).
//
// Partial per-field failures: if the quote succeeds but TA fails (or vice
// versa), the successful data is kept and the failed field is set to nil.
// The caller still sees an error for the symbol so it can be reported via
// WritePartial.
func buildAnalyzeResult(
	ctx context.Context,
	c *client.Ref,
	symbol, interval string,
	points int,
) (analyzeResult, error) {
	var result analyzeResult
	var quoteErr, taErr error

	// Fetch quote via schwab-go. analyze does not expose --fields or
	// --indicative flags, so an empty fields string asks Schwab for the
	// default quote payload while keeping this command on the same marketdata
	// client path as quote, instrument, and market commands.
	quoteResponse, err := c.MarketData.GetQuote(ctx, symbol, "")
	if err != nil {
		quoteErr = mapQuoteSingleError(symbol, err)
	} else {
		quote := quoteResponseValue(quoteResponse)[symbol]
		if quoteEntryMissing(quote) {
			quoteErr = apperr.NewSymbolNotFoundError(fmt.Sprintf("symbol %s not found", symbol), nil)
		} else {
			result.Quote = quote
		}
	}

	// Compute TA dashboard.
	dashboard, err := buildTADashboard(ctx, c, symbol, interval, points)
	if err != nil {
		taErr = err
	} else {
		result.Analysis = dashboard
	}

	// Both failed: return a combined error.
	if quoteErr != nil && taErr != nil {
		return analyzeResult{}, fmt.Errorf("quote: %w; ta: %w", quoteErr, taErr)
	}

	// One failed: return partial data with the error so the caller can
	// include it in WritePartial output.
	if quoteErr != nil {
		return result, fmt.Errorf("quote failed: %w", quoteErr)
	}
	if taErr != nil {
		return result, fmt.Errorf("ta failed: %w", taErr)
	}

	return result, nil
}
