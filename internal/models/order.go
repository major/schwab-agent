package models

// Order and related type definitions for the Schwab API.
// Time fields use SchwabTime to handle the API's non-standard timestamp format.

// OrderType represents the type of order execution.
type OrderType string

const (
	OrderTypeMarket            OrderType = "MARKET"
	OrderTypeLimit             OrderType = "LIMIT"
	OrderTypeStop              OrderType = "STOP"
	OrderTypeStopLimit         OrderType = "STOP_LIMIT"
	OrderTypeTrailingStop      OrderType = "TRAILING_STOP"
	OrderTypeTrailingStopLimit OrderType = "TRAILING_STOP_LIMIT"
	OrderTypeCabinet           OrderType = "CABINET"
	OrderTypeNonMarketable     OrderType = "NON_MARKETABLE"
	OrderTypeMarketOnClose     OrderType = "MARKET_ON_CLOSE"
	OrderTypeExercise          OrderType = "EXERCISE"
	OrderTypeNetDebit          OrderType = "NET_DEBIT"
	OrderTypeNetCredit         OrderType = "NET_CREDIT"
	OrderTypeNetZero           OrderType = "NET_ZERO"
	OrderTypeLimitOnClose      OrderType = "LIMIT_ON_CLOSE"
	OrderTypeUnknown           OrderType = "UNKNOWN"
)

// Instruction represents the action to take on an order leg.
type Instruction string

const (
	InstructionBuy             Instruction = "BUY"
	InstructionSell            Instruction = "SELL"
	InstructionBuyToCover      Instruction = "BUY_TO_COVER"
	InstructionSellShort       Instruction = "SELL_SHORT"
	InstructionBuyToOpen       Instruction = "BUY_TO_OPEN"
	InstructionBuyToClose      Instruction = "BUY_TO_CLOSE"
	InstructionSellToOpen      Instruction = "SELL_TO_OPEN"
	InstructionSellToClose     Instruction = "SELL_TO_CLOSE"
	InstructionExchange        Instruction = "EXCHANGE"
	InstructionSellShortExempt Instruction = "SELL_SHORT_EXEMPT"
)

// Duration represents how long an order remains active.
type Duration string

const (
	DurationDay               Duration = "DAY"
	DurationGoodTillCancel    Duration = "GOOD_TILL_CANCEL"
	DurationFillOrKill        Duration = "FILL_OR_KILL"
	DurationImmediateOrCancel Duration = "IMMEDIATE_OR_CANCEL"
	DurationEndOfWeek         Duration = "END_OF_WEEK"
	DurationEndOfMonth        Duration = "END_OF_MONTH"
	DurationNextEndOfMonth    Duration = "NEXT_END_OF_MONTH"
	DurationUnknown           Duration = "UNKNOWN"
)

// Session represents the market session for order execution.
type Session string

const (
	SessionNormal   Session = "NORMAL"
	SessionAM       Session = "AM"
	SessionPM       Session = "PM"
	SessionSeamless Session = "SEAMLESS"
)

// OrderStatus represents the current status of an order.
type OrderStatus string

const (
	OrderStatusAwaitingParentOrder    OrderStatus = "AWAITING_PARENT_ORDER"
	OrderStatusAwaitingCondition      OrderStatus = "AWAITING_CONDITION"
	OrderStatusAwaitingStopCondition  OrderStatus = "AWAITING_STOP_CONDITION"
	OrderStatusAwaitingManualReview   OrderStatus = "AWAITING_MANUAL_REVIEW"
	OrderStatusAccepted               OrderStatus = "ACCEPTED"
	OrderStatusAwaitingUROut          OrderStatus = "AWAITING_UR_OUT"
	OrderStatusPendingActivation      OrderStatus = "PENDING_ACTIVATION"
	OrderStatusQueued                 OrderStatus = "QUEUED"
	OrderStatusWorking                OrderStatus = "WORKING"
	OrderStatusRejected               OrderStatus = "REJECTED"
	OrderStatusPendingCancel          OrderStatus = "PENDING_CANCEL"
	OrderStatusCanceled               OrderStatus = "CANCELED"
	OrderStatusPendingReplace         OrderStatus = "PENDING_REPLACE"
	OrderStatusReplaced               OrderStatus = "REPLACED"
	OrderStatusFilled                 OrderStatus = "FILLED"
	OrderStatusExpired                OrderStatus = "EXPIRED"
	OrderStatusNew                    OrderStatus = "NEW"
	OrderStatusAwaitingReleaseTime    OrderStatus = "AWAITING_RELEASE_TIME"
	OrderStatusPendingAcknowledgement OrderStatus = "PENDING_ACKNOWLEDGEMENT"
	OrderStatusPendingRecall          OrderStatus = "PENDING_RECALL"
	OrderStatusUnknown                OrderStatus = "UNKNOWN"
)

