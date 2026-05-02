package commands

import (
	"context"
	"io"

	"github.com/spf13/cobra"
	"github.com/urfave/cli/v3"

	"github.com/major/schwab-agent/internal/client"
	"github.com/major/schwab-agent/internal/models"
	"github.com/major/schwab-agent/internal/output"
)

// allMarkets is the full list of Schwab market types, used as the default
// when no specific markets are requested.
var allMarkets = []string{"equity", "option", "bond", "future", "forex"}

// moversData wraps the movers response.
type moversData struct {
	Movers *models.ScreenerResponse `json:"movers"`
}

// MarketCommand returns the CLI command for market hours and movers lookups.
func MarketCommand(c *client.Ref, w io.Writer) *cli.Command {
	return &cli.Command{
		Name:   "market",
		Usage:  "Market hours and top movers",
		Action: requireSubcommand(),
		Commands: []*cli.Command{
			{
				Name:      "hours",
				Usage:     "Get market hours for one or more markets (equity, option, bond, future, forex)",
				ArgsUsage: "[market ...]",
				UsageText: `schwab-agent market hours
schwab-agent market hours equity option`,
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
						return output.WriteSuccess(w, result, output.NewMetadata())
					}

					result, err := c.Markets(ctx, markets)
					if err != nil {
						return err
					}
					return output.WriteSuccess(w, result, output.NewMetadata())
				},
			},
			{
				Name: "movers",
				// Valid index symbols from the Schwab API (not thinkorswim-style).
				// The API returns a 400 with the full list if you pass an invalid one.
				Usage: "Get top movers for an index ($SPX, $DJI, $COMPX, NYSE, NASDAQ, OTCBB, INDEX_ALL, EQUITY_ALL, OPTION_ALL, OPTION_PUT, OPTION_CALL)",
				UsageText: `schwab-agent market movers '$SPX' --sort PERCENT_CHANGE_UP
schwab-agent market movers '$DJI' --sort VOLUME --frequency 5`,
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "sort", Usage: "Sort order (VOLUME, TRADES, PERCENT_CHANGE_UP, PERCENT_CHANGE_DOWN)"},
					&cli.StringFlag{Name: "frequency", Usage: "Minimum percent change magnitude (0, 1, 5, 10, 30, 60)"},
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

					return output.WriteSuccess(w, moversData{Movers: result}, output.NewMetadata())
				},
			},
		},
	}
}

// NewMarketCmd returns the Cobra command for market hours and movers lookups.
func NewMarketCmd(c *client.Ref, w io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "market",
		Short:   "Market hours and top movers",
		GroupID: "market-data",
		RunE:    cobraRequireSubcommand,
	}

	cmd.AddCommand(
		newMarketHoursCmd(c, w),
		newMarketMoversCmd(c, w),
	)

	return cmd
}

// newMarketHoursCmd returns the Cobra subcommand for market hours.
func newMarketHoursCmd(c *client.Ref, w io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:     "hours [market ...]",
		Short:   "Get market hours for one or more markets (equity, option, bond, future, forex)",
		Example: "schwab-agent market hours\nschwab-agent market hours equity option",
		RunE: func(cmd *cobra.Command, args []string) error {
			markets := args
			if len(markets) == 0 {
				markets = allMarkets
			}

			// Use the single-market endpoint when exactly one market
			// is requested, and the multi-market endpoint otherwise.
			if len(markets) == 1 {
				result, err := c.Market(cmd.Context(), markets[0])
				if err != nil {
					return err
				}
				return output.WriteSuccess(w, result, output.NewMetadata())
			}

			result, err := c.Markets(cmd.Context(), markets)
			if err != nil {
				return err
			}
			return output.WriteSuccess(w, result, output.NewMetadata())
		},
	}
}

// newMarketMoversCmd returns the Cobra subcommand for market movers.
func newMarketMoversCmd(c *client.Ref, w io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "movers <index>",
		Short:   "Get top movers for an index ($SPX, $DJI, $COMPX, NYSE, NASDAQ, OTCBB, INDEX_ALL, EQUITY_ALL, OPTION_ALL, OPTION_PUT, OPTION_CALL)",
		Example: "schwab-agent market movers '$SPX' --sort PERCENT_CHANGE_UP\nschwab-agent market movers '$DJI' --sort VOLUME --frequency 5",
		RunE: func(cmd *cobra.Command, args []string) error {
			index := ""
			if len(args) > 0 {
				index = args[0]
			}
			if err := requireArg(index, "index"); err != nil {
				return err
			}

			params := client.MoversParams{
				Sort:      flagString(cmd, "sort"),
				Frequency: flagString(cmd, "frequency"),
			}

			result, err := c.Movers(cmd.Context(), index, params)
			if err != nil {
				return err
			}

			return output.WriteSuccess(w, moversData{Movers: result}, output.NewMetadata())
		},
	}

	cmd.Flags().String("sort", "", "Sort order (VOLUME, TRADES, PERCENT_CHANGE_UP, PERCENT_CHANGE_DOWN)")
	cmd.Flags().String("frequency", "", "Minimum percent change magnitude (0, 1, 5, 10, 30, 60)")

	return cmd
}
