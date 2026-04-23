package models

import (
	"encoding/json"
	"time"
)

// SchwabTime wraps time.Time with custom JSON unmarshaling to handle the
// Schwab API's non-standard timestamp format. The API returns timestamps
// like "2026-04-21T17:25:35+0000" which omits the colon in the timezone
// offset, making it invalid RFC3339. This type tries RFC3339 first, then
// falls back to the Schwab format.
type SchwabTime struct {
	time.Time
}

// schwabTimeLayout is the non-standard ISO 8601 format used by the Schwab API.
// It differs from RFC3339 only in the timezone offset: +0000 instead of +00:00.
const schwabTimeLayout = "2006-01-02T15:04:05-0700"

// UnmarshalJSON implements json.Unmarshaler for SchwabTime.
func (st *SchwabTime) UnmarshalJSON(data []byte) error {
	var raw string
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	if raw == "" {
		return nil
	}

	// Try RFC3339 first (standard Go time format).
	t, err := time.Parse(time.RFC3339, raw)
	if err == nil {
		st.Time = t
		return nil
	}

	// Fall back to the Schwab API format (no colon in timezone offset).
	t, err = time.Parse(schwabTimeLayout, raw)
	if err != nil {
		return err
	}
	st.Time = t
	return nil
}

// MarshalJSON implements json.Marshaler for SchwabTime, using RFC3339 output.
func (st SchwabTime) MarshalJSON() ([]byte, error) {
	if st.IsZero() {
		return json.Marshal(nil)
	}
	return json.Marshal(st.Format(time.RFC3339))
}

// ContractType represents the type of contract (CALL or PUT).
type ContractType string

const (
	ContractTypeCall ContractType = "CALL"
	ContractTypePut  ContractType = "PUT"
)

// AssetMainType represents the main asset type.
type AssetMainType string

const (
	AssetMainTypeEquity     AssetMainType = "EQUITY"
	AssetMainTypeOption     AssetMainType = "OPTION"
	AssetMainTypeFuture     AssetMainType = "FUTURE"
	AssetMainTypeForex      AssetMainType = "FOREX"
	AssetMainTypeIndex      AssetMainType = "INDEX"
	AssetMainTypeMutualFund AssetMainType = "MUTUAL_FUND"
	AssetMainTypeBond       AssetMainType = "BOND"
	AssetMainTypeCommodity  AssetMainType = "COMMODITY"
)

// EquityAssetSubType represents equity asset subtypes.
type EquityAssetSubType string

const (
	EquityAssetSubTypeCommonStock EquityAssetSubType = "COMMON_STOCK"
	EquityAssetSubTypePreferred   EquityAssetSubType = "PREFERRED"
	EquityAssetSubTypeWarrant     EquityAssetSubType = "WARRANT"
)

// MutualFundAssetSubType represents mutual fund subtypes.
type MutualFundAssetSubType string

const (
	MutualFundAssetSubTypeOpenEnd   MutualFundAssetSubType = "OPEN_END"
	MutualFundAssetSubTypeClosedEnd MutualFundAssetSubType = "CLOSED_END"
)

// FundStrategy represents fund strategy types.
type FundStrategy string

const (
	FundStrategyGrowth        FundStrategy = "GROWTH"
	FundStrategyValue         FundStrategy = "VALUE"
	FundStrategyBlendedGrowth FundStrategy = "BLENDED_GROWTH"
	FundStrategyBlendedValue  FundStrategy = "BLENDED_VALUE"
)

// ExpirationType represents option expiration types.
type ExpirationType string

const (
	ExpirationTypeStandard  ExpirationType = "STANDARD"
	ExpirationTypeWeekly    ExpirationType = "WEEKLY"
	ExpirationTypeMonthly   ExpirationType = "MONTHLY"
	ExpirationTypeQuarterly ExpirationType = "QUARTERLY"
)

// SettlementType represents settlement types.
type SettlementType string

const (
	SettlementTypeRegular SettlementType = "REGULAR"
	SettlementTypeNetCash SettlementType = "NET_CASH"
)

// ExerciseType represents exercise types.
type ExerciseType string

const (
	ExerciseTypeAmerican ExerciseType = "AMERICAN"
	ExerciseTypeEuropean ExerciseType = "EUROPEAN"
)

// DivFreq represents dividend frequency.
type DivFreq string

const (
	DivFreqAnnual     DivFreq = "ANNUAL"
	DivFreqSemiAnnual DivFreq = "SEMI_ANNUAL"
	DivFreqQuarterly  DivFreq = "QUARTERLY"
	DivFreqMonthly    DivFreq = "MONTHLY"
)

// QuoteType represents quote types.
type QuoteType string

const (
	QuoteTypeEquity     QuoteType = "EQUITY"
	QuoteTypeOption     QuoteType = "OPTION"
	QuoteTypeFuture     QuoteType = "FUTURE"
	QuoteTypeForex      QuoteType = "FOREX"
	QuoteTypeIndex      QuoteType = "INDEX"
	QuoteTypeMutualFund QuoteType = "MUTUAL_FUND"
)

// TransactionType represents transaction types.
type TransactionType string

const (
	TransactionTypeTrade      TransactionType = "TRADE"
	TransactionTypeAdjustment TransactionType = "ADJUSTMENT"
	TransactionTypeDeposit    TransactionType = "DEPOSIT"
	TransactionTypeWithdrawal TransactionType = "WITHDRAWAL"
	TransactionTypeDividend   TransactionType = "DIVIDEND"
	TransactionTypeInterest   TransactionType = "INTEREST"
	TransactionTypeFee        TransactionType = "FEE"
)
