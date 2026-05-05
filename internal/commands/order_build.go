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
	Primary   string `flag:"primary" flagdescr:"Primary order spec (inline JSON, @file, or - for stdin)" flagrequired:"true"`
	Secondary string `flag:"secondary" flagdescr:"Secondary order spec (inline JSON, @file, or - for stdin)" flagrequired:"true"`
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
			// Resolve --instruction/--order-type aliases before Unmarshal
			// picks up flag values into the opts struct. Safe no-op for
			// multi-leg strategies that don't have alias flags registered.
			if err := resolveOrderFlagAliasesViaFlags(cmd); err != nil {
				return err
			}

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
	cmd.SetFlagErrorFunc(normalizeFlagValidationErrorFunc)

	if flagSetup != nil {
		flagSetup(cmd, opts)
	}

	return cmd
}

// newOrderBuildCmd returns the parent build command for offline order construction.
func newOrderBuildCmd(w io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "build",
		Short: "Build order request JSON without calling the API",
		Long: `Build order request JSON locally without calling the API or requiring
authentication. Output is raw JSON (not envelope-wrapped) that can be piped to
order preview or order place --spec - for a staged workflow. Supports equity,
option, bracket, OCO, and multi-leg option strategies.`,
		RunE:        requireSubcommand,
		Annotations: map[string]string{"skipAuth": "true"},
	}
	cmd.SetFlagErrorFunc(suggestSubcommands)

	equityBuild := makeCobraBuildOrderCommand(w, "equity", "Build an equity order request", func() *equityPlaceOpts { return &equityPlaceOpts{} }, func(cmd *cobra.Command, opts *equityPlaceOpts) { defineAndConstrain(cmd, opts) }, parseEquityParams, orderbuilder.ValidateEquityOrder, orderbuilder.BuildEquityOrder)
	equityBuild.Long = `Build an equity order request JSON. Validates flags and outputs the order
payload without placing it. Pipe to order place --spec - to execute,
or to order preview --spec - to check estimated commissions.`
	equityBuild.Example = `  schwab-agent order build equity --symbol AAPL --action BUY --quantity 10 --type LIMIT --price 200 --duration DAY
  schwab-agent order build equity --symbol AAPL --action SELL --quantity 10 --type STOP --stop-price 145
  schwab-agent order build equity --symbol AAPL --action BUY --quantity 10 --type LIMIT --price 200 | schwab-agent order place --spec -`

	optionBuild := makeCobraBuildOrderCommand(w, "option", "Build an option order request", func() *optionPlaceOpts { return &optionPlaceOpts{} }, func(cmd *cobra.Command, opts *optionPlaceOpts) {
		defineAndConstrain(cmd, opts, []string{"call", "put"})
	}, parseOptionParams, orderbuilder.ValidateOptionOrder, orderbuilder.BuildOptionOrder)
	optionBuild.Long = `Build a single-leg option order request JSON. Requires --underlying,
--expiration, --strike, and exactly one of --call or --put. Output is raw JSON
suitable for piping to order place or order preview.`
	optionBuild.Example = `  schwab-agent order build option --underlying AAPL --expiration 2025-06-20 --strike 200 --call --action BUY_TO_OPEN --quantity 1 --type LIMIT --price 5.00
  schwab-agent order build option --underlying AAPL --expiration 2025-06-20 --strike 190 --put --action SELL_TO_OPEN --quantity 1`

	bracketBuild := makeCobraBuildOrderCommand(w, "bracket", "Build a bracket order request", func() *bracketPlaceOpts { return &bracketPlaceOpts{} }, func(cmd *cobra.Command, opts *bracketPlaceOpts) { defineAndConstrain(cmd, opts) }, parseBracketParams, orderbuilder.ValidateBracketOrder, orderbuilder.BuildBracketOrder)
	bracketBuild.Long = `Build a bracket order request JSON with entry and automatic exit legs. At
least one of --take-profit or --stop-loss is required. Exit instructions are
auto-inverted from the entry action.`
	bracketBuild.Example = `  schwab-agent order build bracket --symbol NVDA --action BUY --quantity 10 --type MARKET --take-profit 150 --stop-loss 120
  schwab-agent order build bracket --symbol AAPL --action BUY --quantity 10 --type LIMIT --price 180 --stop-loss 170`

	ocoBuild := makeCobraBuildOrderCommand(w, "oco", "Build a one-cancels-other order request for an existing position", func() *ocoPlaceOpts { return &ocoPlaceOpts{} }, func(cmd *cobra.Command, opts *ocoPlaceOpts) { defineAndConstrain(cmd, opts) }, parseOCOParams, orderbuilder.ValidateOCOOrder, orderbuilder.BuildOCOOrder)
	ocoBuild.Long = `Build a one-cancels-other order request JSON for an existing position. When
one exit fills, the other is canceled. At least one of --take-profit or
--stop-loss is required.`
	ocoBuild.Example = `  schwab-agent order build oco --symbol AAPL --action SELL --quantity 100 --take-profit 160 --stop-loss 140
  schwab-agent order build oco --symbol TSLA --action BUY --quantity 10 --stop-loss 250`

	verticalBuild := makeCobraBuildOrderCommand(w, "vertical", "Build a vertical spread order request", func() *verticalBuildOpts { return &verticalBuildOpts{} }, func(cmd *cobra.Command, opts *verticalBuildOpts) {
		defineAndConstrain(cmd, opts, []string{"call", "put"}, []string{"open", "close"})
	}, parseVerticalParams, orderbuilder.ValidateVerticalOrder, orderbuilder.BuildVerticalOrder)
	verticalBuild.Long = `Build a vertical spread (bull/bear call or put spread) order request JSON.
Use --long-strike for the strike bought and --short-strike for the strike sold.
NET_DEBIT or NET_CREDIT is auto-determined from the strike relationship.`
	verticalBuild.Example = `  # Bull call spread
  schwab-agent order build vertical --underlying F --expiration 2026-06-18 --long-strike 12 --short-strike 14 --call --open --quantity 1 --price 0.50
  # Bear put spread
  schwab-agent order build vertical --underlying F --expiration 2026-06-18 --long-strike 14 --short-strike 12 --put --open --quantity 1 --price 0.50`

	ironCondorBuild := makeCobraBuildOrderCommand(w, "iron-condor", "Build an iron condor order request", func() *ironCondorBuildOpts { return &ironCondorBuildOpts{} }, func(cmd *cobra.Command, opts *ironCondorBuildOpts) {
		defineAndConstrain(cmd, opts, []string{"open", "close"})
	}, parseIronCondorParams, orderbuilder.ValidateIronCondorOrder, orderbuilder.BuildIronCondorOrder)
	ironCondorBuild.Long = `Build an iron condor order request JSON. Four strikes must be ordered:
put-long < put-short < call-short < call-long. Opening an iron condor
generates a NET_CREDIT.`
	ironCondorBuild.Example = `  schwab-agent order build iron-condor --underlying F --expiration 2026-06-18 --put-long-strike 9 --put-short-strike 10 --call-short-strike 14 --call-long-strike 15 --open --quantity 1 --price 0.50`

	straddleBuild := makeCobraBuildOrderCommand(w, "straddle", "Build a straddle order request", func() *straddleBuildOpts { return &straddleBuildOpts{} }, func(cmd *cobra.Command, opts *straddleBuildOpts) {
		defineAndConstrain(cmd, opts, []string{"buy", "sell"}, []string{"open", "close"})
	}, parseStraddleParams, orderbuilder.ValidateStraddleOrder, orderbuilder.BuildStraddleOrder)
	straddleBuild.Long = `Build a straddle order request JSON. A straddle buys (or sells) a call and
put at the same strike price and expiration. Use --buy for long straddles
(expecting large price movement) or --sell for short straddles.`
	straddleBuild.Example = `  schwab-agent order build straddle --underlying F --expiration 2026-06-18 --strike 12 --buy --open --quantity 1 --price 1.50
  schwab-agent order build straddle --underlying F --expiration 2026-06-18 --strike 12 --sell --open --quantity 1 --price 1.50`

	strangleBuild := makeCobraBuildOrderCommand(w, "strangle", "Build a strangle order request", func() *strangleBuildOpts { return &strangleBuildOpts{} }, func(cmd *cobra.Command, opts *strangleBuildOpts) {
		defineAndConstrain(cmd, opts, []string{"buy", "sell"}, []string{"open", "close"})
	}, parseStrangleParams, orderbuilder.ValidateStrangleOrder, orderbuilder.BuildStrangleOrder)
	strangleBuild.Long = `Build a strangle order request JSON. A strangle uses different strikes for
the call and put legs. Use --buy for long strangles (expecting volatility)
or --sell for short strangles (expecting low volatility).`
	strangleBuild.Example = `  schwab-agent order build strangle --underlying F --expiration 2026-06-18 --call-strike 14 --put-strike 10 --buy --open --quantity 1 --price 0.50
  schwab-agent order build strangle --underlying F --expiration 2026-06-18 --call-strike 14 --put-strike 10 --sell --open --quantity 1 --price 0.50`

	coveredCallBuild := makeCobraBuildOrderCommand(w, "covered-call", "Build a covered call order request (buy shares + sell call)", func() *coveredCallBuildOpts { return &coveredCallBuildOpts{} }, func(cmd *cobra.Command, opts *coveredCallBuildOpts) { defineAndConstrain(cmd, opts) }, parseCoveredCallParams, orderbuilder.ValidateCoveredCallOrder, orderbuilder.BuildCoveredCallOrder)
	coveredCallBuild.Long = `Build a covered call order request JSON that atomically buys shares and sells
a call. Quantity 1 means 100 shares plus 1 call contract. The --price is the
net debit per share after call premium. To sell calls against shares you
already own, use order place option --action SELL_TO_OPEN instead.`
	coveredCallBuild.Example = `  schwab-agent order build covered-call --underlying F --expiration 2026-06-18 --strike 14 --quantity 1 --price 12.00`

	collarBuild := makeCobraBuildOrderCommand(w, "collar", "Build a collar-with-stock order request (buy shares + buy put + sell call)", func() *collarBuildOpts { return &collarBuildOpts{} }, func(cmd *cobra.Command, opts *collarBuildOpts) {
		defineAndConstrain(cmd, opts, []string{"open", "close"})
	}, parseCollarParams, orderbuilder.ValidateCollarOrder, orderbuilder.BuildCollarOrder)
	collarBuild.Long = `Build a collar-with-stock order request JSON. Combines long stock, a
protective long put, and a covered short call. Limits downside risk while
capping upside. Quantity 1 means 100 shares plus 1 put and 1 call contract.`
	collarBuild.Example = `  schwab-agent order build collar --underlying F --put-strike 10 --call-strike 14 --expiration 2026-06-18 --quantity 1 --open --price 12.00`

	calendarBuild := makeCobraBuildOrderCommand(w, "calendar", "Build a calendar spread order request (same strike, different expirations)", func() *calendarBuildOpts { return &calendarBuildOpts{} }, func(cmd *cobra.Command, opts *calendarBuildOpts) {
		defineAndConstrain(cmd, opts, []string{"call", "put"}, []string{"open", "close"})
	}, parseCalendarParams, orderbuilder.ValidateCalendarOrder, orderbuilder.BuildCalendarOrder)
	calendarBuild.Long = `Build a calendar spread order request JSON. Same strike price across two
different expirations: sells the near-term contract and buys the far-term
contract. Profits from time decay differential between the two legs.`
	calendarBuild.Example = `  schwab-agent order build calendar --underlying F --near-expiration 2026-05-16 --far-expiration 2026-07-17 --strike 12 --call --open --quantity 1 --price 0.50`

	diagonalBuild := makeCobraBuildOrderCommand(w, "diagonal", "Build a diagonal spread order request (different strikes and expirations)", func() *diagonalBuildOpts { return &diagonalBuildOpts{} }, func(cmd *cobra.Command, opts *diagonalBuildOpts) {
		defineAndConstrain(cmd, opts, []string{"call", "put"}, []string{"open", "close"})
	}, parseDiagonalParams, orderbuilder.ValidateDiagonalOrder, orderbuilder.BuildDiagonalOrder)
	diagonalBuild.Long = `Build a diagonal spread order request JSON. Different strikes AND different
expirations: sells the near-term contract at one strike and buys the far-term
contract at a different strike. Combines elements of vertical and calendar spreads.`
	diagonalBuild.Example = `  schwab-agent order build diagonal --underlying F --near-strike 12 --far-strike 14 --near-expiration 2026-05-16 --far-expiration 2026-07-17 --call --open --quantity 1 --price 0.50`

	butterflyBuild := makeCobraBuildOrderCommand(w, "butterfly", "Build a butterfly spread order request", func() *butterflyBuildOpts { return &butterflyBuildOpts{} }, func(cmd *cobra.Command, opts *butterflyBuildOpts) {
		defineAndConstrain(cmd, opts, []string{"call", "put"}, []string{"buy", "sell"}, []string{"open", "close"})
	}, parseButterflyParams, orderbuilder.ValidateButterflyOrder, orderbuilder.BuildButterflyOrder)
	butterflyBuild.Long = `Build a butterfly spread order request JSON. Uses three ordered strikes at
the same expiration: lower wing, middle body, upper wing. Buying opens a long
butterfly with long wings and two short body contracts.`
	butterflyBuild.Example = `  schwab-agent order build butterfly --underlying F --expiration 2026-06-18 --lower-strike 10 --middle-strike 12 --upper-strike 14 --call --buy --open --quantity 1 --price 0.50`

	condorBuild := makeCobraBuildOrderCommand(w, "condor", "Build a condor spread order request", func() *condorBuildOpts { return &condorBuildOpts{} }, func(cmd *cobra.Command, opts *condorBuildOpts) {
		defineAndConstrain(cmd, opts, []string{"call", "put"}, []string{"buy", "sell"}, []string{"open", "close"})
	}, parseCondorParams, orderbuilder.ValidateCondorOrder, orderbuilder.BuildCondorOrder)
	condorBuild.Long = `Build a same-expiration condor spread order request JSON. Uses four ordered
strikes: lower wing, lower-middle short, upper-middle short, upper wing. Buying
opens a long condor; selling opens the inverse short condor.`
	condorBuild.Example = `  schwab-agent order build condor --underlying F --expiration 2026-06-18 --lower-strike 10 --lower-middle-strike 12 --upper-middle-strike 14 --upper-strike 16 --call --buy --open --quantity 1 --price 0.75`

	verticalRollBuild := makeCobraBuildOrderCommand(w, "vertical-roll", "Build a vertical roll order request", func() *verticalRollBuildOpts { return &verticalRollBuildOpts{} }, func(cmd *cobra.Command, opts *verticalRollBuildOpts) {
		defineAndConstrain(cmd, opts, []string{"call", "put"}, []string{"debit", "credit"})
	}, parseVerticalRollParams, orderbuilder.ValidateVerticalRollOrder, orderbuilder.BuildVerticalRollOrder)
	verticalRollBuild.Long = `Build a vertical roll order request JSON that closes one vertical spread and
opens another in a single order. Use --debit or --credit to state the roll's net
pricing direction.`
	verticalRollBuild.Example = `  schwab-agent order build vertical-roll --underlying F --close-expiration 2026-06-18 --open-expiration 2026-07-17 --close-long-strike 12 --close-short-strike 14 --open-long-strike 13 --open-short-strike 15 --call --debit --quantity 1 --price 0.25`

	backRatioBuild := makeCobraBuildOrderCommand(w, "back-ratio", "Build a back-ratio spread order request", func() *backRatioBuildOpts { return &backRatioBuildOpts{} }, func(cmd *cobra.Command, opts *backRatioBuildOpts) {
		defineAndConstrain(cmd, opts, []string{"call", "put"}, []string{"open", "close"}, []string{"debit", "credit"})
	}, parseBackRatioParams, orderbuilder.ValidateBackRatioOrder, orderbuilder.BuildBackRatioOrder)
	backRatioBuild.Long = `Build an option back-ratio spread order request JSON. Quantity controls the
short leg, and --long-ratio controls how many long contracts are bought per
short contract. Use --debit or --credit because back-ratios can price either way.`
	backRatioBuild.Example = `  schwab-agent order build back-ratio --underlying F --expiration 2026-06-18 --short-strike 12 --long-strike 14 --call --open --quantity 1 --long-ratio 2 --debit --price 0.20`

	doubleDiagonalBuild := makeCobraBuildOrderCommand(w, "double-diagonal", "Build a double diagonal spread order request", func() *doubleDiagonalBuildOpts { return &doubleDiagonalBuildOpts{} }, func(cmd *cobra.Command, opts *doubleDiagonalBuildOpts) {
		defineAndConstrain(cmd, opts, []string{"open", "close"})
	}, parseDoubleDiagonalParams, orderbuilder.ValidateDoubleDiagonalOrder, orderbuilder.BuildDoubleDiagonalOrder)
	doubleDiagonalBuild.Long = `Build a double diagonal spread order request JSON with far long put/call legs
and near short put/call legs. The near expiration must come before the far
expiration, and strikes must keep the put side below the call side.`
	doubleDiagonalBuild.Example = `  schwab-agent order build double-diagonal --underlying F --near-expiration 2026-06-18 --far-expiration 2026-07-17 --put-far-strike 9 --put-near-strike 10 --call-near-strike 14 --call-far-strike 15 --open --quantity 1 --price 0.80`

	cmd.AddCommand(
		equityBuild, optionBuild, bracketBuild, ocoBuild, newBuyWithStopBuildCmd(w),
		verticalBuild, ironCondorBuild, straddleBuild, strangleBuild,
		coveredCallBuild, collarBuild, calendarBuild, diagonalBuild,
		butterflyBuild, condorBuild, verticalRollBuild, backRatioBuild, doubleDiagonalBuild,
		newBuildFTSCmd(w),
	)

	return cmd
}

// newBuildFTSCmd creates the "order build fts" subcommand.
func newBuildFTSCmd(w io.Writer) *cobra.Command {
	opts := &buildFTSOpts{}
	cmd := &cobra.Command{
		Use:   "fts",
		Short: "Build a first-triggers-second order from two order specs",
		Long: `Build a first-triggers-second order from two JSON order specs. The primary
order executes first; when it fills, the secondary order is automatically
triggered. Both --primary and --secondary accept inline JSON, @file, or
- for stdin.`,
		Example: `  schwab-agent order build fts --primary @entry.json --secondary @exit.json
  schwab-agent order build fts --primary '{"orderType":"LIMIT",...}' --secondary '{"orderType":"STOP",...}'`,
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