// OrderStrategyType represents how an order relates to other orders.
type OrderStrategyType string

const (
	OrderStrategyTypeSingle     OrderStrategyType = "SINGLE"
	OrderStrategyTypeCancel     OrderStrategyType = "CANCEL"
	OrderStrategyTypeRecall     OrderStrategyType = "RECALL"
	OrderStrategyTypePair       OrderStrategyType = "PAIR"
	OrderStrategyTypeFlatten    OrderStrategyType = "FLATTEN"
	OrderStrategyTypeTwoDaySwap OrderStrategyType = "TWO_DAY_SWAP"
	OrderStrategyTypeBlastAll   OrderStrategyType = "BLAST_ALL"
	OrderStrategyTypeOCO        OrderStrategyType = "OCO"
	OrderStrategyTypeTrigger    OrderStrategyType = "TRIGGER"
)

// ComplexOrderStrategyType represents multi-leg option strategy types.
type ComplexOrderStrategyType string

const (
	ComplexOrderStrategyTypeNone                   ComplexOrderStrategyType = "NONE"
	ComplexOrderStrategyTypeCovered                ComplexOrderStrategyType = "COVERED"
	ComplexOrderStrategyTypeVertical               ComplexOrderStrategyType = "VERTICAL"
	ComplexOrderStrategyTypeBackRatio              ComplexOrderStrategyType = "BACK_RATIO"
	ComplexOrderStrategyTypeCalendar               ComplexOrderStrategyType = "CALENDAR"
	ComplexOrderStrategyTypeDiagonal               ComplexOrderStrategyType = "DIAGONAL"
	ComplexOrderStrategyTypeStraddle               ComplexOrderStrategyType = "STRADDLE"
	ComplexOrderStrategyTypeStrangle               ComplexOrderStrategyType = "STRANGLE"
	ComplexOrderStrategyTypeCollarSynthetic        ComplexOrderStrategyType = "COLLAR_SYNTHETIC"
	ComplexOrderStrategyTypeButterfly              ComplexOrderStrategyType = "BUTTERFLY"
	ComplexOrderStrategyTypeCondor                 ComplexOrderStrategyType = "CONDOR"
	ComplexOrderStrategyTypeIronCondor             ComplexOrderStrategyType = "IRON_CONDOR"
	ComplexOrderStrategyTypeVerticalRoll           ComplexOrderStrategyType = "VERTICAL_ROLL"
	ComplexOrderStrategyTypeCollarWithStock        ComplexOrderStrategyType = "COLLAR_WITH_STOCK"
	ComplexOrderStrategyTypeDoubleDiagonal         ComplexOrderStrategyType = "DOUBLE_DIAGONAL"
	ComplexOrderStrategyTypeUnbalancedButterfly    ComplexOrderStrategyType = "UNBALANCED_BUTTERFLY"
	ComplexOrderStrategyTypeUnbalancedCondor       ComplexOrderStrategyType = "UNBALANCED_CONDOR"
	ComplexOrderStrategyTypeUnbalancedIronCondor   ComplexOrderStrategyType = "UNBALANCED_IRON_CONDOR"
	ComplexOrderStrategyTypeUnbalancedVerticalRoll ComplexOrderStrategyType = "UNBALANCED_VERTICAL_ROLL"
	ComplexOrderStrategyTypeMutualFundSwap         ComplexOrderStrategyType = "MUTUAL_FUND_SWAP"
	ComplexOrderStrategyTypeCustom                 ComplexOrderStrategyType = "CUSTOM"
)

