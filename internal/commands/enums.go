package commands

// chainContractType is a CLI-only enum because Schwab's chain endpoint accepts
// ALL in addition to the CALL/PUT values represented by models.ContractType.
type chainContractType string

const (
	chainContractTypeCall chainContractType = "CALL"
	chainContractTypePut  chainContractType = "PUT"
	chainContractTypeAll  chainContractType = "ALL"
)

// chainStrategy is a CLI-only enum for the option-chain strategy query param.
type chainStrategy string

const (
	chainStrategySingle     chainStrategy = "SINGLE"
	chainStrategyAnalytical chainStrategy = "ANALYTICAL"
	chainStrategyCovered    chainStrategy = "COVERED"
	chainStrategyVertical   chainStrategy = "VERTICAL"
	chainStrategyCalendar   chainStrategy = "CALENDAR"
	chainStrategyStrangle   chainStrategy = "STRANGLE"
	chainStrategyStraddle   chainStrategy = "STRADDLE"
	chainStrategyButterfly  chainStrategy = "BUTTERFLY"
	chainStrategyCondor     chainStrategy = "CONDOR"
	chainStrategyDiagonal   chainStrategy = "DIAGONAL"
	chainStrategyCollar     chainStrategy = "COLLAR"
	chainStrategyRoll       chainStrategy = "ROLL"
)

// strikeRange is a CLI-only enum for option-chain moneyness filters.
type strikeRange string

const (
	strikeRangeITM strikeRange = "ITM"
	strikeRangeNTM strikeRange = "NTM"
	strikeRangeOTM strikeRange = "OTM"
	strikeRangeSAK strikeRange = "SAK"
	strikeRangeSBK strikeRange = "SBK"
	strikeRangeSNK strikeRange = "SNK"
	strikeRangeAll strikeRange = "ALL"
)

// historyPeriodType is a CLI-only enum for price history period type.
type historyPeriodType string

const (
	historyPeriodTypeDay   historyPeriodType = "day"
	historyPeriodTypeMonth historyPeriodType = "month"
	historyPeriodTypeYear  historyPeriodType = "year"
	historyPeriodTypeYTD   historyPeriodType = "ytd"
)

// historyFrequencyType is a CLI-only enum for price history frequency type.
type historyFrequencyType string

const (
	historyFrequencyTypeMinute  historyFrequencyType = "minute"
	historyFrequencyTypeDaily   historyFrequencyType = "daily"
	historyFrequencyTypeWeekly  historyFrequencyType = "weekly"
	historyFrequencyTypeMonthly historyFrequencyType = "monthly"
)

// historyFrequency is a CLI-only enum for known Schwab history frequencies.
type historyFrequency string

const (
	historyFrequency1  historyFrequency = "1"
	historyFrequency5  historyFrequency = "5"
	historyFrequency10 historyFrequency = "10"
	historyFrequency15 historyFrequency = "15"
	historyFrequency30 historyFrequency = "30"
)

// instrumentProjection is a CLI-only enum for instrument search projection.
type instrumentProjection string

const (
	instrumentProjectionSymbolSearch instrumentProjection = "symbol-search"
	instrumentProjectionSymbolRegex  instrumentProjection = "symbol-regex"
	instrumentProjectionDescSearch   instrumentProjection = "desc-search"
	instrumentProjectionDescRegex    instrumentProjection = "desc-regex"
	instrumentProjectionSearch       instrumentProjection = "search"
	instrumentProjectionFundamental  instrumentProjection = "fundamental"
)

// moversSort is a CLI-only enum for market movers sorting.
type moversSort string

const (
	moversSortVolume            moversSort = "VOLUME"
	moversSortTrades            moversSort = "TRADES"
	moversSortPercentChangeUp   moversSort = "PERCENT_CHANGE_UP"
	moversSortPercentChangeDown moversSort = "PERCENT_CHANGE_DOWN"
)

// moversFrequency is a CLI-only enum for market movers minimum change filters.
type moversFrequency string

const (
	moversFrequency0  moversFrequency = "0"
	moversFrequency1  moversFrequency = "1"
	moversFrequency5  moversFrequency = "5"
	moversFrequency10 moversFrequency = "10"
	moversFrequency30 moversFrequency = "30"
	moversFrequency60 moversFrequency = "60"
)

// positionSort is a CLI-only enum for local position-list sorting. Schwab does
// not sort account positions for us, so these values describe deterministic
// client-side orderings applied after all filters run.
type positionSort string

const (
	positionSortPnLDesc   positionSort = "pnl-desc"
	positionSortPnLAsc    positionSort = "pnl-asc"
	positionSortValueDesc positionSort = "value-desc"
)

// taInterval is a CLI-only enum shared by technical-analysis commands.
type taInterval string

const (
	taIntervalDaily  taInterval = "daily"
	taIntervalWeekly taInterval = "weekly"
	taInterval1Min   taInterval = "1min"
	taInterval5Min   taInterval = "5min"
	taInterval15Min  taInterval = "15min"
	taInterval30Min  taInterval = "30min"
)

// orderStatusFilter extends OrderStatus with the CLI-only ALL sentinel.
type orderStatusFilter string

const orderStatusFilterAll orderStatusFilter = "all"
