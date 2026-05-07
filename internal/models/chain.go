package models

import (
	"encoding/json"

	"github.com/major/schwab-go/schwab/marketdata"
)

// OptionChain represents an option chain for a symbol.
//
// The command layer now uses schwab-go for the option-chain request mechanics,
// but this agent-facing model intentionally keeps the historical Schwab JSON
// field names. schwab-go v0.1.x names several contract fields with a "Price"
// suffix, while live Schwab chain payloads and prior CLI output use bid, ask,
// last, mark, and inTheMoney. Keeping this model avoids silently zeroing prices
// or breaking downstream agents while the dependency catches up.
type OptionChain struct {
	Symbol            *string     `json:"symbol,omitempty"`
	Status            *string     `json:"status,omitempty"`
	Underlying        *Underlying `json:"underlying,omitempty"`
	Strategy          *string     `json:"strategy,omitempty"`
	Interval          *float64    `json:"interval,omitempty"`
	IsDelayed         *bool       `json:"isDelayed,omitempty"`
	IsIndex           *bool       `json:"isIndex,omitempty"`
	InterestRate      *float64    `json:"interestRate,omitempty"`
	UnderlyingPrice   *float64    `json:"underlyingPrice,omitempty"`
	Volatility        *float64    `json:"volatility,omitempty"`
	DaysToExpiration  *float64    `json:"daysToExpiration,omitempty"`
	DividendYield     *float64    `json:"dividendYield,omitempty"`
	NumberOfContracts *int        `json:"numberOfContracts,omitempty"`
	AssetMainType     *string     `json:"assetMainType,omitempty"`
	AssetSubType      *string     `json:"assetSubType,omitempty"`
	IsChainTruncated  *bool       `json:"isChainTruncated,omitempty"`
	// Each expiration date maps to a set of strike prices, each containing
	// a slice of contracts. Schwab returns an array even when only one contract
	// matches the expiration and strike.
	CallExpDateMap map[string]map[string][]*OptionContract `json:"callExpDateMap,omitempty"`
	PutExpDateMap  map[string]map[string][]*OptionContract `json:"putExpDateMap,omitempty"`
}

// Underlying represents the underlying instrument for an option.
type Underlying struct {
	Ask               *float64 `json:"ask,omitempty"`
	AskSize           *int     `json:"askSize,omitempty"`
	Bid               *float64 `json:"bid,omitempty"`
	BidSize           *int     `json:"bidSize,omitempty"`
	Change            *float64 `json:"change,omitempty"`
	Close             *float64 `json:"close,omitempty"`
	Description       *string  `json:"description,omitempty"`
	ExchangeName      *string  `json:"exchangeName,omitempty"`
	ExpirationDate    *int64   `json:"expirationDate,omitempty"`
	HighPrice         *float64 `json:"highPrice,omitempty"`
	Last              *float64 `json:"last,omitempty"`
	LowPrice          *float64 `json:"lowPrice,omitempty"`
	Mark              *float64 `json:"mark,omitempty"`
	MarkChange        *float64 `json:"markChange,omitempty"`
	MarkPercentChange *float64 `json:"markPercentChange,omitempty"`
	OpenPrice         *float64 `json:"openPrice,omitempty"`
	PercentChange     *float64 `json:"percentChange,omitempty"`
	QuoteTime         *int64   `json:"quoteTime,omitempty"`
	Symbol            *string  `json:"symbol,omitempty"`
	TotalVolume       *int64   `json:"totalVolume,omitempty"`
	TradeTime         *int64   `json:"tradeTime,omitempty"`
}