// AssetType represents the type of financial instrument in an order.
type AssetType string

const (
	AssetTypeEquity               AssetType = "EQUITY"
	AssetTypeOption               AssetType = "OPTION"
	AssetTypeIndex                AssetType = "INDEX"
	AssetTypeMutualFund           AssetType = "MUTUAL_FUND"
	AssetTypeCashEquivalent       AssetType = "CASH_EQUIVALENT"
	AssetTypeFixedIncome          AssetType = "FIXED_INCOME"
	AssetTypeCurrency             AssetType = "CURRENCY"
	AssetTypeCollectiveInvestment AssetType = "COLLECTIVE_INVESTMENT"
	AssetTypeProduct              AssetType = "PRODUCT"
	AssetTypeFuture               AssetType = "FUTURE"
	AssetTypeForex                AssetType = "FOREX"
)

// PositionEffect represents whether a trade opens or closes a position.
type PositionEffect string

const (
	PositionEffectOpening   PositionEffect = "OPENING"
	PositionEffectClosing   PositionEffect = "CLOSING"
	PositionEffectAutomatic PositionEffect = "AUTOMATIC"
)

// PutCall represents the option contract direction.
type PutCall string

const (
	PutCallPut     PutCall = "PUT"
	PutCallCall    PutCall = "CALL"
	PutCallUnknown PutCall = "UNKNOWN"
)

// SpecialInstruction represents special handling instructions for an order.
type SpecialInstruction string

const (
	SpecialInstructionAllOrNone            SpecialInstruction = "ALL_OR_NONE"
	SpecialInstructionDoNotReduce          SpecialInstruction = "DO_NOT_REDUCE"
	SpecialInstructionAllOrNoneDoNotReduce SpecialInstruction = "ALL_OR_NONE_DO_NOT_REDUCE"
)

// TaxLotMethod represents the method for selecting tax lots when selling.
type TaxLotMethod string

const (
	TaxLotMethodFIFO          TaxLotMethod = "FIFO"
	TaxLotMethodLIFO          TaxLotMethod = "LIFO"
	TaxLotMethodHighCost      TaxLotMethod = "HIGH_COST"
	TaxLotMethodLowCost       TaxLotMethod = "LOW_COST"
	TaxLotMethodAverageCost   TaxLotMethod = "AVERAGE_COST"
	TaxLotMethodSpecificLot   TaxLotMethod = "SPECIFIC_LOT"
	TaxLotMethodLossHarvester TaxLotMethod = "LOSS_HARVESTER"
)

// RequestedDestination represents the exchange routing destination.
type RequestedDestination string

const (
	RequestedDestinationINET    RequestedDestination = "INET"
	RequestedDestinationECNArca RequestedDestination = "ECN_ARCA"
	RequestedDestinationCBOE    RequestedDestination = "CBOE"
	RequestedDestinationAMEX    RequestedDestination = "AMEX"
	RequestedDestinationPHLX    RequestedDestination = "PHLX"
	RequestedDestinationISE     RequestedDestination = "ISE"
	RequestedDestinationBOX     RequestedDestination = "BOX"
	RequestedDestinationNYSE    RequestedDestination = "NYSE"
	RequestedDestinationNASDAQ  RequestedDestination = "NASDAQ"
	RequestedDestinationBATS    RequestedDestination = "BATS"
	RequestedDestinationC2      RequestedDestination = "C2"
	RequestedDestinationAUTO    RequestedDestination = "AUTO"
)

// StopPriceLinkBasis represents the reference point for stop price calculation.
type StopPriceLinkBasis string

