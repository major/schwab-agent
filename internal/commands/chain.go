package commands

import (
	"io"

	"github.com/spf13/cobra"

	"github.com/major/schwab-agent/internal/client"
	"github.com/major/schwab-agent/internal/output"
)

// chainGetOpts holds the options for the chain get subcommand.
type chainGetOpts struct {
	Type                  string
	StrikeCount           string
	Strategy              string
	FromDate              string
	ToDate                string
	IncludeUnderlyingQuote bool
	Interval              string
	Strike                string
	StrikeRange           string
	Volatility            string
	UnderlyingPrice       string
	InterestRate          string
	DaysToExpiration      string
}

// NewChainCmd returns the Cobra command for option chain operations.
func NewChainCmd(c *client.Ref, w io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "chain",
		Short:   "Option chain operations",
		GroupID: "market-data",
		RunE:    requireSubcommand,
	}
	cmd.SetFlagErrorFunc(suggestSubcommands)

	cmd.AddCommand(newChainGetCmd(c, w))
	cmd.AddCommand(newChainExpirationCmd(c, w))

	return cmd
}

// newChainGetCmd returns the Cobra subcommand for retrieving an option chain.
func newChainGetCmd(c *client.Ref, w io.Writer) *cobra.Command {
	opts := &chainGetOpts{}
	cmd := &cobra.Command{
		Use:     "get <symbol>",
		Short:   "Get option chain for a symbol",
		Example: "schwab-agent chain get AAPL\nschwab-agent chain get AAPL --type CALL --strike-count 5\nschwab-agent chain get AAPL --from-date 2025-06-01 --to-date 2025-07-31 --type PUT",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			symbol := args[0]

			// Convert the bool flag to a string for the query param builder.
			// Only send "true" when explicitly set; omit otherwise so the API
			// uses its default behavior.
			includeUnderlying := ""
			if opts.IncludeUnderlyingQuote {
				includeUnderlying = "true"
			}

			params := client.ChainParams{
				ContractType:           opts.Type,
				StrikeCount:            opts.StrikeCount,
				Strategy:               opts.Strategy,
				FromDate:               opts.FromDate,
				ToDate:                 opts.ToDate,
				IncludeUnderlyingQuote: includeUnderlying,
				Interval:               opts.Interval,
				Strike:                 opts.Strike,
				StrikeRange:            opts.StrikeRange,
				Volatility:             opts.Volatility,
				UnderlyingPrice:        opts.UnderlyingPrice,
				InterestRate:           opts.InterestRate,
				DaysToExpiration:       opts.DaysToExpiration,
			}

			chain, err := c.OptionChain(cmd.Context(), symbol, &params)
			if err != nil {
				return err
			}
			return output.WriteSuccess(w, chain, output.NewMetadata())
		},
	}

	cmd.Flags().StringVar(&opts.Type, "type", "", "Contract type: CALL, PUT, or ALL")
	cmd.Flags().StringVar(&opts.StrikeCount, "strike-count", "", "Number of strikes to return")
	cmd.Flags().StringVar(&opts.Strategy, "strategy", "", "Option pricing strategy")
	cmd.Flags().StringVar(&opts.FromDate, "from-date", "", "Start date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&opts.ToDate, "to-date", "", "End date (YYYY-MM-DD)")
	cmd.Flags().BoolVar(&opts.IncludeUnderlyingQuote, "include-underlying-quote", false, "Include underlying quote data in response")
	cmd.Flags().StringVar(&opts.Interval, "interval", "", "Strike interval for spread strategy chains")
	cmd.Flags().StringVar(&opts.Strike, "strike", "", "Filter to a specific strike price")
	cmd.Flags().StringVar(&opts.StrikeRange, "strike-range", "", "Moneyness filter: ITM, NTM, OTM, SAK, SBK, SNK, or ALL")
	cmd.Flags().StringVar(&opts.Volatility, "volatility", "", "Volatility for theoretical pricing calculations")
	cmd.Flags().StringVar(&opts.UnderlyingPrice, "underlying-price", "", "Override underlying price for theoretical calculations")
	cmd.Flags().StringVar(&opts.InterestRate, "interest-rate", "", "Interest rate for theoretical pricing calculations")
	cmd.Flags().StringVar(&opts.DaysToExpiration, "days-to-expiration", "", "Days to expiration for theoretical pricing calculations")

	return cmd
}

// newChainExpirationCmd returns the Cobra subcommand for retrieving option expiration dates.
func newChainExpirationCmd(c *client.Ref, w io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:     "expiration <symbol>",
		Short:   "Get expiration dates for a symbol",
		Example: "schwab-agent chain expiration AAPL",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			symbol := args[0]

			chain, err := c.ExpirationChainForSymbol(cmd.Context(), symbol)
			if err != nil {
				return err
			}
			return output.WriteSuccess(w, chain, output.NewMetadata())
		},
	}
}
