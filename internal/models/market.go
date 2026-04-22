package models

// MarketHours represents market hours information.
type MarketHours struct {
	MarketType   *string                `json:"marketType,omitempty"`
	Product      *string                `json:"product,omitempty"`
	ProductName  *string                `json:"productName,omitempty"`
	IsOpen       *bool                  `json:"isOpen,omitempty"`
	SessionHours *SessionHours          `json:"sessionHours,omitempty"`
	Exchange     *string                `json:"exchange,omitempty"`
	Category     *string                `json:"category,omitempty"`
	Date         *string                `json:"date,omitempty"`
}

// SessionHours represents session hours for a market.
type SessionHours struct {
	PreMarket  []MarketSession `json:"preMarket,omitempty"`
	RegularMarket []MarketSession `json:"regularMarket,omitempty"`
	PostMarket []MarketSession `json:"postMarket,omitempty"`
}

// MarketSession represents a single market session.
type MarketSession struct {
	Start *string `json:"start,omitempty"`
	End   *string `json:"end,omitempty"`
}

// RegularMarket represents regular market data.
type RegularMarket struct {
	RegularMarketLastPrice      *float64 `json:"regularMarketLastPrice,omitempty"`
	RegularMarketLastSize       *int     `json:"regularMarketLastSize,omitempty"`
	RegularMarketNetChange      *float64 `json:"regularMarketNetChange,omitempty"`
	RegularMarketPercentChange  *float64 `json:"regularMarketPercentChange,omitempty"`
	RegularMarketTradeTime      *int64   `json:"regularMarketTradeTime,omitempty"`
}

// ExtendedMarket represents extended market data.
type ExtendedMarket struct {
	AskPrice    *float64 `json:"askPrice,omitempty"`
	AskSize     *int     `json:"askSize,omitempty"`
	BidPrice    *float64 `json:"bidPrice,omitempty"`
	BidSize     *int     `json:"bidSize,omitempty"`
	LastPrice   *float64 `json:"lastPrice,omitempty"`
	LastSize    *int     `json:"lastSize,omitempty"`
	Mark        *float64 `json:"mark,omitempty"`
	QuoteTime   *int64   `json:"quoteTime,omitempty"`
	TotalVolume *float64 `json:"totalVolume,omitempty"`
	TradeTime   *int64   `json:"tradeTime,omitempty"`
}
