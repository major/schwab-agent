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
	name, usage string,
	flags []cli.Flag,
	parse func(*cli.Command) (P, error),
	validate func(*P) error,
	build func(*P) (*models.OrderRequest, error),
) *cli.Command {
	return &cli.Command{
		Name:  name,
		Usage: usage,
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
		Name:  "build",
		Usage: "Build order request JSON without calling the API",
		Commands: []*cli.Command{
			makeBuildOrderCommand(w, "equity", "Build an equity order request",
				equityOrderFlags(), parseEquityParams,
				orderbuilder.ValidateEquityOrder, orderbuilder.BuildEquityOrder),
			makeBuildOrderCommand(w, "option", "Build an option order request",
				optionOrderFlags(), parseOptionParams,
				orderbuilder.ValidateOptionOrder, orderbuilder.BuildOptionOrder),
			makeBuildOrderCommand(w, "bracket", "Build a bracket order request",
				bracketOrderFlags(), parseBracketParams,
				orderbuilder.ValidateBracketOrder, orderbuilder.BuildBracketOrder),
			makeBuildOrderCommand(w, "oco", "Build a one-cancels-other order request for an existing position",
				ocoOrderFlags(), parseOCOParams,
				orderbuilder.ValidateOCOOrder, orderbuilder.BuildOCOOrder),
			makeBuildOrderCommand(w, "vertical", "Build a vertical spread order request",
				verticalOrderFlags(), parseVerticalParams,
				orderbuilder.ValidateVerticalOrder, orderbuilder.BuildVerticalOrder),
			makeBuildOrderCommand(w, "iron-condor", "Build an iron condor order request",
				ironCondorOrderFlags(), parseIronCondorParams,
				orderbuilder.ValidateIronCondorOrder, orderbuilder.BuildIronCondorOrder),
			makeBuildOrderCommand(w, "straddle", "Build a straddle order request",
				straddleOrderFlags(), parseStraddleParams,
				orderbuilder.ValidateStraddleOrder, orderbuilder.BuildStraddleOrder),
			makeBuildOrderCommand(w, "strangle", "Build a strangle order request",
				strangleOrderFlags(), parseStrangleParams,
				orderbuilder.ValidateStrangleOrder, orderbuilder.BuildStrangleOrder),
		makeBuildOrderCommand(w, "covered-call", "Build a covered call order request (buy shares + sell call)",
			coveredCallOrderFlags(), parseCoveredCallParams,
			orderbuilder.ValidateCoveredCallOrder, orderbuilder.BuildCoveredCallOrder),
			buildFTSCommand(w),
		},
	}
}

// buildFTSCommand creates the "order build fts" subcommand. FTS orders take
// two full order specs (--primary and --secondary) rather than typed flag params,
// so the generic makeBuildOrderCommand pipeline doesn't fit here.
func buildFTSCommand(w io.Writer) *cli.Command {
	return &cli.Command{
		Name:  "fts",
		Usage: "Build a first-triggers-second order from two order specs",
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
