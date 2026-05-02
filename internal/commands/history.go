// Package commands provides CLI command builders for schwab-agent.
package commands

import (
	"io"

	"github.com/spf13/cobra"

	"github.com/major/schwab-agent/internal/client"
	"github.com/major/schwab-agent/internal/models"
	"github.com/major/schwab-agent/internal/output"
)

// priceHistoryData wraps the price history response.
type priceHistoryData struct {
	PriceHistory *models.CandleList `json:"priceHistory"`
}

// historyGetOpts holds the options for the history get subcommand.
type historyGetOpts struct {
	PeriodType    string
	Period        string
	FrequencyType string
	Frequency     string
	From          string
	To            string
}

// NewHistoryCmd returns the Cobra command for price history lookups.
func NewHistoryCmd(c *client.Ref, w io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:        "history",
		Short:      "Retrieve price history for a symbol",
		SuggestFor: []string{"price-history"},
		GroupID:    "market-data",
		RunE:       requireSubcommand,
	}
	cmd.SetFlagErrorFunc(suggestSubcommands)

	cmd.AddCommand(newHistoryGetCmd(c, w))
	return cmd
}

// newHistoryGetCmd returns the Cobra subcommand for getting price history.
func newHistoryGetCmd(c *client.Ref, w io.Writer) *cobra.Command {
	opts := &historyGetOpts{}
	cmd := &cobra.Command{
		Use:   "get SYMBOL",
		Short: "Get price history candles for a symbol",
		Long: `Get price history candles for a symbol.

Examples:
  schwab-agent history get AAPL
  schwab-agent history get AAPL --period-type day --period 5 --frequency-type minute --frequency 15
  schwab-agent history get AAPL --from 1735689600000 --to 1743379200000`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			symbol := args[0]

			params := client.HistoryParams{
				PeriodType:    opts.PeriodType,
				Period:        opts.Period,
				FrequencyType: opts.FrequencyType,
				Frequency:     opts.Frequency,
				StartDate:     opts.From,
				EndDate:       opts.To,
			}

			result, err := c.PriceHistory(cmd.Context(), symbol, &params)
			if err != nil {
				return err
			}

			return output.WriteSuccess(w, priceHistoryData{PriceHistory: result}, output.NewMetadata())
		},
	}

	cmd.Flags().StringVar(&opts.PeriodType, "period-type", "", "Period type (day, month, year, ytd)")
	cmd.Flags().StringVar(&opts.Period, "period", "", "Number of periods")
	cmd.Flags().StringVar(&opts.FrequencyType, "frequency-type", "", "Frequency type (minute, daily, weekly, monthly)")
	cmd.Flags().StringVar(&opts.Frequency, "frequency", "", "Frequency value")
	cmd.Flags().StringVar(&opts.From, "from", "", "Start date (milliseconds since epoch)")
	cmd.Flags().StringVar(&opts.To, "to", "", "End date (milliseconds since epoch)")

	return cmd
}
