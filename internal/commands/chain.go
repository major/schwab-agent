package commands

import (
	"context"
	"io"

	"github.com/urfave/cli/v3"

	"github.com/major/schwab-agent/internal/client"
	"github.com/major/schwab-agent/internal/output"
)

// ChainCommand returns the CLI command for option chain operations.
func ChainCommand(c *client.Ref, w io.Writer) *cli.Command {
	return &cli.Command{
		Name:  "chain",
		Usage: "Option chain operations",
		Commands: []*cli.Command{
			chainGetCommand(c, w),
			chainExpirationCommand(c, w),
		},
	}
}

// chainGetCommand returns the subcommand for retrieving an option chain.
func chainGetCommand(c *client.Ref, w io.Writer) *cli.Command {
	return &cli.Command{
		Name:      "get",
		Usage:     "Get option chain for a symbol",
		ArgsUsage: "<symbol>",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "type",
				Usage: "Contract type: CALL, PUT, or ALL",
			},
			&cli.StringFlag{
				Name:  "strike-count",
				Usage: "Number of strikes to return",
			},
			&cli.StringFlag{
				Name:  "strategy",
				Usage: "Option pricing strategy",
			},
			&cli.StringFlag{
				Name:  "from-date",
				Usage: "Start date (YYYY-MM-DD)",
			},
			&cli.StringFlag{
				Name:  "to-date",
				Usage: "End date (YYYY-MM-DD)",
			},
			&cli.BoolFlag{
				Name:  "include-underlying-quote",
				Usage: "Include underlying quote data in response",
			},
			&cli.StringFlag{
				Name:  "interval",
				Usage: "Strike interval for spread strategy chains",
			},
			&cli.StringFlag{
				Name:  "strike",
				Usage: "Filter to a specific strike price",
			},
			&cli.StringFlag{
				Name:  "strike-range",
				Usage: "Moneyness filter: ITM, NTM, OTM, SAK, SBK, SNK, or ALL",
			},
			&cli.StringFlag{
				Name:  "volatility",
				Usage: "Volatility for theoretical pricing calculations",
			},
			&cli.StringFlag{
				Name:  "underlying-price",
				Usage: "Override underlying price for theoretical calculations",
			},
			&cli.StringFlag{
				Name:  "interest-rate",
				Usage: "Interest rate for theoretical pricing calculations",
			},
			&cli.StringFlag{
				Name:  "days-to-expiration",
				Usage: "Days to expiration for theoretical pricing calculations",
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			symbol := cmd.Args().First()
			if err := requireArg(symbol, "symbol"); err != nil {
				return err
			}

			// Convert the bool flag to a string for the query param builder.
			// Only send "true" when explicitly set; omit otherwise so the API
			// uses its default behavior.
			includeUnderlying := ""
			if cmd.Bool("include-underlying-quote") {
				includeUnderlying = "true"
			}

			params := client.ChainParams{
				ContractType:           cmd.String("type"),
				StrikeCount:            cmd.String("strike-count"),
				Strategy:               cmd.String("strategy"),
				FromDate:               cmd.String("from-date"),
				ToDate:                 cmd.String("to-date"),
				IncludeUnderlyingQuote: includeUnderlying,
				Interval:               cmd.String("interval"),
				Strike:                 cmd.String("strike"),
				StrikeRange:            cmd.String("strike-range"),
				Volatility:             cmd.String("volatility"),
				UnderlyingPrice:        cmd.String("underlying-price"),
				InterestRate:           cmd.String("interest-rate"),
				DaysToExpiration:       cmd.String("days-to-expiration"),
			}

			chain, err := c.OptionChain(ctx, symbol, &params)
			if err != nil {
				return err
			}
			return output.WriteSuccess(w, chain, output.TimestampMeta())
		},
	}
}

// chainExpirationCommand returns the subcommand for retrieving option expiration dates.
func chainExpirationCommand(c *client.Ref, w io.Writer) *cli.Command {
	return &cli.Command{
		Name:      "expiration",
		Usage:     "Get expiration dates for a symbol",
		ArgsUsage: "<symbol>",
		Action: func(ctx context.Context, cmd *cli.Command) error {
			symbol := cmd.Args().First()
			if err := requireArg(symbol, "symbol"); err != nil {
				return err
			}

			chain, err := c.ExpirationChainForSymbol(ctx, symbol)
			if err != nil {
				return err
			}
			return output.WriteSuccess(w, chain, output.TimestampMeta())
		},
	}
}
