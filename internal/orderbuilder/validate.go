package orderbuilder

import (
	"math"
	"strings"
	"time"

	"github.com/major/schwab-agent/internal/apperr"
	"github.com/major/schwab-agent/internal/models"
)

// ValidateEquityOrder validates equity order parameters.
func ValidateEquityOrder(params *EquityParams) error {
	if strings.TrimSpace(params.Symbol) == "" {
		return validationError("symbol is required", "Add `--symbol <TICKER>` to specify the stock symbol")
	}

	if params.Quantity <= 0 {
		return validationError("quantity must be greater than zero", "Add `--quantity <number>` with a positive value")
	}

	if err := validateEquityOrderTypePrices(params); err != nil {
		return err
	}

	return validatePriceLinkPair(params.PriceLinkBasis, params.PriceLinkType)
}

func validateEquityOrderTypePrices(params *EquityParams) error {
	//nolint:exhaustive // Only order types with local equity-specific price rules need cases.
	switch params.OrderType {
	case models.OrderTypeLimit:
		return validateRequiredOrderPrice(params.Price, "LIMIT")
	case models.OrderTypeStop:
		return validateRequiredStopPrice(params.StopPrice, "STOP")
	case models.OrderTypeStopLimit:
		return validateStopLimitPrices(params)
	case models.OrderTypeTrailingStop:
		return validateRequiredStopOffset(params.StopPriceOffset, "TRAILING_STOP")
	case models.OrderTypeTrailingStopLimit:
		return validateTrailingStopLimitPrices(params)
	case models.OrderTypeMarketOnClose:
		return validateMarketOnClosePrices(params)
	case models.OrderTypeLimitOnClose:
		return validateLimitOnClosePrices(params)
	}

	return nil
}

func validateRequiredOrderPrice(price float64, orderType string) error {
	if price != 0 {
		return nil
	}

	return validationError(orderType+" order requires a price", "Add `--price <amount>` to specify the limit price")
}

func validateRequiredStopPrice(stopPrice float64, orderType string) error {
	if stopPrice != 0 {
		return nil
	}

	return validationError(
		orderType+" order requires a stop price",
		"Add `--stop-price <amount>` to specify the stop price",
	)
}

func validateRequiredStopOffset(stopPriceOffset float64, orderType string) error {
	if stopPriceOffset > 0 {
		return nil
	}

	return validationError(
		orderType+" order requires a stop price offset",
		"Add `--stop-offset <amount>` to specify how far the stop trails",
	)
}

func validateStopLimitPrices(params *EquityParams) error {
	if params.Price != 0 && params.StopPrice != 0 {
		return nil
	}

	return validationError(
		"STOP_LIMIT order requires both price and stop price",
		"Add `--price <amount> --stop-price <amount>`",
	)
}

func validateTrailingStopLimitPrices(params *EquityParams) error {
	if err := validateRequiredStopOffset(params.StopPriceOffset, "TRAILING_STOP_LIMIT"); err != nil {
		return err
	}

	if params.Price != 0 {
		return nil
	}

	return validationError(
		"TRAILING_STOP_LIMIT order requires a limit price",
		"Add `--price <amount>` to specify the limit price",
	)
}

func validateMarketOnClosePrices(params *EquityParams) error {
	// MOC orders are like MARKET orders - no price or stop price allowed.
	if params.Price != 0 {
		return validationError(
			"MARKET_ON_CLOSE order does not accept a price",
			"Remove `--price` flag for MOC orders",
		)
	}
	if params.StopPrice != 0 {
		return validationError(
			"MARKET_ON_CLOSE order does not accept a stop price",
			"Remove `--stop-price` flag for MOC orders",
		)
	}

	return nil
}

func validateLimitOnClosePrices(params *EquityParams) error {
	// LOC orders are like LIMIT orders - price is required.
	if err := validateRequiredOrderPrice(params.Price, "LIMIT_ON_CLOSE"); err != nil {
		return err
	}
	if params.StopPrice != 0 {
		return validationError(
			"LIMIT_ON_CLOSE order does not accept a stop price",
			"Remove `--stop-price` flag for LOC orders",
		)
	}

	return nil
}

