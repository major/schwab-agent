package commands

import (
	"io"
	"strings"
	"time"

	"github.com/leodido/structcli"
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

// symbolBuildOpts holds the options for the symbol build subcommand.
type symbolBuildOpts struct {
	Underlying string  `flag:"underlying" flagdescr:"Underlying symbol (e.g. AAPL)"`
	Expiration string  `flag:"expiration" flagdescr:"Expiration date (YYYY-MM-DD)"`
	Strike     float64 `flag:"strike" flagdescr:"Strike price (e.g. 200, 450.50)"`
	Call       bool    `flag:"call" flagdescr:"Call option"`
	Put        bool    `flag:"put" flagdescr:"Put option"`
}

// Attach implements structcli.Options interface.
func (o *symbolBuildOpts) Attach(_ *cobra.Command) error { return nil }

// NewSymbolCmd returns the Cobra command for option symbol utilities.
// These are pure computation commands that do not require API authentication.
func NewSymbolCmd(w io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "symbol",
		Short: "OCC symbol operations",
		Long: `Build and parse OCC-format option symbols locally. These are pure computation
commands that make no API calls and require no authentication.`,
		GroupID:     "tools",
		Annotations: map[string]string{"skipAuth": "true"},
		RunE:        requireSubcommand,
	}
	cmd.SetFlagErrorFunc(suggestSubcommands)
	cmd.AddCommand(newSymbolBuildCmd(w))
	cmd.AddCommand(newSymbolParseCmd(w))
	return cmd
}

// newSymbolBuildCmd returns the Cobra subcommand for building OCC option symbols from components.
func newSymbolBuildCmd(w io.Writer) *cobra.Command {
	opts := &symbolBuildOpts{}
	cmd := &cobra.Command{
		Use:   "build",
		Short: "Build an OCC option symbol from components",
		Long: `Build an OCC option symbol from underlying, expiration date, strike price, and
contract type. No API call or authentication required. Output is JSON with the
constructed symbol and its components.`,
		Example: `  schwab-agent symbol build --underlying AAPL --expiration 2025-06-20 --strike 200 --call
  schwab-agent symbol build --underlying TSLA --expiration 2025-12-19 --strike 350.50 --put`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := structcli.Unmarshal(cmd, opts); err != nil {
				return err
			}

			if opts.Underlying == "" {
				return apperr.NewValidationError("underlying is required", nil)
			}

			if opts.Expiration == "" {
				return apperr.NewValidationError("expiration is required", nil)
			}

			putCall, err := parsePutCall(opts.Call, opts.Put)
			if err != nil {
				return err
			}

			expirationTime, err := time.Parse("2006-01-02", strings.TrimSpace(opts.Expiration))
			if err != nil {
				return apperr.NewValidationError("expiration must use YYYY-MM-DD format", err)
			}

			underlying := strings.TrimSpace(opts.Underlying)
			symbol := orderbuilder.BuildOCCSymbol(underlying, expirationTime, opts.Strike, string(putCall))

			result := symbolResult{
				Symbol:     symbol,
				Underlying: underlying,
				Expiration: expirationTime.Format("2006-01-02"),
				PutCall:    string(putCall),
				Strike:     opts.Strike,
			}

			return output.WriteSuccess(w, result, output.NewMetadata())
		},
	}

	if err := structcli.Define(cmd, opts); err != nil {
		panic(err)
	}

	cmd.MarkFlagsMutuallyExclusive("call", "put")
	cmd.MarkFlagsOneRequired("call", "put")
	return cmd
}

// newSymbolParseCmd returns the Cobra subcommand for parsing OCC option symbols into components.
func newSymbolParseCmd(w io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "parse <symbol>",
		Short: "Parse an OCC option symbol into components",
		Long: `Parse an OCC option symbol string into its underlying, expiration date, strike
price, and contract type components. No API call or authentication required.`,
		Example: `  schwab-agent symbol parse "AAPL  250620C00200000"`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			symbol := args[0]
			components, err := orderbuilder.ParseOCCSymbol(symbol)
			if err != nil {
				return apperr.NewValidationError(err.Error(), err)
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
