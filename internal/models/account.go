package models

// AccountNumber represents a Schwab account number with its hash value.
type AccountNumber struct {
	AccountNumber string `json:"accountNumber"`
	HashValue     string `json:"hashValue"`
}

// Account represents a Schwab trading account.
// NickName and PrimaryAccount are enriched from the user preferences API
// during account list/get operations, not from the accounts API itself.
type Account struct {
	SecuritiesAccount *SecuritiesAccount `json:"securitiesAccount,omitempty"`
	NickName          *string            `json:"nickName,omitempty"`
	PrimaryAccount    *bool              `json:"primaryAccount,omitempty"`
}

// SecuritiesAccount represents the securities account details (can be MARGIN or CASH).
type SecuritiesAccount struct {
	Type              *string               `json:"type,omitempty"`
	AccountNumber     *string               `json:"accountNumber,omitempty"`
	RoundTrips        *int                  `json:"roundTrips,omitempty"`
	IsForeign         *bool                 `json:"isForeign,omitempty"`
	CurrentBalances   *MarginBalance        `json:"currentBalances,omitempty"`
	InitialBalances   *MarginInitialBalance `json:"initialBalances,omitempty"`
	ProjectedBalances *MarginBalance        `json:"projectedBalances,omitempty"`
	Positions         []Position            `json:"positions,omitempty"`
}

// Position represents a position in an account.
type Position struct {
	ShortQuantity                  *float64            `json:"shortQuantity,omitempty"`
	AverageLongPrice               *float64            `json:"averageLongPrice,omitempty"`
	CurrentDayProfitLoss           *float64            `json:"currentDayProfitLoss,omitempty"`
	CurrentDayProfitLossPercentage *float64            `json:"currentDayProfitLossPercentage,omitempty"`
	LongOpenProfitLoss             *float64            `json:"longOpenProfitLoss,omitempty"`
	PreviousSessionLongQuantity    *float64            `json:"previousSessionLongQuantity,omitempty"`
	AverageShortPrice              *float64            `json:"averageShortPrice,omitempty"`
	CurrentDayCost                 *float64            `json:"currentDayCost,omitempty"`
	LongQuantity                   *float64            `json:"longQuantity,omitempty"`
	ShortOpenProfitLoss            *float64            `json:"shortOpenProfitLoss,omitempty"`
	PreviousSessionShortQuantity   *float64            `json:"previousSessionShortQuantity,omitempty"`
	MaintenanceRequirement         *float64            `json:"maintenanceRequirement,omitempty"`
	AveragePrice                   *float64            `json:"averagePrice,omitempty"`
	TaxLotAverageLongPrice         *float64            `json:"taxLotAverageLongPrice,omitempty"`
	TaxLotAverageShortPrice        *float64            `json:"taxLotAverageShortPrice,omitempty"`
	SettledLongQuantity            *float64            `json:"settledLongQuantity,omitempty"`
	SettledShortQuantity           *float64            `json:"settledShortQuantity,omitempty"`
	AgedQuantity                   *float64            `json:"agedQuantity,omitempty"`
	MarketValue                    *float64            `json:"marketValue,omitempty"`
	Instrument                     *AccountsInstrument `json:"instrument,omitempty"`
}

// AccountsInstrument represents an instrument in an account position.
type AccountsInstrument struct {
	AssetType   *string `json:"assetType,omitempty"`
	Cusip       *string `json:"cusip,omitempty"`
	Symbol      *string `json:"symbol,omitempty"`
	Description *string `json:"description,omitempty"`
	Type        *string `json:"type,omitempty"`
	Exchange    *string `json:"exchange,omitempty"`
}

// MarginBalance represents margin account balance information.
type MarginBalance struct {
	ClosingTradesPL                  *float64 `json:"closingTradesPL,omitempty"`
	FutureOptionBuyingPower          *float64 `json:"futureOptionBuyingPower,omitempty"`
	NetLiquidatingValue              *float64 `json:"netLiquidatingValue,omitempty"`
	CashBalance                      *float64 `json:"cashBalance,omitempty"`
	CashReceipts                     *float64 `json:"cashReceipts,omitempty"`
	ProjectedCashBalance             *float64 `json:"projectedCashBalance,omitempty"`
	SegmentIsolationEquity           *float64 `json:"segmentIsolationEquity,omitempty"`
	LongStockValue                   *float64 `json:"longStockValue,omitempty"`
	MutualFundValue                  *float64 `json:"mutualFundValue,omitempty"`
	LongOptionNonIraValue            *float64 `json:"longOptionNonIraValue,omitempty"`
	LongNonMarginableValue           *float64 `json:"longNonMarginableValue,omitempty"`
	LongMarginableValue              *float64 `json:"longMarginableValue,omitempty"`
	PendingDeposits                  *float64 `json:"pendingDeposits,omitempty"`
	MarginCallValue                  *float64 `json:"marginCallValue,omitempty"`
	TotalCash                        *float64 `json:"totalCash,omitempty"`
	CashAvailableForTrading          *float64 `json:"cashAvailableForTrading,omitempty"`
	CashAvailableForWithdrawal       *float64 `json:"cashAvailableForWithdrawal,omitempty"`
	CashCall                         *float64 `json:"cashCall,omitempty"`
	LongNonMarginableMarketValue     *float64 `json:"longNonMarginableMarketValue,omitempty"`
	TotalLongValue                   *float64 `json:"totalLongValue,omitempty"`
	DayTradingBuyingPower            *float64 `json:"dayTradingBuyingPower,omitempty"`
	DayTradingBuyingPowerCall        *float64 `json:"dayTradingBuyingPowerCall,omitempty"`
	OptionBuyingPower                *float64 `json:"optionBuyingPower,omitempty"`
	EquityPercentage                 *float64 `json:"equityPercentage,omitempty"`
	Equity                           *float64 `json:"equity,omitempty"`
	ShortMarginValue                 *float64 `json:"shortMarginValue,omitempty"`
	ShortBalance                     *float64 `json:"shortBalance,omitempty"`
	SMA                              *float64 `json:"sma,omitempty"`
	BuyingPower                      *float64 `json:"buyingPower,omitempty"`
	RegTCall                         *float64 `json:"regTCall,omitempty"`
	MaintenanceCall                  *float64 `json:"maintenanceCall,omitempty"`
	MaintenanceRequirement           *float64 `json:"maintenanceRequirement,omitempty"`
	AmountNeededToMaintainEquity     *float64 `json:"amountNeededToMaintainEquity,omitempty"`
	IsInCall                         *bool    `json:"isInCall,omitempty"`
	StockBuyingPower                 *float64 `json:"stockBuyingPower,omitempty"`
	BuyingPowerNonMarginableTrade    *float64 `json:"buyingPowerNonMarginableTrade,omitempty"`
	AvailableFundsNonMarginableTrade *float64 `json:"availableFundsNonMarginableTrade,omitempty"`
	AvailableFunds                   *float64 `json:"availableFunds,omitempty"`
	LongMarginValue                  *float64 `json:"longMarginValue,omitempty"`
	MarginBalance                    *float64 `json:"marginBalance,omitempty"`
	CashDebitCallValue               *float64 `json:"cashDebitCallValue,omitempty"`
}

