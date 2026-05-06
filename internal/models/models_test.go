package models

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestQuoteEquityJSONRoundTrip verifies that QuoteEquity can be marshaled and unmarshaled correctly.
func TestQuoteEquityJSONRoundTrip(t *testing.T) {
	t.Parallel()

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

	// Marshal to JSON.
	jsonData, err := json.Marshal(original)
	require.NoError(t, err)

	// Unmarshal back to struct.
	unmarshaled := &QuoteEquity{}
	err = json.Unmarshal(jsonData, unmarshaled)
	require.NoError(t, err)

	// Verify top-level fields.
	require.NotNil(t, unmarshaled.Symbol)
	assert.Equal(t, symbol, *unmarshaled.Symbol)

	// Verify nested quote fields.
	require.NotNil(t, unmarshaled.Quote)
	assert.InDelta(t, bidPrice, *unmarshaled.Quote.BidPrice, 0.001)
	assert.InDelta(t, askPrice, *unmarshaled.Quote.AskPrice, 0.001)
	assert.InDelta(t, lastPrice, *unmarshaled.Quote.LastPrice, 0.001)
	assert.InDelta(t, netChange, *unmarshaled.Quote.NetChange, 0.001)
	assert.InDelta(t, netPercentChange, *unmarshaled.Quote.NetPercentChange, 0.001)
	assert.Equal(t, totalVolume, *unmarshaled.Quote.TotalVolume)
	assert.Equal(t, bidSize, *unmarshaled.Quote.BidSize)
	assert.Equal(t, askSize, *unmarshaled.Quote.AskSize)

	// Verify nested reference fields.
	require.NotNil(t, unmarshaled.Reference)
	assert.Equal(t, description, *unmarshaled.Reference.Description)
}

// TestPositionJSONRoundTrip verifies that Position can be marshaled and unmarshaled correctly.
func TestPositionJSONRoundTrip(t *testing.T) {
	t.Parallel()

	longQuantity := 100.0
	marketValue := 15000.0
	averagePrice := 150.0

	original := &Position{
		LongQuantity: &longQuantity,
		MarketValue:  &marketValue,
		AveragePrice: &averagePrice,
	}

	// Marshal to JSON.
	jsonData, err := json.Marshal(original)
	require.NoError(t, err)

	// Unmarshal back to struct.
	unmarshaled := &Position{}
	err = json.Unmarshal(jsonData, unmarshaled)
	require.NoError(t, err)

	// Verify fields match.
	assert.InDelta(t, longQuantity, *unmarshaled.LongQuantity, 0.001)
	assert.InDelta(t, marketValue, *unmarshaled.MarketValue, 0.001)
	assert.InDelta(t, averagePrice, *unmarshaled.AveragePrice, 0.001)
}

// --- SchwabTime tests ---

// TestSchwabTimeUnmarshalJSON verifies that SchwabTime handles both RFC3339
// and the Schwab API's non-standard timestamp format (no colon in timezone offset).
func TestSchwabTimeUnmarshalJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantErr bool
		check   func(t *testing.T, st SchwabTime)
	}{
		{
			name:  "RFC3339 UTC",
			input: `"2026-04-21T17:25:35Z"`,
			check: func(t *testing.T, st SchwabTime) {
				assert.Equal(t, 2026, st.Year())
				assert.Equal(t, time.April, st.Month())
				assert.Equal(t, 21, st.Day())
				assert.Equal(t, 17, st.Hour())
				assert.Equal(t, 25, st.Minute())
				assert.Equal(t, 35, st.Second())
			},
		},
		{
			name:  "RFC3339 positive offset",
			input: `"2026-04-21T17:25:35+05:30"`,
			check: func(t *testing.T, st SchwabTime) {
				assert.Equal(t, 2026, st.Year())
				_, offset := st.Zone()
				assert.Equal(t, 5*3600+30*60, offset)
			},
		},
		{
			name:  "Schwab API format positive offset",
			input: `"2026-04-21T17:25:35+0000"`,
			check: func(t *testing.T, st SchwabTime) {
				assert.Equal(t, 2026, st.Year())
				assert.Equal(t, time.April, st.Month())
				assert.Equal(t, 21, st.Day())
				assert.Equal(t, 17, st.Hour())
			},
		},
		{
			name:  "Schwab API format negative offset",
			input: `"2026-04-21T12:25:35-0500"`,
			check: func(t *testing.T, st SchwabTime) {
				assert.Equal(t, 12, st.Hour())
				_, offset := st.Zone()
				assert.Equal(t, -5*3600, offset)
			},
		},
		{
			name:  "empty string returns zero time",
			input: `""`,
			check: func(t *testing.T, st SchwabTime) {
				assert.True(t, st.IsZero())
			},
		},
		{
			name:    "invalid timestamp format",
			input:   `"not-a-timestamp"`,
			wantErr: true,
		},
		{
			name:    "non-string JSON value",
			input:   `12345`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var st SchwabTime
			err := json.Unmarshal([]byte(tt.input), &st)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			if tt.check != nil {
				tt.check(t, st)
			}
		})
	}
}

