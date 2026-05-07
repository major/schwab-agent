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

			instrument, err := c.MarketData.GetInstrumentByCUSIP(cmd.Context(), cusip)
			if err != nil {
				return mapInstrumentGetError(err)
			}
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
	if mappedErr == nil {
		return nil
	}

	var httpErr *apperr.HTTPError
	if errors.As(mappedErr, &httpErr) && httpErr.StatusCode == http.StatusNotFound {
		return apperr.NewSymbolNotFoundError("instrument not found", nil)
	}
	// TODO: Replace this fallback when schwab-go exposes a sentinel or typed
	// error for empty CUSIP responses. v0.1.3 returns only a formatted error, so
	// keep the narrow message match to preserve the CLI's structured contract.
	if strings.Contains(mappedErr.Error(), "no instrument found for CUSIP") {
		return apperr.NewSymbolNotFoundError("instrument not found", nil)
	}

	return mappedErr
}