// ValidateOptionOrder validates option order parameters.
func ValidateOptionOrder(params *OptionParams) error {
	// Trailing stop orders are only supported for equity orders.
	if params.OrderType == models.OrderTypeTrailingStop || params.OrderType == models.OrderTypeTrailingStopLimit {
		return validationError(
			"trailing stop orders are not supported for options",
			"Use `order build equity` for trailing stop orders",
		)
	}

	if err := validateUnderlying(params.Underlying); err != nil {
		return err
	}

	if err := validateQuantity(params.Quantity); err != nil {
		return err
	}

	if err := validateExpiration(params.Expiration); err != nil {
		return err
	}

	if params.Strike <= 0 {
		return validationError(
			"option strike price must be greater than zero",
			"Add `--strike <price>` with a positive value",
		)
	}

	if err := validatePriceLinkPair(params.PriceLinkBasis, params.PriceLinkType); err != nil {
		return err
	}

	return nil
}

// ValidateOCOOrder validates standalone OCO order parameters.
// OCO orders exit an existing position, so there is no entry order type or
// entry price to validate. The action determines whether prices make sense
// (e.g., a SELL OCO means take-profit > stop-loss).
func ValidateOCOOrder(params *OCOParams) error {
	if strings.TrimSpace(params.Symbol) == "" {
		return validationError("symbol is required", "Add `--symbol <TICKER>` to specify the stock symbol")
	}

	if params.Quantity <= 0 {
		return validationError("quantity must be greater than zero", "Add `--quantity <number>` with a positive value")
	}

	if params.TakeProfit == 0 && params.StopLoss == 0 {
		return validationError(
			"OCO order requires at least one exit condition",
			"Add `--take-profit <amount>` and/or `--stop-loss <amount>`",
		)
	}

	if err := validateOCOPriceRelationships(params); err != nil {
		return err
	}

	return nil
}

// validateOCOPriceRelationships checks that take-profit and stop-loss prices
// are logically ordered relative to each other. For a SELL exit (closing a long),
// take-profit should be above stop-loss. For a BUY exit (closing a short),
// take-profit should be below stop-loss.
func validateOCOPriceRelationships(params *OCOParams) error {
	if params.TakeProfit == 0 || params.StopLoss == 0 {
		return nil
	}

	// Selling to close a long: take-profit is the higher price, stop-loss is lower.
	if params.Action == models.InstructionSell {
		if params.TakeProfit <= params.StopLoss {
			return validationError(
				"take-profit must be above stop-loss for a SELL OCO",
				"The take-profit target should be higher than the stop-loss level",
			)
		}

		return nil
	}

	// Buying to close a short: take-profit is the lower price, stop-loss is higher.
	if params.TakeProfit >= params.StopLoss {
		return validationError(
			"take-profit must be below stop-loss for a BUY OCO",
			"The take-profit target should be lower than the stop-loss level",
		)
	}

	return nil
}

// ValidateBracketOrder validates bracket order parameters.
func ValidateBracketOrder(params *BracketParams) error {
	if err := ValidateEquityOrder(&EquityParams{
		Symbol:    params.Symbol,
		Action:    params.Action,
		Quantity:  params.Quantity,
		OrderType: params.OrderType,
		Price:     params.Price,
	}); err != nil {
		return err
	}

	if params.TakeProfit == 0 && params.StopLoss == 0 {
		return validationError(
			"bracket order requires at least one exit condition",
			"Add `--take-profit <amount>` and/or `--stop-loss <amount>`",
		)
	}

	if params.Price > 0 {
		if err := validateBracketPriceRelationships(params); err != nil {
			return err
		}
	}

	return nil
}