// MarginInitialBalance represents initial balance for margin accounts. Schwab
// returns the same field set for current and initial margin balances, so keep a
// distinct named type without duplicating the schema body.
type MarginInitialBalance MarginBalance

// CashBalance represents cash account balance information.
type CashBalance struct {
	ClosingTradesPL              *float64 `json:"closingTradesPL,omitempty"`
	FutureOptionBuyingPower      *float64 `json:"futureOptionBuyingPower,omitempty"`
	NetLiquidatingValue          *float64 `json:"netLiquidatingValue,omitempty"`
	CashBalance                  *float64 `json:"cashBalance,omitempty"`
	CashReceipts                 *float64 `json:"cashReceipts,omitempty"`
	ProjectedCashBalance         *float64 `json:"projectedCashBalance,omitempty"`
	SegmentIsolationEquity       *float64 `json:"segmentIsolationEquity,omitempty"`
	LongStockValue               *float64 `json:"longStockValue,omitempty"`
	MutualFundValue              *float64 `json:"mutualFundValue,omitempty"`
	LongOptionNonIraValue        *float64 `json:"longOptionNonIraValue,omitempty"`
	LongNonMarginableValue       *float64 `json:"longNonMarginableValue,omitempty"`
	LongMarginableValue          *float64 `json:"longMarginableValue,omitempty"`
	PendingDeposits              *float64 `json:"pendingDeposits,omitempty"`
	MarginCallValue              *float64 `json:"marginCallValue,omitempty"`
	TotalCash                    *float64 `json:"totalCash,omitempty"`
	CashAvailableForTrading      *float64 `json:"cashAvailableForTrading,omitempty"`
	CashAvailableForWithdrawal   *float64 `json:"cashAvailableForWithdrawal,omitempty"`
	CashCall                     *float64 `json:"cashCall,omitempty"`
	LongNonMarginableMarketValue *float64 `json:"longNonMarginableMarketValue,omitempty"`
	TotalLongValue               *float64 `json:"totalLongValue,omitempty"`
	UnsettledCash                *float64 `json:"unsettledCash,omitempty"`
	CashDebitCallValue           *float64 `json:"cashDebitCallValue,omitempty"`
}

// CashInitialBalance represents initial balance for cash accounts.
type CashInitialBalance struct {
	ClosingTradesPL              *float64 `json:"closingTradesPL,omitempty"`
	FutureOptionBuyingPower      *float64 `json:"futureOptionBuyingPower,omitempty"`
	NetLiquidatingValue          *float64 `json:"netLiquidatingValue,omitempty"`
	CashBalance                  *float64 `json:"cashBalance,omitempty"`
	CashReceipts                 *float64 `json:"cashReceipts,omitempty"`
	ProjectedCashBalance         *float64 `json:"projectedCashBalance,omitempty"`
	SegmentIsolationEquity       *float64 `json:"segmentIsolationEquity,omitempty"`
	LongStockValue               *float64 `json:"longStockValue,omitempty"`
	MutualFundValue              *float64 `json:"mutualFundValue,omitempty"`
	LongOptionNonIraValue        *float64 `json:"longOptionNonIraValue,omitempty"`
	LongNonMarginableValue       *float64 `json:"longNonMarginableValue,omitempty"`
	LongMarginableValue          *float64 `json:"longMarginableValue,omitempty"`
	PendingDeposits              *float64 `json:"pendingDeposits,omitempty"`
	MarginCallValue              *float64 `json:"marginCallValue,omitempty"`
	TotalCash                    *float64 `json:"totalCash,omitempty"`
	CashAvailableForTrading      *float64 `json:"cashAvailableForTrading,omitempty"`
	CashAvailableForWithdrawal   *float64 `json:"cashAvailableForWithdrawal,omitempty"`
	CashCall                     *float64 `json:"cashCall,omitempty"`
	LongNonMarginableMarketValue *float64 `json:"longNonMarginableMarketValue,omitempty"`
	TotalLongValue               *float64 `json:"totalLongValue,omitempty"`
	UnsettledCash                *float64 `json:"unsettledCash,omitempty"`
	CashDebitCallValue           *float64 `json:"cashDebitCallValue,omitempty"`
}
