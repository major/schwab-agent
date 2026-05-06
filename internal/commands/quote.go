package commands

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"

	"github.com/major/schwab-go/schwab/marketdata"
	"github.com/spf13/cobra"

	"github.com/major/schwab-agent/internal/apperr"
	"github.com/major/schwab-agent/internal/client"
	"github.com/major/schwab-agent/internal/output"
)

const (
	quoteFieldExtended    = "extended"
	quoteFieldFundamental = "fundamental"
	quoteFieldQuote       = "quote"
	quoteFieldReference   = "reference"
	quoteFieldRegular     = "regular"
)

// isValidQuoteField reports whether field names one of the Schwab API field
// groups accepted by --fields.
func isValidQuoteField(field string) bool {
	switch field {
	case quoteFieldQuote, quoteFieldFundamental, quoteFieldExtended, quoteFieldReference, quoteFieldRegular:
		return true
	default:
		return false
	}
}

// NewQuoteCmd returns the Cobra command for stock quote operations.
func NewQuoteCmd(c *client.Ref, w io.Writer) *cobra.Command {
	getCmd := newQuoteGetCmd(c, w)
	cmd := &cobra.Command{
		Use:     commandUseQuote,
		Short:   "Stock quote operations",
		Long:    "Get quotes for one or more symbols",
		Args:    cobra.ArbitraryArgs,
		RunE:    defaultSubcommand(getCmd),
		GroupID: groupIDMarketData,
	}
	cmd.SetFlagErrorFunc(suggestSubcommands)

	cmd.AddCommand(getCmd)

	return cmd
}

// quoteGetOpts holds the options for the quote get subcommand.
type quoteGetOpts struct {
	Fields     []string `flag:"fields"     flagdescr:"Quote fields to return (repeatable): quote, fundamental, extended, reference, regular"`
	Indicative bool     `flag:"indicative" flagdescr:"Request indicative (non-tradeable) quotes"`
	Underlying string   `flag:"underlying" flagdescr:"Underlying symbol for option quote"`
	Expiration string   `flag:"expiration" flagdescr:"Expiration date (YYYY-MM-DD) for option quote"`
	Strike     float64  `flag:"strike"     flagdescr:"Strike price for option quote"`
	Call       bool     `flag:"call"       flagdescr:"Call option"`
	Put        bool     `flag:"put"        flagdescr:"Put option"`
}

// schwabGoQuoteParams holds quote options in the shape accepted by schwab-go.
type schwabGoQuoteParams struct {
	Fields     string
	Indicative bool
}

// Validate is called by validateCobraOptions after Cobra decodes bound flags.
// Checks that all --fields values are recognized quote field names.
func (o *quoteGetOpts) Validate(_ context.Context) []error {
	var errs []error
	for _, f := range o.Fields {
		for part := range strings.SplitSeq(f, ",") {
			trimmed := strings.TrimSpace(part)
			if trimmed == "" {
				continue
			}
			lower := strings.ToLower(trimmed)
			if !isValidQuoteField(lower) {
				errs = append(errs, fmt.Errorf("%s", invalidQuoteFieldMessage(trimmed, lower)))
			}
		}
	}

	if len(errs) > 0 {
		return errs
	}

	return nil
}

// invalidQuoteFieldMessage explains invalid quote fields and gives the most
// common remediation for agents that guess technical after seeing fundamental.
func invalidQuoteFieldMessage(field, lower string) string {
	message := fmt.Sprintf("invalid field %q: must be one of %s", field, sortedQuoteFieldNames())
	if lower == "technical" {
		return message + ". For technical indicators (ATR, RSI, SMA), see: schwab-agent ta --help"
	}

	return message
}