// TestSchwabTimeMarshalJSON verifies MarshalJSON output for zero and populated values.
func TestSchwabTimeMarshalJSON(t *testing.T) {
	t.Parallel()

	t.Run("zero value marshals to null", func(t *testing.T) {
		t.Parallel()
		st := SchwabTime{}
		data, err := json.Marshal(st)
		require.NoError(t, err)
		assert.Equal(t, "null", string(data))
	})

	t.Run("populated value marshals to RFC3339", func(t *testing.T) {
		t.Parallel()
		ts := time.Date(2026, 4, 21, 17, 25, 35, 0, time.UTC)
		st := SchwabTime{Time: ts}
		data, err := json.Marshal(st)
		require.NoError(t, err)
		assert.Equal(t, `"2026-04-21T17:25:35Z"`, string(data))
	})

	t.Run("non-UTC offset preserved in RFC3339", func(t *testing.T) {
		t.Parallel()
		loc := time.FixedZone("EST", -5*3600)
		ts := time.Date(2026, 4, 21, 12, 0, 0, 0, loc)
		st := SchwabTime{Time: ts}
		data, err := json.Marshal(st)
		require.NoError(t, err)
		assert.Equal(t, `"2026-04-21T12:00:00-05:00"`, string(data))
	})
}

// TestSchwabTimeRoundTrip verifies that Schwab API timestamps survive
// the unmarshal-marshal-unmarshal cycle without data loss.
func TestSchwabTimeRoundTrip(t *testing.T) {
	t.Parallel()

	t.Run("Schwab format survives round-trip through RFC3339", func(t *testing.T) {
		t.Parallel()
		// Unmarshal from Schwab API format.
		var st SchwabTime
		err := json.Unmarshal([]byte(`"2026-04-21T17:25:35+0000"`), &st)
		require.NoError(t, err)

		// Marshal back (outputs RFC3339 with colon in offset).
		data, err := json.Marshal(st)
		require.NoError(t, err)
		assert.Equal(t, `"2026-04-21T17:25:35Z"`, string(data))

		// Unmarshal the RFC3339 output.
		var st2 SchwabTime
		err = json.Unmarshal(data, &st2)
		require.NoError(t, err)
		assert.True(t, st.Equal(st2.Time))
	})

	t.Run("non-UTC offset round-trips correctly", func(t *testing.T) {
		t.Parallel()

		var st SchwabTime
		err := json.Unmarshal([]byte(`"2026-04-21T12:25:35-0500"`), &st)
		require.NoError(t, err)

		data, err := json.Marshal(st)
		require.NoError(t, err)

		var st2 SchwabTime
		err = json.Unmarshal(data, &st2)
		require.NoError(t, err)
		assert.True(t, st.Equal(st2.Time))
	})
}

// TestSchwabTimeInStructField verifies SchwabTime works when embedded in an
// API response struct (ExecutionLeg uses it for its Time field).
func TestSchwabTimeInStructField(t *testing.T) {
	t.Parallel()

	input := `{
		"legId": 1,
		"price": 150.25,
		"quantity": 10,
		"mismarkedQuantity": 0,
		"instrumentId": 12345,
		"time": "2026-04-21T17:25:35+0000"
	}`

	var leg ExecutionLeg
	err := json.Unmarshal([]byte(input), &leg)
	require.NoError(t, err)
	assert.Equal(t, int64(1), leg.LegID)
	assert.InDelta(t, 150.25, leg.Price, 0.001)
	assert.Equal(t, 2026, leg.Time.Year())
	assert.Equal(t, time.April, leg.Time.Month())
}

// --- Wire type round-trip tests ---
// These verify that types used in API requests/responses can be correctly
// deserialized from JSON (simulating API responses) and survive round-trips.

