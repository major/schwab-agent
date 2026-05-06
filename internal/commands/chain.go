package commands

import (
	"fmt"
	"io"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/major/schwab-go/schwab/marketdata"

	"github.com/major/schwab-agent/internal/apperr"
	"github.com/major/schwab-agent/internal/client"
	"github.com/major/schwab-agent/internal/output"
)

// chainGetOpts holds the options for the chain get subcommand.
type chainGetOpts struct {
	Type                   chainContractType `flag:"type"                     flagdescr:"Contract type: CALL, PUT, or ALL"                        flaggroup:"contract"`
	StrikeCount            string            `flag:"strike-count"             flagdescr:"Number of strikes to return"                             flaggroup:"filtering"`
	Strategy               chainStrategy     `flag:"strategy"                 flagdescr:"Option pricing strategy"                                 flaggroup:"contract"`
	FromDate               string            `flag:"from-date"                flagdescr:"Start date (YYYY-MM-DD)"                                 flaggroup:"filtering"`
	ToDate                 string            `flag:"to-date"                  flagdescr:"End date (YYYY-MM-DD)"                                   flaggroup:"filtering"`
	IncludeUnderlyingQuote bool              `flag:"include-underlying-quote" flagdescr:"Include underlying quote data in response"               flaggroup:"filtering"`
	Interval               string            `flag:"interval"                 flagdescr:"Strike interval for spread strategy chains"              flaggroup:"filtering"`
	Strike                 string            `flag:"strike"                   flagdescr:"Filter to a specific strike price"                       flaggroup:"filtering"`
	StrikeRange            strikeRange       `flag:"strike-range"             flagdescr:"Moneyness filter: ITM, NTM, OTM, SAK, SBK, SNK, or ALL"  flaggroup:"filtering"`
	Volatility             string            `flag:"volatility"               flagdescr:"Volatility for theoretical pricing calculations"         flaggroup:"pricing"`
	UnderlyingPrice        string            `flag:"underlying-price"         flagdescr:"Override underlying price for theoretical calculations"  flaggroup:"pricing"`
	InterestRate           string            `flag:"interest-rate"            flagdescr:"Interest rate for theoretical pricing calculations"      flaggroup:"pricing"`
	DaysToExpiration       string            `flag:"days-to-expiration"       flagdescr:"Days to expiration for theoretical pricing calculations" flaggroup:"pricing"`
}

// NewChainCmd returns the Cobra command for option chain operations.
func NewChainCmd(c *client.Ref, w io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "chain",
		Short:   "Option chain operations",
		GroupID: groupIDMarketData,
		RunE:    requireSubcommand,
	}
	cmd.SetFlagErrorFunc(suggestSubcommands)

	cmd.AddCommand(newChainGetCmd(c, w))
	cmd.AddCommand(newChainExpirationCmd(c, w))

	return cmd
}

// newChainGetCmd returns the Cobra subcommand for retrieving an option chain.
func newChainGetCmd(c *client.Ref, w io.Writer) *cobra.Command {
	opts := &chainGetOpts{}
	cmd := &cobra.Command{
		Use:   "get <symbol>",
		Short: "Get option chain for a symbol",
		Long: `Get the full option chain for a symbol with optional filtering by contract type,
strike range, expiration dates, and strategy. Use --strategy ANALYTICAL with
--volatility and --days-to-expiration for theoretical pricing. Filter to
specific moneyness with --strike-range (ITM, NTM, OTM, ALL).`,
		Example: `  schwab-agent chain get AAPL
  schwab-agent chain get AAPL --type CALL --strike-count 5
  schwab-agent chain get AAPL --from-date 2025-06-01 --to-date 2025-07-31 --type PUT
  schwab-agent chain get AAPL --strategy ANALYTICAL --volatility 30.5 --days-to-expiration 45
  schwab-agent chain get AAPL --strike-range NTM --include-underlying-quote`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateCobraOptions(cmd.Context(), opts); err != nil {
				return err
			}

			params, err := optionChainParams(args[0], opts)
			if err != nil {
				return err
			}

			chain, err := c.OptionChain(cmd.Context(), params)
			if err != nil {
				return err
			}
			return output.WriteSuccess(w, chain, output.NewMetadata())
		},
	}

	defineCobraFlags(cmd, opts)
	cmd.SetFlagErrorFunc(normalizeFlagValidationErrorFunc)

	return cmd
}