// ValidateFTSOrder validates first-triggers-second order parameters.
// Each sub-order must be non-empty (has an OrderType or legs) and must not
// already be a complex order (e.g., TRIGGER, OCO). The content of each
// sub-order is the caller's responsibility - this only checks structural rules.
func ValidateFTSOrder(params *FTSParams) error {
	if err := validateFTSLeg("primary", &params.Primary); err != nil {
		return err
	}

	if err := validateFTSLeg("secondary", &params.Secondary); err != nil {
		return err
	}

	return nil
}

// validateFTSLeg checks that a single FTS leg is non-empty and not already a
// complex order. An order is considered empty when it has no OrderType and no
// legs. A complex order has an OrderStrategyType other than "" or "SINGLE".
func validateFTSLeg(name string, order *models.OrderRequest) error {
	if order.OrderType == "" && len(order.OrderLegCollection) == 0 {
		return validationError(
			name+" order is empty",
			"Provide a valid order with an order type and at least one leg",
		)
	}

	if order.OrderStrategyType != "" && order.OrderStrategyType != models.OrderStrategyTypeSingle {
		return validationError(
			name+" order must be a simple order, not a "+string(order.OrderStrategyType)+" strategy",
			"FTS legs cannot be nested complex orders (TRIGGER, OCO, etc.)",
		)
	}

	return nil
}

// validateBracketPriceRelationships checks take-profit and stop-loss levels against the entry price.
// Only validates relationships for exit conditions that are present (either or both).
func validateBracketPriceRelationships(params *BracketParams) error {
	if params.Action == models.InstructionBuy {
		if params.TakeProfit > 0 && params.TakeProfit <= params.Price {
			return validationError(
				"take-profit price must be above the entry price for a BUY bracket",
				"Increase `--take-profit` so it is above the entry price",
			)
		}

		if params.StopLoss > 0 && params.StopLoss >= params.Price {
			return validationError(
				"stop-loss price must be below the entry price for a BUY bracket",
				"Lower `--stop-loss` so it is below the entry price",
			)
		}

		return nil
	}

	if params.TakeProfit > 0 && params.TakeProfit >= params.Price {
		return validationError(
			"take-profit price must be below the entry price for a SELL bracket",
			"Lower `--take-profit` so it is below the entry price",
		)
	}

	if params.StopLoss > 0 && params.StopLoss <= params.Price {
		return validationError(
			"stop-loss price must be above the entry price for a SELL bracket",
			"Raise `--stop-loss` so it is above the entry price",
		)
	}

	return nil
}

// ValidateVerticalOrder validates vertical spread parameters.
func ValidateVerticalOrder(params *VerticalParams) error {
	if err := validateUnderlying(params.Underlying); err != nil {
		return err
	}

	if err := validateQuantity(params.Quantity); err != nil {
		return err
	}

	if err := validateExpiration(params.Expiration); err != nil {
		return err
	}

	if params.LongStrike <= 0 {
		return validationError(
			"long strike price must be greater than zero",
			"Add `--long-strike <price>` with a positive value",
		)
	}

	if params.ShortStrike <= 0 {
		return validationError(
			"short strike price must be greater than zero",
			"Add `--short-strike <price>` with a positive value",
		)
	}

	if params.LongStrike == params.ShortStrike {
		return validationError(
			"long and short strikes must be different",
			"Use different values for `--long-strike` and `--short-strike`",
		)
	}

	return validateSpreadPrice(params.Price, "vertical spreads")
}