// TestOrderJSONRoundTrip verifies a full order response with SchwabTime fields,
// nested legs, and execution activities.
func TestOrderJSONRoundTrip(t *testing.T) {
	t.Parallel()

	input := `{
		"session": "NORMAL",
		"duration": "DAY",
		"orderType": "LIMIT",
		"price": 150.50,
		"quantity": 10,
		"filledQuantity": 5,
		"remainingQuantity": 5,
		"orderStrategyType": "SINGLE",
		"orderId": 123456789,
		"cancelable": true,
		"editable": false,
		"status": "WORKING",
		"enteredTime": "2026-04-21T09:30:00+0000",
		"closeTime": "2026-04-21T16:00:00+0000",
		"tag": "API_TOS:schwab-agent",
		"accountNumber": 12345678,
		"orderLegCollection": [
			{
				"orderLegType": "EQUITY",
				"legId": 1,
				"instrument": {
					"assetType": "EQUITY",
					"symbol": "AAPL"
				},
				"instruction": "BUY",
				"positionEffect": "OPENING",
				"quantity": 10
			}
		],
		"orderActivityCollection": [
			{
				"activityType": "EXECUTION",
				"executionType": "FILL",
				"quantity": 5,
				"orderRemainingQuantity": 5,
				"executionLegs": [
					{
						"legId": 1,
						"price": 150.25,
						"quantity": 5,
						"mismarkedQuantity": 0,
						"instrumentId": 12345,
						"time": "2026-04-21T10:15:30+0000"
					}
				]
			}
		]
	}`

	// Arrange: Unmarshal from API-style JSON.
	var order Order
	err := json.Unmarshal([]byte(input), &order)
	require.NoError(t, err)

	// Assert: Top-level fields.
	assert.Equal(t, SessionNormal, order.Session)
	assert.Equal(t, DurationDay, order.Duration)
	assert.Equal(t, OrderTypeLimit, order.OrderType)
	assert.InDelta(t, 150.50, *order.Price, 0.001)
	assert.Equal(t, OrderStatusWorking, *order.Status)
	assert.Equal(t, int64(123456789), *order.OrderID)

	// Assert: SchwabTime fields parsed correctly.
	require.NotNil(t, order.EnteredTime)
	assert.Equal(t, 2026, order.EnteredTime.Year())
	assert.Equal(t, 9, order.EnteredTime.Hour())
	require.NotNil(t, order.CloseTime)
	assert.Equal(t, 16, order.CloseTime.Hour())

	// Assert: Nested order leg.
	require.Len(t, order.OrderLegCollection, 1)
	leg := order.OrderLegCollection[0]
	assert.Equal(t, AssetTypeEquity, leg.Instrument.AssetType)
	assert.Equal(t, "AAPL", leg.Instrument.Symbol)
	assert.Equal(t, InstructionBuy, leg.Instruction)

	// Assert: Nested activity with execution leg containing SchwabTime.
	require.Len(t, order.OrderActivityCollection, 1)
	activity := order.OrderActivityCollection[0]
	assert.Equal(t, ActivityTypeExecution, activity.ActivityType)
	require.Len(t, activity.ExecutionLegs, 1)
	assert.InDelta(t, 150.25, activity.ExecutionLegs[0].Price, 0.001)
	assert.Equal(t, 2026, activity.ExecutionLegs[0].Time.Year())

	// Act: Round-trip marshal/unmarshal.
	data, err := json.Marshal(order)
	require.NoError(t, err)

	var order2 Order
	err = json.Unmarshal(data, &order2)
	require.NoError(t, err)
	assert.Equal(t, order.Session, order2.Session)
	assert.Equal(t, *order.OrderID, *order2.OrderID)
	assert.True(t, order.EnteredTime.Equal(order2.EnteredTime.Time))
}

// TestOrderRequestJSONRoundTrip verifies the outgoing order submission type.
func TestOrderRequestJSONRoundTrip(t *testing.T) {
	t.Parallel()

	input := `{
		"session": "NORMAL",
		"duration": "DAY",
		"orderType": "LIMIT",
		"price": 150.50,
		"orderStrategyType": "SINGLE",
		"orderLegCollection": [
			{
				"instrument": {
					"assetType": "EQUITY",
					"symbol": "AAPL"
				},
				"instruction": "BUY",
				"quantity": 10
			}
		]
	}`

	var req OrderRequest
	err := json.Unmarshal([]byte(input), &req)
	require.NoError(t, err)

	assert.Equal(t, SessionNormal, req.Session)
	assert.Equal(t, DurationDay, req.Duration)
	assert.Equal(t, OrderTypeLimit, req.OrderType)
	assert.InDelta(t, 150.50, *req.Price, 0.001)
	assert.Equal(t, OrderStrategyTypeSingle, req.OrderStrategyType)
	require.Len(t, req.OrderLegCollection, 1)
	assert.Equal(t, "AAPL", req.OrderLegCollection[0].Instrument.Symbol)
	assert.Equal(t, InstructionBuy, req.OrderLegCollection[0].Instruction)

	// Round-trip.
	data, err := json.Marshal(req)
	require.NoError(t, err)
	var req2 OrderRequest
	err = json.Unmarshal(data, &req2)
	require.NoError(t, err)
	assert.Equal(t, req.OrderType, req2.OrderType)
	assert.InDelta(t, *req.Price, *req2.Price, 0.001)
}

// TestOrderRequestWithChildStrategies verifies recursive child order nesting (OTO/OCO).
func TestOrderRequestWithChildStrategies(t *testing.T) {
	t.Parallel()

	input := `{
		"session": "NORMAL",
		"duration": "DAY",
		"orderType": "LIMIT",
		"price": 150.00,
		"orderStrategyType": "TRIGGER",
		"orderLegCollection": [
			{
				"instrument": {"assetType": "EQUITY", "symbol": "AAPL"},
				"instruction": "BUY",
				"quantity": 100
			}
		],
		"childOrderStrategies": [
			{
				"session": "NORMAL",
				"duration": "GOOD_TILL_CANCEL",
				"orderType": "LIMIT",
				"price": 160.00,
				"orderStrategyType": "OCO",
				"orderLegCollection": [
					{
						"instrument": {"assetType": "EQUITY", "symbol": "AAPL"},
						"instruction": "SELL",
						"quantity": 100
					}
				],
				"childOrderStrategies": [
					{
						"session": "NORMAL",
						"duration": "GOOD_TILL_CANCEL",
						"orderType": "STOP",
						"stopPrice": 140.00,
						"orderStrategyType": "SINGLE",
						"orderLegCollection": [
							{
								"instrument": {"assetType": "EQUITY", "symbol": "AAPL"},
								"instruction": "SELL",
								"quantity": 100
							}
						]
					}
				]
			}
		]
	}`

	var req OrderRequest
	err := json.Unmarshal([]byte(input), &req)
	require.NoError(t, err)

	assert.Equal(t, OrderStrategyTypeTrigger, req.OrderStrategyType)
	require.Len(t, req.ChildOrderStrategies, 1)

	child := req.ChildOrderStrategies[0]
	assert.Equal(t, OrderStrategyTypeOCO, child.OrderStrategyType)
	assert.Equal(t, DurationGoodTillCancel, child.Duration)
	assert.InDelta(t, 160.0, *child.Price, 0.001)

	// Verify grandchild (stop-loss leg of OCO).
	require.Len(t, child.ChildOrderStrategies, 1)
	grandchild := child.ChildOrderStrategies[0]
	assert.Equal(t, OrderStrategyTypeSingle, grandchild.OrderStrategyType)
	assert.Equal(t, OrderTypeStop, grandchild.OrderType)
	assert.InDelta(t, 140.0, *grandchild.StopPrice, 0.001)
}

