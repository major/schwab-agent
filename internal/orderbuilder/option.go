package orderbuilder

import (
	"cmp"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/major/schwab-agent/internal/apperr"
	"github.com/major/schwab-agent/internal/models"
	"github.com/major/schwab-agent/internal/ptr"
)

const optionContractMultiplier = 100.0

// OptionParams holds parameters for building an option order.
type OptionParams struct {
	Underlying         string
	Expiration         time.Time
	Strike             float64
	PutCall            models.PutCall
	Action             models.Instruction
	Quantity           float64
	OrderType          models.OrderType
	Price              float64
	Duration           models.Duration
	Session            models.Session
	SpecialInstruction models.SpecialInstruction

	// Routing and price link fields (optional, pass-through to Schwab API).
	Destination    models.RequestedDestination
	PriceLinkBasis models.PriceLinkBasis
	PriceLinkType  models.PriceLinkType
}

// BuildOptionOrder constructs an OrderRequest for an option order.
func BuildOptionOrder(params *OptionParams) (*models.OrderRequest, error) {
	putCall := params.PutCall
	underlying := params.Underlying
	expiration := params.Expiration.Format("2006-01-02")
	strike := params.Strike
	multiplier := optionContractMultiplier

	order := &models.OrderRequest{
		Session:           cmp.Or(params.Session, models.SessionNormal),
		Duration:          cmp.Or(params.Duration, models.DurationDay),
		OrderType:         params.OrderType,
		OrderStrategyType: models.OrderStrategyTypeSingle,
		OrderLegCollection: []models.OrderLegCollection{
			{
				Instruction: params.Action,
				Quantity:    params.Quantity,
				Instrument: models.OrderInstrument{
					AssetType:            models.AssetTypeOption,
					Symbol:               BuildOCCSymbol(params.Underlying, params.Expiration, params.Strike, string(params.PutCall)),
					PutCall:              &putCall,
					UnderlyingSymbol:     &underlying,
					OptionMultiplier:     &multiplier,
					OptionExpirationDate: &expiration,
					OptionStrikePrice:    &strike,
				},
			},
		},
	}

	if params.OrderType == models.OrderTypeLimit {
		order.Price = ptr.To(params.Price)
	}

	if params.SpecialInstruction != "" {
		order.SpecialInstruction = ptr.To(params.SpecialInstruction)
	}

	if params.Destination != "" {
		order.RequestedDestination = ptr.To(params.Destination)
	}

	// Market-style orders have no price to link against, so reject price-link fields early.
	if (params.OrderType == models.OrderTypeMarket || params.OrderType == models.OrderTypeMarketOnClose) &&
		(params.PriceLinkBasis != "" || params.PriceLinkType != "") {
		return nil, apperr.NewValidationError(
			"price-link-basis and price-link-type are not allowed on market orders",
			nil,
		)
	}

	if params.PriceLinkBasis != "" {
		order.PriceLinkBasis = ptr.To(params.PriceLinkBasis)
	}

	if params.PriceLinkType != "" {
		order.PriceLinkType = ptr.To(params.PriceLinkType)
	}

	return order, nil
}

// BuildOCCSymbol constructs the OCC option symbol.
func BuildOCCSymbol(underlying string, expiration time.Time, strike float64, putCall string) string {
	paddedUnderlying := fmt.Sprintf("%-6s", underlying)
	expirationCode := expiration.Format("060102")
	putCallCode := optionPutCallCode(putCall)
	strikeCode := fmt.Sprintf("%08d", int64(math.Round(strike*1000)))

	return paddedUnderlying + expirationCode + putCallCode + strikeCode
}

// occSymbolLength is the fixed length of an OCC option symbol:
// 6 (underlying) + 6 (YYMMDD) + 1 (P/C) + 8 (strike*1000) = 21.
const occSymbolLength = 21

// OCCComponents holds the parsed components of an OCC option symbol.
type OCCComponents struct {
	Underlying string
	Expiration time.Time
	PutCall    string // "CALL" or "PUT"
	Strike     float64
	Symbol     string // the original OCC symbol
}

// ParseOCCSymbol decomposes an OCC option symbol string into its components.
// OCC format: {underlying:6}{YYMMDD:6}{P|C:1}{strike*1000:8} (21 characters).
func ParseOCCSymbol(symbol string) (*OCCComponents, error) {
	if len(symbol) != occSymbolLength {
		return nil, fmt.Errorf("OCC symbol must be %d characters, got %d", occSymbolLength, len(symbol))
	}

	underlying := strings.TrimRight(symbol[:6], " ")
	if underlying == "" {
		return nil, fmt.Errorf("OCC symbol has empty underlying")
	}

	dateStr := symbol[6:12]
	expiration, err := time.Parse("060102", dateStr)
	if err != nil {
		return nil, fmt.Errorf("invalid expiration date %q in OCC symbol: %w", dateStr, err)
	}

	putCallChar := symbol[12:13]
	var putCall string
	switch putCallChar {
	case "C":
		putCall = string(models.PutCallCall)
	case "P":
		putCall = string(models.PutCallPut)
	default:
		return nil, fmt.Errorf("invalid put/call indicator %q in OCC symbol, expected C or P", putCallChar)
	}

	strikeStr := symbol[13:21]
	strikeInt, err := strconv.ParseInt(strikeStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid strike price %q in OCC symbol: %w", strikeStr, err)
	}

	if strikeInt <= 0 {
		return nil, fmt.Errorf("strike price must be positive, got %d", strikeInt)
	}

	return &OCCComponents{
		Underlying: underlying,
		Expiration: expiration,
		PutCall:    putCall,
		Strike:     float64(strikeInt) / 1000.0,
		Symbol:     symbol,
	}, nil
}

// optionPutCallCode converts a full put/call label into the OCC single-character code.
func optionPutCallCode(putCall string) string {
	if strings.EqualFold(putCall, string(models.PutCallPut)) {
		return "P"
	}

	return "C"
}