// ValidateIronCondorOrder validates iron condor parameters.
// Strikes must be ordered: put-long < put-short < call-short < call-long.
func ValidateIronCondorOrder(params *IronCondorParams) error {
	if err := validateUnderlying(params.Underlying); err != nil {
		return err
	}

	if err := validateQuantity(params.Quantity); err != nil {
		return err
	}

	if err := validateExpiration(params.Expiration); err != nil {
		return err
	}

	strikes := []struct {
		value float64
		name  string
	}{
		{params.PutLongStrike, "put-long-strike"},
		{params.PutShortStrike, "put-short-strike"},
		{params.CallShortStrike, "call-short-strike"},
		{params.CallLongStrike, "call-long-strike"},
	}

	for _, s := range strikes {
		if s.value <= 0 {
			return validationError(
				s.name+" must be greater than zero",
				"Add `--"+s.name+" <price>` with a positive value",
			)
		}
	}

	if params.PutLongStrike >= params.PutShortStrike {
		return validationError(
			"put-long-strike must be below put-short-strike",
			"The protective put (bought) must be at a lower strike than the sold put",
		)
	}

	if params.PutShortStrike >= params.CallShortStrike {
		return validationError(
			"put-short-strike must be below call-short-strike",
			"The put and call short strikes define the profit zone and must not overlap",
		)
	}

	if params.CallShortStrike >= params.CallLongStrike {
		return validationError(
			"call-short-strike must be below call-long-strike",
			"The protective call (bought) must be at a higher strike than the sold call",
		)
	}

	return validateSpreadPrice(params.Price, "iron condors")
}

// ValidateButterflyOrder validates butterfly spread parameters.
func ValidateButterflyOrder(params *ButterflyParams) error {
	if err := validateUnderlying(params.Underlying); err != nil {
		return err
	}

	if err := validateQuantity(params.Quantity); err != nil {
		return err
	}

	if err := validateExpiration(params.Expiration); err != nil {
		return err
	}

	if err := validatePositiveStrikes([]namedStrike{
		{params.LowerStrike, "lower-strike"},
		{params.MiddleStrike, "middle-strike"},
		{params.UpperStrike, "upper-strike"},
	}); err != nil {
		return err
	}

	if params.LowerStrike >= params.MiddleStrike || params.MiddleStrike >= params.UpperStrike {
		return validationError(
			"butterfly strikes must be ordered lower < middle < upper",
			"Use three increasing strikes with `--lower-strike`, `--middle-strike`, and `--upper-strike`",
		)
	}

	if !sameStrikeWidth(params.MiddleStrike-params.LowerStrike, params.UpperStrike-params.MiddleStrike) {
		return validationError(
			"butterfly wings must be balanced",
			"Use equal distances from `--middle-strike` to `--lower-strike` and `--upper-strike`; unbalanced butterflies are a separate strategy type",
		)
	}

	if params.PutCall == "" {
		return validationError("option type (call or put) is required", "Add `--call` or `--put`")
	}

	return validateSpreadPrice(params.Price, "butterflies")
}

// ValidateCondorOrder validates condor spread parameters.
func ValidateCondorOrder(params *CondorParams) error {
	if err := validateUnderlying(params.Underlying); err != nil {
		return err
	}

	if err := validateQuantity(params.Quantity); err != nil {
		return err
	}

	if err := validateExpiration(params.Expiration); err != nil {
		return err
	}

	if err := validatePositiveStrikes([]namedStrike{
		{params.LowerStrike, "lower-strike"},
		{params.LowerMiddleStrike, "lower-middle-strike"},
		{params.UpperMiddleStrike, "upper-middle-strike"},
		{params.UpperStrike, "upper-strike"},
	}); err != nil {
		return err
	}

	if params.LowerStrike >= params.LowerMiddleStrike || params.LowerMiddleStrike >= params.UpperMiddleStrike ||
		params.UpperMiddleStrike >= params.UpperStrike {
		return validationError(
			"condor strikes must be ordered lower < lower-middle < upper-middle < upper",
			"Use four increasing strikes for the condor wings and middle strikes",
		)
	}

	if !sameStrikeWidth(params.LowerMiddleStrike-params.LowerStrike, params.UpperStrike-params.UpperMiddleStrike) {
		return validationError(
			"condor wings must be balanced",
			"Use equal wing widths between lower/lower-middle and upper-middle/upper; unbalanced condors are a separate strategy type",
		)
	}

	if params.PutCall == "" {
		return validationError("option type (call or put) is required", "Add `--call` or `--put`")
	}

	return validateSpreadPrice(params.Price, "condors")
}

