package models

// Transaction represents a transaction in an account.
type Transaction struct {
	ActivityID     *int64           `json:"activityId,omitempty"`
	Time           *string          `json:"time,omitempty"`
	Type           *TransactionType `json:"type,omitempty"`
	Status         *string          `json:"status,omitempty"`
	Description    *string          `json:"description,omitempty"`
	NetAmount      *float64         `json:"netAmount,omitempty"`
	AccountNumber  *string          `json:"accountNumber,omitempty"`
	SubAccount     *string          `json:"subAccount,omitempty"`
	ActivityType   *string          `json:"activityType,omitempty"`
	OrderID        *int64           `json:"orderId,omitempty"`
	PositionID     *int64           `json:"positionId,omitempty"`
	TradeDate      *string          `json:"tradeDate,omitempty"`
	SettlementDate *string          `json:"settlementDate,omitempty"`
	TransferItems  []TransferItem   `json:"transferItems,omitempty"`
	User           *UserDetails     `json:"user,omitempty"`
}

// TransferItem represents a transfer item in a transaction.
type TransferItem struct {
	Instrument       *TransferInstrument `json:"instrument,omitempty"`
	Position         *float64            `json:"position,omitempty"`
	PositionEffect   *string             `json:"positionEffect,omitempty"`
	PositionQuantity *float64            `json:"positionQuantity,omitempty"`
}

// TransferInstrument represents an instrument in a transfer.
type TransferInstrument struct {
	AssetType   *string `json:"assetType,omitempty"`
	Cusip       *string `json:"cusip,omitempty"`
	Symbol      *string `json:"symbol,omitempty"`
	Description *string `json:"description,omitempty"`
	Type        *string `json:"type,omitempty"`
	Exchange    *string `json:"exchange,omitempty"`
}

// UserDetails represents user details in a transaction.
type UserDetails struct {
	UserID *string `json:"userId,omitempty"`
	Name   *string `json:"name,omitempty"`
}
