package commands

import (
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/major/schwab-go/schwab/marketdata"
	"github.com/spf13/cobra"

	"github.com/major/schwab-agent/internal/apperr"
	"github.com/major/schwab-agent/internal/client"
	"github.com/major/schwab-agent/internal/output"
)

// instrumentSearchData wraps the instrument search response.
type instrumentSearchData struct {
	Instruments []marketdata.Instrument `json:"instruments"`
}

// instrumentGetData wraps a single instrument response.
type instrumentGetData struct {
	Instrument *marketdata.Instrument `json:"instrument"`
}

// instrumentSearchOpts holds the options for the instrument search subcommand.
type instrumentSearchOpts struct {
	Projection instrumentProjection `flag:"projection" flagdescr:"Search projection (symbol-search, symbol-regex, desc-search, desc-regex, search, fundamental)" default:"symbol-search"`
}

// NewInstrumentCmd returns the Cobra command for instrument search and lookup.
func NewInstrumentCmd(c *client.Ref, w io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:     commandUseInstrument,
		Short:   "Search and look up instruments",
		GroupID: groupIDMarketData,
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
			query := args[0]

			result, err := c.MarketData.SearchInstruments(
				cmd.Context(),
				query,
				marketdata.InstrumentProjection(opts.Projection),
			)
			if err != nil {
				return mapSchwabGoError(err)
			}

			return output.WriteSuccess(w, instrumentSearchData{Instruments: result.Instruments}, output.NewMetadata())
		},
	}

	defineCobraFlags(cmd, opts)
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

			// Schwab's live CUSIP endpoint currently returns the same wrapper shape as
			// SearchInstruments (`{"instruments":[...]}`), while schwab-go v0.1.1
			// decodes GetInstrumentByCUSIP as a bare Instrument and silently produces a
			// zero-value result. Querying the schwab-go search method with the CUSIP keeps
			// this command on the shared schwab-go client while matching live API behavior.
			// Upstream bug: https://github.com/major/schwab-go/issues/33
			result, err := c.MarketData.SearchInstruments(cmd.Context(), cusip, marketdata.ProjectionSymbolSearch)
			if err != nil {
				return mapInstrumentGetError(err)
			}

			instrument := findInstrumentByCUSIP(result, cusip)
			if instrument == nil {
				return apperr.NewSymbolNotFoundError("instrument not found", nil)
			}

			return output.WriteSuccess(w, instrumentGetData{Instrument: instrument}, output.NewMetadata())
		},
	}

	return cmd
}

// mapInstrumentGetError preserves the CLI's historical not-found contract for
// CUSIP lookups while reusing the shared schwab-go mapper for every other API
// error. The old custom instruments client returned SymbolNotFoundError for
// Schwab 404 responses, so callers can keep treating missing CUSIPs like other
// symbol lookup failures instead of generic HTTP failures.
func mapInstrumentGetError(err error) error {
	mappedErr := mapSchwabGoError(err)
	var httpErr *apperr.HTTPError
	if errors.As(mappedErr, &httpErr) && httpErr.StatusCode == http.StatusNotFound {
		return apperr.NewSymbolNotFoundError("instrument not found", nil)
	}

	return mappedErr
}

// findInstrumentByCUSIP returns the exact CUSIP match from a schwab-go search response.
func findInstrumentByCUSIP(result *marketdata.InstrumentResponse, cusip string) *marketdata.Instrument {
	if result == nil {
		return nil
	}

	for i := range result.Instruments {
		if strings.EqualFold(result.Instruments[i].Cusip, cusip) {
			return &result.Instruments[i]
		}
	}

	return nil
}
