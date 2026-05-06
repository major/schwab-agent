package commands

import (
	"context"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/major/schwab-agent/internal/apperr"
	"github.com/major/schwab-agent/internal/client"
	"github.com/major/schwab-agent/internal/output"
)

// validQuoteFields lists the allowed values for the --fields flag.
// The Schwab API accepts these field groups to control which data
// sections are returned in the quote response.
var validQuoteFields = map[string]bool{
	"quote":       true,
	"fundamental": true,
	"extended":    true,
	"reference":   true,
	"regular":     true,
}

// NewQuoteCmd returns the Cobra command for stock quote operations.
func NewQuoteCmd(c *client.Ref, w io.Writer) *cobra.Command {
	getCmd := newQuoteGetCmd(c, w)
	cmd := &cobra.Command{
		Use:     "quote",
		Short:   "Stock quote operations",
		Long:    "Get quotes for one or more symbols",
		Args:    cobra.ArbitraryArgs,
		RunE:    defaultSubcommand(getCmd),
		GroupID: "market-data",
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
			if !validQuoteFields[lower] {
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
				cmd.Flags().Changed("call") ||
				cmd.Flags().Changed("put")

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
	cmd.MarkFlagsMutuallyExclusive("call", "put")

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
	if !cmd.Flags().Changed("call") && !cmd.Flags().Changed("put") {
		missing = append(missing, "--call or --put")
	}
	if len(missing) > 0 {
		return apperr.NewValidationError(
			"option quote requires: "+strings.Join(missing, ", "), nil)
	}
	return nil
}

// buildCobraQuoteParams constructs a QuoteParams from the validated option struct.
// Field validation has already been performed by quoteGetOpts.Validate().
func buildCobraQuoteParams(opts *quoteGetOpts) client.QuoteParams {
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
	return client.QuoteParams{
		Fields:     fields,
		Indicative: opts.Indicative,
	}
}

// sortedQuoteFieldNames returns the valid field names sorted alphabetically,
// joined as a comma-separated string for use in error messages.
func sortedQuoteFieldNames() string {
	names := make([]string, 0, len(validQuoteFields))
	for k := range validQuoteFields {
		names = append(names, k)
	}
	slices.Sort(names)
	return strings.Join(names, ", ")
}

// quoteSingle fetches and writes a quote for a single symbol.
// The result is wrapped in a symbol-keyed map so the response shape
// matches multi-symbol requests (data.SYMBOL.field). This lets agents
// parse quotes without branching on argument count. See issue #48.
func quoteSingle(ctx context.Context, c *client.Ref, w io.Writer, symbol string, p client.QuoteParams) error {
	quote, err := c.Quote(ctx, symbol, p)
	if err != nil {
		return err
	}
	return output.WriteSuccess(w, map[string]any{symbol: quote}, output.NewMetadata())
}

// quoteMulti fetches quotes for multiple symbols, using WritePartial when some are missing.
func quoteMulti(ctx context.Context, c *client.Ref, w io.Writer, symbols []string, p client.QuoteParams) error {
	quotes, err := c.Quotes(ctx, symbols, p)
	if err != nil {
		return err
	}

	// Identify symbols absent from the response.
	var missing []string
	for _, sym := range symbols {
		if _, ok := quotes[sym]; !ok {
			missing = append(missing, fmt.Sprintf("symbol %s not found", sym))
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
