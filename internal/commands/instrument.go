package commands

import (
	"io"

	"github.com/spf13/cobra"

	"github.com/major/schwab-agent/internal/apperr"
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

// NewInstrumentCmd returns the Cobra command for instrument search and lookup.
func NewInstrumentCmd(c *client.Ref, w io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "instrument",
		Short:   "Search and look up instruments",
		GroupID: "market-data",
		RunE:    requireSubcommand,
	}

	cmd.AddCommand(newInstrumentSearchCmd(c, w))
	cmd.AddCommand(newInstrumentGetCmd(c, w))

	return cmd
}

// newInstrumentSearchCmd returns the search subcommand.
func newInstrumentSearchCmd(c *client.Ref, w io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "search QUERY",
		Short:   "Search instruments by symbol or description",
		Example: "schwab-agent instrument search AAPL\nschwab-agent instrument search Apple --projection desc-search",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return apperr.NewValidationError("search query is required", nil)
			}

			query := args[0]
			projection := flagString(cmd, "projection")

			result, err := c.SearchInstruments(cmd.Context(), query, projection)
			if err != nil {
				return err
			}

			return output.WriteSuccess(w, instrumentSearchData{Instruments: result}, output.NewMetadata())
		},
	}

	cmd.Flags().String("projection", "symbol-search", "Search projection (symbol-search, symbol-regex, desc-search, desc-regex, search, fundamental)")

	return cmd
}

// newInstrumentGetCmd returns the get subcommand.
func newInstrumentGetCmd(c *client.Ref, w io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "get CUSIP",
		Short:   "Get instrument details by CUSIP",
		Example: "schwab-agent instrument get 037833100",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return apperr.NewValidationError("CUSIP is required", nil)
			}

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