const (
	StopPriceLinkBasisManual  StopPriceLinkBasis = "MANUAL"
	StopPriceLinkBasisBase    StopPriceLinkBasis = "BASE"
	StopPriceLinkBasisTrigger StopPriceLinkBasis = "TRIGGER"
	StopPriceLinkBasisLast    StopPriceLinkBasis = "LAST"
	StopPriceLinkBasisBid     StopPriceLinkBasis = "BID"
	StopPriceLinkBasisAsk     StopPriceLinkBasis = "ASK"
	StopPriceLinkBasisAskBid  StopPriceLinkBasis = "ASK_BID"
	StopPriceLinkBasisMark    StopPriceLinkBasis = "MARK"
	StopPriceLinkBasisAverage StopPriceLinkBasis = "AVERAGE"
)

// StopPriceLinkType represents how the stop price offset is applied.
type StopPriceLinkType string

const (
	StopPriceLinkTypeValue   StopPriceLinkType = "VALUE"
	StopPriceLinkTypePercent StopPriceLinkType = "PERCENT"
	StopPriceLinkTypeTick    StopPriceLinkType = "TICK"
)

// PriceLinkBasis represents the reference point for limit price calculation.
type PriceLinkBasis string

const (
	PriceLinkBasisManual  PriceLinkBasis = "MANUAL"
	PriceLinkBasisBase    PriceLinkBasis = "BASE"
	PriceLinkBasisTrigger PriceLinkBasis = "TRIGGER"
	PriceLinkBasisLast    PriceLinkBasis = "LAST"
	PriceLinkBasisBid     PriceLinkBasis = "BID"
	PriceLinkBasisAsk     PriceLinkBasis = "ASK"
	PriceLinkBasisAskBid  PriceLinkBasis = "ASK_BID"
	PriceLinkBasisMark    PriceLinkBasis = "MARK"
	PriceLinkBasisAverage PriceLinkBasis = "AVERAGE"
)

// PriceLinkType represents how the price offset is applied.
type PriceLinkType string

const (
	PriceLinkTypeValue   PriceLinkType = "VALUE"
	PriceLinkTypePercent PriceLinkType = "PERCENT"
	PriceLinkTypeTick    PriceLinkType = "TICK"
)

// StopType represents the price basis for stop orders.
type StopType string

const (
	StopTypeStandard StopType = "STANDARD"
	StopTypeBid      StopType = "BID"
	StopTypeAsk      StopType = "ASK"
	StopTypeLast     StopType = "LAST"
	StopTypeMark     StopType = "MARK"
)

// SettlementInstruction represents the settlement type for an order.
type SettlementInstruction string

const (
	SettlementInstructionRegular SettlementInstruction = "REGULAR"
	SettlementInstructionCash    SettlementInstruction = "CASH"
	SettlementInstructionNextDay SettlementInstruction = "NEXT_DAY"
	SettlementInstructionUnknown SettlementInstruction = "UNKNOWN"
)

// AmountIndicator represents how the order quantity is specified.
type AmountIndicator string

const (
	AmountIndicatorDollars    AmountIndicator = "DOLLARS"
	AmountIndicatorShares     AmountIndicator = "SHARES"
	AmountIndicatorAllShares  AmountIndicator = "ALL_SHARES"
	AmountIndicatorPercentage AmountIndicator = "PERCENTAGE"
	AmountIndicatorUnknown    AmountIndicator = "UNKNOWN"
)

// AdvancedOrderType represents advanced composite order types.
type AdvancedOrderType string

const (
	AdvancedOrderTypeNone     AdvancedOrderType = "NONE"
	AdvancedOrderTypeOTO      AdvancedOrderType = "OTO"
	AdvancedOrderTypeOCO      AdvancedOrderType = "OCO"
	AdvancedOrderTypeOTOCO    AdvancedOrderType = "OTOCO"
	AdvancedOrderTypeOT2OCO   AdvancedOrderType = "OT2OCO"
	AdvancedOrderTypeOT3OCO   AdvancedOrderType = "OT3OCO"
	AdvancedOrderTypeBlastAll AdvancedOrderType = "BLAST_ALL"
	AdvancedOrderTypeOTA      AdvancedOrderType = "OTA"
	AdvancedOrderTypePair     AdvancedOrderType = "PAIR"
)

