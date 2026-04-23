// Package commands provides CLI command definitions for schwab-agent.
package commands

import (
	"context"
	"fmt"
	"io"
	"slices"
	"strings"

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
		Name:  "quote",
		Usage: "Stock quote operations",
		Commands: []*cli.Command{
			quoteGetCommand(c, w),
		},
	}
}

// quoteGetCommand returns the subcommand for retrieving quotes.
func quoteGetCommand(c *client.Ref, w io.Writer) *cli.Command {
	return &cli.Command{
		Name:      "get",
		Usage:     "Get quotes for one or more symbols",
		ArgsUsage: "<symbol> [symbol...]",
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
		parts := strings.Split(f, ",")
		for _, part := range parts {
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
func quoteSingle(ctx context.Context, c *client.Ref, w io.Writer, symbol string, p client.QuoteParams) error {
	quote, err := c.Quote(ctx, symbol, p)
	if err != nil {
		return err
	}
	return output.WriteSuccess(w, quote, output.NewMetadata())
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
