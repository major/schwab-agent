package commands

import (
	"github.com/leodido/structcli"

	"github.com/major/schwab-agent/internal/models"
)

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

// init registers CLI-facing enums with structcli without adding a structcli
// dependency to the pure models package. Registered enums populate JSON Schema,
// shell completions, and pflag validation for typed option fields.
func init() {
	structcli.RegisterEnum(enumMapWithEmpty(validInstructions))
	structcli.RegisterEnum(enumMapWithEmptyAndAliases(validOrderTypes, map[models.OrderType][]string{
		models.OrderTypeMarketOnClose: []string{"MOC"},
		models.OrderTypeLimitOnClose:  []string{"LOC"},
	}))
	structcli.RegisterEnum(enumMapWithEmptyAndAliases(validDurations, map[models.Duration][]string{
		models.DurationGoodTillCancel:    []string{"GTC"},
		models.DurationFillOrKill:        []string{"FOK"},
		models.DurationImmediateOrCancel: []string{"IOC"},
	}))
	structcli.RegisterEnum(enumMapWithEmpty(validSessions))
	structcli.RegisterEnum(enumMapWithEmpty(validStopPriceLinkBases))
	structcli.RegisterEnum(enumMapWithEmpty(validStopPriceLinkTypes))
	structcli.RegisterEnum(enumMapWithEmpty(validStopTypes))
	structcli.RegisterEnum(enumMapWithEmpty(validSpecialInstructions))
	structcli.RegisterEnum(enumMapWithEmpty(validDestinations))
	structcli.RegisterEnum(enumMapWithEmpty(validPriceLinkBases))
	structcli.RegisterEnum(enumMapWithEmpty(validPriceLinkTypes))

	structcli.RegisterEnum(enumMap(validOrderStatusFilters))
	structcli.RegisterEnum(enumMapWithEmpty([]chainContractType{chainContractTypeCall, chainContractTypePut, chainContractTypeAll}))
	structcli.RegisterEnum(enumMapWithEmpty([]chainStrategy{chainStrategySingle, chainStrategyAnalytical, chainStrategyCovered, chainStrategyVertical, chainStrategyCalendar, chainStrategyStrangle, chainStrategyStraddle, chainStrategyButterfly, chainStrategyCondor, chainStrategyDiagonal, chainStrategyCollar, chainStrategyRoll}))
	structcli.RegisterEnum(enumMapWithEmpty([]strikeRange{strikeRangeITM, strikeRangeNTM, strikeRangeOTM, strikeRangeSAK, strikeRangeSBK, strikeRangeSNK, strikeRangeAll}))
	structcli.RegisterEnum(enumMapWithEmpty([]historyPeriodType{historyPeriodTypeDay, historyPeriodTypeMonth, historyPeriodTypeYear, historyPeriodTypeYTD}))
	structcli.RegisterEnum(enumMapWithEmpty([]historyFrequencyType{historyFrequencyTypeMinute, historyFrequencyTypeDaily, historyFrequencyTypeWeekly, historyFrequencyTypeMonthly}))
	structcli.RegisterEnum(enumMapWithEmpty([]historyFrequency{historyFrequency1, historyFrequency5, historyFrequency10, historyFrequency15, historyFrequency30}))
	structcli.RegisterEnum(enumMap([]instrumentProjection{instrumentProjectionSymbolSearch, instrumentProjectionSymbolRegex, instrumentProjectionDescSearch, instrumentProjectionDescRegex, instrumentProjectionSearch, instrumentProjectionFundamental}))
	structcli.RegisterEnum(enumMapWithEmpty([]moversSort{moversSortVolume, moversSortTrades, moversSortPercentChangeUp, moversSortPercentChangeDown}))
	structcli.RegisterEnum(enumMapWithEmpty([]moversFrequency{moversFrequency0, moversFrequency1, moversFrequency5, moversFrequency10, moversFrequency30, moversFrequency60}))
	structcli.RegisterEnum(enumMap([]taInterval{taIntervalDaily, taIntervalWeekly, taInterval1Min, taInterval5Min, taInterval15Min, taInterval30Min}))
}

func enumMap[E ~string](values []E) map[E][]string {
	registered := make(map[E][]string, len(values))
	for _, value := range values {
		registered[value] = []string{string(value)}
	}
	return registered
}

func enumMapWithEmpty[E ~string](values []E) map[E][]string {
	registered := enumMap(values)
	registered[""] = []string{""}
	return registered
}

func enumMapWithEmptyAndAliases[E ~string](values []E, aliases map[E][]string) map[E][]string {
	registered := enumMapWithEmpty(values)
	for value, valueAliases := range aliases {
		registered[value] = append(registered[value], valueAliases...)
	}
	return registered
}
