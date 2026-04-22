package models

// CandleList represents a list of candles for a symbol.
type CandleList struct {
	Symbol                *string   `json:"symbol,omitempty"`
	Empty                 *bool     `json:"empty,omitempty"`
	Candles               []Candle  `json:"candles,omitempty"`
	PreviousClose         *float64  `json:"previousClose,omitempty"`
	PreviousCloseDate     *int64    `json:"previousCloseDate,omitempty"`
	PreviousCloseDateISO8601 *string `json:"previousCloseDateISO8601,omitempty"`
}

// Candle represents a single candlestick.
type Candle struct {
	Open          *float64 `json:"open,omitempty"`
	High          *float64 `json:"high,omitempty"`
	Low           *float64 `json:"low,omitempty"`
	Close         *float64 `json:"close,omitempty"`
	Volume        *int64   `json:"volume,omitempty"`
	Datetime      *int64   `json:"datetime,omitempty"`
	DatetimeISO8601 *string `json:"datetimeISO8601,omitempty"`
}