// OptionContract represents a single option contract.
// Field names match the Schwab API response (for example, "bid" not "bidPrice").
type OptionContract struct {
	PutCall                *string              `json:"putCall,omitempty"`
	Symbol                 *string              `json:"symbol,omitempty"`
	Description            *string              `json:"description,omitempty"`
	ExchangeName           *string              `json:"exchangeName,omitempty"`
	Bid                    *float64             `json:"bid,omitempty"`
	Ask                    *float64             `json:"ask,omitempty"`
	Last                   *float64             `json:"last,omitempty"`
	Mark                   *float64             `json:"mark,omitempty"`
	BidSize                *int                 `json:"bidSize,omitempty"`
	AskSize                *int                 `json:"askSize,omitempty"`
	BidAskSize             *string              `json:"bidAskSize,omitempty"`
	LastSize               *int                 `json:"lastSize,omitempty"`
	OpenPrice              *float64             `json:"openPrice,omitempty"`
	HighPrice              *float64             `json:"highPrice,omitempty"`
	LowPrice               *float64             `json:"lowPrice,omitempty"`
	ClosePrice             *float64             `json:"closePrice,omitempty"`
	TotalVolume            *int64               `json:"totalVolume,omitempty"`
	OpenInterest           *int64               `json:"openInterest,omitempty"`
	MarkChange             *float64             `json:"markChange,omitempty"`
	MarkPercentChange      *float64             `json:"markPercentChange,omitempty"`
	StrikePrice            *float64             `json:"strikePrice,omitempty"`
	ExpirationDate         *string              `json:"expirationDate,omitempty"`
	DaysToExpiration       *int                 `json:"daysToExpiration,omitempty"`
	ExpirationType         *ExpirationType      `json:"expirationType,omitempty"`
	LastTradingDay         *int64               `json:"lastTradingDay,omitempty"`
	QuoteTimeInLong        *int64               `json:"quoteTimeInLong,omitempty"`
	TradeTimeInLong        *int64               `json:"tradeTimeInLong,omitempty"`
	NetChange              *float64             `json:"netChange,omitempty"`
	PercentChange          *float64             `json:"percentChange,omitempty"`
	Delta                  *float64             `json:"delta,omitempty"`
	Gamma                  *float64             `json:"gamma,omitempty"`
	Theta                  *float64             `json:"theta,omitempty"`
	Vega                   *float64             `json:"vega,omitempty"`
	Rho                    *float64             `json:"rho,omitempty"`
	TimeValue              *float64             `json:"timeValue,omitempty"`
	TheoreticalOptionValue *float64             `json:"theoreticalOptionValue,omitempty"`
	TheoreticalVolatility  *float64             `json:"theoreticalVolatility,omitempty"`
	OptionDeliverablesList []OptionDeliverables `json:"optionDeliverablesList,omitempty"`
	InTheMoney             *bool                `json:"inTheMoney,omitempty"`
	Mini                   *bool                `json:"mini,omitempty"`
	NonStandard            *bool                `json:"nonStandard,omitempty"`
	PennyPilot             *bool                `json:"pennyPilot,omitempty"`
	OptionRoot             *string              `json:"optionRoot,omitempty"`
	Multiplier             *float64             `json:"multiplier,omitempty"`
	SettlementType         *SettlementType      `json:"settlementType,omitempty"`
	DeliverableNote        *string              `json:"deliverableNote,omitempty"`
	IntrinsicValue         *float64             `json:"intrinsicValue,omitempty"`
	ExtrinsicValue         *float64             `json:"extrinsicValue,omitempty"`
	ExerciseType           *string              `json:"exerciseType,omitempty"`
	High52Week             *float64             `json:"high52Week,omitempty"`
	Low52Week              *float64             `json:"low52Week,omitempty"`
	Volatility             *float64             `json:"volatility,omitempty"`
}

// UnmarshalJSON accepts both Schwab's observed chain contract field names and
// schwab-go's v0.1.x field names. This keeps local command output stable while
// allowing fixtures copied from schwab-go tests to decode correctly.
func (o *OptionContract) UnmarshalJSON(data []byte) error {
	type optionContract OptionContract
	var raw struct {
		*optionContract

		BidPrice      *float64 `json:"bidPrice"`
		AskPrice      *float64 `json:"askPrice"`
		LastPrice     *float64 `json:"lastPrice"`
		MarkPrice     *float64 `json:"markPrice"`
		IsInTheMoney  *bool    `json:"isInTheMoney"`
		IsMini        *bool    `json:"isMini"`
		IsNonStandard *bool    `json:"isNonStandard"`
		IsPennyPilot  *bool    `json:"isPennyPilot"`
	}
	raw.optionContract = (*optionContract)(o)
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	assignFallbackFloat(&o.Bid, raw.BidPrice)
	assignFallbackFloat(&o.Ask, raw.AskPrice)
	assignFallbackFloat(&o.Last, raw.LastPrice)
	assignFallbackFloat(&o.Mark, raw.MarkPrice)
	assignFallbackBool(&o.InTheMoney, raw.IsInTheMoney)
	assignFallbackBool(&o.Mini, raw.IsMini)
	assignFallbackBool(&o.NonStandard, raw.IsNonStandard)
	assignFallbackBool(&o.PennyPilot, raw.IsPennyPilot)
	return nil
}

func assignFallbackFloat(dst **float64, fallback *float64) {
	if *dst == nil && fallback != nil {
		*dst = fallback
	}
}

func assignFallbackBool(dst **bool, fallback *bool) {
	if *dst == nil && fallback != nil {
		*dst = fallback
	}
}

// OptionDeliverable is an alias for schwab-go's singular deliverable type.
type OptionDeliverable = marketdata.OptionDeliverable

// OptionDeliverables represents deliverables for an option contract.
type OptionDeliverables struct {
	Symbol           *string  `json:"symbol,omitempty"`
	DeliverableUnits *float64 `json:"deliverableUnits,omitempty"`
	CurrencyType     *string  `json:"currencyType,omitempty"`
	AssetType        *string  `json:"assetType,omitempty"`
	Position         *int     `json:"position,omitempty"`
}