// QuantityType represents how the leg quantity is expressed.
type QuantityType string

const (
	QuantityTypeAllShares QuantityType = "ALL_SHARES"
	QuantityTypeDollars   QuantityType = "DOLLARS"
	QuantityTypeShares    QuantityType = "SHARES"
)

// DivCapGains represents dividend and capital gains reinvestment preference.
type DivCapGains string

const (
	DivCapGainsReinvest DivCapGains = "REINVEST"
	DivCapGainsPayout   DivCapGains = "PAYOUT"
)

// ActivityType represents the type of order activity.
type ActivityType string

const (
	ActivityTypeExecution   ActivityType = "EXECUTION"
	ActivityTypeOrderAction ActivityType = "ORDER_ACTION"
)

// ExecutionType represents the type of execution.
type ExecutionType string

const (
	ExecutionTypeFill     ExecutionType = "FILL"
	ExecutionTypeCanceled ExecutionType = "CANCELED"
)

// RuleAction represents the action taken by an API validation rule.
type RuleAction string

const (
	RuleActionAccept  RuleAction = "ACCEPT"
	RuleActionAlert   RuleAction = "ALERT"
	RuleActionReject  RuleAction = "REJECT"
	RuleActionReview  RuleAction = "REVIEW"
	RuleActionUnknown RuleAction = "UNKNOWN"
)

// OrderInstrument represents a financial instrument within an order leg.
// For equity orders, only AssetType and Symbol are needed.
// For option orders, PutCall, UnderlyingSymbol, and related fields are populated.
type OrderInstrument struct {
	AssetType            AssetType `json:"assetType"`
	Symbol               string    `json:"symbol"`
	CUSIP                *string   `json:"cusip,omitempty"`
	Description          *string   `json:"description,omitempty"`
	InstrumentID         *int64    `json:"instrumentId,omitempty"`
	NetChange            *float64  `json:"netChange,omitempty"`
	PutCall              *PutCall  `json:"putCall,omitempty"`
	UnderlyingSymbol     *string   `json:"underlyingSymbol,omitempty"`
	OptionMultiplier     *float64  `json:"optionMultiplier,omitempty"`
	OptionExpirationDate *string   `json:"optionExpirationDate,omitempty"`
	OptionStrikePrice    *float64  `json:"optionStrikePrice,omitempty"`
}

// OrderLegCollection represents a single leg of an order.
type OrderLegCollection struct {
	OrderLegType   *AssetType      `json:"orderLegType,omitempty"`
	LegID          *int64          `json:"legId,omitempty"`
	Instrument     OrderInstrument `json:"instrument"`
	Instruction    Instruction     `json:"instruction"`
	PositionEffect *PositionEffect `json:"positionEffect,omitempty"`
	Quantity       float64         `json:"quantity"`
	QuantityType   *QuantityType   `json:"quantityType,omitempty"`
	DivCapGains    *DivCapGains    `json:"divCapGains,omitempty"`
	ToSymbol       *string         `json:"toSymbol,omitempty"`
}

// ExecutionLeg represents a single execution within an order activity.
type ExecutionLeg struct {
	LegID             int64      `json:"legId"`
	Price             float64    `json:"price"`
	Quantity          float64    `json:"quantity"`
	MismarkedQuantity float64    `json:"mismarkedQuantity"`
	InstrumentID      int64      `json:"instrumentId"`
	Time              SchwabTime `json:"time"`
}

// OrderActivity represents an activity (execution or action) on an order.
type OrderActivity struct {
	ActivityType           ActivityType   `json:"activityType"`
	ExecutionType          *ExecutionType `json:"executionType,omitempty"`
	Quantity               float64        `json:"quantity"`
	OrderRemainingQuantity float64        `json:"orderRemainingQuantity"`
	ExecutionLegs          []ExecutionLeg `json:"executionLegs,omitempty"`
}