// ValidateBackRatioOrder validates back-ratio spread parameters.
func ValidateBackRatioOrder(params *BackRatioParams) error {
	if err := validateUnderlying(params.Underlying); err != nil {
		return err
	}

	if err := validateQuantity(params.Quantity); err != nil {
		return err
	}

	if err := validateExpiration(params.Expiration); err != nil {
		return err
	}

	if err := validatePositiveStrikes([]namedStrike{
		{params.ShortStrike, "short-strike"},
		{params.LongStrike, "long-strike"},
	}); err != nil {
		return err
	}

	if params.ShortStrike == params.LongStrike {
		return validationError(
			"short and long strikes must be different",
			"Use different values for `--short-strike` and `--long-strike`",
		)
	}

	if params.PutCall == models.PutCallCall && params.LongStrike <= params.ShortStrike {
		return validationError(
			"call back-ratio long strike must be above short strike",
			"For call back-ratios, sell the lower strike and buy more contracts at the higher strike",
		)
	}

	if params.PutCall == models.PutCallPut && params.LongStrike >= params.ShortStrike {
		return validationError(
			"put back-ratio long strike must be below short strike",
			"For put back-ratios, sell the higher strike and buy more contracts at the lower strike",
		)
	}

	if params.LongRatio <= 1 {
		return validationError(
			"long ratio must be greater than one",
			"Use `--long-ratio 2` for the standard one-by-two back-ratio",
		)
	}

	if math.Trunc(params.LongRatio) != params.LongRatio {
		return validationError("long ratio must be a whole number", "Use an integer value like `--long-ratio 2`")
	}

	if params.PutCall == "" {
		return validationError("option type (call or put) is required", "Add `--call` or `--put`")
	}

	return validateSpreadPrice(params.Price, "back-ratios")
}

// ValidateVerticalRollOrder validates vertical roll parameters.
func ValidateVerticalRollOrder(params *VerticalRollParams) error {
	if err := validateUnderlying(params.Underlying); err != nil {
		return err
	}

	if err := validateQuantity(params.Quantity); err != nil {
		return err
	}

	if err := validateExpiration(params.CloseExpiration); err != nil {
		return err
	}

	if err := validateExpiration(params.OpenExpiration); err != nil {
		return err
	}

	if params.OpenExpiration.Before(params.CloseExpiration) {
		return validationError(
			"open expiration must not be before close expiration",
			"Use `--open-expiration` on or after `--close-expiration`",
		)
	}

	if err := validatePositiveStrikes([]namedStrike{
		{params.CloseLongStrike, "close-long-strike"},
		{params.CloseShortStrike, "close-short-strike"},
		{params.OpenLongStrike, "open-long-strike"},
		{params.OpenShortStrike, "open-short-strike"},
	}); err != nil {
		return err
	}

	if params.CloseLongStrike == params.CloseShortStrike {
		return validationError(
			"close long and short strikes must be different",
			"Use different values for `--close-long-strike` and `--close-short-strike`",
		)
	}

	if params.OpenLongStrike == params.OpenShortStrike {
		return validationError(
			"open long and short strikes must be different",
			"Use different values for `--open-long-strike` and `--open-short-strike`",
		)
	}

	if !sameStrikeWidth(
		spreadWidth(params.CloseLongStrike, params.CloseShortStrike),
		spreadWidth(params.OpenLongStrike, params.OpenShortStrike),
	) {
		return validationError(
			"vertical roll widths must match",
			"Use equal close and open spread widths for `VERTICAL_ROLL`; unequal widths are a separate unbalanced strategy type",
		)
	}

	if params.CloseExpiration.Equal(params.OpenExpiration) && params.CloseLongStrike == params.OpenLongStrike &&
		params.CloseShortStrike == params.OpenShortStrike {
		return validationError(
			"vertical roll must change strikes or expiration",
			"Use different open strikes or a later `--open-expiration` so the roll changes the position",
		)
	}

	if params.PutCall == "" {
		return validationError("option type (call or put) is required", "Add `--call` or `--put`")
	}

	return validateSpreadPrice(params.Price, "vertical rolls")
}

