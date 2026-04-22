package models

import (
	"encoding/json"
	"testing"
)

// TestQuoteEquityJSONRoundTrip verifies that QuoteEquity can be marshaled and unmarshaled correctly.
func TestQuoteEquityJSONRoundTrip(t *testing.T) {
	symbol := "AAPL"
	bidPrice := 150.25
	askPrice := 150.35
	lastPrice := 150.30
	netChange := 1.50
	netPercentChange := 1.01
	totalVolume := int64(45000000)
	bidSize := 1000
	askSize := 1500
	bidTime := int64(1640000000000)
	askTime := int64(1640000000000)
	description := "Apple Inc"

	original := &QuoteEquity{
		Symbol: &symbol,
		Quote: &QuoteData{
			BidPrice:         &bidPrice,
			AskPrice:         &askPrice,
			LastPrice:        &lastPrice,
			NetChange:        &netChange,
			NetPercentChange: &netPercentChange,
			TotalVolume:      &totalVolume,
			BidSize:          &bidSize,
			AskSize:          &askSize,
			BidTime:          &bidTime,
			AskTime:          &askTime,
		},
		Reference: &QuoteReference{
			Description: &description,
		},
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("failed to marshal QuoteEquity: %v", err)
	}

	// Unmarshal back to struct
	unmarshaled := &QuoteEquity{}
	err = json.Unmarshal(jsonData, unmarshaled)
	if err != nil {
		t.Fatalf("failed to unmarshal QuoteEquity: %v", err)
	}

	// Verify top-level fields
	if unmarshaled.Symbol == nil || *unmarshaled.Symbol != symbol {
		t.Errorf("symbol mismatch: expected %s, got %v", symbol, unmarshaled.Symbol)
	}

	// Verify nested quote fields
	if unmarshaled.Quote == nil {
		t.Fatal("quote sub-object is nil after unmarshal")
	}
	if unmarshaled.Quote.BidPrice == nil || *unmarshaled.Quote.BidPrice != bidPrice {
		t.Errorf("bidPrice mismatch: expected %f, got %v", bidPrice, unmarshaled.Quote.BidPrice)
	}
	if unmarshaled.Quote.AskPrice == nil || *unmarshaled.Quote.AskPrice != askPrice {
		t.Errorf("askPrice mismatch: expected %f, got %v", askPrice, unmarshaled.Quote.AskPrice)
	}
	if unmarshaled.Quote.LastPrice == nil || *unmarshaled.Quote.LastPrice != lastPrice {
		t.Errorf("lastPrice mismatch: expected %f, got %v", lastPrice, unmarshaled.Quote.LastPrice)
	}
	if unmarshaled.Quote.NetChange == nil || *unmarshaled.Quote.NetChange != netChange {
		t.Errorf("netChange mismatch: expected %f, got %v", netChange, unmarshaled.Quote.NetChange)
	}
	if unmarshaled.Quote.NetPercentChange == nil || *unmarshaled.Quote.NetPercentChange != netPercentChange {
		t.Errorf("netPercentChange mismatch: expected %f, got %v", netPercentChange, unmarshaled.Quote.NetPercentChange)
	}
	if unmarshaled.Quote.TotalVolume == nil || *unmarshaled.Quote.TotalVolume != totalVolume {
		t.Errorf("totalVolume mismatch: expected %d, got %v", totalVolume, unmarshaled.Quote.TotalVolume)
	}
	if unmarshaled.Quote.BidSize == nil || *unmarshaled.Quote.BidSize != bidSize {
		t.Errorf("bidSize mismatch: expected %d, got %v", bidSize, unmarshaled.Quote.BidSize)
	}
	if unmarshaled.Quote.AskSize == nil || *unmarshaled.Quote.AskSize != askSize {
		t.Errorf("askSize mismatch: expected %d, got %v", askSize, unmarshaled.Quote.AskSize)
	}

	// Verify nested reference fields
	if unmarshaled.Reference == nil {
		t.Fatal("reference sub-object is nil after unmarshal")
	}
	if unmarshaled.Reference.Description == nil || *unmarshaled.Reference.Description != description {
		t.Errorf("description mismatch: expected %s, got %v", description, unmarshaled.Reference.Description)
	}
}

// TestCandleJSONRoundTrip verifies that Candle can be marshaled and unmarshaled correctly.
func TestCandleJSONRoundTrip(t *testing.T) {
	open := 150.00
	high := 151.50
	low := 149.75
	closePrice := 150.50
	volume := int64(1000000)
	datetime := int64(1640000000000)

	original := &Candle{
		Open:     &open,
		High:     &high,
		Low:      &low,
		Close:    &closePrice,
		Volume:   &volume,
		Datetime: &datetime,
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("failed to marshal Candle: %v", err)
	}

	// Unmarshal back to struct
	unmarshaled := &Candle{}
	err = json.Unmarshal(jsonData, unmarshaled)
	if err != nil {
		t.Fatalf("failed to unmarshal Candle: %v", err)
	}

	// Verify fields match
	if unmarshaled.Open == nil || *unmarshaled.Open != open {
		t.Errorf("open mismatch: expected %f, got %v", open, unmarshaled.Open)
	}
	if unmarshaled.High == nil || *unmarshaled.High != high {
		t.Errorf("high mismatch: expected %f, got %v", high, unmarshaled.High)
	}
	if unmarshaled.Low == nil || *unmarshaled.Low != low {
		t.Errorf("low mismatch: expected %f, got %v", low, unmarshaled.Low)
	}
	if unmarshaled.Close == nil || *unmarshaled.Close != closePrice {
		t.Errorf("close mismatch: expected %f, got %v", closePrice, unmarshaled.Close)
	}
	if unmarshaled.Volume == nil || *unmarshaled.Volume != volume {
		t.Errorf("volume mismatch: expected %d, got %v", volume, unmarshaled.Volume)
	}
	if unmarshaled.Datetime == nil || *unmarshaled.Datetime != datetime {
		t.Errorf("datetime mismatch: expected %d, got %v", datetime, unmarshaled.Datetime)
	}
}

// TestPositionJSONRoundTrip verifies that Position can be marshaled and unmarshaled correctly.
func TestPositionJSONRoundTrip(t *testing.T) {
	longQuantity := 100.0
	marketValue := 15000.0
	averagePrice := 150.0

	original := &Position{
		LongQuantity:  &longQuantity,
		MarketValue:   &marketValue,
		AveragePrice:  &averagePrice,
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("failed to marshal Position: %v", err)
	}

	// Unmarshal back to struct
	unmarshaled := &Position{}
	err = json.Unmarshal(jsonData, unmarshaled)
	if err != nil {
		t.Fatalf("failed to unmarshal Position: %v", err)
	}

	// Verify fields match
	if unmarshaled.LongQuantity == nil || *unmarshaled.LongQuantity != longQuantity {
		t.Errorf("longQuantity mismatch: expected %f, got %v", longQuantity, unmarshaled.LongQuantity)
	}
	if unmarshaled.MarketValue == nil || *unmarshaled.MarketValue != marketValue {
		t.Errorf("marketValue mismatch: expected %f, got %v", marketValue, unmarshaled.MarketValue)
	}
	if unmarshaled.AveragePrice == nil || *unmarshaled.AveragePrice != averagePrice {
		t.Errorf("averagePrice mismatch: expected %f, got %v", averagePrice, unmarshaled.AveragePrice)
	}
}
