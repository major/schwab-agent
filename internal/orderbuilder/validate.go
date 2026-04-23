// Package orderbuilder validates and constructs Schwab order payloads.
package orderbuilder

import (
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

	switch params.OrderType {
	case models.OrderTypeLimit:
		if params.Price == 0 {
			return validationError("LIMIT order requires a price", "Add `--price <amount>` to specify the limit price")
		}
	case models.OrderTypeStop:
		if params.StopPrice == 0 {
			return validationError("STOP order requires a stop price", "Add `--stop-price <amount>` to specify the stop price")
		}
	case models.OrderTypeStopLimit:
		if params.Price == 0 || params.StopPrice == 0 {
			return validationError("STOP_LIMIT order requires both price and stop price", "Add `--price <amount> --stop-price <amount>`")
		}
	case models.OrderTypeTrailingStop:
		if params.StopPriceOffset <= 0 {
			return validationError("TRAILING_STOP order requires a stop price offset", "Add `--stop-offset <amount>` to specify how far the stop trails")
		}
	case models.OrderTypeTrailingStopLimit:
		if params.StopPriceOffset <= 0 {
			return validationError("TRAILING_STOP_LIMIT order requires a stop price offset", "Add `--stop-offset <amount>` to specify how far the stop trails")
		}

		if params.Price == 0 {
			return validationError("TRAILING_STOP_LIMIT order requires a limit price", "Add `--price <amount>` to specify the limit price")
		}
	case models.OrderTypeMarketOnClose:
		// MOC orders are like MARKET orders - no price or stop price allowed
		if params.Price != 0 {
			return validationError("MARKET_ON_CLOSE order does not accept a price", "Remove `--price` flag for MOC orders")
		}
		if params.StopPrice != 0 {
			return validationError("MARKET_ON_CLOSE order does not accept a stop price", "Remove `--stop-price` flag for MOC orders")
		}
	case models.OrderTypeLimitOnClose:
		// LOC orders are like LIMIT orders - price is required
		if params.Price == 0 {
			return validationError("LIMIT_ON_CLOSE order requires a price", "Add `--price <amount>` to specify the limit price")
		}
		if params.StopPrice != 0 {
			return validationError("LIMIT_ON_CLOSE order does not accept a stop price", "Remove `--stop-price` flag for LOC orders")
		}
	}

	if err := validatePriceLinkPair(params.PriceLinkBasis, params.PriceLinkType); err != nil {
		return err
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
		return validationError("option strike price must be greater than zero", "Add `--strike <price>` with a positive value")
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
		return validationError("OCO order requires at least one exit condition", "Add `--take-profit <amount>` and/or `--stop-loss <amount>`")
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
		return validationError("bracket order requires at least one exit condition", "Add `--take-profit <amount>` and/or `--stop-loss <amount>`")
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
			return validationError("take-profit price must be above the entry price for a BUY bracket", "Increase `--take-profit` so it is above the entry price")
		}

		if params.StopLoss > 0 && params.StopLoss >= params.Price {
			return validationError("stop-loss price must be below the entry price for a BUY bracket", "Lower `--stop-loss` so it is below the entry price")
		}

		return nil
	}

	if params.TakeProfit > 0 && params.TakeProfit >= params.Price {
		return validationError("take-profit price must be below the entry price for a SELL bracket", "Lower `--take-profit` so it is below the entry price")
	}

	if params.StopLoss > 0 && params.StopLoss <= params.Price {
		return validationError("stop-loss price must be above the entry price for a SELL bracket", "Raise `--stop-loss` so it is above the entry price")
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
		return validationError("long strike price must be greater than zero", "Add `--long-strike <price>` with a positive value")
	}

	if params.ShortStrike <= 0 {
		return validationError("short strike price must be greater than zero", "Add `--short-strike <price>` with a positive value")
	}

	if params.LongStrike == params.ShortStrike {
		return validationError("long and short strikes must be different", "Use different values for `--long-strike` and `--short-strike`")
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
			return validationError(s.name+" must be greater than zero", "Add `--"+s.name+" <price>` with a positive value")
		}
	}

	if params.PutLongStrike >= params.PutShortStrike {
		return validationError("put-long-strike must be below put-short-strike", "The protective put (bought) must be at a lower strike than the sold put")
	}

	if params.PutShortStrike >= params.CallShortStrike {
		return validationError("put-short-strike must be below call-short-strike", "The put and call short strikes define the profit zone and must not overlap")
	}

	if params.CallShortStrike >= params.CallLongStrike {
		return validationError("call-short-strike must be below call-long-strike", "The protective call (bought) must be at a higher strike than the sold call")
	}

	return validateSpreadPrice(params.Price, "iron condors")
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
		return validationError("call strike price must be greater than zero", "Add `--call-strike <price>` with a positive value")
	}

	if params.PutStrike <= 0 {
		return validationError("put strike price must be greater than zero", "Add `--put-strike <price>` with a positive value")
	}

	if params.CallStrike == params.PutStrike {
		return validationError("call and put strikes must be different (use straddle for same strike)", "Use different values for `--call-strike` and `--put-strike`")
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
		return validationError("call strike price must be greater than zero", "Add `--strike <price>` with a positive value")
	}

	// Covered calls are always a net debit, so the generic "debit or credit"
	// message from validateSpreadPrice would be misleading here.
	if params.Price <= 0 {
		return validationError("price is required for covered calls", "Add `--price <amount>` to specify the net debit")
	}

	return nil
}

// validateUnderlying checks that the underlying symbol is not empty.
func validateUnderlying(underlying string) error {
	if strings.TrimSpace(underlying) == "" {
		return validationError("underlying symbol is required", "Add `--underlying <TICKER>` to specify the underlying stock")
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
		return validationError("option expiration date is in the past", "Use a future expiration date with `--expiration YYYY-MM-DD`")
	}

	return nil
}

// validateSpreadPrice checks that the spread price is positive.
func validateSpreadPrice(price float64, strategy string) error {
	if price <= 0 {
		return validationError("price is required for "+strategy, "Add `--price <amount>` to specify the net debit or credit")
	}

	return nil
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
