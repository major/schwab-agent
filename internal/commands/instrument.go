package commands

import (
	"context"
	"io"

	"github.com/spf13/cobra"
	"github.com/urfave/cli/v3"

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

// InstrumentCommand returns the CLI command for instrument search and lookup.
func InstrumentCommand(c *client.Ref, w io.Writer) *cli.Command {
	return &cli.Command{
		Name:   "instrument",
		Usage:  "Search and look up instruments",
		Action: requireSubcommand(),
		Commands: []*cli.Command{
			{
				Name:  "search",
				Usage: "Search instruments by symbol or description",
				UsageText: `schwab-agent instrument search AAPL
schwab-agent instrument search Apple --projection desc-search`,
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "projection",
						Usage: "Search projection (symbol-search, symbol-regex, desc-search, desc-regex, search, fundamental)",
						Value: "symbol-search",
					},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					query := cmd.Args().First()
					if err := requireArg(query, "search query"); err != nil {
						return err
					}

					projection := cmd.String("projection")
					result, err := c.SearchInstruments(ctx, query, projection)
					if err != nil {
						return err
					}

					return output.WriteSuccess(w, instrumentSearchData{Instruments: result}, output.NewMetadata())
				},
			},
			{
				Name:      "get",
				Usage:     "Get instrument details by CUSIP",
				UsageText: "schwab-agent instrument get 037833100",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					cusip := cmd.Args().First()
					if err := requireArg(cusip, "CUSIP"); err != nil {
						return err
					}

					result, err := c.GetInstrument(ctx, cusip)
					if err != nil {
						return err
					}

					return output.WriteSuccess(w, instrumentGetData{Instrument: result}, output.NewMetadata())
				},
			},
		},
	}
}

// NewInstrumentCmd returns the Cobra command for instrument search and lookup.
func NewInstrumentCmd(c *client.Ref, w io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "instrument",
		Short:   "Search and look up instruments",
		GroupID: "market-data",
		RunE:    cobraRequireSubcommand,
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
