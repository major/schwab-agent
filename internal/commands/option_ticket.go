package commands

import (
	"context"
	"fmt"
	"io"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/major/schwab-agent/internal/apperr"
	"github.com/major/schwab-agent/internal/client"
	"github.com/major/schwab-agent/internal/models"
	"github.com/major/schwab-agent/internal/orderbuilder"
	"github.com/major/schwab-agent/internal/output"
)

const strikeComparisonEpsilon = 0.0001

// optionTicketGetOpts holds the inputs for narrowing an option chain to one actionable contract.
type optionTicketGetOpts struct {
	Expiration string  `flag:"expiration" flagdescr:"Option expiration date (YYYY-MM-DD)" flaggroup:"contract"`
	Strike     float64 `flag:"strike"     flagdescr:"Option strike price"                 flaggroup:"contract"`
	Call       bool    `flag:"call"       flagdescr:"Select the call contract"            flaggroup:"contract"`
	Put        bool    `flag:"put"        flagdescr:"Select the put contract"             flaggroup:"contract"`
}

// optionTicketData is the agent-facing payload for an option ticket lookup.
type optionTicketData struct {
	Symbol          string                   `json:"symbol"`
	Expiration      string                   `json:"expiration"`
	Strike          float64                  `json:"strike"`
	PutCall         models.PutCall           `json:"putCall"`
	OCCSymbol       string                   `json:"occSymbol"`
	UnderlyingQuote *models.QuoteEquity      `json:"underlyingQuote"`
	Chain           optionTicketChainSummary `json:"chain"`
	Contracts       []*models.OptionContract `json:"contracts"`
}

// optionTicketChainSummary preserves chain-level context without returning the full chain.
type optionTicketChainSummary struct {
	Status            *string            `json:"status,omitempty"`
	Underlying        *models.Underlying `json:"underlying,omitempty"`
	UnderlyingPrice   *float64           `json:"underlyingPrice,omitempty"`
	IsDelayed         *bool              `json:"isDelayed,omitempty"`
	IsChainTruncated  *bool              `json:"isChainTruncated,omitempty"`
	NumberOfContracts *int               `json:"numberOfContracts,omitempty"`
}

// NewOptionCmd returns the Cobra command for option planning workflows.
func NewOptionCmd(c *client.Ref, w io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "option",
		Short:   "Option planning workflows",
		Long:    "Option planning workflows that combine quote, chain, and symbol context for agents.",
		RunE:    requireSubcommand,
		GroupID: "market-data",
	}
	cmd.SetFlagErrorFunc(suggestSubcommands)

	cmd.AddCommand(newOptionTicketCmd(c, w))

	return cmd
}

// newOptionTicketCmd returns the parent command for option ticket workflows.
func newOptionTicketCmd(c *client.Ref, w io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ticket",
		Short: "Build an option planning ticket",
		Long: `Build an option planning ticket from live market data. The ticket combines
the underlying quote, a narrowly filtered option chain, the matching contract,
and the OCC symbol in one read-only command. Use order preview option when you
are ready to turn the ticket into a Schwab preview request.`,
		RunE: requireSubcommand,
	}
	cmd.SetFlagErrorFunc(suggestSubcommands)

	cmd.AddCommand(newOptionTicketGetCmd(c, w))

	return cmd
}

// newOptionTicketGetCmd returns the command that resolves one option contract ticket.
func newOptionTicketGetCmd(c *client.Ref, w io.Writer) *cobra.Command {
	opts := &optionTicketGetOpts{}
	cmd := &cobra.Command{
		Use:   "get <symbol>",
		Short: "Get quote, chain, and OCC context for one option contract",
		Long: `Get a compact option ticket for one underlying, expiration, strike, and
contract side. This collapses the common agent workflow of quote lookup, chain
lookup, and OCC symbol construction into one read-only CLI call.`,
		Example: `  schwab-agent option ticket get AAPL --expiration 2026-01-16 --strike 200 --call
  schwab-agent option ticket get TSLA --expiration 2026-03-20 --strike 180 --put`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateCobraOptions(cmd.Context(), opts); err != nil {
				return err
			}

			ticket, err := buildOptionTicket(cmd.Context(), c, args[0], opts)
			if err != nil {
				return err
			}

			return output.WriteSuccess(w, ticket, output.NewMetadata())
		},
	}

	defineCobraFlags(cmd, opts)
	cmd.MarkFlagsOneRequired("call", "put")
	cmd.MarkFlagsMutuallyExclusive("call", "put")
	cmd.SetFlagErrorFunc(normalizeFlagValidationErrorFunc)

	return cmd
}

