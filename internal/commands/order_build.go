package commands

import (
	"context"
	"encoding/json"
	"io"

	"github.com/urfave/cli/v3"

	"github.com/major/schwab-agent/internal/models"
	"github.com/major/schwab-agent/internal/orderbuilder"
)

// makeBuildOrderCommand creates a build subcommand that parses flags, validates,
// and builds an order request without calling the API. Each order type provides
// its own parse/validate/build functions while sharing the common pipeline.
func makeBuildOrderCommand[P any](
	w io.Writer,
	name, usage, usageText string,
	flags []cli.Flag,
	parse func(*cli.Command) (P, error),
	validate func(*P) error,
	build func(*P) (*models.OrderRequest, error),
) *cli.Command {
	return &cli.Command{
		Name:      name,
		Usage:     usage,
		UsageText: usageText,
		Flags: flags,
		Action: func(ctx context.Context, cmd *cli.Command) error {
			_ = ctx

			params, err := parse(cmd)
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
}

// orderBuildCommand returns the parent build command for offline order construction.
func orderBuildCommand(w io.Writer) *cli.Command {
	return &cli.Command{
		Name:   "build",
		Usage:  "Build order request JSON without calling the API",
		Action: requireSubcommand(),
		Commands: []*cli.Command{
			makeBuildOrderCommand(w, "equity", "Build an equity order request",
				"schwab-agent order build equity --symbol AAPL --action BUY --quantity 10 --type LIMIT --price 150.00 --duration DAY",
				equityOrderFlags(), parseEquityParams,
				orderbuilder.ValidateEquityOrder, orderbuilder.BuildEquityOrder),
			makeBuildOrderCommand(w, "option", "Build an option order request",
				"schwab-agent order build option --underlying AAPL --expiration 2025-06-20 --strike 200 --call --action BUY_TO_OPEN --quantity 1 --type LIMIT --price 5.00 --duration DAY",
				optionOrderFlags(), parseOptionParams,
				orderbuilder.ValidateOptionOrder, orderbuilder.BuildOptionOrder),
			makeBuildOrderCommand(w, "bracket", "Build a bracket order request",
				"schwab-agent order build bracket --symbol AAPL --action BUY --quantity 10 --type LIMIT --price 150.00 --take-profit 170.00 --stop-loss 140.00 --duration DAY",
				bracketOrderFlags(), parseBracketParams,
				orderbuilder.ValidateBracketOrder, orderbuilder.BuildBracketOrder),
			makeBuildOrderCommand(w, "oco", "Build a one-cancels-other order request for an existing position",
				"schwab-agent order build oco --symbol AAPL --action SELL --quantity 10 --take-profit 170.00 --stop-loss 140.00 --duration DAY",
				ocoOrderFlags(), parseOCOParams,
				orderbuilder.ValidateOCOOrder, orderbuilder.BuildOCOOrder),
			makeBuildOrderCommand(w, "vertical", "Build a vertical spread order request",
				"schwab-agent order build vertical --underlying AAPL --expiration 2025-06-20 --long-strike 195 --short-strike 200 --call --open --quantity 1 --price 2.50 --duration DAY",
				verticalOrderFlags(), parseVerticalParams,
				orderbuilder.ValidateVerticalOrder, orderbuilder.BuildVerticalOrder),
			makeBuildOrderCommand(w, "iron-condor", "Build an iron condor order request",
				"schwab-agent order build iron-condor --underlying SPY --expiration 2025-06-20 --put-long-strike 530 --put-short-strike 540 --call-short-strike 570 --call-long-strike 580 --open --quantity 1 --price 1.50 --duration DAY",
				ironCondorOrderFlags(), parseIronCondorParams,
				orderbuilder.ValidateIronCondorOrder, orderbuilder.BuildIronCondorOrder),
			makeBuildOrderCommand(w, "straddle", "Build a straddle order request",
				"schwab-agent order build straddle --underlying AAPL --expiration 2025-06-20 --strike 200 --buy --open --quantity 1 --price 10.00 --duration DAY",
				straddleOrderFlags(), parseStraddleParams,
				orderbuilder.ValidateStraddleOrder, orderbuilder.BuildStraddleOrder),
			makeBuildOrderCommand(w, "strangle", "Build a strangle order request",
				"schwab-agent order build strangle --underlying AAPL --expiration 2025-06-20 --call-strike 210 --put-strike 190 --buy --open --quantity 1 --price 8.00 --duration DAY",
				strangleOrderFlags(), parseStrangleParams,
				orderbuilder.ValidateStrangleOrder, orderbuilder.BuildStrangleOrder),
			makeBuildOrderCommand(w, "covered-call", "Build a covered call order request (buy shares + sell call)",
				"schwab-agent order build covered-call --underlying AAPL --expiration 2025-06-20 --strike 210 --quantity 1 --price 195.00 --duration DAY",
				coveredCallOrderFlags(), parseCoveredCallParams,
				orderbuilder.ValidateCoveredCallOrder, orderbuilder.BuildCoveredCallOrder),
			makeBuildOrderCommand(w, "collar", "Build a collar-with-stock order request (buy shares + buy put + sell call)",
				"schwab-agent order build collar --underlying AAPL --expiration 2025-06-20 --put-strike 190 --call-strike 210 --quantity 1 --open --price 195.00 --duration DAY",
				collarOrderFlags(), parseCollarParams,
				orderbuilder.ValidateCollarOrder, orderbuilder.BuildCollarOrder),
			makeBuildOrderCommand(w, "calendar", "Build a calendar spread order request (same strike, different expirations)",
				"schwab-agent order build calendar --underlying AAPL --near-expiration 2025-06-20 --far-expiration 2025-07-18 --strike 200 --call --open --quantity 1 --price 3.00 --duration DAY",
				calendarOrderFlags(), parseCalendarParams,
				orderbuilder.ValidateCalendarOrder, orderbuilder.BuildCalendarOrder),
			makeBuildOrderCommand(w, "diagonal", "Build a diagonal spread order request (different strikes and expirations)",
				"schwab-agent order build diagonal --underlying AAPL --near-expiration 2025-06-20 --far-expiration 2025-07-18 --near-strike 205 --far-strike 200 --call --open --quantity 1 --price 4.00 --duration DAY",
				diagonalOrderFlags(), parseDiagonalParams,
				orderbuilder.ValidateDiagonalOrder, orderbuilder.BuildDiagonalOrder),
			buildFTSCommand(w),
		},
	}
}

// buildFTSCommand creates the "order build fts" subcommand. FTS orders take
// two full order specs (--primary and --secondary) rather than typed flag params,
// so the generic makeBuildOrderCommand pipeline doesn't fit here.
func buildFTSCommand(w io.Writer) *cli.Command {
	return &cli.Command{
		Name:      "fts",
		Usage:     "Build a first-triggers-second order from two order specs",
		UsageText: "schwab-agent order build fts --primary @primary-order.json --secondary @secondary-order.json",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "primary",
				Usage:    "Primary order spec (inline JSON, @file, or - for stdin)",
				Required: true,
			},
			&cli.StringFlag{
				Name:     "secondary",
				Usage:    "Secondary order spec (inline JSON, @file, or - for stdin)",
				Required: true,
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			_ = ctx

			primaryData, err := readSpecSource(cmd, cmd.String("primary"))
			if err != nil {
				return err
			}

			secondaryData, err := readSpecSource(cmd, cmd.String("secondary"))
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

			params := orderbuilder.FTSParams{
				Primary:   primary,
				Secondary: secondary,
			}

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
}

// writeOrderRequestJSON writes a raw order request payload to the configured writer.
func writeOrderRequestJSON(w io.Writer, order any) error {
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(order)
}
