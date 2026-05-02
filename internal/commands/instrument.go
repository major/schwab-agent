package commands

import (
	"io"

	"github.com/leodido/structcli"
	"github.com/spf13/cobra"

	"github.com/major/schwab-agent/internal/client"
	"github.com/major/schwab-agent/internal/models"
	"github.com/major/schwab-agent/internal/output"
)

// instrumentSearchData wraps the instrument search response.
type instrumentSearchData struct {
	Instruments []models.Instrument `json:"instruments"`
}

// instrumentGetData wraps a single instrument response.
type instrumentGetData struct {
	Instrument *models.Instrument `json:"instrument"`
}

// instrumentSearchOpts holds the options for the instrument search subcommand.
type instrumentSearchOpts struct {
	Projection instrumentProjection `flag:"projection" flagdescr:"Search projection (symbol-search, symbol-regex, desc-search, desc-regex, search, fundamental)" default:"symbol-search"`
}

// Attach implements structcli.Options interface.
func (o *instrumentSearchOpts) Attach(_ *cobra.Command) error { return nil }

// NewInstrumentCmd returns the Cobra command for instrument search and lookup.
func NewInstrumentCmd(c *client.Ref, w io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "instrument",
		Short:   "Search and look up instruments",
		GroupID: "market-data",
		RunE:    requireSubcommand,
	}
	cmd.SetFlagErrorFunc(suggestSubcommands)

	cmd.AddCommand(newInstrumentSearchCmd(c, w))
	cmd.AddCommand(newInstrumentGetCmd(c, w))

	return cmd
}

// newInstrumentSearchCmd returns the search subcommand.
func newInstrumentSearchCmd(c *client.Ref, w io.Writer) *cobra.Command {
	opts := &instrumentSearchOpts{}
	cmd := &cobra.Command{
		Use:   "search QUERY",
		Short: "Search instruments by symbol or description",
		Long: `Search for instruments by symbol or description. Use --projection to control
the search type: symbol-search (default), symbol-regex, desc-search,
desc-regex, search, or fundamental.`,
		Example: `  schwab-agent instrument search AAPL
  schwab-agent instrument search "Apple" --projection desc-search
  schwab-agent instrument search AAPL --projection fundamental`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := structcli.Unmarshal(cmd, opts); err != nil {
				return err
			}

			query := args[0]

			result, err := c.SearchInstruments(cmd.Context(), query, string(opts.Projection))
			if err != nil {
				return err
			}

			return output.WriteSuccess(w, instrumentSearchData{Instruments: result}, output.NewMetadata())
		},
	}

	if err := structcli.Define(cmd, opts); err != nil {
		panic(err)
	}
	cmd.SetFlagErrorFunc(normalizeFlagValidationErrorFunc)

	return cmd
}

// newInstrumentGetCmd returns the get subcommand.
func newInstrumentGetCmd(c *client.Ref, w io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get CUSIP",
		Short: "Get instrument details by CUSIP",
		Long: `Get instrument details by CUSIP identifier. Returns the instrument type,
description, exchange, and other metadata for the specified CUSIP.`,
		Example: "  schwab-agent instrument get 037833100",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cusip := args[0]

			result, err := c.GetInstrument(cmd.Context(), cusip)
			if err != nil {
				return err
			}

			return output.WriteSuccess(w, instrumentGetData{Instrument: result}, output.NewMetadata())
		},
	}

	return cmd
}
