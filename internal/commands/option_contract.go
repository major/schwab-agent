package commands

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/major/schwab-agent/internal/apperr"
	"github.com/major/schwab-agent/internal/client"
	"github.com/major/schwab-agent/internal/models"
	"github.com/major/schwab-agent/internal/orderbuilder"
	"github.com/major/schwab-agent/internal/output"
)

// optionContractData is the agent-facing payload for a single option contract lookup.
type optionContractData struct {
	Underlying      string                        `json:"underlying"`
	Expiration      string                        `json:"expiration"`
	Strike          float64                       `json:"strike"`
	PutCall         models.PutCall                `json:"putCall"`
	OCCSymbol       string                        `json:"occSymbol"`
	UnderlyingQuote optionContractUnderlyingQuote `json:"underlyingQuote"`
	Contract        optionContractDetail          `json:"contract"`
}

// optionContractUnderlyingQuote is the curated underlying quote from the chain response.
type optionContractUnderlyingQuote struct {
	Symbol        string  `json:"symbol"`
	Bid           float64 `json:"bid"`
	Ask           float64 `json:"ask"`
	Mark          float64 `json:"mark"`
	Last          float64 `json:"last"`
	NetChange     float64 `json:"netChange"`
	PercentChange float64 `json:"percentChange"`
	TotalVolume   int64   `json:"totalVolume"`
}

// optionContractDetail is the curated fields from the matched option contract.
type optionContractDetail struct {
	Symbol                 string  `json:"symbol"`
	Bid                    float64 `json:"bid"`
	Ask                    float64 `json:"ask"`
	Mark                   float64 `json:"mark"`
	Delta                  float64 `json:"delta"`
	Volatility             float64 `json:"volatility"`
	OpenInterest           int64   `json:"openInterest"`
	TotalVolume            int64   `json:"totalVolume"`
	InTheMoney             bool    `json:"inTheMoney"`
	TheoreticalOptionValue float64 `json:"theoreticalOptionValue"`
	Gamma                  float64 `json:"gamma"`
	Theta                  float64 `json:"theta"`
	Vega                   float64 `json:"vega"`
	Rho                    float64 `json:"rho"`
	Last                   float64 `json:"last"`
	HighPrice              float64 `json:"highPrice"`
	LowPrice               float64 `json:"lowPrice"`
	OpenPrice              float64 `json:"openPrice"`
	ClosePrice             float64 `json:"closePrice"`
	NetChange              float64 `json:"netChange"`
	PercentChange          float64 `json:"percentChange"`
	DaysToExpiration       int     `json:"daysToExpiration"`
}

// newOptionContractCmd returns the command that fetches a single curated option contract.
func newOptionContractCmd(c *client.Ref, w io.Writer) *cobra.Command {
	opts := &optionContractOpts{}
	cmd := &cobra.Command{
		Use:   "contract <symbol>",
		Short: "Get a single curated option contract with quote context",
		Long: `Fetch one option contract by underlying, expiration, strike, and side.
Returns the contract details along with the underlying quote from the option
chain response. Use this for single-contract lookups before order placement.`,
		Example: `  schwab-agent option contract AAPL --expiration 2026-06-19 --strike 200 --call
	  schwab-agent option contract TSLA --expiration 2026-03-20 --strike 180 --put
	  schwab-agent option contract MSFT --expiration 2026-01-16 --strike 450 --call`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateCobraOptions(cmd.Context(), opts); err != nil {
				return err
			}

			data, err := fetchOptionContract(cmd.Context(), c, args[0], opts)
			if err != nil {
				return err
			}

			meta := output.NewMetadata()
			meta.Returned = 1
			return output.WriteSuccess(w, data, meta)
		},
	}

	defineCobraFlags(cmd, opts)
	cmd.MarkFlagsOneRequired(flagCall, flagPut)
	cmd.MarkFlagsMutuallyExclusive(flagCall, flagPut)
	cmd.SetFlagErrorFunc(normalizeFlagValidationErrorFunc)

	return cmd
}

