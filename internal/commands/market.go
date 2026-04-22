package commands

import (
	"context"
	"io"

	"github.com/urfave/cli/v3"

	"github.com/major/schwab-agent/internal/client"
	"github.com/major/schwab-agent/internal/output"
)

// allMarkets is the full list of Schwab market types, used as the default
// when no specific markets are requested.
var allMarkets = []string{"equity", "option", "bond", "future", "forex"}

// MarketCommand returns the CLI command for market hours and movers lookups.
func MarketCommand(c *client.Client, w io.Writer) *cli.Command {
	return &cli.Command{
		Name:  "market",
		Usage: "Market hours and top movers",
		Commands: []*cli.Command{
			{
				Name:      "hours",
				Usage:     "Get market hours for one or more markets (equity, option, bond, future, forex)",
				ArgsUsage: "[market ...]",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					markets := cmd.Args().Slice()
					if len(markets) == 0 {
						markets = allMarkets
					}

					// Use the single-market endpoint when exactly one market
					// is requested, and the multi-market endpoint otherwise.
					if len(markets) == 1 {
						result, err := c.Market(ctx, markets[0])
						if err != nil {
							return err
						}
						return output.WriteSuccess(w, result, output.TimestampMeta())
					}

					result, err := c.Markets(ctx, markets)
					if err != nil {
						return err
					}
					return output.WriteSuccess(w, result, output.TimestampMeta())
				},
			},
			{
				Name:  "movers",
				Usage: "Get top movers for an index (e.g. $SPX.X, $DJI, $COMPX)",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "sort", Usage: "Sort order (VOLUME, TRADES, PERCENT_CHANGE_UP, PERCENT_CHANGE_DOWN)"},
					&cli.StringFlag{Name: "frequency", Usage: "Frequency (0, 1, 5, 10, 30, 60)"},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					index := cmd.Args().First()
				if err := requireArg(index, "index"); err != nil {
					return err
				}

					params := client.MoversParams{
						Sort:      cmd.String("sort"),
						Frequency: cmd.String("frequency"),
					}

					result, err := c.Movers(ctx, index, params)
					if err != nil {
						return err
					}

					return output.WriteSuccess(w, map[string]any{"movers": result}, output.TimestampMeta())
				},
			},
		},
	}
}
