package commands

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/major/schwab-go/schwab/marketdata"
	"github.com/spf13/cobra"

	"github.com/major/schwab-agent/internal/apperr"
	"github.com/major/schwab-agent/internal/client"
	"github.com/major/schwab-agent/internal/output"
)

// analyzeOpts holds CLI flags for the analyze command. Same data-frequency
// controls as ta dashboard since analyze wraps it alongside a quote fetch.
type analyzeOpts struct {
	Interval taInterval `flag:"interval" flagdescr:"Data interval (daily, weekly, 1min, 5min, 15min, 30min)" default:"daily" flaggroup:"data"`

	Points int `flag:"points" flagdescr:"Number of TA output points (default 1; 0 = all)" default:"1" flaggroup:"data"`

	LatestOnly bool `flag:"latest-only" flagdescr:"Omit duplicate dashboard values rows" flaggroup:"output"`

	Compact bool `flag:"compact" flagdescr:"Return compact quote and TA fields for multi-symbol scans" flaggroup:"output"`
}

// analyzeResult holds the combined quote and TA dashboard data for a single
// symbol. Fields use `any` to avoid tight coupling to specific response types;
// either field is nil when that data source failed.
type analyzeResult struct {
	Quote    any `json:"quote"`
	Analysis any `json:"analysis"`
}

type compactAnalyzeResult struct {
	Quote     map[string]any `json:"quote,omitempty"`
	Technical map[string]any `json:"technical,omitempty"`
	Signals   map[string]any `json:"signals,omitempty"`
}

type analyzeRequest struct {
	Interval   string
	Points     int
	LatestOnly bool
	Compact    bool
}

type analyzeSymbolResult struct {
	Payload any
	Err     error
}

type analyzeTAResult struct {
	Dashboard dashboardOutput
	Err       error
}

