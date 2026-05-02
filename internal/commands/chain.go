package commands

import (
	"io"

	"github.com/spf13/cobra"

	"github.com/major/schwab-agent/internal/client"
	"github.com/major/schwab-agent/internal/output"
)

// NewChainCmd returns the Cobra command for option chain operations.
func NewChainCmd(c *client.Ref, w io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "chain",
		Short:   "Option chain operations",
		GroupID: "market-data",
		RunE:    requireSubcommand,
	}

	cmd.AddCommand(newChainGetCmd(c, w))
	cmd.AddCommand(newChainExpirationCmd(c, w))

	return cmd
}

// newChainGetCmd returns the Cobra subcommand for retrieving an option chain.
func newChainGetCmd(c *client.Ref, w io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "get <symbol>",
		Short:   "Get option chain for a symbol",
		Example: "schwab-agent chain get AAPL\nschwab-agent chain get AAPL --type CALL --strike-count 5\nschwab-agent chain get AAPL --from-date 2025-06-01 --to-date 2025-07-31 --type PUT",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return requireArg("", "symbol")
			}
			symbol := args[0]

			// Convert the bool flag to a string for the query param builder.
			// Only send "true" when explicitly set; omit otherwise so the API
			// uses its default behavior.
			includeUnderlying := ""
			if flagBool(cmd, "include-underlying-quote") {
				includeUnderlying = "true"
			}

			params := client.ChainParams{
				ContractType:           flagString(cmd, "type"),
				StrikeCount:            flagString(cmd, "strike-count"),
				Strategy:               flagString(cmd, "strategy"),
				FromDate:               flagString(cmd, "from-date"),
				ToDate:                 flagString(cmd, "to-date"),
				IncludeUnderlyingQuote: includeUnderlying,
				Interval:               flagString(cmd, "interval"),
				Strike:                 flagString(cmd, "strike"),
				StrikeRange:            flagString(cmd, "strike-range"),
				Volatility:             flagString(cmd, "volatility"),
				UnderlyingPrice:        flagString(cmd, "underlying-price"),
				InterestRate:           flagString(cmd, "interest-rate"),
				DaysToExpiration:       flagString(cmd, "days-to-expiration"),
			}

			chain, err := c.OptionChain(cmd.Context(), symbol, &params)
			if err != nil {
				return err
			}
			return output.WriteSuccess(w, chain, output.NewMetadata())
		},
	}

	cmd.Flags().String("type", "", "Contract type: CALL, PUT, or ALL")
	cmd.Flags().String("strike-count", "", "Number of strikes to return")
	cmd.Flags().String("strategy", "", "Option pricing strategy")
	cmd.Flags().String("from-date", "", "Start date (YYYY-MM-DD)")
	cmd.Flags().String("to-date", "", "End date (YYYY-MM-DD)")
	cmd.Flags().Bool("include-underlying-quote", false, "Include underlying quote data in response")
	cmd.Flags().String("interval", "", "Strike interval for spread strategy chains")
	cmd.Flags().String("strike", "", "Filter to a specific strike price")
	cmd.Flags().String("strike-range", "", "Moneyness filter: ITM, NTM, OTM, SAK, SBK, SNK, or ALL")
	cmd.Flags().String("volatility", "", "Volatility for theoretical pricing calculations")
	cmd.Flags().String("underlying-price", "", "Override underlying price for theoretical calculations")
	cmd.Flags().String("interest-rate", "", "Interest rate for theoretical pricing calculations")
	cmd.Flags().String("days-to-expiration", "", "Days to expiration for theoretical pricing calculations")

	return cmd
}

// newChainExpirationCmd returns the Cobra subcommand for retrieving option expiration dates.
func newChainExpirationCmd(c *client.Ref, w io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:     "expiration <symbol>",
		Short:   "Get expiration dates for a symbol",
		Example: "schwab-agent chain expiration AAPL",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return requireArg("", "symbol")
			}
			symbol := args[0]

			chain, err := c.ExpirationChainForSymbol(cmd.Context(), symbol)
			if err != nil {
				return err
			}
			return output.WriteSuccess(w, chain, output.NewMetadata())
		},
	}
}
