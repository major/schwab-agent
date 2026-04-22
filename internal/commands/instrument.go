package commands

import (
	"context"
	"io"

	"github.com/urfave/cli/v3"

	"github.com/major/schwab-agent/internal/client"
	"github.com/major/schwab-agent/internal/output"
)

// InstrumentCommand returns the CLI command for instrument search and lookup.
func InstrumentCommand(c *client.Ref, w io.Writer) *cli.Command {
	return &cli.Command{
		Name:  "instrument",
		Usage: "Search and look up instruments",
		Commands: []*cli.Command{
			{
				Name:  "search",
				Usage: "Search instruments by symbol or description",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "projection",
						Usage: "Search projection (symbol-search, symbol-regex, desc-search, desc-regex, search, fundamental)",
						Value: "symbol-search",
					},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					query := cmd.Args().First()
					if err := requireArg(query, "search query"); err != nil {
						return err
					}

					projection := cmd.String("projection")
					result, err := c.SearchInstruments(ctx, query, projection)
					if err != nil {
						return err
					}

					return output.WriteSuccess(w, map[string]any{"instruments": result}, output.TimestampMeta())
				},
			},
			{
				Name:  "get",
				Usage: "Get instrument details by CUSIP",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					cusip := cmd.Args().First()
					if err := requireArg(cusip, "CUSIP"); err != nil {
						return err
					}

					result, err := c.GetInstrument(ctx, cusip)
					if err != nil {
						return err
					}

					return output.WriteSuccess(w, map[string]any{"instrument": result}, output.TimestampMeta())
				},
			},
		},
	}
}
