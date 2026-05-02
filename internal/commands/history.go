// Package commands provides CLI command builders for schwab-agent.
package commands

import (
	"context"
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

// NewHistoryCmd returns the Cobra command for price history lookups.
func NewHistoryCmd(c *client.Ref, w io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:        "history",
		Short:      "Retrieve price history for a symbol",
		SuggestFor: []string{"price-history"},
		GroupID:    "market-data",
		RunE:       requireSubcommand,
	}

	getCmd := &cobra.Command{
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
				PeriodType:    flagString(cmd, "period-type"),
				Period:        flagString(cmd, "period"),
				FrequencyType: flagString(cmd, "frequency-type"),
				Frequency:     flagString(cmd, "frequency"),
				StartDate:     flagString(cmd, "from"),
				EndDate:       flagString(cmd, "to"),
			}

			result, err := c.PriceHistory(context.Background(), symbol, &params)
			if err != nil {
				return err
			}

			return output.WriteSuccess(w, priceHistoryData{PriceHistory: result}, output.NewMetadata())
		},
	}

	getCmd.Flags().String("period-type", "", "Period type (day, month, year, ytd)")
	getCmd.Flags().String("period", "", "Number of periods")
	getCmd.Flags().String("frequency-type", "", "Frequency type (minute, daily, weekly, monthly)")
	getCmd.Flags().String("frequency", "", "Frequency value")
	getCmd.Flags().String("from", "", "Start date (milliseconds since epoch)")
	getCmd.Flags().String("to", "", "End date (milliseconds since epoch)")

	cmd.AddCommand(getCmd)
	return cmd
}
