package commands

import (
	"io"

	"github.com/spf13/cobra"

	"github.com/major/schwab-agent/internal/client"
	"github.com/major/schwab-agent/internal/models"
	"github.com/major/schwab-agent/internal/output"
)

// allMarkets returns the full list of Schwab market types, used as the default
// when no specific markets are requested.
func allMarkets() []string {
	return []string{"equity", "option", "bond", "future", "forex"}
}

// moversData wraps the movers response.
type moversData struct {
	Movers *models.ScreenerResponse `json:"movers"`
}

// marketMoversOpts holds the options for the market movers subcommand.
type marketMoversOpts struct {
	Sort      moversSort      `flag:"sort"      flagdescr:"Sort order (VOLUME, TRADES, PERCENT_CHANGE_UP, PERCENT_CHANGE_DOWN)"`
	Frequency moversFrequency `flag:"frequency" flagdescr:"Minimum percent change magnitude (0, 1, 5, 10, 30, 60)"`
}

// NewMarketCmd returns the Cobra command for market hours and movers lookups.
func NewMarketCmd(c *client.Ref, w io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "market",
		Short:   "Market hours and top movers",
		GroupID: "market-data",
		RunE:    requireSubcommand,
	}
	cmd.SetFlagErrorFunc(suggestSubcommands)

	cmd.AddCommand(
		newMarketHoursCmd(c, w),
		newMarketMoversCmd(c, w),
	)

	return cmd
}

// newMarketHoursCmd returns the Cobra subcommand for market hours.
func newMarketHoursCmd(c *client.Ref, w io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "hours [market ...]",
		Short: "Get market hours for one or more markets (equity, option, bond, future, forex)",
		Long: `Get current market hours for one or more markets. With no arguments, returns
hours for all markets (equity, option, bond, future, forex). Specify one or
more market names to filter.`,
		Example: `  schwab-agent market hours
  schwab-agent market hours equity
  schwab-agent market hours equity option`,
		RunE: func(cmd *cobra.Command, args []string) error {
			markets := args
			if len(markets) == 0 {
				markets = allMarkets()
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
	opts := &marketMoversOpts{}
	cmd := &cobra.Command{
		Use:   "movers <index>",
		Short: "Get top movers for an index ($SPX, $DJI, $COMPX, NYSE, NASDAQ, OTCBB, INDEX_ALL, EQUITY_ALL, OPTION_ALL, OPTION_PUT, OPTION_CALL)",
		Long: `Get top movers for a market index. Sort by VOLUME, TRADES, PERCENT_CHANGE_UP,
or PERCENT_CHANGE_DOWN. Use --frequency to filter by minimum percent change
magnitude (0, 1, 5, 10, 30, 60). Quote shell-special characters in index
names (e.g. '$SPX').`,
		Example: `  schwab-agent market movers '$SPX' --sort PERCENT_CHANGE_UP
  schwab-agent market movers '$DJI' --sort VOLUME --frequency 5
  schwab-agent market movers EQUITY_ALL --sort PERCENT_CHANGE_UP`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateCobraOptions(cmd.Context(), opts); err != nil {
				return err
			}

			index := ""
			if len(args) > 0 {
				index = args[0]
			}
			if err := requireArg(index, "index"); err != nil {
				return err
			}

			params := client.MoversParams{
				Sort:      string(opts.Sort),
				Frequency: string(opts.Frequency),
			}

			result, err := c.Movers(cmd.Context(), index, params)
			if err != nil {
				return err
			}

			return output.WriteSuccess(w, moversData{Movers: result}, output.NewMetadata())
		},
	}

	defineCobraFlags(cmd, opts)
	cmd.SetFlagErrorFunc(normalizeFlagValidationErrorFunc)

	return cmd
}