// buildOptionTicket fetches the quote and filtered option chain needed for an agent order ticket.
func buildOptionTicket(
	ctx context.Context,
	c *client.Ref,
	rawSymbol string,
	opts *optionTicketGetOpts,
) (*optionTicketData, error) {
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

	quote, err := c.Quote(ctx, symbol, client.QuoteParams{Fields: []string{"quote", "fundamental", "reference"}})
	if err != nil {
		return nil, err
	}

	chain, err := c.OptionChain(ctx, symbol, optionTicketChainParams(expiration, putCall, opts.Strike))
	if err != nil {
		return nil, err
	}

	contracts := matchingTicketContracts(chain, expiration, putCall, opts.Strike)
	if len(contracts) == 0 {
		return nil, apperr.NewSymbolNotFoundError(
			fmt.Sprintf(
				"option contract not found for %s %s %.3f %s",
				symbol,
				expiration.Format("2006-01-02"),
				opts.Strike,
				putCall,
			),
			nil,
		)
	}

	return &optionTicketData{
		Symbol:          symbol,
		Expiration:      expiration.Format("2006-01-02"),
		Strike:          opts.Strike,
		PutCall:         putCall,
		OCCSymbol:       orderbuilder.BuildOCCSymbol(symbol, expiration, opts.Strike, string(putCall)),
		UnderlyingQuote: quote,
		Chain:           summarizeTicketChain(chain),
		Contracts:       contracts,
	}, nil
}

// optionTicketChainParams keeps Schwab's chain response narrow enough for LLM-friendly output.
func optionTicketChainParams(expiration time.Time, putCall models.PutCall, strike float64) *client.ChainParams {
	expirationDate := expiration.Format("2006-01-02")
	return &client.ChainParams{
		ContractType:           string(putCall),
		Strategy:               string(chainStrategySingle),
		FromDate:               expirationDate,
		ToDate:                 expirationDate,
		IncludeUnderlyingQuote: "true",
		Strike:                 strconv.FormatFloat(strike, 'f', -1, 64),
	}
}

// matchingTicketContracts extracts the matching expiration and strike contracts from a chain response.
func matchingTicketContracts(
	chain *models.OptionChain,
	expiration time.Time,
	putCall models.PutCall,
	strike float64,
) []*models.OptionContract {
	if chain == nil {
		return nil
	}

	contractMap := chain.CallExpDateMap
	if putCall == models.PutCallPut {
		contractMap = chain.PutExpDateMap
	}

	expirationPrefix := expiration.Format("2006-01-02")
	var matches []*models.OptionContract
	for expirationKey, strikes := range contractMap {
		if !strings.HasPrefix(expirationKey, expirationPrefix) {
			continue
		}

		matches = append(matches, matchingStrikeContracts(strikes, strike)...)
	}

	slices.SortFunc(matches, func(a, b *models.OptionContract) int {
		return strings.Compare(contractSymbol(a), contractSymbol(b))
	})
	return matches
}

// matchingStrikeContracts returns contracts whose strike key or contract field matches the requested strike.
func matchingStrikeContracts(strikes map[string][]*models.OptionContract, strike float64) []*models.OptionContract {
	var matches []*models.OptionContract
	for strikeKey, contracts := range strikes {
		parsedStrike, err := strconv.ParseFloat(strikeKey, 64)
		if err == nil && nearlyEqual(parsedStrike, strike) {
			matches = append(matches, contracts...)
			continue
		}

		for _, contract := range contracts {
			if contract != nil && contract.StrikePrice != nil && nearlyEqual(*contract.StrikePrice, strike) {
				matches = append(matches, contract)
			}
		}
	}
	return matches
}

// summarizeTicketChain retains the chain fields agents need without echoing the full option matrix.
func summarizeTicketChain(chain *models.OptionChain) optionTicketChainSummary {
	if chain == nil {
		return optionTicketChainSummary{}
	}

	return optionTicketChainSummary{
		Status:            chain.Status,
		Underlying:        chain.Underlying,
		UnderlyingPrice:   chain.UnderlyingPrice,
		IsDelayed:         chain.IsDelayed,
		IsChainTruncated:  chain.IsChainTruncated,
		NumberOfContracts: chain.NumberOfContracts,
	}
}

// nearlyEqual compares decimal prices from CLI flags and Schwab string keys safely.
func nearlyEqual(a, b float64) bool {
	return math.Abs(a-b) < strikeComparisonEpsilon
}

// contractSymbol safely dereferences optional symbols for deterministic sorting.
func contractSymbol(contract *models.OptionContract) string {
	if contract == nil || contract.Symbol == nil {
		return ""
	}
	return *contract.Symbol
}
