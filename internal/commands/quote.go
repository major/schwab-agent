// Package commands provides CLI command definitions for schwab-agent.
package commands

import (
	"context"
	"fmt"
	"io"

	"github.com/urfave/cli/v3"

	"github.com/major/schwab-agent/internal/client"
	schwabErrors "github.com/major/schwab-agent/internal/errors"
	"github.com/major/schwab-agent/internal/output"
)

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
		Action: func(ctx context.Context, cmd *cli.Command) error {
			args := cmd.Args().Slice()
			if len(args) == 0 {
				return schwabErrors.NewValidationError("at least one symbol is required", nil)
			}

			if len(args) == 1 {
				return quoteSingle(ctx, c, w, args[0])
			}
			return quoteMulti(ctx, c, w, args)
		},
	}
}

// quoteSingle fetches and writes a quote for a single symbol.
func quoteSingle(ctx context.Context, c *client.Ref, w io.Writer, symbol string) error {
	quote, err := c.Quote(ctx, symbol)
	if err != nil {
		return err
	}
	return output.WriteSuccess(w, quote, output.NewMetadata())
}

// quoteMulti fetches quotes for multiple symbols, using WritePartial when some are missing.
func quoteMulti(ctx context.Context, c *client.Ref, w io.Writer, symbols []string) error {
	quotes, err := c.Quotes(ctx, symbols)
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