// newQuoteGetCmd returns the Cobra subcommand for retrieving quotes.
func newQuoteGetCmd(c *client.Ref, w io.Writer) *cobra.Command {
	opts := &quoteGetOpts{}
	cmd := &cobra.Command{
		Use:   "get [SYMBOL...]",
		Short: "Get quotes for one or more symbols",
		Long: `Get quotes for one or more symbols. Multiple symbols return a partial response
when some are missing. Use --fields to select specific data groups (quote,
fundamental, extended, reference, regular). Key output fields include last
price, bid/ask, volume, 52-week high/low, PE ratio, dividend yield, and
market cap.

Use --underlying, --expiration, --strike, and --call/--put to quote an option
by its components instead of providing a raw OCC symbol. These flags are
mutually exclusive with positional symbol arguments.`,
		Example: `  schwab-agent quote get AAPL
  schwab-agent quote get AAPL NVDA TSLA
  schwab-agent quote get AAPL --fields quote,fundamental
  schwab-agent quote get AAPL --fields fundamental --indicative
  schwab-agent quote get --underlying AMZN --expiration 2026-05-15 --strike 270 --call`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Unmarshal decodes flags and runs quoteGetOpts.Validate(),
			// which rejects unrecognized --fields values.
			if err := validateCobraOptions(cmd.Context(), opts); err != nil {
				return err
			}

			params := buildCobraQuoteParams(opts)

			// Check if any option flag was explicitly set.
			optionMode := cmd.Flags().Changed("underlying") ||
				cmd.Flags().Changed("expiration") ||
				cmd.Flags().Changed("strike") ||
				cmd.Flags().Changed(flagCall) ||
				cmd.Flags().Changed(flagPut)

			if optionMode {
				// Option flags and positional symbols are mutually exclusive.
				if len(args) > 0 {
					return apperr.NewValidationError(
						"cannot combine positional symbols with option flags (--underlying, --expiration, --strike, --call/--put)",
						nil,
					)
				}

				// Validate all required option flags are present.
				if err := validateOptionQuoteFlags(cmd); err != nil {
					return err
				}

				// Build OCC symbol from option components.
				occ, err := buildOCCSymbol(opts.Underlying, opts.Expiration, opts.Strike, opts.Call, opts.Put)
				if err != nil {
					return err
				}

				return quoteSingle(cmd.Context(), c, w, occ, params)
			}

			// Positional symbol mode: require at least one symbol.
			if len(args) == 0 {
				return apperr.NewValidationError(
					"at least one symbol is required (or use option flags: --underlying, --expiration, --strike, --call/--put)",
					nil,
				)
			}

			if len(args) == 1 {
				return quoteSingle(cmd.Context(), c, w, args[0], params)
			}
			return quoteMulti(cmd.Context(), c, w, args, params)
		},
	}

	defineCobraFlags(cmd, opts)
	cmd.MarkFlagsMutuallyExclusive(flagCall, flagPut)

	return cmd
}

// validateOptionQuoteFlags checks that all required option quote flags are
// present when at least one option flag was set. Returns a ValidationError
// listing any missing flags.
func validateOptionQuoteFlags(cmd *cobra.Command) error {
	var missing []string
	if !cmd.Flags().Changed("underlying") {
		missing = append(missing, "--underlying")
	}
	if !cmd.Flags().Changed("expiration") {
		missing = append(missing, "--expiration")
	}
	if !cmd.Flags().Changed("strike") {
		missing = append(missing, "--strike")
	}
	if !cmd.Flags().Changed(flagCall) && !cmd.Flags().Changed(flagPut) {
		missing = append(missing, "--call or --put")
	}
	if len(missing) > 0 {
		return apperr.NewValidationError(
			"option quote requires: "+strings.Join(missing, ", "), nil)
	}
	return nil
}

// buildCobraQuoteParams constructs schwab-go quote params from the validated option struct.
// Field validation has already been performed by quoteGetOpts.Validate().
func buildCobraQuoteParams(opts *quoteGetOpts) schwabGoQuoteParams {
	fields := make([]string, 0, len(opts.Fields))
	for _, f := range opts.Fields {
		// Support both repeatable (--fields QUOTE --fields FUNDAMENTAL)
		// and comma-separated (--fields QUOTE,FUNDAMENTAL) forms.
		for part := range strings.SplitSeq(f, ",") {
			trimmed := strings.TrimSpace(part)
			if trimmed == "" {
				continue
			}
			fields = append(fields, strings.ToLower(trimmed))
		}
	}
	return schwabGoQuoteParams{
		Fields:     strings.Join(fields, ","),
		Indicative: opts.Indicative,
	}
}

