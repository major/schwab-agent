package commands

import (
	"encoding/json"
	"io"
	"strings"

	"github.com/leodido/structcli"
	"github.com/spf13/cobra"

	"github.com/major/schwab-agent/internal/models"
	"github.com/major/schwab-agent/internal/orderbuilder"
)

// buildFTSOpts holds local flags for first-triggers-second order builds.
type buildFTSOpts struct {
	Primary   string `flag:"primary" flagdescr:"Primary order spec (inline JSON, @file, or - for stdin)"`
	Secondary string `flag:"secondary" flagdescr:"Secondary order spec (inline JSON, @file, or - for stdin)"`
}

// Attach implements structcli.Options interface.
func (o *buildFTSOpts) Attach(_ *cobra.Command) error { return nil }

// makeCobraBuildOrderCommand creates a Cobra build subcommand that parses
// flags, validates params, and writes raw order request JSON without API calls.
func makeCobraBuildOrderCommand[O any, P any](
	w io.Writer,
	name, usage string,
	newOpts func() *O,
	flagSetup func(*cobra.Command, *O),
	parse func(*O, []string) (*P, error),
	validate func(*P) error,
	build func(*P) (*models.OrderRequest, error),
) *cobra.Command {
	opts := newOpts()
	cmd := &cobra.Command{
		Use:   name,
		Short: usage,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := structcli.Unmarshal(cmd, any(opts).(structcli.Options)); err != nil {
				return err
			}

			params, err := parse(opts, args)
			if err != nil {
				return err
			}

			if err := validate(params); err != nil {
				return err
			}

			order, err := build(params)
			if err != nil {
				return err
			}

			return writeOrderRequestJSON(w, order)
		},
	}

	if flagSetup != nil {
		flagSetup(cmd, opts)
	}

	return cmd
}

// newOrderBuildCmd returns the parent build command for offline order construction.
func newOrderBuildCmd(w io.Writer) *cobra.Command {
	cmd := &cobra.Command{Use: "build", Short: "Build order request JSON without calling the API", RunE: requireSubcommand}
	cmd.SetFlagErrorFunc(suggestSubcommands)
	cmd.AddCommand(
		makeCobraBuildOrderCommand(w, "equity", "Build an equity order request", func() *equityPlaceOpts { return &equityPlaceOpts{} }, equityOrderFlagSetup, parseEquityParams, orderbuilder.ValidateEquityOrder, orderbuilder.BuildEquityOrder),
		makeCobraBuildOrderCommand(w, "option", "Build an option order request", func() *optionPlaceOpts { return &optionPlaceOpts{} }, optionOrderFlagSetup, parseOptionParams, orderbuilder.ValidateOptionOrder, orderbuilder.BuildOptionOrder),
		makeCobraBuildOrderCommand(w, "bracket", "Build a bracket order request", func() *bracketPlaceOpts { return &bracketPlaceOpts{} }, bracketOrderFlagSetup, parseBracketParams, orderbuilder.ValidateBracketOrder, orderbuilder.BuildBracketOrder),
		makeCobraBuildOrderCommand(w, "oco", "Build a one-cancels-other order request for an existing position", func() *ocoPlaceOpts { return &ocoPlaceOpts{} }, ocoOrderFlagSetup, parseOCOParams, orderbuilder.ValidateOCOOrder, orderbuilder.BuildOCOOrder),
		makeCobraBuildOrderCommand(w, "vertical", "Build a vertical spread order request", func() *verticalBuildOpts { return &verticalBuildOpts{} }, verticalOrderFlagSetup, parseVerticalParams, orderbuilder.ValidateVerticalOrder, orderbuilder.BuildVerticalOrder),
		makeCobraBuildOrderCommand(w, "iron-condor", "Build an iron condor order request", func() *ironCondorBuildOpts { return &ironCondorBuildOpts{} }, ironCondorOrderFlagSetup, parseIronCondorParams, orderbuilder.ValidateIronCondorOrder, orderbuilder.BuildIronCondorOrder),
		makeCobraBuildOrderCommand(w, "straddle", "Build a straddle order request", func() *straddleBuildOpts { return &straddleBuildOpts{} }, straddleOrderFlagSetup, parseStraddleParams, orderbuilder.ValidateStraddleOrder, orderbuilder.BuildStraddleOrder),
		makeCobraBuildOrderCommand(w, "strangle", "Build a strangle order request", func() *strangleBuildOpts { return &strangleBuildOpts{} }, strangleOrderFlagSetup, parseStrangleParams, orderbuilder.ValidateStrangleOrder, orderbuilder.BuildStrangleOrder),
		makeCobraBuildOrderCommand(w, "covered-call", "Build a covered call order request (buy shares + sell call)", func() *coveredCallBuildOpts { return &coveredCallBuildOpts{} }, coveredCallOrderFlagSetup, parseCoveredCallParams, orderbuilder.ValidateCoveredCallOrder, orderbuilder.BuildCoveredCallOrder),
		makeCobraBuildOrderCommand(w, "collar", "Build a collar-with-stock order request (buy shares + buy put + sell call)", func() *collarBuildOpts { return &collarBuildOpts{} }, collarOrderFlagSetup, parseCollarParams, orderbuilder.ValidateCollarOrder, orderbuilder.BuildCollarOrder),
		makeCobraBuildOrderCommand(w, "calendar", "Build a calendar spread order request (same strike, different expirations)", func() *calendarBuildOpts { return &calendarBuildOpts{} }, calendarOrderFlagSetup, parseCalendarParams, orderbuilder.ValidateCalendarOrder, orderbuilder.BuildCalendarOrder),
		makeCobraBuildOrderCommand(w, "diagonal", "Build a diagonal spread order request (different strikes and expirations)", func() *diagonalBuildOpts { return &diagonalBuildOpts{} }, diagonalOrderFlagSetup, parseDiagonalParams, orderbuilder.ValidateDiagonalOrder, orderbuilder.BuildDiagonalOrder),
		newBuildFTSCmd(w),
	)

	return cmd
}

// newBuildFTSCmd creates the "order build fts" subcommand.
func newBuildFTSCmd(w io.Writer) *cobra.Command {
	opts := &buildFTSOpts{}
	cmd := &cobra.Command{
		Use:     "fts",
		Short:   "Build a first-triggers-second order from two order specs",
		Example: "schwab-agent order build fts --primary @primary-order.json --secondary @secondary-order.json",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := structcli.Unmarshal(cmd, opts); err != nil {
				return err
			}

			if strings.TrimSpace(opts.Primary) == "" {
				return newValidationError("primary is required")
			}

			if strings.TrimSpace(opts.Secondary) == "" {
				return newValidationError("secondary is required")
			}

			primaryData, err := readSpecSource(cmd, opts.Primary)
			if err != nil {
				return err
			}

			secondaryData, err := readSpecSource(cmd, opts.Secondary)
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

	if err := structcli.Define(cmd, opts); err != nil {
		panic(err)
	}

	return cmd
}

// writeOrderRequestJSON writes a raw order request payload to the configured writer.
func writeOrderRequestJSON(w io.Writer, order any) error {
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(order)
}