const analyzeTAConcurrency = 4

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
but TA fails, that symbol appears with its quote and a null analysis field.
Use --latest-only to omit duplicate dashboard values rows, or --compact for
watchlist scans that only need key quote fields, latest TA fields, and signals.`,
		Example: `  schwab-agent analyze AAPL
	  schwab-agent analyze AAPL NVDA
  schwab-agent analyze AAPL --interval weekly --points 5
  schwab-agent analyze AAPL MSFT NVDA --compact
  schwab-agent analyze AAPL --latest-only`,
		GroupID: groupIDMarketData,
		Args:    cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateCobraOptions(cmd.Context(), opts); err != nil {
				return err
			}

			interval := string(opts.Interval)
			points := opts.Points

			return writeAnalyzeResults(cmd.Context(), c, w, args, analyzeRequest{
				Interval:   interval,
				Points:     points,
				LatestOnly: opts.LatestOnly,
				Compact:    opts.Compact,
			})
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
	req analyzeRequest,
) error {
	results := buildAnalyzeResults(ctx, c, symbols, req)
	data := make(map[string]any, len(symbols))
	partialErrors := make([]string, 0)
	joinedErrors := make([]error, 0)

	for _, symbol := range symbols {
		result := results[symbol]
		if result.Err != nil {
			if hasAnalyzePayload(result.Payload) {
				data[symbol] = result.Payload
			}
			partialErrors = append(partialErrors, fmt.Sprintf("%s: %v", symbol, result.Err))
			joinedErrors = append(joinedErrors, fmt.Errorf("%s: %w", symbol, result.Err))
			continue
		}
		data[symbol] = result.Payload
	}

	if len(symbols) == 1 {
		symbol := symbols[0]
		result := results[symbol]
		if result.Err != nil {
			// Keep the single-symbol output contract aligned with the multi-symbol
			// path below. Young IPOs, thin history, or transient quote failures can
			// leave one side useful, so return a partial envelope instead of discarding
			// the successful data and turning the whole command into a hard failure.
			// Symbol-not-found stays fatal because TA can still succeed from historical
			// candles even when the quote endpoint proves the requested symbol is not
			// available as a current quote.
			var symErr *apperr.SymbolNotFoundError
			if !errors.As(result.Err, &symErr) && hasAnalyzePayload(result.Payload) {
				meta := output.NewMetadata()
				meta.Requested = 1
				meta.Returned = 1
				return output.WritePartial(
					w,
					map[string]any{symbol: result.Payload},
					[]string{fmt.Sprintf("%s: %v", symbol, result.Err)},
					meta,
				)
			}
			return result.Err
		}
		return output.WriteSuccess(w, map[string]any{symbol: result.Payload}, output.NewMetadata())
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

func hasAnalyzePayload(payload any) bool {
	switch result := payload.(type) {
	case analyzeResult:
		return hasAnalyzeData(result)
	case compactAnalyzeResult:
		return result.Quote != nil || result.Technical != nil || result.Signals != nil
	default:
		return payload != nil
	}
}

func buildAnalyzeResults(
	ctx context.Context,
	c *client.Ref,
	symbols []string,
	req analyzeRequest,
) map[string]analyzeSymbolResult {
	quotes, quoteErrors := fetchAnalyzeQuotes(ctx, c, symbols)
	dashboards := fetchAnalyzeDashboards(ctx, c, symbols, req)

	results := make(map[string]analyzeSymbolResult, len(symbols))
	for _, symbol := range symbols {
		quote := quotes[symbol]
		quoteErr := quoteErrors[symbol]
		taResult := dashboards[symbol]
		err := analyzeResultError(quoteErr, taResult.Err)

		if req.Compact {
			results[symbol] = analyzeSymbolResult{
				Payload: buildCompactAnalyzeResult(quote, taResult.Dashboard),
				Err:     err,
			}
			continue
		}

		var result analyzeResult
		if quote != nil {
			// Do not assign a typed nil *QuoteEntry into the any field. A typed nil
			// inside an interface is non-nil, which makes the partial-output check
			// think we have usable quote data when both quote and TA failed.
			result.Quote = quote
		}
		if taResult.Err == nil {
			result.Analysis = taResult.Dashboard
		}
		results[symbol] = analyzeSymbolResult{Payload: result, Err: err}
	}

	return results
}

func fetchAnalyzeQuotes(
	ctx context.Context,
	c *client.Ref,
	symbols []string,
) (marketdata.QuoteResponse, map[string]error) {
	response, quoteErr, err := c.MarketData.GetQuotes(ctx, symbols, "", false)
	quoteErrors := make(map[string]error)
	if err != nil {
		mappedErr := mapSchwabGoError(err)
		if len(symbols) == 1 {
			mappedErr = mapQuoteSingleError(symbols[0], err)
		}
		for _, symbol := range symbols {
			quoteErrors[symbol] = mappedErr
		}
		return marketdata.QuoteResponse{}, quoteErrors
	}

	quotes := quoteResponseValue(response)
	for symbol, quote := range quotes {
		if quoteEntryMissing(quote) {
			delete(quotes, symbol)
		}
	}
	for _, symbol := range symbols {
		if _, ok := quotes[symbol]; !ok {
			quoteErrors[symbol] = apperr.NewSymbolNotFoundError(fmt.Sprintf("symbol %s not found", symbol), nil)
		}
	}
	if quoteErr != nil {
		for _, symbol := range quoteErr.InvalidSymbols {
			quoteErrors[symbol] = apperr.NewSymbolNotFoundError(fmt.Sprintf("symbol %s not found", symbol), nil)
		}
	}

	return quotes, quoteErrors
}

func fetchAnalyzeDashboards(
	ctx context.Context,
	c *client.Ref,
	symbols []string,
	req analyzeRequest,
) map[string]analyzeTAResult {
	results := make(map[string]analyzeTAResult, len(symbols))
	if len(symbols) == 0 {
		return results
	}

	limit := min(len(symbols), analyzeTAConcurrency)
	jobs := make(chan string)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for range limit {
		wg.Go(func() {
			for symbol := range jobs {
				dashboard, err := buildTADashboard(ctx, c, symbol, req.Interval, req.Points)
				if err == nil && (req.LatestOnly || req.Compact) {
					// latest already contains the only row most scans need. Dropping values
					// here keeps ta dashboard's historical contract while avoiding duplicate
					// analyze output for latest-only workflows.
					dashboard.Values = nil
				}

				mu.Lock()
				results[symbol] = analyzeTAResult{Dashboard: dashboard, Err: err}
				mu.Unlock()
			}
		})
	}

	sendAnalyzeDashboardJobs(ctx, jobs, symbols, results, &mu)
	wg.Wait()

	return results
}

func sendAnalyzeDashboardJobs(
	ctx context.Context,
	jobs chan<- string,
	symbols []string,
	results map[string]analyzeTAResult,
	mu *sync.Mutex,
) {
	defer close(jobs)

	for i, symbol := range symbols {
		select {
		case jobs <- symbol:
		case <-ctx.Done():
			mu.Lock()
			for _, pendingSymbol := range symbols[i:] {
				if _, ok := results[pendingSymbol]; !ok {
					results[pendingSymbol] = analyzeTAResult{Err: ctx.Err()}
				}
			}
			mu.Unlock()
			return
		}
	}
}

func analyzeResultError(quoteErr, taErr error) error {
	if quoteErr != nil && taErr != nil {
		return fmt.Errorf("quote: %w; ta: %w", quoteErr, taErr)
	}
	if quoteErr != nil {
		return fmt.Errorf("quote failed: %w", quoteErr)
	}
	if taErr != nil {
		return fmt.Errorf("ta failed: %w", taErr)
	}
	return nil
}

func buildCompactAnalyzeResult(quote *marketdata.QuoteEntry, dashboard dashboardOutput) compactAnalyzeResult {
	return compactAnalyzeResult{
		Quote:     compactAnalyzeQuote(quote),
		Technical: compactAnalyzeTechnical(dashboard),
		Signals:   compactAnalyzeSignals(dashboard),
	}
}

func compactAnalyzeQuote(quote *marketdata.QuoteEntry) map[string]any {
	if quoteEntryMissing(quote) || len(quote.Quote) == 0 {
		return nil
	}

	var raw map[string]any
	if err := json.Unmarshal(quote.Quote, &raw); err != nil {
		return nil
	}

	compact := map[string]any{commandUseSymbol: quote.Symbol}
	copyCompactField(compact, raw, "bid", "bidPrice")
	copyCompactField(compact, raw, "ask", "askPrice")
	copyCompactField(compact, raw, "last", "lastPrice")
	copyCompactField(compact, raw, "mark", "mark")
	copyCompactField(compact, raw, "netPercentChange", "netPercentChange")
	copyCompactField(compact, raw, "volume", "totalVolume")
	copyCompactField(compact, raw, "quoteTime", "quoteTime")
	return compact
}

func compactAnalyzeTechnical(dashboard dashboardOutput) map[string]any {
	if dashboard.Latest == nil {
		return nil
	}

	compact := make(map[string]any)
	for _, key := range []string{
		"datetime",
		"close",
		"distance_from_sma_21_percent",
		"distance_from_sma_50_percent",
		"distance_from_sma_200_percent",
		dashboardKeyRSI14,
		"atr_percent",
		"relative_volume",
	} {
		copyCompactField(compact, dashboard.Latest, key, key)
	}
	return compact
}

func compactAnalyzeSignals(dashboard dashboardOutput) map[string]any {
	if dashboard.Signals == nil {
		return nil
	}
	return map[string]any{
		"trend":            dashboard.Signals["trend"],
		"momentum":         dashboard.Signals["momentum"],
		fieldVolatility:    dashboard.Signals[fieldVolatility],
		dashboardKeyVolume: dashboard.Signals[dashboardKeyVolume],
	}
}

func copyCompactField(dst, src map[string]any, dstKey, srcKey string) {
	if value, ok := src[srcKey]; ok {
		dst[dstKey] = value
	}
}
