package commands

import (
	"io"

	"github.com/leodido/structcli"
	"github.com/spf13/cobra"

	"github.com/major/schwab-agent/internal/client"
	"github.com/major/schwab-agent/internal/output"
)

// chainGetOpts holds the options for the chain get subcommand.
type chainGetOpts struct {
	Type                   string `flag:"type" flagdescr:"Contract type: CALL, PUT, or ALL" flaggroup:"contract"`
	StrikeCount            string `flag:"strike-count" flagdescr:"Number of strikes to return" flaggroup:"filtering"`
	Strategy               string `flag:"strategy" flagdescr:"Option pricing strategy" flaggroup:"contract"`
	FromDate               string `flag:"from-date" flagdescr:"Start date (YYYY-MM-DD)" flaggroup:"filtering"`
	ToDate                 string `flag:"to-date" flagdescr:"End date (YYYY-MM-DD)" flaggroup:"filtering"`
	IncludeUnderlyingQuote bool   `flag:"include-underlying-quote" flagdescr:"Include underlying quote data in response" flaggroup:"filtering"`
	Interval               string `flag:"interval" flagdescr:"Strike interval for spread strategy chains" flaggroup:"filtering"`
	Strike                 string `flag:"strike" flagdescr:"Filter to a specific strike price" flaggroup:"filtering"`
	StrikeRange            string `flag:"strike-range" flagdescr:"Moneyness filter: ITM, NTM, OTM, SAK, SBK, SNK, or ALL" flaggroup:"filtering"`
	Volatility             string `flag:"volatility" flagdescr:"Volatility for theoretical pricing calculations" flaggroup:"pricing"`
	UnderlyingPrice        string `flag:"underlying-price" flagdescr:"Override underlying price for theoretical calculations" flaggroup:"pricing"`
	InterestRate           string `flag:"interest-rate" flagdescr:"Interest rate for theoretical pricing calculations" flaggroup:"pricing"`
	DaysToExpiration       string `flag:"days-to-expiration" flagdescr:"Days to expiration for theoretical pricing calculations" flaggroup:"pricing"`
}

// Attach implements structcli.Options interface.
func (o *chainGetOpts) Attach(_ *cobra.Command) error { return nil }

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
		Use:   "get <symbol>",
		Short: "Get option chain for a symbol",
		Long: `Get the full option chain for a symbol with optional filtering by contract type,
strike range, expiration dates, and strategy. Use --strategy ANALYTICAL with
--volatility and --days-to-expiration for theoretical pricing. Filter to
specific moneyness with --strike-range (ITM, NTM, OTM, ALL).`,
		Example: `  schwab-agent chain get AAPL
  schwab-agent chain get AAPL --type CALL --strike-count 5
  schwab-agent chain get AAPL --from-date 2025-06-01 --to-date 2025-07-31 --type PUT
  schwab-agent chain get AAPL --strategy ANALYTICAL --volatility 30.5 --days-to-expiration 45
  schwab-agent chain get AAPL --strike-range NTM --include-underlying-quote`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := structcli.Unmarshal(cmd, opts); err != nil {
				return err
			}

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

	if err := structcli.Define(cmd, opts); err != nil {
		panic(err)
	}

	return cmd
}

// newChainExpirationCmd returns the Cobra subcommand for retrieving option expiration dates.
func newChainExpirationCmd(c *client.Ref, w io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "expiration <symbol>",
		Short: "Get expiration dates for a symbol",
		Long: `Get available option expiration dates for a symbol without fetching the full
chain data. Useful for discovering valid expirations before requesting a
detailed chain.`,
		Example: "  schwab-agent chain expiration AAPL",
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