// TestAccountJSONRoundTrip verifies the Account -> SecuritiesAccount -> Position hierarchy.
func TestAccountJSONRoundTrip(t *testing.T) {
	t.Parallel()

	input := `{
		"securitiesAccount": {
			"type": "MARGIN",
			"accountNumber": "12345678",
			"roundTrips": 2,
			"isForeign": false,
			"currentBalances": {
				"cashBalance": 10000.50,
				"buyingPower": 20000.00,
				"equity": 50000.00,
				"dayTradingBuyingPower": 80000.00
			},
			"positions": [
				{
					"longQuantity": 100,
					"averagePrice": 150.00,
					"marketValue": 17550.00,
					"currentDayProfitLoss": 50.00,
					"instrument": {
						"assetType": "EQUITY",
						"symbol": "AAPL",
						"cusip": "037833100",
						"description": "APPLE INC"
					}
				}
			]
		},
		"nickName": "Trading Account",
		"primaryAccount": true
	}`

	var acct Account
	err := json.Unmarshal([]byte(input), &acct)
	require.NoError(t, err)

	assert.Equal(t, "Trading Account", *acct.NickName)
	assert.True(t, *acct.PrimaryAccount)

	require.NotNil(t, acct.SecuritiesAccount)
	assert.Equal(t, "MARGIN", *acct.SecuritiesAccount.Type)
	assert.Equal(t, "12345678", *acct.SecuritiesAccount.AccountNumber)

	require.NotNil(t, acct.SecuritiesAccount.CurrentBalances)
	assert.InDelta(t, 10000.50, *acct.SecuritiesAccount.CurrentBalances.CashBalance, 0.001)
	assert.InDelta(t, 20000.00, *acct.SecuritiesAccount.CurrentBalances.BuyingPower, 0.001)

	require.Len(t, acct.SecuritiesAccount.Positions, 1)
	pos := acct.SecuritiesAccount.Positions[0]
	assert.InDelta(t, 100.0, *pos.LongQuantity, 0.001)
	assert.InDelta(t, 150.0, *pos.AveragePrice, 0.001)
	require.NotNil(t, pos.Instrument)
	assert.Equal(t, "AAPL", *pos.Instrument.Symbol)

	// Round-trip.
	data, err := json.Marshal(acct)
	require.NoError(t, err)
	var acct2 Account
	err = json.Unmarshal(data, &acct2)
	require.NoError(t, err)
	assert.Equal(t, *acct.NickName, *acct2.NickName)
	assert.InDelta(t, *acct.SecuritiesAccount.CurrentBalances.CashBalance,
		*acct2.SecuritiesAccount.CurrentBalances.CashBalance, 0.001)
}

// TestTransactionJSONRoundTrip verifies nested transfer items and user details.
func TestTransactionJSONRoundTrip(t *testing.T) {
	t.Parallel()

	input := `{
		"activityId": 98765,
		"time": "2026-04-21T10:30:00+0000",
		"type": "TRADE",
		"status": "VALID",
		"description": "BUY TRADE",
		"netAmount": -15050.00,
		"accountNumber": "12345678",
		"orderId": 123456,
		"tradeDate": "2026-04-21",
		"settlementDate": "2026-04-23",
		"transferItems": [
			{
				"instrument": {
					"assetType": "EQUITY",
					"symbol": "AAPL",
					"cusip": "037833100",
					"description": "APPLE INC"
				},
				"positionEffect": "OPENING",
				"positionQuantity": 100
			}
		],
		"user": {
			"userId": "user123",
			"name": "Test User"
		}
	}`

	var tx Transaction
	err := json.Unmarshal([]byte(input), &tx)
	require.NoError(t, err)

	assert.Equal(t, int64(98765), *tx.ActivityID)
	assert.Equal(t, TransactionTypeTrade, *tx.Type)
	assert.InDelta(t, -15050.00, *tx.NetAmount, 0.001)

	require.Len(t, tx.TransferItems, 1)
	item := tx.TransferItems[0]
	require.NotNil(t, item.Instrument)
	assert.Equal(t, "AAPL", *item.Instrument.Symbol)
	assert.InDelta(t, 100.0, *item.PositionQuantity, 0.001)

	require.NotNil(t, tx.User)
	assert.Equal(t, "Test User", *tx.User.Name)

	// Round-trip.
	data, err := json.Marshal(tx)
	require.NoError(t, err)
	var tx2 Transaction
	err = json.Unmarshal(data, &tx2)
	require.NoError(t, err)
	assert.Equal(t, *tx.ActivityID, *tx2.ActivityID)
}

