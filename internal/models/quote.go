package models

// QuoteEquity represents a full equity quote response from the Schwab API.
// The API nests data under quote, reference, fundamental, extended, and regular sub-objects.
type QuoteEquity struct {
	AssetMainType *string           `json:"assetMainType,omitempty"`
	AssetSubType  *string           `json:"assetSubType,omitempty"`
	QuoteType     *string           `json:"quoteType,omitempty"`
	Realtime      *bool             `json:"realtime,omitempty"`
	SSID          *int64            `json:"ssid,omitempty"`
	Symbol        *string           `json:"symbol,omitempty"`
	Extended      *QuoteExtended    `json:"extended,omitempty"`
	Fundamental   *QuoteFundamental `json:"fundamental,omitempty"`
	Quote         *QuoteData        `json:"quote,omitempty"`
	Reference     *QuoteReference   `json:"reference,omitempty"`
	Regular       *QuoteRegular     `json:"regular,omitempty"`
}

// QuoteData holds the core pricing and volume fields for an equity quote.
type QuoteData struct {
	WeekHigh52              *float64 `json:"52WeekHigh,omitempty"`
	WeekLow52               *float64 `json:"52WeekLow,omitempty"`
	AskMICId                *string  `json:"askMICId,omitempty"`
	AskPrice                *float64 `json:"askPrice,omitempty"`
	AskSize                 *int     `json:"askSize,omitempty"`
	AskTime                 *int64   `json:"askTime,omitempty"`
	BidMICId                *string  `json:"bidMICId,omitempty"`
	BidPrice                *float64 `json:"bidPrice,omitempty"`
	BidSize                 *int     `json:"bidSize,omitempty"`
	BidTime                 *int64   `json:"bidTime,omitempty"`
	ClosePrice              *float64 `json:"closePrice,omitempty"`
	HighPrice               *float64 `json:"highPrice,omitempty"`
	LastMICId               *string  `json:"lastMICId,omitempty"`
	LastPrice               *float64 `json:"lastPrice,omitempty"`
	LastSize                *int     `json:"lastSize,omitempty"`
	LowPrice                *float64 `json:"lowPrice,omitempty"`
	Mark                    *float64 `json:"mark,omitempty"`
	MarkChange              *float64 `json:"markChange,omitempty"`
	MarkPercentChange       *float64 `json:"markPercentChange,omitempty"`
	NetChange               *float64 `json:"netChange,omitempty"`
	NetPercentChange        *float64 `json:"netPercentChange,omitempty"`
	OpenPrice               *float64 `json:"openPrice,omitempty"`
	PostMarketChange        *float64 `json:"postMarketChange,omitempty"`
	PostMarketPercentChange *float64 `json:"postMarketPercentChange,omitempty"`
	QuoteTime               *int64   `json:"quoteTime,omitempty"`
	SecurityStatus          *string  `json:"securityStatus,omitempty"`
	TotalVolume             *int64   `json:"totalVolume,omitempty"`
	TradeTime               *int64   `json:"tradeTime,omitempty"`
}

// QuoteReference holds static reference data for a quoted instrument.
type QuoteReference struct {
	Cusip          *string  `json:"cusip,omitempty"`
	Description    *string  `json:"description,omitempty"`
	Exchange       *string  `json:"exchange,omitempty"`
	ExchangeName   *string  `json:"exchangeName,omitempty"`
	HTBRate        *float64 `json:"htbRate,omitempty"`
	IsHardToBorrow *bool    `json:"isHardToBorrow,omitempty"`
	IsShortable    *bool    `json:"isShortable,omitempty"`
	Optionable     *bool    `json:"optionable,omitempty"`
}

// QuoteFundamental holds fundamental analysis data for an equity.
type QuoteFundamental struct {
	Avg10DaysVolume    *float64 `json:"avg10DaysVolume,omitempty"`
	Avg1YearVolume     *float64 `json:"avg1YearVolume,omitempty"`
	DeclarationDate    *string  `json:"declarationDate,omitempty"`
	DivAmount          *float64 `json:"divAmount,omitempty"`
	DivExDate          *string  `json:"divExDate,omitempty"`
	DivFreq            *int     `json:"divFreq,omitempty"`
	DivPayAmount       *float64 `json:"divPayAmount,omitempty"`
	DivPayDate         *string  `json:"divPayDate,omitempty"`
	DivYield           *float64 `json:"divYield,omitempty"`
	EPS                *float64 `json:"eps,omitempty"`
	FundLeverageFactor *float64 `json:"fundLeverageFactor,omitempty"`
	LastEarningsDate   *string  `json:"lastEarningsDate,omitempty"`
	NextDivExDate      *string  `json:"nextDivExDate,omitempty"`
	NextDivPayDate     *string  `json:"nextDivPayDate,omitempty"`
	PERatio            *float64 `json:"peRatio,omitempty"`
	SharesOutstanding  *float64 `json:"sharesOutstanding,omitempty"`
}