// ValidateDoubleDiagonalOrder validates double diagonal spread parameters.
func ValidateDoubleDiagonalOrder(params *DoubleDiagonalParams) error {
	if err := validateUnderlying(params.Underlying); err != nil {
		return err
	}

	if err := validateQuantity(params.Quantity); err != nil {
		return err
	}

	if err := validateExpiration(params.NearExpiration); err != nil {
		return err
	}

	if err := validateExpiration(params.FarExpiration); err != nil {
		return err
	}

	if params.NearExpiration.Equal(params.FarExpiration) || params.NearExpiration.After(params.FarExpiration) {
		return validationError(
			"near expiration must be before far expiration",
			"Use `--near-expiration` before `--far-expiration`",
		)
	}

	if err := validatePositiveStrikes([]namedStrike{
		{params.PutFarStrike, "put-far-strike"},
		{params.PutNearStrike, "put-near-strike"},
		{params.CallNearStrike, "call-near-strike"},
		{params.CallFarStrike, "call-far-strike"},
	}); err != nil {
		return err
	}

	if params.PutFarStrike >= params.PutNearStrike || params.PutNearStrike >= params.CallNearStrike ||
		params.CallNearStrike >= params.CallFarStrike {
		return validationError(
			"double diagonal strikes must be ordered put-far < put-near < call-near < call-far",
			"Keep put strikes below call strikes so the near short legs do not overlap",
		)
	}

	return validateSpreadPrice(params.Price, "double diagonals")
}

// ValidateStrangleOrder validates strangle parameters.
func ValidateStrangleOrder(params *StrangleParams) error {
	if err := validateUnderlying(params.Underlying); err != nil {
		return err
	}

	if err := validateQuantity(params.Quantity); err != nil {
		return err
	}

	if err := validateExpiration(params.Expiration); err != nil {
		return err
	}

	if params.CallStrike <= 0 {
		return validationError(
			"call strike price must be greater than zero",
			"Add `--call-strike <price>` with a positive value",
		)
	}

	if params.PutStrike <= 0 {
		return validationError(
			"put strike price must be greater than zero",
			"Add `--put-strike <price>` with a positive value",
		)
	}

	if params.CallStrike == params.PutStrike {
		return validationError(
			"call and put strikes must be different (use straddle for same strike)",
			"Use different values for `--call-strike` and `--put-strike`",
		)
	}

	return validateSpreadPrice(params.Price, "strangles")
}

// ValidateStraddleOrder validates straddle parameters.
func ValidateStraddleOrder(params *StraddleParams) error {
	if err := validateUnderlying(params.Underlying); err != nil {
		return err
	}

	if err := validateQuantity(params.Quantity); err != nil {
		return err
	}

	if err := validateExpiration(params.Expiration); err != nil {
		return err
	}

	if params.Strike <= 0 {
		return validationError("strike price must be greater than zero", "Add `--strike <price>` with a positive value")
	}

	return validateSpreadPrice(params.Price, "straddles")
}

// ValidateCalendarOrder validates calendar spread parameters.
// Calendar spreads require different expirations and the near expiration must
// be before the far expiration. Both legs share the same strike.
func ValidateCalendarOrder(params *CalendarParams) error {
	if err := validateUnderlying(params.Underlying); err != nil {
		return err
	}

	if err := validateQuantity(params.Quantity); err != nil {
		return err
	}

	if err := validateExpiration(params.NearExpiration); err != nil {
		return err
	}

	if err := validateExpiration(params.FarExpiration); err != nil {
		return err
	}

	if params.NearExpiration.Equal(params.FarExpiration) {
		return validationError(
			"near and far expirations must be different for calendar spreads",
			"Use different dates for `--near-expiration` and `--far-expiration`",
		)
	}

	if params.NearExpiration.After(params.FarExpiration) {
		return validationError(
			"near expiration must be before far expiration",
			"Swap `--near-expiration` and `--far-expiration` so the near date comes first",
		)
	}

	if params.Strike <= 0 {
		return validationError("strike price must be greater than zero", "Add `--strike <price>` with a positive value")
	}

	if params.PutCall == "" {
		return validationError("option type (call or put) is required", "Add `--call` or `--put`")
	}

	return validateSpreadPrice(params.Price, "calendar spreads")
}