// TestQuoteOptionJSONRoundTrip verifies option quote with Greeks.
func TestQuoteOptionJSONRoundTrip(t *testing.T) {
	t.Parallel()

	input := `{
		"symbol": "AAPL  260516C00175000",
		"description": "AAPL May 16 2026 175 Call",
		"bidPrice": 5.10,
		"askPrice": 5.30,
		"lastPrice": 5.20,
		"totalVolume": 12000,
		"openInterest": 15000,
		"strikePrice": 175.0,
		"expirationDate": "2026-05-16",
		"daysToExpiration": 25,
		"delta": 0.52,
		"gamma": 0.03,
		"theta": -0.08,
		"vega": 0.25,
		"rho": 0.04,
		"theoreticalOptionValue": 5.15,
		"theoreticalVolatility": 0.28,
		"inTheMoney": true,
		"putCall": "CALL"
	}`

	var q QuoteOption
	err := json.Unmarshal([]byte(input), &q)
	require.NoError(t, err)

	assert.InDelta(t, 175.0, *q.StrikePrice, 0.001)
	assert.InDelta(t, 0.52, *q.Delta, 0.001)
	assert.InDelta(t, -0.08, *q.Theta, 0.001)
	assert.InDelta(t, 0.25, *q.Vega, 0.001)
	assert.True(t, *q.InTheMoney)
	assert.Equal(t, "CALL", *q.PutCall)
	assert.Equal(t, int64(15000), *q.OpenInterest)

	// Round-trip.
	data, err := json.Marshal(q)
	require.NoError(t, err)
	var q2 QuoteOption
	err = json.Unmarshal(data, &q2)
	require.NoError(t, err)
	assert.InDelta(t, *q.Delta, *q2.Delta, 0.001)
	assert.InDelta(t, *q.StrikePrice, *q2.StrikePrice, 0.001)
}

// TestPreviewOrderJSONRoundTrip verifies deeply nested commission/fee structures.
func TestPreviewOrderJSONRoundTrip(t *testing.T) {
	t.Parallel()

	input := `{
		"orderId": 999,
		"orderStrategy": {
			"accountNumber": "12345678",
			"advancedOrderType": "NONE",
			"orderType": "LIMIT",
			"status": "ACCEPTED",
			"duration": "DAY",
			"session": "NORMAL",
			"price": 150.50,
			"quantity": 10,
			"orderStrategyType": "SINGLE",
			"orderLegs": [
				{
					"instrument": {"assetType": "EQUITY", "symbol": "AAPL"},
					"instruction": "BUY",
					"quantity": 10
				}
			],
			"orderBalance": {
				"orderValue": 1505.00,
				"projectedAvailableFund": 8495.00,
				"projectedBuyingPower": 16990.00,
				"projectedCommission": 0.00
			},
			"commissionAndFee": {
				"commission": {
					"commissionLegs": [
						{
							"commissionValues": [
								{"value": 0.00, "type": "COMMISSION"}
							]
						}
					]
				},
				"fee": {
					"feeLegs": [
						{
							"feeValues": [
								{"value": 0.02, "type": "SEC_FEE"},
								{"value": 0.01, "type": "TAF_FEE"}
							]
						}
					]
				}
			}
		},
		"orderValidationResult": {
			"alerts": [
				{
					"validationRuleName": "MarketVolatility",
					"message": "Market is experiencing high volatility"
				}
			],
			"accepts": [
				{
					"validationRuleName": "BuyingPower",
					"message": "Sufficient buying power"
				}
			]
		}
	}`

	var preview PreviewOrder
	err := json.Unmarshal([]byte(input), &preview)
	require.NoError(t, err)

	assert.Equal(t, int64(999), *preview.OrderID)

	// Nested strategy.
	require.NotNil(t, preview.OrderStrategy)
	assert.Equal(t, OrderTypeLimit, preview.OrderStrategy.OrderType)
	assert.InDelta(t, 150.50, *preview.OrderStrategy.Price, 0.001)

	// Nested balance.
	require.NotNil(t, preview.OrderStrategy.OrderBalance)
	assert.InDelta(t, 1505.00, *preview.OrderStrategy.OrderBalance.OrderValue, 0.001)

	// Nested commission/fee.
	require.NotNil(t, preview.OrderStrategy.CommissionAndFee)
	require.NotNil(t, preview.OrderStrategy.CommissionAndFee.Fee)
	require.Len(t, preview.OrderStrategy.CommissionAndFee.Fee.FeeLegs, 1)
	require.Len(t, preview.OrderStrategy.CommissionAndFee.Fee.FeeLegs[0].FeeValues, 2)
	assert.Equal(t, "SEC_FEE", preview.OrderStrategy.CommissionAndFee.Fee.FeeLegs[0].FeeValues[0].Type)

	// Validation results.
	require.NotNil(t, preview.OrderValidationResult)
	require.Len(t, preview.OrderValidationResult.Alerts, 1)
	require.Len(t, preview.OrderValidationResult.Accepts, 1)

	// Round-trip.
	data, err := json.Marshal(preview)
	require.NoError(t, err)
	var preview2 PreviewOrder
	err = json.Unmarshal(data, &preview2)
	require.NoError(t, err)
	assert.Equal(t, *preview.OrderID, *preview2.OrderID)
}