// Order represents an order response from the Schwab API.
// This is the full order object returned by GET /orders endpoints.
type Order struct {
	Session                  Session                   `json:"session"`
	Duration                 Duration                  `json:"duration"`
	OrderType                OrderType                 `json:"orderType"`
	CancelTime               *SchwabTime               `json:"cancelTime,omitempty"`
	ComplexOrderStrategyType *ComplexOrderStrategyType `json:"complexOrderStrategyType,omitempty"`
	Quantity                 *float64                  `json:"quantity,omitempty"`
	FilledQuantity           *float64                  `json:"filledQuantity,omitempty"`
	RemainingQuantity        *float64                  `json:"remainingQuantity,omitempty"`
	RequestedDestination     *RequestedDestination     `json:"requestedDestination,omitempty"`
	DestinationLinkName      *string                   `json:"destinationLinkName,omitempty"`
	ReleaseTime              *SchwabTime               `json:"releaseTime,omitempty"`
	StopPrice                *float64                  `json:"stopPrice,omitempty"`
	StopPriceLinkBasis       *StopPriceLinkBasis       `json:"stopPriceLinkBasis,omitempty"`
	StopPriceLinkType        *StopPriceLinkType        `json:"stopPriceLinkType,omitempty"`
	StopPriceOffset          *float64                  `json:"stopPriceOffset,omitempty"`
	StopType                 *StopType                 `json:"stopType,omitempty"`
	PriceLinkBasis           *PriceLinkBasis           `json:"priceLinkBasis,omitempty"`
	PriceLinkType            *PriceLinkType            `json:"priceLinkType,omitempty"`
	PriceOffset              *float64                  `json:"priceOffset,omitempty"`
	Price                    *float64                  `json:"price,omitempty"`
	TaxLotMethod             *TaxLotMethod             `json:"taxLotMethod,omitempty"`
	OrderLegCollection       []OrderLegCollection      `json:"orderLegCollection"`
	ActivationPrice          *float64                  `json:"activationPrice,omitempty"`
	SpecialInstruction       *SpecialInstruction       `json:"specialInstruction,omitempty"`
	OrderStrategyType        OrderStrategyType         `json:"orderStrategyType"`
	OrderID                  *int64                    `json:"orderId,omitempty"`
	Cancelable               *bool                     `json:"cancelable,omitempty"`
	Editable                 *bool                     `json:"editable,omitempty"`
	Status                   *OrderStatus              `json:"status,omitempty"`
	EnteredTime              *SchwabTime               `json:"enteredTime,omitempty"`
	CloseTime                *SchwabTime               `json:"closeTime,omitempty"`
	Tag                      *string                   `json:"tag,omitempty"`
	AccountNumber            *int64                    `json:"accountNumber,omitempty"`
	OrderActivityCollection  []OrderActivity           `json:"orderActivityCollection,omitempty"`
	ReplacingOrderCollection []Order                   `json:"replacingOrderCollection,omitempty"`
	ChildOrderStrategies     []Order                   `json:"childOrderStrategies,omitempty"`
	StatusDescription        *string                   `json:"statusDescription,omitempty"`
}

