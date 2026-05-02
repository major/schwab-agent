package commands

import (
	"encoding/json"
	"io"

	"github.com/spf13/cobra"

	"github.com/major/schwab-agent/internal/models"
	"github.com/major/schwab-agent/internal/orderbuilder"
)

// makeCobraBuildOrderCommand creates a Cobra build subcommand that parses
// flags, validates params, and writes raw order request JSON without API calls.
func makeCobraBuildOrderCommand[P any](
	w io.Writer,
	name, usage string,
	flagSetup func(*cobra.Command),
	parse func(*cobra.Command, []string) (P, error),
	validate func(*P) error,
	build func(*P) (*models.OrderRequest, error),
) *cobra.Command {
	cmd := &cobra.Command{
		Use:   name,
		Short: usage,
		RunE: func(cmd *cobra.Command, args []string) error {
			params, err := parse(cmd, args)
			if err != nil {
				return err
			}

			if err := validate(&params); err != nil {
				return err
			}

			order, err := build(&params)
			if err != nil {
				return err
			}

			return writeOrderRequestJSON(w, order)
		},
	}

	if flagSetup != nil {
		flagSetup(cmd)
	}

	return cmd
}

// newOrderBuildCmd returns the parent build command for offline order construction.
func newOrderBuildCmd(w io.Writer) *cobra.Command {
	cmd := &cobra.Command{Use: "build", Short: "Build order request JSON without calling the API", RunE: requireSubcommand}
	cmd.SetFlagErrorFunc(suggestSubcommands)
	cmd.AddCommand(
		makeCobraBuildOrderCommand(w, "equity", "Build an equity order request", equityOrderFlagSetup, parseEquityParams, orderbuilder.ValidateEquityOrder, orderbuilder.BuildEquityOrder),
		makeCobraBuildOrderCommand(w, "option", "Build an option order request", optionOrderFlagSetup, parseOptionParams, orderbuilder.ValidateOptionOrder, orderbuilder.BuildOptionOrder),
		makeCobraBuildOrderCommand(w, "bracket", "Build a bracket order request", bracketOrderFlagSetup, parseBracketParams, orderbuilder.ValidateBracketOrder, orderbuilder.BuildBracketOrder),
		makeCobraBuildOrderCommand(w, "oco", "Build a one-cancels-other order request for an existing position", ocoOrderFlagSetup, parseOCOParams, orderbuilder.ValidateOCOOrder, orderbuilder.BuildOCOOrder),
		makeCobraBuildOrderCommand(w, "vertical", "Build a vertical spread order request", verticalOrderFlagSetup, parseVerticalParams, orderbuilder.ValidateVerticalOrder, orderbuilder.BuildVerticalOrder),
		makeCobraBuildOrderCommand(w, "iron-condor", "Build an iron condor order request", ironCondorOrderFlagSetup, parseIronCondorParams, orderbuilder.ValidateIronCondorOrder, orderbuilder.BuildIronCondorOrder),
		makeCobraBuildOrderCommand(w, "straddle", "Build a straddle order request", straddleOrderFlagSetup, parseStraddleParams, orderbuilder.ValidateStraddleOrder, orderbuilder.BuildStraddleOrder),
		makeCobraBuildOrderCommand(w, "strangle", "Build a strangle order request", strangleOrderFlagSetup, parseStrangleParams, orderbuilder.ValidateStrangleOrder, orderbuilder.BuildStrangleOrder),
		makeCobraBuildOrderCommand(w, "covered-call", "Build a covered call order request (buy shares + sell call)", coveredCallOrderFlagSetup, parseCoveredCallParams, orderbuilder.ValidateCoveredCallOrder, orderbuilder.BuildCoveredCallOrder),
		makeCobraBuildOrderCommand(w, "collar", "Build a collar-with-stock order request (buy shares + buy put + sell call)", collarOrderFlagSetup, parseCollarParams, orderbuilder.ValidateCollarOrder, orderbuilder.BuildCollarOrder),
		makeCobraBuildOrderCommand(w, "calendar", "Build a calendar spread order request (same strike, different expirations)", calendarOrderFlagSetup, parseCalendarParams, orderbuilder.ValidateCalendarOrder, orderbuilder.BuildCalendarOrder),
		makeCobraBuildOrderCommand(w, "diagonal", "Build a diagonal spread order request (different strikes and expirations)", diagonalOrderFlagSetup, parseDiagonalParams, orderbuilder.ValidateDiagonalOrder, orderbuilder.BuildDiagonalOrder),
		newBuildFTSCmd(w),
	)

	return cmd
}

// newBuildFTSCmd creates the "order build fts" subcommand.
func newBuildFTSCmd(w io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "fts",
		Short:   "Build a first-triggers-second order from two order specs",
		Example: "schwab-agent order build fts --primary @primary-order.json --secondary @secondary-order.json",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if flagString(cmd, "primary") == "" {
				return newValidationError("primary is required")
			}

			if flagString(cmd, "secondary") == "" {
				return newValidationError("secondary is required")
			}

			primaryData, err := readSpecSource(cmd, flagString(cmd, "primary"))
			if err != nil {
				return err
			}

			secondaryData, err := readSpecSource(cmd, flagString(cmd, "secondary"))
			if err != nil {
				return err
			}

			var primary models.OrderRequest
			if err := json.Unmarshal(primaryData, &primary); err != nil {
				return newValidationError("invalid primary order JSON: " + err.Error())
			}

			var secondary models.OrderRequest
			if err := json.Unmarshal(secondaryData, &secondary); err != nil {
				return newValidationError("invalid secondary order JSON: " + err.Error())
			}

			params := orderbuilder.FTSParams{Primary: primary, Secondary: secondary}
			if err := orderbuilder.ValidateFTSOrder(&params); err != nil {
				return err
			}

			result, err := orderbuilder.BuildFTSOrder(&params)
			if err != nil {
				return err
			}

			return writeOrderRequestJSON(w, result)
		},
	}

	cmd.Flags().String("primary", "", "Primary order spec (inline JSON, @file, or - for stdin)")
	cmd.Flags().String("secondary", "", "Secondary order spec (inline JSON, @file, or - for stdin)")

	return cmd
}

// writeOrderRequestJSON writes a raw order request payload to the configured writer.
func writeOrderRequestJSON(w io.Writer, order any) error {
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(order)
}
