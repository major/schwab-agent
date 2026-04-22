package commands

import (
	"context"
	"io"
	"strings"
	"time"

	"github.com/urfave/cli/v3"

	"github.com/major/schwab-agent/internal/models"
	"github.com/major/schwab-agent/internal/orderbuilder"
	"github.com/major/schwab-agent/internal/output"
)

// symbolResult is the JSON output for symbol build/parse operations.
type symbolResult struct {
	Symbol     string  `json:"symbol"`
	Underlying string  `json:"underlying"`
	Expiration string  `json:"expiration"`
	PutCall    string  `json:"put_call"`
	Strike     float64 `json:"strike"`
}

// SymbolCommand returns the CLI command for option symbol utilities.
// These are pure computation commands that do not require API authentication.
func SymbolCommand(w io.Writer) *cli.Command {
	return &cli.Command{
		Name:  "symbol",
		Usage: "Option symbol utilities (build and parse OCC symbols)",
		Commands: []*cli.Command{
			symbolBuildCommand(w),
			symbolParseCommand(w),
		},
	}
}

// symbolBuildCommand returns the subcommand for building OCC option symbols from components.
func symbolBuildCommand(w io.Writer) *cli.Command {
	return &cli.Command{
		Name:  "build",
		Usage: "Build an OCC option symbol from components",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "underlying", Usage: "Underlying symbol (e.g. AAPL)", Required: true},
			&cli.StringFlag{Name: "expiration", Usage: "Expiration date (YYYY-MM-DD)", Required: true},
			&cli.Float64Flag{Name: "strike", Usage: "Strike price (e.g. 200, 450.50)", Required: true},
			&cli.BoolFlag{Name: "call", Usage: "Call option"},
			&cli.BoolFlag{Name: "put", Usage: "Put option"},
		},
		Action: func(_ context.Context, cmd *cli.Command) error {
			putCall, err := parsePutCall(cmd.Bool("call"), cmd.Bool("put"))
			if err != nil {
				return err
			}

			expiration, err := time.Parse("2006-01-02", strings.TrimSpace(cmd.String("expiration")))
			if err != nil {
				return newValidationError("expiration must use YYYY-MM-DD format")
			}

			underlying := strings.TrimSpace(cmd.String("underlying"))
			strike := cmd.Float64("strike")

			symbol := orderbuilder.BuildOCCSymbol(underlying, expiration, strike, string(putCall))

			result := symbolResult{
				Symbol:     symbol,
				Underlying: underlying,
				Expiration: expiration.Format("2006-01-02"),
				PutCall:    string(putCall),
				Strike:     strike,
			}

			return output.WriteSuccess(w, result, output.TimestampMeta())
		},
	}
}

// symbolParseCommand returns the subcommand for parsing OCC option symbols into components.
func symbolParseCommand(w io.Writer) *cli.Command {
	return &cli.Command{
		Name:      "parse",
		Usage:     "Parse an OCC option symbol into components",
		ArgsUsage: "<symbol>",
		Action: func(_ context.Context, cmd *cli.Command) error {
			if cmd.NArg() == 0 {
				return newValidationError("an OCC symbol argument is required (e.g. \"AAPL  250620C00200000\")")
			}

			symbol := cmd.Args().First()
			components, err := orderbuilder.ParseOCCSymbol(symbol)
			if err != nil {
				return newValidationError(err.Error())
			}

			// Map putCall string back to the canonical models constant for consistent
			// output. ParseOCCSymbol already returns "CALL" or "PUT", but this
			// ensures we stay in sync with the models package.
			putCall := string(models.PutCallCall)
			if components.PutCall == string(models.PutCallPut) {
				putCall = string(models.PutCallPut)
			}

			result := symbolResult{
				Symbol:     components.Symbol,
				Underlying: components.Underlying,
				Expiration: components.Expiration.Format("2006-01-02"),
				PutCall:    putCall,
				Strike:     components.Strike,
			}

			return output.WriteSuccess(w, result, output.TimestampMeta())
		},
	}
}