// OrderRequest represents an order submission to the Schwab API.
// Used for placing new orders and replacing existing orders.
type OrderRequest struct {
	Session                  Session                   `json:"session,omitempty"`
	Duration                 Duration                  `json:"duration,omitempty"`
	OrderType                OrderType                 `json:"orderType,omitempty"`
	CancelTime               *SchwabTime               `json:"cancelTime,omitempty"`
	ComplexOrderStrategyType *ComplexOrderStrategyType `json:"complexOrderStrategyType,omitempty"`
	Quantity                 *float64                  `json:"quantity,omitempty"`
	RequestedDestination     *RequestedDestination     `json:"requestedDestination,omitempty"`
	DestinationLinkName      *string                   `json:"destinationLinkName,omitempty"`
	StopPrice                *float64                  `json:"stopPrice,omitempty"`
	StopPriceLinkBasis       *StopPriceLinkBasis       `json:"stopPriceLinkBasis,omitempty"`
	StopPriceLinkType        *StopPriceLinkType        `json:"stopPriceLinkType,omitempty"`
	StopPriceOffset          *float64                  `json:"stopPriceOffset,omitempty"`
	StopType                 *StopType                 `json:"stopType,omitempty"`
	PriceLinkBasis           *PriceLinkBasis           `json:"priceLinkBasis,omitempty"`
	PriceLinkType            *PriceLinkType            `json:"priceLinkType,omitempty"`
	PriceOffset              *float64                  `json:"priceOffset,omitempty"`
	Price                    *float64                  `json:"price,omitempty"`
	TaxLotMethod             *TaxLotMethod             `json:"taxLotMethod,omitempty"`
	OrderLegCollection       []OrderLegCollection      `json:"orderLegCollection,omitempty"`
	ActivationPrice          *float64                  `json:"activationPrice,omitempty"`
	SpecialInstruction       *SpecialInstruction       `json:"specialInstruction,omitempty"`
	OrderStrategyType        OrderStrategyType         `json:"orderStrategyType"`
	ChildOrderStrategies     []OrderRequest            `json:"childOrderStrategies,omitempty"`
}

// OrderValidationDetail represents a single validation finding from order preview.
type OrderValidationDetail struct {
	ValidationRuleName string      `json:"validationRuleName"`
	Message            string      `json:"message"`
	ActivityMessage    *string     `json:"activityMessage,omitempty"`
	OriginalSeverity   *RuleAction `json:"originalSeverity,omitempty"`
	OverrideName       *string     `json:"overrideName,omitempty"`
	OverrideSeverity   *RuleAction `json:"overrideSeverity,omitempty"`
}

// OrderValidationResult contains the validation findings from an order preview.
type OrderValidationResult struct {
	Alerts  []OrderValidationDetail `json:"alerts,omitempty"`
	Accepts []OrderValidationDetail `json:"accepts,omitempty"`
	Rejects []OrderValidationDetail `json:"rejects,omitempty"`
	Reviews []OrderValidationDetail `json:"reviews,omitempty"`
	Warns   []OrderValidationDetail `json:"warns,omitempty"`
}

// CommissionValue represents a single cost component (e.g., base charge, SEC fee).
type CommissionValue struct {
	Value *float64 `json:"value,omitempty"`
	Type  string   `json:"type"`
}

// CommissionLeg groups cost values for one leg of an order.
type CommissionLeg struct {
	CommissionValues []CommissionValue `json:"commissionValues,omitempty"`
}

// CommissionDetail represents the commission breakdown returned by the API.
// The API nests commissions as objects with commissionLegs arrays, not flat floats.
type CommissionDetail struct {
	CommissionLegs []CommissionLeg `json:"commissionLegs,omitempty"`
}

// FeeValue represents a single fee component (e.g., SEC fee, TAF fee).
type FeeValue struct {
	Value *float64 `json:"value,omitempty"`
	Type  string   `json:"type"`
}

// FeeLeg groups fee values for one leg of an order.
type FeeLeg struct {
	FeeValues []FeeValue `json:"feeValues,omitempty"`
}

// FeeDetail represents the fee breakdown returned by the API.
type FeeDetail struct {
	FeeLegs []FeeLeg `json:"feeLegs,omitempty"`
}

// CommissionAndFee represents the estimated costs for an order.
// The Schwab API returns commission/fee/trueCommission as nested objects
// (not flat floats), with per-leg breakdowns by cost type.
type CommissionAndFee struct {
	TotalCommission *float64          `json:"totalCommission,omitempty"`
	TotalFees       *float64          `json:"totalFees,omitempty"`
	Commission      *CommissionDetail `json:"commission,omitempty"`
	Fee             *FeeDetail        `json:"fee,omitempty"`
	TrueCommission  *CommissionDetail `json:"trueCommission,omitempty"`
}