// TestUserPreferenceJSONRoundTrip verifies preferences with nested arrays.
func TestUserPreferenceJSONRoundTrip(t *testing.T) {
	t.Parallel()

	input := `{
		"accounts": [
			{
				"accountNumber": "12345678",
				"primaryAccount": true,
				"type": "MARGIN",
				"nickName": "My Trading Account",
				"accountColor": "Blue",
				"displayAcctId": "...5678",
				"autoPositionEffect": true
			}
		],
		"offers": [
			{
				"id": "offer1",
				"name": "Premium Trading",
				"description": "Advanced trading features",
				"status": "ACTIVE"
			}
		],
		"streamerInfo": [
			{
				"streamerUrl": "wss://streamer.schwab.com",
				"token": "abc123",
				"tokenExpTime": 1745280000000,
				"appId": "app123",
				"acl": "acl-value"
			}
		]
	}`

	var pref UserPreference
	err := json.Unmarshal([]byte(input), &pref)
	require.NoError(t, err)

	require.Len(t, pref.Accounts, 1)
	assert.Equal(t, "12345678", *pref.Accounts[0].AccountNumber)
	assert.True(t, *pref.Accounts[0].PrimaryAccount)
	assert.Equal(t, "My Trading Account", *pref.Accounts[0].NickName)

	require.Len(t, pref.Offers, 1)
	assert.Equal(t, "Premium Trading", *pref.Offers[0].Name)

	require.Len(t, pref.StreamerInfo, 1)
	assert.Equal(t, "wss://streamer.schwab.com", *pref.StreamerInfo[0].StreamerURL)

	// Round-trip.
	data, err := json.Marshal(pref)
	require.NoError(t, err)
	var pref2 UserPreference
	err = json.Unmarshal(data, &pref2)
	require.NoError(t, err)
	assert.Equal(t, *pref.Accounts[0].NickName, *pref2.Accounts[0].NickName)
}

// --- Edge case tests ---

