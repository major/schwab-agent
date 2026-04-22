package models

// ScreenerResponse represents the response from a screener request.
type ScreenerResponse struct {
	Screeners []Screener `json:"screeners,omitempty"`
}

// Screener represents a single screener result.
type Screener struct {
	Symbol           *string  `json:"symbol,omitempty"`
	Description      *string  `json:"description,omitempty"`
	LastPrice        *float64 `json:"lastPrice,omitempty"`
	NetChange        *float64 `json:"netChange,omitempty"`
	NetPercentChange *float64 `json:"netPercentChange,omitempty"`
	Volume           *int64   `json:"volume,omitempty"`
	TotalVolume      *int64   `json:"totalVolume,omitempty"`
	Trades           *int64   `json:"trades,omitempty"`
	MarketShare      *float64 `json:"marketShare,omitempty"`
}
