// Package commands provides CLI command builders for schwab-agent.
package commands

import (
	"io"

	"github.com/leodido/structcli"
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
	PeriodType    historyPeriodType    `flag:"period-type" flagdescr:"Period type (day, month, year, ytd)" flaggroup:"period"`
	Period        string               `flag:"period" flagdescr:"Number of periods" flaggroup:"period"`
	FrequencyType historyFrequencyType `flag:"frequency-type" flagdescr:"Frequency type (minute, daily, weekly, monthly)" flaggroup:"frequency"`
	Frequency     historyFrequency     `flag:"frequency" flagdescr:"Frequency value" flaggroup:"frequency"`
	From          string               `flag:"from" flagdescr:"Start date (milliseconds since epoch)" flaggroup:"range"`
	To            string               `flag:"to" flagdescr:"End date (milliseconds since epoch)" flaggroup:"range"`
}

// Attach implements structcli.Options interface.
func (o *historyGetOpts) Attach(_ *cobra.Command) error { return nil }

// NewHistoryCmd returns the Cobra command for price history lookups.
func NewHistoryCmd(c *client.Ref, w io.Writer) *cobra.Command {
	getCmd := newHistoryGetCmd(c, w)
	cmd := &cobra.Command{
		Use:        "history",
		Short:      "Retrieve price history for a symbol",
		Aliases:    []string{"price-history"},
		SuggestFor: []string{"price-history"},
		GroupID:    "market-data",
		Args:       cobra.ArbitraryArgs,
		RunE:       defaultSubcommand(getCmd),
	}
	cmd.SetFlagErrorFunc(suggestSubcommands)

	cmd.AddCommand(getCmd)
	return cmd
}

// newHistoryGetCmd returns the Cobra subcommand for getting price history.
func newHistoryGetCmd(c *client.Ref, w io.Writer) *cobra.Command {
	opts := &historyGetOpts{}
	cmd := &cobra.Command{
		Use:   "get SYMBOL",
		Short: "Get price history candles for a symbol",
		Long: `Get price history candles for a symbol. Supports configurable period types (day,
month, year, ytd), frequency types (minute, daily, weekly, monthly), and
date ranges via epoch milliseconds. Returns OHLCV candle data.`,
		Example: `  schwab-agent history get AAPL
  schwab-agent history get AAPL --period-type month --period 3 --frequency-type daily --frequency 1
  schwab-agent history get AAPL --period-type day --period 5 --frequency-type minute --frequency 15
  schwab-agent history get AAPL --from 1735689600000 --to 1743379200000`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := structcli.Unmarshal(cmd, opts); err != nil {
				return err
			}

			symbol := args[0]

			params := client.HistoryParams{
				PeriodType:    string(opts.PeriodType),
				Period:        opts.Period,
				FrequencyType: string(opts.FrequencyType),
				Frequency:     string(opts.Frequency),
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

	if err := structcli.Define(cmd, opts); err != nil {
		panic(err)
	}
	cmd.SetFlagErrorFunc(normalizeFlagValidationErrorFunc)

	return cmd
}