// OrderBalance represents the projected account balance impact of an order.
type OrderBalance struct {
	OrderValue             *float64 `json:"orderValue,omitempty"`
	ProjectedAvailableFund *float64 `json:"projectedAvailableFund,omitempty"`
	ProjectedBuyingPower   *float64 `json:"projectedBuyingPower,omitempty"`
	ProjectedCommission    *float64 `json:"projectedCommission,omitempty"`
}

// OrderStrategy represents an order within a preview response.
type OrderStrategy struct {
	AccountNumber            *string                   `json:"accountNumber,omitempty"`
	AdvancedOrderType        *AdvancedOrderType        `json:"advancedOrderType,omitempty"`
	OrderType                OrderType                 `json:"orderType"`
	Status                   *OrderStatus              `json:"status,omitempty"`
	Duration                 Duration                  `json:"duration"`
	Session                  Session                   `json:"session"`
	Price                    *float64                  `json:"price,omitempty"`
	Quantity                 *float64                  `json:"quantity,omitempty"`
	OrderStrategyType        OrderStrategyType         `json:"orderStrategyType"`
	ComplexOrderStrategyType *ComplexOrderStrategyType `json:"complexOrderStrategyType,omitempty"`
	OrderLegs                []OrderLegCollection      `json:"orderLegs,omitempty"`
	OrderBalance             *OrderBalance             `json:"orderBalance,omitempty"`
	CommissionAndFee         *CommissionAndFee         `json:"commissionAndFee,omitempty"`
}

// PreviewOrder represents the response from the order preview endpoint.
type PreviewOrder struct {
	OrderID               *int64                 `json:"orderId,omitempty"`
	OrderStrategy         *OrderStrategy         `json:"orderStrategy,omitempty"`
	OrderValidationResult *OrderValidationResult `json:"orderValidationResult,omitempty"`
	CommissionAndFee      *CommissionAndFee      `json:"commissionAndFee,omitempty"`
}

// OrderToRequest converts an Order (response) to an OrderRequest (submission) using an allowlist approach.
// Only fields that exist in OrderRequest are copied from Order.
// ChildOrderStrategies are recursively converted from []Order to []OrderRequest.
// Response-only fields (OrderID, Status, FilledQuantity, etc.) are intentionally excluded.
func OrderToRequest(order *Order) OrderRequest {
	// Defensive copy of OrderLegCollection so the caller's slice is not aliased.
	var legs []OrderLegCollection
	if len(order.OrderLegCollection) > 0 {
		legs = append([]OrderLegCollection(nil), order.OrderLegCollection...)
	}

	req := OrderRequest{
		Session:                  order.Session,
		Duration:                 order.Duration,
		OrderType:                order.OrderType,
		CancelTime:               order.CancelTime,
		ComplexOrderStrategyType: order.ComplexOrderStrategyType,
		Quantity:                 order.Quantity,
		RequestedDestination:     order.RequestedDestination,
		DestinationLinkName:      order.DestinationLinkName,
		StopPrice:                order.StopPrice,
		StopPriceLinkBasis:       order.StopPriceLinkBasis,
		StopPriceLinkType:        order.StopPriceLinkType,
		StopPriceOffset:          order.StopPriceOffset,
		StopType:                 order.StopType,
		PriceLinkBasis:           order.PriceLinkBasis,
		PriceLinkType:            order.PriceLinkType,
		PriceOffset:              order.PriceOffset,
		Price:                    order.Price,
		TaxLotMethod:             order.TaxLotMethod,
		OrderLegCollection:       legs,
		ActivationPrice:          order.ActivationPrice,
		SpecialInstruction:       order.SpecialInstruction,
		OrderStrategyType:        order.OrderStrategyType,
	}

	// Recursively convert child order strategies from []Order to []OrderRequest.
	if len(order.ChildOrderStrategies) > 0 {
		req.ChildOrderStrategies = make([]OrderRequest, len(order.ChildOrderStrategies))
		for i := range order.ChildOrderStrategies {
			req.ChildOrderStrategies[i] = OrderToRequest(&order.ChildOrderStrategies[i])
		}
	}

	return req
}