// TestQuoteDataSpecialJSONTags verifies that the numeric-prefix json tags
// "52WeekHigh" and "52WeekLow" work correctly.
func TestQuoteDataSpecialJSONTags(t *testing.T) {
	t.Parallel()

	input := `{
		"52WeekHigh": 200.00,
		"52WeekLow": 130.00,
		"lastPrice": 175.50,
		"totalVolume": 45000000
	}`

	var qd QuoteData
	err := json.Unmarshal([]byte(input), &qd)
	require.NoError(t, err)

	require.NotNil(t, qd.WeekHigh52)
	assert.InDelta(t, 200.0, *qd.WeekHigh52, 0.001)
	require.NotNil(t, qd.WeekLow52)
	assert.InDelta(t, 130.0, *qd.WeekLow52, 0.001)

	// Round-trip preserves the numeric-prefix tags.
	data, err := json.Marshal(qd)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"52WeekHigh"`)
	assert.Contains(t, string(data), `"52WeekLow"`)
}

// TestNilFieldsOmittedInJSON verifies that nil pointer fields with omitempty
// produce clean JSON without null entries.
func TestNilFieldsOmittedInJSON(t *testing.T) {
	t.Parallel()

	// Position with all nil pointer fields should marshal to empty JSON.
	pos := Position{}
	data, err := json.Marshal(pos)
	require.NoError(t, err)
	assert.Equal(t, "{}", string(data))
}

// TestOrderToRequest verifies that Order (response) converts correctly to OrderRequest (submission).
func TestOrderToRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		test func(t *testing.T)
	}{
		{
			name: "simple order conversion",
			test: func(t *testing.T) {
				// Arrange: Create a simple Order with basic fields.
				price := 150.50
				quantity := 10.0
				order := Order{
					Session:           SessionNormal,
					Duration:          DurationDay,
					OrderType:         OrderTypeLimit,
					Price:             &price,
					Quantity:          &quantity,
					OrderStrategyType: OrderStrategyTypeSingle,
					OrderLegCollection: []OrderLegCollection{
						{
							Instruction: InstructionBuy,
							Quantity:    10.0,
							Instrument: OrderInstrument{
								AssetType: AssetTypeEquity,
								Symbol:    "AAPL",
							},
						},
					},
				}

				// Act: Convert to OrderRequest.
				req := OrderToRequest(&order)

				// Assert: Verify all allowlist fields are copied.
				assert.Equal(t, SessionNormal, req.Session)
				assert.Equal(t, DurationDay, req.Duration)
				assert.Equal(t, OrderTypeLimit, req.OrderType)
				assert.NotNil(t, req.Price)
				assert.InDelta(t, 150.50, *req.Price, 0.001)
				assert.NotNil(t, req.Quantity)
				assert.InDelta(t, 10.0, *req.Quantity, 0.001)
				assert.Equal(t, OrderStrategyTypeSingle, req.OrderStrategyType)
				assert.Len(t, req.OrderLegCollection, 1)
				assert.Equal(t, InstructionBuy, req.OrderLegCollection[0].Instruction)
			},
		},
		{
			name: "order with child order strategies",
			test: func(t *testing.T) {
				// Arrange: Create a parent Order with child orders (e.g., OCO).
				price1 := 150.00
				price2 := 145.00
				quantity := 10.0

				childOrder1 := Order{
					Session:           SessionNormal,
					Duration:          DurationGoodTillCancel,
					OrderType:         OrderTypeLimit,
					Price:             &price1,
					Quantity:          &quantity,
					OrderStrategyType: OrderStrategyTypeSingle,
					OrderLegCollection: []OrderLegCollection{
						{
							Instruction: InstructionSell,
							Quantity:    10.0,
							Instrument: OrderInstrument{
								AssetType: AssetTypeEquity,
								Symbol:    "AAPL",
							},
						},
					},
				}

				childOrder2 := Order{
					Session:           SessionNormal,
					Duration:          DurationGoodTillCancel,
					OrderType:         OrderTypeLimit,
					Price:             &price2,
					Quantity:          &quantity,
					OrderStrategyType: OrderStrategyTypeSingle,
					OrderLegCollection: []OrderLegCollection{
						{
							Instruction: InstructionSell,
							Quantity:    10.0,
							Instrument: OrderInstrument{
								AssetType: AssetTypeEquity,
								Symbol:    "AAPL",
							},
						},
					},
				}

				parentOrder := Order{
					Session:              SessionNormal,
					Duration:             DurationGoodTillCancel,
					OrderType:            OrderTypeLimit,
					OrderStrategyType:    OrderStrategyTypeOCO,
					ChildOrderStrategies: []Order{childOrder1, childOrder2},
				}

				// Act: Convert to OrderRequest.
				req := OrderToRequest(&parentOrder)

				// Assert: Verify parent and children are converted.
				assert.Equal(t, OrderStrategyTypeOCO, req.OrderStrategyType)
				assert.Len(t, req.ChildOrderStrategies, 2)

				// Verify first child.
				assert.Equal(t, OrderTypeLimit, req.ChildOrderStrategies[0].OrderType)
				assert.NotNil(t, req.ChildOrderStrategies[0].Price)
				assert.InDelta(t, 150.00, *req.ChildOrderStrategies[0].Price, 0.001)
				assert.Len(t, req.ChildOrderStrategies[0].OrderLegCollection, 1)

				// Verify second child.
				assert.Equal(t, OrderTypeLimit, req.ChildOrderStrategies[1].OrderType)
				assert.NotNil(t, req.ChildOrderStrategies[1].Price)
				assert.InDelta(t, 145.00, *req.ChildOrderStrategies[1].Price, 0.001)
				assert.Len(t, req.ChildOrderStrategies[1].OrderLegCollection, 1)
			},
		},
		{
			name: "minimal order with zero-value optional fields",
			test: func(t *testing.T) {
				// Arrange: Create a minimal Order with only required fields.
				order := Order{
					Session:           SessionNormal,
					Duration:          DurationDay,
					OrderType:         OrderTypeMarket,
					OrderStrategyType: OrderStrategyTypeSingle,
					OrderLegCollection: []OrderLegCollection{
						{
							Instruction: InstructionBuy,
							Quantity:    5.0,
							Instrument: OrderInstrument{
								AssetType: AssetTypeEquity,
								Symbol:    "TSLA",
							},
						},
					},
					// All optional fields are nil/zero.
				}

				// Act: Convert to OrderRequest.
				req := OrderToRequest(&order)

				// Assert: Verify required fields are present, optional fields are nil.
				assert.Equal(t, SessionNormal, req.Session)
				assert.Equal(t, DurationDay, req.Duration)
				assert.Equal(t, OrderTypeMarket, req.OrderType)
				assert.Equal(t, OrderStrategyTypeSingle, req.OrderStrategyType)
				assert.Nil(t, req.Price)
				assert.Nil(t, req.StopPrice)
				assert.Nil(t, req.TaxLotMethod)
				assert.Nil(t, req.SpecialInstruction)
				assert.Empty(t, req.ChildOrderStrategies)
			},
		},
		{
			name: "response-only fields are excluded",
			test: func(t *testing.T) {
				// Arrange: Create an Order with response-only fields populated.
				orderID := int64(12345)
				status := OrderStatusFilled
				filledQty := 10.0
				remainingQty := 0.0
				tag := "my-tag"
				accountNum := int64(987654)
				cancelable := true
				editable := false
				statusDesc := "Order filled"

				order := Order{
					Session:           SessionNormal,
					Duration:          DurationDay,
					OrderType:         OrderTypeMarket,
					OrderStrategyType: OrderStrategyTypeSingle,
					OrderID:           &orderID,
					Status:            &status,
					FilledQuantity:    &filledQty,
					RemainingQuantity: &remainingQty,
					Tag:               &tag,
					AccountNumber:     &accountNum,
					Cancelable:        &cancelable,
					Editable:          &editable,
					StatusDescription: &statusDesc,
					OrderLegCollection: []OrderLegCollection{
						{
							Instruction: InstructionBuy,
							Quantity:    10.0,
							Instrument: OrderInstrument{
								AssetType: AssetTypeEquity,
								Symbol:    "MSFT",
							},
						},
					},
				}

				// Act: Convert to OrderRequest.
				req := OrderToRequest(&order)

				// Assert: Verify response-only fields are NOT in OrderRequest.
				// OrderRequest doesn't have these fields, so we just verify the conversion succeeds
				// and the basic fields are present.
				assert.Equal(t, SessionNormal, req.Session)
				assert.Equal(t, OrderTypeMarket, req.OrderType)
				assert.Equal(t, OrderStrategyTypeSingle, req.OrderStrategyType)
				assert.Len(t, req.OrderLegCollection, 1)
			},
		},
		{
			name: "nested child order strategies (recursive)",
			test: func(t *testing.T) {
				// Arrange: Create a deeply nested structure (parent -> child -> grandchild).
				price := 100.0
				qty := 5.0

				grandchild := Order{
					Session:           SessionNormal,
					Duration:          DurationDay,
					OrderType:         OrderTypeMarket,
					Quantity:          &qty,
					OrderStrategyType: OrderStrategyTypeSingle,
					OrderLegCollection: []OrderLegCollection{
						{
							Instruction: InstructionBuy,
							Quantity:    5.0,
							Instrument: OrderInstrument{
								AssetType: AssetTypeEquity,
								Symbol:    "GOOGL",
							},
						},
					},
				}

				child := Order{
					Session:              SessionNormal,
					Duration:             DurationDay,
					OrderType:            OrderTypeLimit,
					Price:                &price,
					Quantity:             &qty,
					OrderStrategyType:    OrderStrategyTypeTrigger,
					ChildOrderStrategies: []Order{grandchild},
					OrderLegCollection: []OrderLegCollection{
						{
							Instruction: InstructionSell,
							Quantity:    5.0,
							Instrument: OrderInstrument{
								AssetType: AssetTypeEquity,
								Symbol:    "GOOGL",
							},
						},
					},
				}

				parent := Order{
					Session:              SessionNormal,
					Duration:             DurationDay,
					OrderType:            OrderTypeMarket,
					OrderStrategyType:    OrderStrategyTypeTrigger,
					ChildOrderStrategies: []Order{child},
					OrderLegCollection: []OrderLegCollection{
						{
							Instruction: InstructionBuy,
							Quantity:    5.0,
							Instrument: OrderInstrument{
								AssetType: AssetTypeEquity,
								Symbol:    "GOOGL",
							},
						},
					},
				}

				// Act: Convert to OrderRequest.
				req := OrderToRequest(&parent)

				// Assert: Verify all levels are converted correctly.
				assert.Equal(t, OrderStrategyTypeTrigger, req.OrderStrategyType)
				assert.Len(t, req.ChildOrderStrategies, 1)

				childReq := req.ChildOrderStrategies[0]
				assert.Equal(t, OrderStrategyTypeTrigger, childReq.OrderStrategyType)
				assert.NotNil(t, childReq.Price)
				assert.InDelta(t, 100.0, *childReq.Price, 0.001)
				assert.Len(t, childReq.ChildOrderStrategies, 1)

				grandchildReq := childReq.ChildOrderStrategies[0]
				assert.Equal(t, OrderStrategyTypeSingle, grandchildReq.OrderStrategyType)
				assert.Empty(t, grandchildReq.ChildOrderStrategies)
			},
		},
		{
			name: "order with special instruction and tax lot method",
			test: func(t *testing.T) {
				// Arrange: Create an Order with special instruction and tax lot method.
				specialInstr := SpecialInstructionAllOrNone
				taxLot := TaxLotMethodFIFO

				order := Order{
					Session:            SessionNormal,
					Duration:           DurationDay,
					OrderType:          OrderTypeLimit,
					OrderStrategyType:  OrderStrategyTypeSingle,
					SpecialInstruction: &specialInstr,
					TaxLotMethod:       &taxLot,
					OrderLegCollection: []OrderLegCollection{
						{
							Instruction: InstructionSell,
							Quantity:    100.0,
							Instrument: OrderInstrument{
								AssetType: AssetTypeEquity,
								Symbol:    "IBM",
							},
						},
					},
				}

				// Act: Convert to OrderRequest.
				req := OrderToRequest(&order)

				// Assert: Verify special instruction and tax lot method are copied.
				assert.NotNil(t, req.SpecialInstruction)
				assert.Equal(t, SpecialInstructionAllOrNone, *req.SpecialInstruction)
				assert.NotNil(t, req.TaxLotMethod)
				assert.Equal(t, TaxLotMethodFIFO, *req.TaxLotMethod)
			},
		},
		{
			name: "cancel time is preserved",
			test: func(t *testing.T) {
				// Arrange: Order with CancelTime set.
				cancelTime := &SchwabTime{Time: time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC)}
				order := Order{
					Session:           SessionNormal,
					Duration:          DurationGoodTillCancel,
					OrderType:         OrderTypeLimit,
					CancelTime:        cancelTime,
					OrderStrategyType: OrderStrategyTypeSingle,
					OrderLegCollection: []OrderLegCollection{
						{
							Instruction: InstructionBuy,
							Quantity:    10.0,
							Instrument: OrderInstrument{
								AssetType: AssetTypeEquity,
								Symbol:    "AAPL",
							},
						},
					},
				}

				// Act
				req := OrderToRequest(&order)

				// Assert: CancelTime is copied.
				require.NotNil(t, req.CancelTime)
				assert.Equal(t, cancelTime.Time, req.CancelTime.Time)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.test(t)
		})
	}
}