// fetchOptionContract fetches the option chain and extracts a single matching contract.
func fetchOptionContract(
	ctx context.Context,
	c *client.Ref,
	rawSymbol string,
	opts *optionContractOpts,
) (*optionContractData, error) {
	symbol := strings.ToUpper(strings.TrimSpace(rawSymbol))
	if symbol == "" {
		return nil, newValidationError("symbol is required")
	}

	expiration, err := parseExpiration(opts.Expiration)
	if err != nil {
		return nil, err
	}

	putCall, err := parsePutCall(opts.Call, opts.Put)
	if err != nil {
		return nil, err
	}

	if opts.Strike <= 0 {
		return nil, newValidationError("strike must be greater than zero")
	}

	chain, err := c.OptionChain(ctx, optionTicketChainParams(symbol, expiration, putCall, opts.Strike))
	if err != nil {
		return nil, fmt.Errorf(
			"fetch option chain for %s %s %s %g: %w",
			symbol,
			expiration.Format("2006-01-02"),
			putCall,
			opts.Strike,
			err,
		)
	}

	contracts := matchingTicketContracts(chain, expiration, putCall, opts.Strike)
	if len(contracts) == 0 {
		expirationStr := expiration.Format("2006-01-02")
		return nil, apperr.NewSymbolNotFoundError(
			fmt.Sprintf(
				"no contract found for %s %s %s %g; try: option chain %s --expiration %s --type %s --strike-count 10",
				symbol, expirationStr, putCall, opts.Strike,
				symbol, expirationStr, putCall,
			),
			nil,
		)
	}

	// Use the first matching contract. The narrow chain filter (exact expiration,
	// strike, and contract type) should return exactly one.
	matched := contracts[0]

	return &optionContractData{
		Underlying:      symbol,
		Expiration:      expiration.Format("2006-01-02"),
		Strike:          opts.Strike,
		PutCall:         putCall,
		OCCSymbol:       orderbuilder.BuildOCCSymbol(symbol, expiration, opts.Strike, string(putCall)),
		UnderlyingQuote: curateUnderlyingQuote(chain.Underlying),
		Contract:        curateContract(matched),
	}, nil
}

// curateContract extracts the agent-relevant fields from a matched option contract.
func curateContract(c *models.OptionContract) optionContractDetail {
	if c == nil {
		return optionContractDetail{}
	}

	return optionContractDetail{
		Symbol:                 stringValue(c.Symbol),
		Bid:                    floatValue(c.Bid),
		Ask:                    floatValue(c.Ask),
		Mark:                   floatValue(c.Mark),
		Delta:                  floatValue(c.Delta),
		Volatility:             floatValue(c.Volatility),
		OpenInterest:           int64Value(c.OpenInterest),
		TotalVolume:            int64Value(c.TotalVolume),
		InTheMoney:             boolValue(c.InTheMoney),
		TheoreticalOptionValue: floatValue(c.TheoreticalOptionValue),
		Gamma:                  floatValue(c.Gamma),
		Theta:                  floatValue(c.Theta),
		Vega:                   floatValue(c.Vega),
		Rho:                    floatValue(c.Rho),
		Last:                   floatValue(c.Last),
		HighPrice:              floatValue(c.HighPrice),
		LowPrice:               floatValue(c.LowPrice),
		OpenPrice:              floatValue(c.OpenPrice),
		ClosePrice:             floatValue(c.ClosePrice),
		NetChange:              floatValue(c.NetChange),
		PercentChange:          floatValue(c.PercentChange),
		DaysToExpiration:       intValue(c.DaysToExpiration),
	}
}

// curateUnderlyingQuote extracts the agent-relevant underlying quote fields from the chain.
func curateUnderlyingQuote(u *models.Underlying) optionContractUnderlyingQuote {
	if u == nil {
		return optionContractUnderlyingQuote{}
	}

	return optionContractUnderlyingQuote{
		Symbol:        stringValue(u.Symbol),
		Bid:           floatValue(u.Bid),
		Ask:           floatValue(u.Ask),
		Mark:          floatValue(u.Mark),
		Last:          floatValue(u.Last),
		NetChange:     floatValue(u.Change),
		PercentChange: floatValue(u.PercentChange),
		TotalVolume:   int64Value(u.TotalVolume),
	}
}

// int64Value safely dereferences a *int64, returning 0 for nil.
func int64Value(value *int64) int64 {
	if value == nil {
		return 0
	}

	return *value
}
