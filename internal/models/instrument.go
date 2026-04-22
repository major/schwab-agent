package models

// Instrument represents a financial instrument.
type Instrument struct {
	Cusip       *string      `json:"cusip,omitempty"`
	Symbol      *string      `json:"symbol,omitempty"`
	Description *string      `json:"description,omitempty"`
	Exchange    *string      `json:"exchange,omitempty"`
	AssetType   *string      `json:"assetType,omitempty"`
	Type        *string      `json:"type,omitempty"`
	Fundamental *Fundamental `json:"fundamental,omitempty"`
}

// InstrumentResponse represents the response from an instrument search.
// The Schwab API wraps instrument results in {"instruments": [...]} for both
// search and single-instrument-by-CUSIP endpoints.
type InstrumentResponse struct {
	Instruments []Instrument `json:"instruments,omitempty"`
}

// Fundamental represents fundamental data for an instrument, returned when
// using the "fundamental" projection on the instruments search endpoint.
// Field names match the Schwab API response exactly.
type Fundamental struct {
	Symbol              *string  `json:"symbol,omitempty"`
	High52              *float64 `json:"high52,omitempty"`
	Low52               *float64 `json:"low52,omitempty"`
	DividendAmount      *float64 `json:"dividendAmount,omitempty"`
	DividendYield       *float64 `json:"dividendYield,omitempty"`
	DividendDate        *string  `json:"dividendDate,omitempty"`
	DividendPayAmount   *float64 `json:"dividendPayAmount,omitempty"`
	DividendPayDate     *string  `json:"dividendPayDate,omitempty"`
	DividendFreq        *int     `json:"dividendFreq,omitempty"`
	DivGrowthRate3Year  *float64 `json:"divGrowthRate3Year,omitempty"`
	DeclarationDate     *string  `json:"declarationDate,omitempty"`
	NextDividendPayDate *string  `json:"nextDividendPayDate,omitempty"`
	NextDividendDate    *string  `json:"nextDividendDate,omitempty"`
	PeRatio             *float64 `json:"peRatio,omitempty"`
	PegRatio            *float64 `json:"pegRatio,omitempty"`
	PbRatio             *float64 `json:"pbRatio,omitempty"`
	PrRatio             *float64 `json:"prRatio,omitempty"`
	PcfRatio            *float64 `json:"pcfRatio,omitempty"`
	GrossMarginTTM      *float64 `json:"grossMarginTTM,omitempty"`
	GrossMarginMRQ      *float64 `json:"grossMarginMRQ,omitempty"`
	NetProfitMarginTTM  *float64 `json:"netProfitMarginTTM,omitempty"`
	NetProfitMarginMRQ  *float64 `json:"netProfitMarginMRQ,omitempty"`
	OperatingMarginTTM  *float64 `json:"operatingMarginTTM,omitempty"`
	OperatingMarginMRQ  *float64 `json:"operatingMarginMRQ,omitempty"`
	ReturnOnEquity      *float64 `json:"returnOnEquity,omitempty"`
	ReturnOnAssets      *float64 `json:"returnOnAssets,omitempty"`
	ReturnOnInvestment  *float64 `json:"returnOnInvestment,omitempty"`
	QuickRatio          *float64 `json:"quickRatio,omitempty"`
	CurrentRatio        *float64 `json:"currentRatio,omitempty"`
	InterestCoverage    *float64 `json:"interestCoverage,omitempty"`
	TotalDebtToCapital  *float64 `json:"totalDebtToCapital,omitempty"`
	LtDebtToEquity      *float64 `json:"ltDebtToEquity,omitempty"`
	TotalDebtToEquity   *float64 `json:"totalDebtToEquity,omitempty"`
	EpsTTM              *float64 `json:"epsTTM,omitempty"`
	EpsChangePercentTTM *float64 `json:"epsChangePercentTTM,omitempty"`
	EpsChangeYear       *float64 `json:"epsChangeYear,omitempty"`
	EpsChange           *float64 `json:"epsChange,omitempty"`
	Eps                 *float64 `json:"eps,omitempty"`
	RevChangeYear       *float64 `json:"revChangeYear,omitempty"`
	RevChangeTTM        *float64 `json:"revChangeTTM,omitempty"`
	RevChangeIn         *float64 `json:"revChangeIn,omitempty"`
	SharesOutstanding   *float64 `json:"sharesOutstanding,omitempty"`
	MarketCapFloat      *float64 `json:"marketCapFloat,omitempty"`
	MarketCap           *float64 `json:"marketCap,omitempty"`
	BookValuePerShare   *float64 `json:"bookValuePerShare,omitempty"`
	ShortIntToFloat     *float64 `json:"shortIntToFloat,omitempty"`
	ShortIntDayToCover  *float64 `json:"shortIntDayToCover,omitempty"`
	Beta                *float64 `json:"beta,omitempty"`
	Vol1DayAvg          *float64 `json:"vol1DayAvg,omitempty"`
	Vol10DayAvg         *float64 `json:"vol10DayAvg,omitempty"`
	Vol3MonthAvg        *float64 `json:"vol3MonthAvg,omitempty"`
	Avg10DaysVolume     *float64 `json:"avg10DaysVolume,omitempty"`
	Avg1DayVolume       *float64 `json:"avg1DayVolume,omitempty"`
	Avg3MonthVolume     *float64 `json:"avg3MonthVolume,omitempty"`
	DtnVolume           *float64 `json:"dtnVolume,omitempty"`
	FundLeverageFactor  *float64 `json:"fundLeverageFactor,omitempty"`
}