// QuoteExtended holds extended/after-hours trading data.
type QuoteExtended struct {
	AskPrice    *float64 `json:"askPrice,omitempty"`
	AskSize     *int     `json:"askSize,omitempty"`
	BidPrice    *float64 `json:"bidPrice,omitempty"`
	BidSize     *int     `json:"bidSize,omitempty"`
	LastPrice   *float64 `json:"lastPrice,omitempty"`
	LastSize    *int     `json:"lastSize,omitempty"`
	Mark        *float64 `json:"mark,omitempty"`
	QuoteTime   *int64   `json:"quoteTime,omitempty"`
	TotalVolume *int64   `json:"totalVolume,omitempty"`
	TradeTime   *int64   `json:"tradeTime,omitempty"`
}

// QuoteRegular holds regular market session data.
type QuoteRegular struct {
	RegularMarketLastPrice     *float64 `json:"regularMarketLastPrice,omitempty"`
	RegularMarketLastSize      *int     `json:"regularMarketLastSize,omitempty"`
	RegularMarketNetChange     *float64 `json:"regularMarketNetChange,omitempty"`
	RegularMarketPercentChange *float64 `json:"regularMarketPercentChange,omitempty"`
	RegularMarketTradeTime     *int64   `json:"regularMarketTradeTime,omitempty"`
}

// QuoteOption represents option quote data.
type QuoteOption struct {
	Symbol                 *string  `json:"symbol,omitempty"`
	Description            *string  `json:"description,omitempty"`
	BidPrice               *float64 `json:"bidPrice,omitempty"`
	BidSize                *int     `json:"bidSize,omitempty"`
	AskPrice               *float64 `json:"askPrice,omitempty"`
	AskSize                *int     `json:"askSize,omitempty"`
	LastPrice              *float64 `json:"lastPrice,omitempty"`
	LastSize               *int     `json:"lastSize,omitempty"`
	OpenPrice              *float64 `json:"openPrice,omitempty"`
	HighPrice              *float64 `json:"highPrice,omitempty"`
	LowPrice               *float64 `json:"lowPrice,omitempty"`
	ClosePrice             *float64 `json:"closePrice,omitempty"`
	NetChange              *float64 `json:"netChange,omitempty"`
	NetPercentChange       *float64 `json:"netPercentChange,omitempty"`
	Mark                   *float64 `json:"mark,omitempty"`
	MarkChange             *float64 `json:"markChange,omitempty"`
	MarkPercentChange      *float64 `json:"markPercentChange,omitempty"`
	TotalVolume            *int64   `json:"totalVolume,omitempty"`
	QuoteTime              *int64   `json:"quoteTime,omitempty"`
	TradeTime              *int64   `json:"tradeTime,omitempty"`
	SecurityStatus         *string  `json:"securityStatus,omitempty"`
	Volatility             *float64 `json:"volatility,omitempty"`
	OpenInterest           *int64   `json:"openInterest,omitempty"`
	StrikePrice            *float64 `json:"strikePrice,omitempty"`
	ExpirationDate         *string  `json:"expirationDate,omitempty"`
	DaysToExpiration       *int     `json:"daysToExpiration,omitempty"`
	Delta                  *float64 `json:"delta,omitempty"`
	Gamma                  *float64 `json:"gamma,omitempty"`
	Theta                  *float64 `json:"theta,omitempty"`
	Vega                   *float64 `json:"vega,omitempty"`
	Rho                    *float64 `json:"rho,omitempty"`
	TheoreticalOptionValue *float64 `json:"theoreticalOptionValue,omitempty"`
	TheoreticalVolatility  *float64 `json:"theoreticalVolatility,omitempty"`
	InTheMoney             *bool    `json:"inTheMoney,omitempty"`
	PutCall                *string  `json:"putCall,omitempty"`
}

// QuoteIndex represents index quote data.
type QuoteIndex struct {
	Symbol           *string  `json:"symbol,omitempty"`
	Description      *string  `json:"description,omitempty"`
	BidPrice         *float64 `json:"bidPrice,omitempty"`
	AskPrice         *float64 `json:"askPrice,omitempty"`
	LastPrice        *float64 `json:"lastPrice,omitempty"`
	OpenPrice        *float64 `json:"openPrice,omitempty"`
	HighPrice        *float64 `json:"highPrice,omitempty"`
	LowPrice         *float64 `json:"lowPrice,omitempty"`
	ClosePrice       *float64 `json:"closePrice,omitempty"`
	NetChange        *float64 `json:"netChange,omitempty"`
	NetPercentChange *float64 `json:"netPercentChange,omitempty"`
	Mark             *float64 `json:"mark,omitempty"`
	MarkChange       *float64 `json:"markChange,omitempty"`
	MarkPercentChange *float64 `json:"markPercentChange,omitempty"`
	TotalVolume      *int64   `json:"totalVolume,omitempty"`
	QuoteTime        *int64   `json:"quoteTime,omitempty"`
	TradeTime        *int64   `json:"tradeTime,omitempty"`
	SecurityStatus   *string  `json:"securityStatus,omitempty"`
	Volatility       *float64 `json:"volatility,omitempty"`
}