// sortedQuoteFieldNames returns the valid field names sorted alphabetically,
// joined as a comma-separated string for use in error messages.
func sortedQuoteFieldNames() string {
	names := []string{
		quoteFieldQuote,
		quoteFieldFundamental,
		quoteFieldExtended,
		quoteFieldReference,
		quoteFieldRegular,
	}
	slices.Sort(names)
	return strings.Join(names, ", ")
}

// quoteSingle fetches and writes a quote for a single symbol.
// The result is wrapped in a symbol-keyed map so the response shape
// matches multi-symbol requests (data.SYMBOL.field). This lets agents
// parse quotes without branching on argument count. See issue #48.
func quoteSingle(ctx context.Context, c *client.Ref, w io.Writer, symbol string, p schwabGoQuoteParams) error {
	var quotes marketdata.QuoteResponse
	if p.Indicative {
		// schwab-go only exposes the indicative flag on GetQuotes, not GetQuote.
		// Route a one-symbol request through the multi-symbol endpoint so the CLI
		// keeps honoring --indicative while still returning the normalized
		// symbol-keyed response shape.
		response, _, err := c.MarketData.GetQuotes(ctx, []string{symbol}, p.Fields, p.Indicative)
		if err != nil {
			return mapQuoteSingleError(symbol, err)
		}
		quotes = quoteResponseValue(response)
	} else {
		response, err := c.MarketData.GetQuote(ctx, symbol, p.Fields)
		if err != nil {
			return mapQuoteSingleError(symbol, err)
		}
		quotes = quoteResponseValue(response)
	}

	quote, ok := quotes[symbol]
	if !ok || quote == nil {
		return apperr.NewSymbolNotFoundError(fmt.Sprintf("symbol %s not found", symbol), nil)
	}

	return output.WriteSuccess(w, marketdata.QuoteResponse{symbol: quote}, output.NewMetadata())
}

// quoteMulti fetches quotes for multiple symbols, using WritePartial when some are missing.
func quoteMulti(ctx context.Context, c *client.Ref, w io.Writer, symbols []string, p schwabGoQuoteParams) error {
	response, quoteErr, err := c.MarketData.GetQuotes(ctx, symbols, p.Fields, p.Indicative)
	if err != nil {
		return mapSchwabGoError(err)
	}
	quotes := quoteResponseValue(response)

	var missing []string
	seenMissing := make(map[string]struct{})
	addMissing := func(sym string) {
		if _, seen := seenMissing[sym]; seen {
			return
		}
		seenMissing[sym] = struct{}{}
		missing = append(missing, fmt.Sprintf("symbol %s not found", sym))
	}

	// Identify symbols absent from the response.
	for _, sym := range symbols {
		if quote, ok := quotes[sym]; !ok || quote == nil {
			addMissing(sym)
		}
	}
	if quoteErr != nil {
		for _, sym := range quoteErr.InvalidSymbols {
			addMissing(sym)
		}
	}

	if len(missing) == 0 {
		return output.WriteSuccess(w, quotes, output.NewMetadata())
	}

	meta := output.NewMetadata()
	meta.Requested = len(symbols)
	meta.Returned = len(quotes)
	return output.WritePartial(w, quotes, missing, meta)
}

// quoteResponseValue converts schwab-go's optional response pointer to the
// zero-value-safe map used by output writers.
func quoteResponseValue(response *marketdata.QuoteResponse) marketdata.QuoteResponse {
	if response == nil {
		return marketdata.QuoteResponse{}
	}
	return *response
}

// mapQuoteSingleError keeps the command's historical 404 contract while using
// the shared schwab-go mapper for all other Schwab API failures.
func mapQuoteSingleError(symbol string, err error) error {
	mappedErr := mapSchwabGoError(err)
	var httpErr *apperr.HTTPError
	if errors.As(mappedErr, &httpErr) && httpErr.StatusCode == http.StatusNotFound {
		return apperr.NewSymbolNotFoundError(fmt.Sprintf("symbol %s not found", symbol), mappedErr)
	}
	return mappedErr
}
