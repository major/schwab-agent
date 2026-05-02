package commands

import (
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/major/schwab-agent/internal/apperr"
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

// NewSymbolCmd returns the Cobra command for option symbol utilities.
// These are pure computation commands that do not require API authentication.
func NewSymbolCmd(w io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "symbol",
		Short:       "OCC symbol operations",
		GroupID:     "tools",
		Annotations: map[string]string{"skipAuth": "true"},
		RunE:        requireSubcommand,
	}
	cmd.AddCommand(newSymbolBuildCmd(w))
	cmd.AddCommand(newSymbolParseCmd(w))
	return cmd
}

// newSymbolBuildCmd returns the Cobra subcommand for building OCC option symbols from components.
func newSymbolBuildCmd(w io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "build",
		Short: "Build an OCC option symbol from components",
		Long:  "Build an OCC option symbol from components (underlying, expiration, strike, call/put)",
		RunE: func(cmd *cobra.Command, args []string) error {
			underlying := flagString(cmd, "underlying")
			if underlying == "" {
				return apperr.NewValidationError("underlying is required", nil)
			}

			expiration := flagString(cmd, "expiration")
			if expiration == "" {
				return apperr.NewValidationError("expiration is required", nil)
			}

			strike := flagFloat64(cmd, "strike")
			call := flagBool(cmd, "call")
			put := flagBool(cmd, "put")

			putCall, err := parsePutCall(call, put)
			if err != nil {
				return err
			}

			expirationTime, err := time.Parse("2006-01-02", strings.TrimSpace(expiration))
			if err != nil {
				return apperr.NewValidationError("expiration must use YYYY-MM-DD format", nil)
			}

			underlying = strings.TrimSpace(underlying)
			symbol := orderbuilder.BuildOCCSymbol(underlying, expirationTime, strike, string(putCall))

			result := symbolResult{
				Symbol:     symbol,
				Underlying: underlying,
				Expiration: expirationTime.Format("2006-01-02"),
				PutCall:    string(putCall),
				Strike:     strike,
			}

			return output.WriteSuccess(w, result, output.NewMetadata())
		},
	}
	cmd.Flags().String("underlying", "", "Underlying symbol (e.g. AAPL)")
	cmd.Flags().String("expiration", "", "Expiration date (YYYY-MM-DD)")
	cmd.Flags().Float64("strike", 0, "Strike price (e.g. 200, 450.50)")
	cmd.Flags().Bool("call", false, "Call option")
	cmd.Flags().Bool("put", false, "Put option")
	return cmd
}

// newSymbolParseCmd returns the Cobra subcommand for parsing OCC option symbols into components.
func newSymbolParseCmd(w io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "parse <symbol>",
		Short: "Parse an OCC option symbol into components",
		Long:  "Parse an OCC option symbol into components (underlying, expiration, strike, call/put)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			symbol := args[0]
			components, err := orderbuilder.ParseOCCSymbol(symbol)
			if err != nil {
				return apperr.NewValidationError(err.Error(), nil)
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

			return output.WriteSuccess(w, result, output.NewMetadata())
		},
	}
	return cmd
}