// newChainExpirationCmd returns the Cobra subcommand for retrieving option expiration dates.
func newChainExpirationCmd(c *client.Ref, w io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "expiration <symbol>",
		Short: "Get expiration dates for a symbol",
		Long: `Get available option expiration dates for a symbol without fetching the full
chain data. Useful for discovering valid expirations before requesting a
detailed chain.`,
		Example: "  schwab-agent chain expiration AAPL",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain, err := c.ExpirationChainForSymbol(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return output.WriteSuccess(w, chain, output.NewMetadata())
		},
	}
}

func optionChainParams(symbol string, opts *chainGetOpts) (*marketdata.OptionChainParams, error) {
	strikeCount, err := optionalPositiveInt(opts.StrikeCount, "strike-count")
	if err != nil {
		return nil, err
	}

	interval, err := optionalPositiveFloat(opts.Interval, "interval")
	if err != nil {
		return nil, err
	}

	strike, err := optionalPositiveFloat(opts.Strike, "strike")
	if err != nil {
		return nil, err
	}

	volatility, err := optionalPositiveFloat(opts.Volatility, "volatility")
	if err != nil {
		return nil, err
	}

	underlyingPrice, err := optionalPositiveFloat(opts.UnderlyingPrice, "underlying-price")
	if err != nil {
		return nil, err
	}

	interestRate, err := optionalFloat(opts.InterestRate, "interest-rate")
	if err != nil {
		return nil, err
	}

	daysToExpiration, err := optionalPositiveInt(opts.DaysToExpiration, "days-to-expiration")
	if err != nil {
		return nil, err
	}

	return &marketdata.OptionChainParams{
		Symbol:                 symbol,
		ContractType:           marketdata.OptionChainContractType(opts.Type),
		StrikeCount:            strikeCount,
		IncludeUnderlyingQuote: opts.IncludeUnderlyingQuote,
		Strategy:               marketdata.OptionChainStrategy(opts.Strategy),
		Interval:               interval,
		Strike:                 strike,
		Range:                  marketdata.OptionChainRange(opts.StrikeRange),
		FromDate:               opts.FromDate,
		ToDate:                 opts.ToDate,
		Volatility:             volatility,
		UnderlyingPrice:        underlyingPrice,
		InterestRate:           interestRate,
		DaysToExpiration:       daysToExpiration,
	}, nil
}

func optionalPositiveInt(raw, flagName string) (int, error) {
	value, err := optionalInt(raw, flagName)
	if err != nil || value == 0 {
		return value, err
	}
	if value < 0 {
		return 0, apperr.NewValidationError(
			fmt.Sprintf("invalid %s: %s (must be > 0)", flagName, raw),
			nil,
		)
	}
	return value, nil
}

func optionalFloat(raw, flagName string) (float64, error) {
	if raw == "" {
		return 0, nil
	}

	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, apperr.NewValidationError(fmt.Sprintf("invalid %s: %s", flagName, raw), err)
	}
	return value, nil
}

func optionalPositiveFloat(raw, flagName string) (float64, error) {
	value, err := optionalFloat(raw, flagName)
	if err != nil || value == 0 {
		return value, err
	}
	if value < 0 {
		return 0, apperr.NewValidationError(
			fmt.Sprintf("invalid %s: %s (must be > 0)", flagName, raw),
			nil,
		)
	}
	return value, nil
}
