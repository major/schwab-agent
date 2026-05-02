// Package commands provides CLI command definitions for schwab-agent.
package commands

import (
	"context"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	"github.com/urfave/cli/v3"

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

// QuoteCommand returns the CLI command for stock quote operations.
func QuoteCommand(c *client.Ref, w io.Writer) *cli.Command {
	return &cli.Command{
		Name:           "quote",
		Usage:          "Stock quote operations",
		UsageText:      "schwab-agent quote AAPL\nschwab-agent quote AAPL MSFT NVDA\nschwab-agent quote get AAPL",
		DefaultCommand: "get",
		Commands: []*cli.Command{
			quoteGetCommand(c, w),
		},
	}
}

// NewQuoteCmd returns the Cobra command for stock quote operations.
// The parent command requires a subcommand (no DefaultCommand behavior).
func NewQuoteCmd(c *client.Ref, w io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "quote",
		Short:   "Stock quote operations",
		Long:    "Get quotes for one or more symbols",
		RunE:    cobraRequireSubcommand,
		GroupID: "market-data",
	}

	cmd.AddCommand(newQuoteGetCmd(c, w))

	return cmd
}

// newQuoteGetCmd returns the Cobra subcommand for retrieving quotes.
func newQuoteGetCmd(c *client.Ref, w io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get quotes for one or more symbols",
		Long:  "Get quotes for one or more symbols with optional field filtering and indicative quotes",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			params, err := buildCobraQuoteParams(cmd)
			if err != nil {
				return err
			}

			if len(args) == 1 {
				return quoteSingle(cmd.Context(), c, w, args[0], params)
			}
			return quoteMulti(cmd.Context(), c, w, args, params)
		},
	}

	cmd.Flags().StringSlice("fields", nil, "Quote fields to return (repeatable): quote, fundamental, extended, reference, regular")
	cmd.Flags().Bool("indicative", false, "Request indicative (non-tradeable) quotes")

	return cmd
}

// buildCobraQuoteParams constructs a QuoteParams from Cobra flags, validating
// field values against the allowed set (case-insensitive).
func buildCobraQuoteParams(cmd *cobra.Command) (client.QuoteParams, error) {
	raw := flagStringSlice(cmd, "fields")
	fields := make([]string, 0, len(raw))
	for _, f := range raw {
		// Support both repeatable (--fields QUOTE --fields FUNDAMENTAL)
		// and comma-separated (--fields QUOTE,FUNDAMENTAL) forms.
		parts := strings.SplitSeq(f, ",")
		for part := range parts {
			trimmed := strings.TrimSpace(part)
			if trimmed == "" {
				continue
			}
			lower := strings.ToLower(trimmed)
			if !validQuoteFields[lower] {
				return client.QuoteParams{}, apperr.NewValidationError(
					fmt.Sprintf("invalid field %q: must be one of %s", trimmed, sortedQuoteFieldNames()),
					nil,
				)
			}
			fields = append(fields, lower)
		}
	}
	return client.QuoteParams{
		Fields:     fields,
		Indicative: flagBool(cmd, "indicative"),
	}, nil
}

// quoteGetCommand returns the subcommand for retrieving quotes.
func quoteGetCommand(c *client.Ref, w io.Writer) *cli.Command {
	return &cli.Command{
		Name:      "get",
		Usage:     "Get quotes for one or more symbols",
		ArgsUsage: "<symbol> [symbol...]",
		UsageText: `schwab-agent quote get AAPL
schwab-agent quote get AAPL MSFT NVDA
schwab-agent quote get AAPL --fields quote,fundamental --indicative`,
		Flags: []cli.Flag{
			&cli.StringSliceFlag{
				Name:  "fields",
				Usage: "Quote fields to return (repeatable): quote, fundamental, extended, reference, regular",
			},
			&cli.BoolFlag{
				Name:  "indicative",
				Usage: "Request indicative (non-tradeable) quotes",
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			args := cmd.Args().Slice()
			if len(args) == 0 {
				return apperr.NewValidationError("at least one symbol is required", nil)
			}

			params, err := buildQuoteParams(cmd)
			if err != nil {
				return err
			}

			if len(args) == 1 {
				return quoteSingle(ctx, c, w, args[0], params)
			}
			return quoteMulti(ctx, c, w, args, params)
		},
	}
}

// buildQuoteParams constructs a QuoteParams from CLI flags, validating
// field values against the allowed set (case-insensitive).
func buildQuoteParams(cmd *cli.Command) (client.QuoteParams, error) {
	raw := cmd.StringSlice("fields")
	fields := make([]string, 0, len(raw))
	for _, f := range raw {
		// Support both repeatable (--fields QUOTE --fields FUNDAMENTAL)
		// and comma-separated (--fields QUOTE,FUNDAMENTAL) forms.
		parts := strings.SplitSeq(f, ",")
		for part := range parts {
			trimmed := strings.TrimSpace(part)
			if trimmed == "" {
				continue
			}
			lower := strings.ToLower(trimmed)
			if !validQuoteFields[lower] {
				return client.QuoteParams{}, apperr.NewValidationError(
					fmt.Sprintf("invalid field %q: must be one of %s", trimmed, sortedQuoteFieldNames()),
					nil,
				)
			}
			fields = append(fields, lower)
		}
	}
	return client.QuoteParams{
		Fields:     fields,
		Indicative: cmd.Bool("indicative"),
	}, nil
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