// ValidateDiagonalOrder validates diagonal spread parameters.
// Diagonal spreads require different expirations AND different strikes.
func ValidateDiagonalOrder(params *DiagonalParams) error {
	if err := validateUnderlying(params.Underlying); err != nil {
		return err
	}

	if err := validateQuantity(params.Quantity); err != nil {
		return err
	}

	if err := validateExpiration(params.NearExpiration); err != nil {
		return err
	}

	if err := validateExpiration(params.FarExpiration); err != nil {
		return err
	}

	if params.NearExpiration.Equal(params.FarExpiration) {
		return validationError(
			"near and far expirations must be different for diagonal spreads",
			"Use different dates for `--near-expiration` and `--far-expiration`",
		)
	}

	if params.NearExpiration.After(params.FarExpiration) {
		return validationError(
			"near expiration must be before far expiration",
			"Swap `--near-expiration` and `--far-expiration` so the near date comes first",
		)
	}

	if params.NearStrike <= 0 {
		return validationError(
			"near strike price must be greater than zero",
			"Add `--near-strike <price>` with a positive value",
		)
	}

	if params.FarStrike <= 0 {
		return validationError(
			"far strike price must be greater than zero",
			"Add `--far-strike <price>` with a positive value",
		)
	}

	if params.NearStrike == params.FarStrike {
		return validationError(
			"near and far strikes must be different for diagonal spreads",
			"Use different values for `--near-strike` and `--far-strike` (use calendar for same strike)",
		)
	}

	if params.PutCall == "" {
		return validationError("option type (call or put) is required", "Add `--call` or `--put`")
	}

	return validateSpreadPrice(params.Price, "diagonal spreads")
}

// ValidateCoveredCallOrder validates covered call parameters.
func ValidateCoveredCallOrder(params *CoveredCallParams) error {
	if err := validateUnderlying(params.Underlying); err != nil {
		return err
	}

	if err := validateQuantity(params.Quantity); err != nil {
		return err
	}

	if err := validateExpiration(params.Expiration); err != nil {
		return err
	}

	if params.Strike <= 0 {
		return validationError(
			"call strike price must be greater than zero",
			"Add `--strike <price>` with a positive value",
		)
	}

	// Covered calls are always a net debit, so the generic "debit or credit"
	// message from validateSpreadPrice would be misleading here.
	if params.Price <= 0 {
		return validationError("price is required for covered calls", "Add `--price <amount>` to specify the net debit")
	}

	return nil
}

// ValidateCollarOrder validates collar-with-stock parameters.
func ValidateCollarOrder(params *CollarParams) error {
	if err := validateUnderlying(params.Underlying); err != nil {
		return err
	}

	if err := validateQuantity(params.Quantity); err != nil {
		return err
	}

	if err := validateExpiration(params.Expiration); err != nil {
		return err
	}

	if params.PutStrike <= 0 {
		return validationError(
			"put strike price must be greater than zero",
			"Add `--put-strike <price>` with a positive value",
		)
	}

	if params.CallStrike <= 0 {
		return validationError(
			"call strike price must be greater than zero",
			"Add `--call-strike <price>` with a positive value",
		)
	}

	if params.PutStrike >= params.CallStrike {
		return validationError(
			"put strike must be below call strike for a collar",
			"Use `--put-strike` below `--call-strike` (protective put sits below the covered call)",
		)
	}

	return validateSpreadPrice(params.Price, "collars")
}

