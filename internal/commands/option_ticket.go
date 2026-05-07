package commands

import (
	"io"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/major/schwab-go/schwab/marketdata"

	"github.com/major/schwab-agent/internal/client"
	"github.com/major/schwab-agent/internal/models"
)

const strikeComparisonEpsilon = 0.0001

// NewOptionCmd returns the Cobra command for option planning workflows.
func NewOptionCmd(c *client.Ref, w io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:     commandUseOption,
		Short:   "Option planning workflows",
		Long:    "Option planning workflows that combine quote, chain, and symbol context for agents.",
		RunE:    requireSubcommand,
		GroupID: groupIDMarketData,
	}
	cmd.SetFlagErrorFunc(suggestSubcommands)

	cmd.AddCommand(newOptionExpirationsCmd(c, w))
	cmd.AddCommand(newOptionChainCmd(c, w))
	cmd.AddCommand(newOptionContractCmd(c, w))

	return cmd
}

// optionTicketChainParams keeps Schwab's chain response narrow enough for LLM-friendly output.
func optionTicketChainParams(
	symbol string,
	expiration time.Time,
	putCall models.PutCall,
	strike float64,
) *marketdata.OptionChainParams {
	expirationDate := expiration.Format("2006-01-02")
	return &marketdata.OptionChainParams{
		Symbol:                 symbol,
		ContractType:           marketdata.OptionChainContractType(putCall),
		Strategy:               marketdata.OptionChainStrategySingle,
		FromDate:               expirationDate,
		ToDate:                 expirationDate,
		IncludeUnderlyingQuote: true,
		Strike:                 strike,
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
func matchingStrikeContracts(
	strikes map[string][]*models.OptionContract,
	strike float64,
) []*models.OptionContract {
	var matches []*models.OptionContract
	for strikeKey, contracts := range strikes {
		parsedStrike, err := strconv.ParseFloat(strikeKey, 64)
		if err == nil && nearlyEqual(parsedStrike, strike) {
			matches = append(matches, contracts...)
			continue
		}

		for _, contract := range contracts {
			if contract.StrikePrice != nil && nearlyEqual(*contract.StrikePrice, strike) {
				matches = append(matches, contract)
			}
		}
	}
	return matches
}

// nearlyEqual compares decimal prices from CLI flags and Schwab string keys safely.
func nearlyEqual(a, b float64) bool {
	return math.Abs(a-b) < strikeComparisonEpsilon
}

// contractSymbol returns the contract symbol for deterministic sorting.
func contractSymbol(contract *models.OptionContract) string {
	if contract == nil {
		return ""
	}
	return stringValue(contract.Symbol)
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func floatValue(value *float64) float64 {
	if value == nil {
		return 0
	}
	return *value
}

func boolValue(value *bool) bool {
	if value == nil {
		return false
	}
	return *value
}

func intValue(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}
