package commands

import (
	"io"

	"github.com/leodido/structcli"
	"github.com/spf13/cobra"

	"github.com/major/schwab-agent/internal/client"
)

// NewIndicatorsCmd returns a top-level shortcut command that produces the same
// output as "ta dashboard". It saves agents a subcommand hop for the most
// common TA workflow: getting an opinionated indicator snapshot for one or more
// symbols.
func NewIndicatorsCmd(c *client.Ref, w io.Writer) *cobra.Command {
	opts := &dashboardOpts{}
	cmd := &cobra.Command{
		Use:   "indicators SYMBOL [SYMBOL...]",
		Short: "Technical indicators dashboard (shortcut for ta dashboard)",
		Long: `Shortcut for "ta dashboard". Computes an opinionated technical-analysis
dashboard for one or more symbols with one price-history fetch per symbol.
Includes SMA 21/50/200, RSI 14, MACD 12/26/9, ATR 14, Bollinger Bands 20/2,
volume context, and 20/252-candle high-low ranges.`,
		Example: `  schwab-agent indicators AAPL
  schwab-agent indicators AAPL --points 5
  schwab-agent indicators NVDA --interval weekly --points 10
  schwab-agent indicators AAPL MSFT NVDA`,
		GroupID: "market-data",
		Args:   cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := structcli.Unmarshal(cmd, opts); err != nil {
				return err
			}

			return writeTASymbolResults(w, args, func(symbol string) (any, error) {
				return buildTADashboard(cmd.Context(), c, symbol, string(opts.Interval), opts.Points)
			})
		},
	}

	if err := structcli.Define(cmd, opts); err != nil {
		panic(err)
	}
	cmd.SetFlagErrorFunc(normalizeFlagValidationErrorFunc)

	return cmd
}