// ValidateJadeLizardOrder validates jade lizard parameters.
//
// Strikes must be ordered: put < short-call < long-call. The put provides
// downside premium, the short call provides upside premium, and the long call
// caps the maximum loss on the call side.
func ValidateJadeLizardOrder(params *JadeLizardParams) error {
	if err := validateUnderlying(params.Underlying); err != nil {
		return err
	}

	if err := validateQuantity(params.Quantity); err != nil {
		return err
	}

	if err := validateExpiration(params.Expiration); err != nil {
		return err
	}

	if err := validatePositiveStrikes([]namedStrike{
		{value: params.PutStrike, name: "put-strike"},
		{value: params.ShortCallStrike, name: "short-call-strike"},
		{value: params.LongCallStrike, name: "long-call-strike"},
	}); err != nil {
		return err
	}

	// Strikes must be ordered: put < short-call < long-call.
	if params.PutStrike >= params.ShortCallStrike {
		return validationError(
			"put strike must be below short call strike",
			"Adjust `--put-strike` to be lower than `--short-call-strike`",
		)
	}

	if params.ShortCallStrike >= params.LongCallStrike {
		return validationError(
			"short call strike must be below long call strike",
			"Adjust `--short-call-strike` to be lower than `--long-call-strike`",
		)
	}

	return validateSpreadPrice(params.Price, "jade lizards")
}

// validateUnderlying checks that the underlying symbol is not empty.
func validateUnderlying(underlying string) error {
	if strings.TrimSpace(underlying) == "" {
		return validationError(
			"underlying symbol is required",
			"Add `--underlying <TICKER>` to specify the underlying stock",
		)
	}

	return nil
}

// validateQuantity checks that quantity is positive.
func validateQuantity(quantity float64) error {
	if quantity <= 0 {
		return validationError("quantity must be greater than zero", "Add `--quantity <number>` with a positive value")
	}

	return nil
}

// validateExpiration checks that the expiration date is in the future.
func validateExpiration(expiration time.Time) error {
	if expiration.IsZero() || !expiration.After(time.Now().UTC()) {
		return validationError(
			"option expiration date is in the past",
			"Use a future expiration date with `--expiration YYYY-MM-DD`",
		)
	}

	return nil
}

// validateSpreadPrice checks that the spread price is positive.
func validateSpreadPrice(price float64, strategy string) error {
	if price <= 0 {
		return validationError(
			"price is required for "+strategy,
			"Add `--price <amount>` to specify the net debit or credit",
		)
	}

	return nil
}

// namedStrike pairs a strike value with its CLI flag name for validation errors.
type namedStrike struct {
	value float64
	name  string
}

// validatePositiveStrikes checks a set of strike flags for positive values.
func validatePositiveStrikes(strikes []namedStrike) error {
	for _, strike := range strikes {
		if strike.value <= 0 {
			return validationError(
				strike.name+" must be greater than zero",
				"Add `--"+strike.name+" <price>` with a positive value",
			)
		}
	}

	return nil
}

// spreadWidth returns the absolute distance between two strikes. Vertical spreads
// can be entered with the long leg above or below the short leg, so validation
// compares widths rather than relying on strike ordering.
func spreadWidth(firstStrike, secondStrike float64) float64 {
	return math.Abs(firstStrike - secondStrike)
}

// sameStrikeWidth compares option strike distances with a small tolerance. Strike
// flags are decimal values, and direct float64 equality can reject valid spreads
// when equivalent decimal widths have slightly different binary representations.
func sameStrikeWidth(left, right float64) bool {
	const strikeWidthTolerance = 1e-9

	return math.Abs(left-right) <= strikeWidthTolerance
}

// validatePriceLinkPair checks that price-link-basis and price-link-type are
// either both set or both unset. Setting one without the other is an error.
func validatePriceLinkPair(basis models.PriceLinkBasis, linkType models.PriceLinkType) error {
	basisSet := basis != ""
	typeSet := linkType != ""

	if basisSet != typeSet {
		return validationError(
			"price-link-basis and price-link-type must be specified together",
			"Add both `--price-link-basis <value>` and `--price-link-type <value>`, or omit both",
		)
	}

	return nil
}

// validationError creates a ValidationError with a fix-suggesting details message.
func validationError(message, details string) error {
	return apperr.NewValidationError(message, nil, apperr.WithDetails(details))
}