// QuoteFuture represents futures quote data.
type QuoteFuture struct {
	Symbol           *string  `json:"symbol,omitempty"`
	Description      *string  `json:"description,omitempty"`
	BidPrice         *float64 `json:"bidPrice,omitempty"`
	AskPrice         *float64 `json:"askPrice,omitempty"`
	LastPrice        *float64 `json:"lastPrice,omitempty"`
	OpenPrice        *float64 `json:"openPrice,omitempty"`
	HighPrice        *float64 `json:"highPrice,omitempty"`
	LowPrice         *float64 `json:"lowPrice,omitempty"`
	ClosePrice       *float64 `json:"closePrice,omitempty"`
	NetChange        *float64 `json:"netChange,omitempty"`
	NetPercentChange *float64 `json:"netPercentChange,omitempty"`
	Mark             *float64 `json:"mark,omitempty"`
	MarkChange       *float64 `json:"markChange,omitempty"`
	MarkPercentChange *float64 `json:"markPercentChange,omitempty"`
	TotalVolume      *int64   `json:"totalVolume,omitempty"`
	QuoteTime        *int64   `json:"quoteTime,omitempty"`
	TradeTime        *int64   `json:"tradeTime,omitempty"`
	SecurityStatus   *string  `json:"securityStatus,omitempty"`
	Volatility       *float64 `json:"volatility,omitempty"`
}

// QuoteForex represents forex quote data.
type QuoteForex struct {
	Symbol           *string  `json:"symbol,omitempty"`
	Description      *string  `json:"description,omitempty"`
	BidPrice         *float64 `json:"bidPrice,omitempty"`
	AskPrice         *float64 `json:"askPrice,omitempty"`
	LastPrice        *float64 `json:"lastPrice,omitempty"`
	OpenPrice        *float64 `json:"openPrice,omitempty"`
	HighPrice        *float64 `json:"highPrice,omitempty"`
	LowPrice         *float64 `json:"lowPrice,omitempty"`
	ClosePrice       *float64 `json:"closePrice,omitempty"`
	NetChange        *float64 `json:"netChange,omitempty"`
	NetPercentChange *float64 `json:"netPercentChange,omitempty"`
	Mark             *float64 `json:"mark,omitempty"`
	MarkChange       *float64 `json:"markChange,omitempty"`
	MarkPercentChange *float64 `json:"markPercentChange,omitempty"`
	TotalVolume      *int64   `json:"totalVolume,omitempty"`
	QuoteTime        *int64   `json:"quoteTime,omitempty"`
	TradeTime        *int64   `json:"tradeTime,omitempty"`
	SecurityStatus   *string  `json:"securityStatus,omitempty"`
	Volatility       *float64 `json:"volatility,omitempty"`
}

// QuoteMutualFund represents mutual fund quote data.
type QuoteMutualFund struct {
	Symbol           *string  `json:"symbol,omitempty"`
	Description      *string  `json:"description,omitempty"`
	BidPrice         *float64 `json:"bidPrice,omitempty"`
	AskPrice         *float64 `json:"askPrice,omitempty"`
	LastPrice        *float64 `json:"lastPrice,omitempty"`
	OpenPrice        *float64 `json:"openPrice,omitempty"`
	HighPrice        *float64 `json:"highPrice,omitempty"`
	LowPrice         *float64 `json:"lowPrice,omitempty"`
	ClosePrice       *float64 `json:"closePrice,omitempty"`
	NetChange        *float64 `json:"netChange,omitempty"`
	NetPercentChange *float64 `json:"netPercentChange,omitempty"`
	Mark             *float64 `json:"mark,omitempty"`
	MarkChange       *float64 `json:"markChange,omitempty"`
	MarkPercentChange *float64 `json:"markPercentChange,omitempty"`
	TotalVolume      *int64   `json:"totalVolume,omitempty"`
	QuoteTime        *int64   `json:"quoteTime,omitempty"`
	TradeTime        *int64   `json:"tradeTime,omitempty"`
	SecurityStatus   *string  `json:"securityStatus,omitempty"`
	Volatility       *float64 `json:"volatility,omitempty"`
}
