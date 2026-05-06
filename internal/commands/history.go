package commands

import (
	"fmt"
	"io"
	"strconv"

	"github.com/major/schwab-go/schwab/marketdata"
	"github.com/spf13/cobra"

	"github.com/major/schwab-agent/internal/apperr"
	"github.com/major/schwab-agent/internal/client"
	"github.com/major/schwab-agent/internal/output"
)

// priceHistoryData wraps the price history response.
type priceHistoryData struct {
	PriceHistory *marketdata.CandleList `json:"priceHistory"`
}

// historyGetOpts holds the options for the history get subcommand.
type historyGetOpts struct {
	PeriodType    historyPeriodType    `flag:"period-type"    flagdescr:"Period type (day, month, year, ytd)"             flaggroup:"period"`
	Period        string               `flag:"period"         flagdescr:"Number of periods"                               flaggroup:"period"`
	FrequencyType historyFrequencyType `flag:"frequency-type" flagdescr:"Frequency type (minute, daily, weekly, monthly)" flaggroup:"frequency"`
	Frequency     historyFrequency     `flag:"frequency"      flagdescr:"Frequency value"                                 flaggroup:"frequency"`
	From          string               `flag:"from"           flagdescr:"Start date (milliseconds since epoch)"           flaggroup:"range"`
	To            string               `flag:"to"             flagdescr:"End date (milliseconds since epoch)"             flaggroup:"range"`
}

// priceHistoryParamsInput keeps Cobra's string-backed CLI flags at the command
// boundary while letting schwab-go own the typed request structure. The Schwab
// API represents absent numeric query params by omission, so zero values in the
// returned marketdata.PriceHistoryParams intentionally mean "do not send".
type priceHistoryParamsInput struct {
	PeriodType    string
	Period        string
	FrequencyType string
	Frequency     string
	StartDate     string
	EndDate       string
}

// NewHistoryCmd returns the Cobra command for price history lookups.
func NewHistoryCmd(c *client.Ref, w io.Writer) *cobra.Command {
	getCmd := newHistoryGetCmd(c, w)
	cmd := &cobra.Command{
		Use:        "history",
		Short:      "Retrieve price history for a symbol",
		Aliases:    []string{"price-history"},
		SuggestFor: []string{"price-history"},
		GroupID:    groupIDMarketData,
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
			if err := validateCobraOptions(cmd.Context(), opts); err != nil {
				return err
			}

			symbol := args[0]
			params, err := newPriceHistoryParams(priceHistoryParamsInput{
				PeriodType:    string(opts.PeriodType),
				Period:        opts.Period,
				FrequencyType: string(opts.FrequencyType),
				Frequency:     string(opts.Frequency),
				StartDate:     opts.From,
				EndDate:       opts.To,
			})
			if err != nil {
				return err
			}

			result, err := c.MarketData.GetPriceHistory(cmd.Context(), symbol, params)
			if err != nil {
				return mapSchwabGoError(err)
			}

			return output.WriteSuccess(w, priceHistoryData{PriceHistory: result}, output.NewMetadata())
		},
	}

	defineCobraFlags(cmd, opts)
	cmd.SetFlagErrorFunc(normalizeFlagValidationErrorFunc)

	return cmd
}

// newPriceHistoryParams converts CLI string values into schwab-go's typed price
// history params. Keeping this conversion shared lets the history and TA commands
// use the same omission and validation behavior after the internal client wrapper
// is removed.
func newPriceHistoryParams(input priceHistoryParamsInput) (*marketdata.PriceHistoryParams, error) {
	period, err := optionalInt(input.Period, "period")
	if err != nil {
		return nil, err
	}
	frequency, err := optionalInt(input.Frequency, "frequency")
	if err != nil {
		return nil, err
	}
	startDate, err := optionalInt64(input.StartDate, "from")
	if err != nil {
		return nil, err
	}
	endDate, err := optionalInt64(input.EndDate, "to")
	if err != nil {
		return nil, err
	}

	return &marketdata.PriceHistoryParams{
		PeriodType:    marketdata.PeriodType(input.PeriodType),
		Period:        period,
		FrequencyType: marketdata.FrequencyType(input.FrequencyType),
		Frequency:     frequency,
		StartDate:     startDate,
		EndDate:       endDate,
	}, nil
}

func optionalInt(raw, flagName string) (int, error) {
	if raw == "" {
		return 0, nil
	}

	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, apperr.NewValidationError(fmt.Sprintf("invalid %s: %s", flagName, raw), err)
	}
	if value <= 0 {
		return 0, apperr.NewValidationError(
			fmt.Sprintf("invalid %s: %s (must be > 0)", flagName, raw),
			nil,
		)
	}
	return value, nil
}

func optionalInt64(raw, flagName string) (int64, error) {
	if raw == "" {
		return 0, nil
	}

	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, apperr.NewValidationError(fmt.Sprintf("invalid %s: %s", flagName, raw), err)
	}
	return value, nil
}
