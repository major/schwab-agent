package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/major/schwab-agent/internal/apperr"
	"github.com/major/schwab-agent/internal/auth"
	"github.com/major/schwab-agent/internal/models"
	"github.com/major/schwab-agent/internal/orderbuilder"
)

const mutableDisabledMessage = `Mutable operations are disabled by default. ` +
	`Set "i-also-like-to-live-dangerously": true in your config file to enable order placement, cancellation, and replacement.`

// equityPlaceOpts holds flags shared by equity place, build, and replace flows.
type equityPlaceOpts struct {
	Symbol             string                      `flag:"symbol" flagdescr:"Equity symbol" flagrequired:"true" flaggroup:"order"`
	Action             models.Instruction          `flag:"action" flagdescr:"Order action" flagrequired:"true" flaggroup:"order"`
	Quantity           float64                     `flag:"quantity" flagdescr:"Share quantity" flagrequired:"true" flaggroup:"execution"`
	Type               models.OrderType            `flag:"type" flagdescr:"Order type" flaggroup:"order"`
	Price              float64                     `flag:"price" flagdescr:"Limit price" flaggroup:"pricing"`
	StopPrice          float64                     `flag:"stop-price" flagdescr:"Stop price" flaggroup:"pricing"`
	StopOffset         float64                     `flag:"stop-offset" flagdescr:"Trailing stop offset amount" flaggroup:"pricing"`
	StopLinkBasis      models.StopPriceLinkBasis   `flag:"stop-link-basis" flagdescr:"Trailing stop reference price (LAST, BID, ASK, MARK)" flaggroup:"pricing"`
	StopLinkType       models.StopPriceLinkType    `flag:"stop-link-type" flagdescr:"Trailing stop offset type (VALUE, PERCENT, TICK)" flaggroup:"pricing"`
	StopType           models.StopType             `flag:"stop-type" flagdescr:"Trailing stop trigger type (STANDARD, BID, ASK, LAST, MARK)" flaggroup:"pricing"`
	ActivationPrice    float64                     `flag:"activation-price" flagdescr:"Price that activates the trailing stop" flaggroup:"pricing"`
	Duration           models.Duration             `flag:"duration" flagdescr:"Order duration" flaggroup:"order"`
	Session            models.Session              `flag:"session" flagdescr:"Trading session" flaggroup:"order"`
	SpecialInstruction models.SpecialInstruction   `flag:"special-instruction" flagdescr:"Special instruction (ALL_OR_NONE, DO_NOT_REDUCE, ALL_OR_NONE_DO_NOT_REDUCE)" flaggroup:"execution"`
	Destination        models.RequestedDestination `flag:"destination" flagdescr:"Order routing destination (INET, ECN_ARCA, CBOE, AMEX, PHLX, ISE, BOX, NYSE, NASDAQ, BATS, C2, AUTO)" flaggroup:"execution"`
	PriceLinkBasis     models.PriceLinkBasis       `flag:"price-link-basis" flagdescr:"Price link reference price (MANUAL, BASE, TRIGGER, LAST, BID, ASK, ASK_BID, MARK, AVERAGE)" flaggroup:"pricing"`
	PriceLinkType      models.PriceLinkType        `flag:"price-link-type" flagdescr:"Price link offset type (VALUE, PERCENT, TICK)" flaggroup:"pricing"`
}

// optionPlaceOpts holds flags shared by option place and build flows.
type optionPlaceOpts struct {
	Underlying         string                      `flag:"underlying" flagdescr:"Underlying symbol" flagrequired:"true" flaggroup:"contract"`
	Expiration         string                      `flag:"expiration" flagdescr:"Expiration date (YYYY-MM-DD)" flagrequired:"true" flaggroup:"contract"`
	Strike             float64                     `flag:"strike" flagdescr:"Strike price" flagrequired:"true" flaggroup:"contract"`
	Call               bool                        `flag:"call" flagdescr:"Call option" flaggroup:"contract"`
	Put                bool                        `flag:"put" flagdescr:"Put option" flaggroup:"contract"`
	Action             models.Instruction          `flag:"action" flagdescr:"Order action" flagrequired:"true" flaggroup:"order"`
	Quantity           float64                     `flag:"quantity" flagdescr:"Contract quantity" flagrequired:"true" flaggroup:"execution"`
	Type               models.OrderType            `flag:"type" flagdescr:"Order type" flaggroup:"order"`
	Price              float64                     `flag:"price" flagdescr:"Limit price" flaggroup:"pricing"`
	Duration           models.Duration             `flag:"duration" flagdescr:"Order duration" flaggroup:"order"`
	Session            models.Session              `flag:"session" flagdescr:"Trading session" flaggroup:"order"`
	SpecialInstruction models.SpecialInstruction   `flag:"special-instruction" flagdescr:"Special instruction (ALL_OR_NONE, DO_NOT_REDUCE, ALL_OR_NONE_DO_NOT_REDUCE)" flaggroup:"execution"`
	Destination        models.RequestedDestination `flag:"destination" flagdescr:"Order routing destination (INET, ECN_ARCA, CBOE, AMEX, PHLX, ISE, BOX, NYSE, NASDAQ, BATS, C2, AUTO)" flaggroup:"execution"`
	PriceLinkBasis     models.PriceLinkBasis       `flag:"price-link-basis" flagdescr:"Price link reference price (MANUAL, BASE, TRIGGER, LAST, BID, ASK, ASK_BID, MARK, AVERAGE)" flaggroup:"pricing"`
	PriceLinkType      models.PriceLinkType        `flag:"price-link-type" flagdescr:"Price link offset type (VALUE, PERCENT, TICK)" flaggroup:"pricing"`
}

// bracketPlaceOpts holds flags shared by bracket place and build flows.
type bracketPlaceOpts struct {
	Symbol     string             `flag:"symbol" flagdescr:"Equity symbol" flagrequired:"true" flaggroup:"order"`
	Action     models.Instruction `flag:"action" flagdescr:"Order action" flagrequired:"true" flaggroup:"order"`
	Quantity   float64            `flag:"quantity" flagdescr:"Share quantity" flagrequired:"true" flaggroup:"execution"`
	Type       models.OrderType   `flag:"type" flagdescr:"Entry order type" flaggroup:"order"`
	Price      float64            `flag:"price" flagdescr:"Entry price" flaggroup:"pricing"`
	TakeProfit float64            `flag:"take-profit" flagdescr:"Take-profit exit price" flaggroup:"pricing"`
	StopLoss   float64            `flag:"stop-loss" flagdescr:"Stop-loss exit price" flaggroup:"pricing"`
	Duration   models.Duration    `flag:"duration" flagdescr:"Order duration" flaggroup:"order"`
	Session    models.Session     `flag:"session" flagdescr:"Trading session" flaggroup:"order"`
}

// ocoPlaceOpts holds flags shared by OCO place and build flows.
type ocoPlaceOpts struct {
	Symbol     string             `flag:"symbol" flagdescr:"Equity symbol" flagrequired:"true" flaggroup:"order"`
	Action     models.Instruction `flag:"action" flagdescr:"Exit action (SELL to close long, BUY to close short)" flagrequired:"true" flaggroup:"order"`
	Quantity   float64            `flag:"quantity" flagdescr:"Share quantity" flagrequired:"true" flaggroup:"execution"`
	TakeProfit float64            `flag:"take-profit" flagdescr:"Take-profit exit price (limit order)" flaggroup:"pricing"`
	StopLoss   float64            `flag:"stop-loss" flagdescr:"Stop-loss exit price (stop order)" flaggroup:"pricing"`
	Duration   models.Duration    `flag:"duration" flagdescr:"Order duration" flaggroup:"order"`
	Session    models.Session     `flag:"session" flagdescr:"Trading session" flaggroup:"order"`
}

// verticalBuildOpts holds flags for vertical spread build flows.
type verticalBuildOpts struct {
	Underlying  string          `flag:"underlying" flagdescr:"Underlying symbol" flagrequired:"true" flaggroup:"contract"`
	Expiration  string          `flag:"expiration" flagdescr:"Expiration date (YYYY-MM-DD)" flagrequired:"true" flaggroup:"contract"`
	LongStrike  float64         `flag:"long-strike" flagdescr:"Strike price of the option being bought" flagrequired:"true" flaggroup:"contract"`
	ShortStrike float64         `flag:"short-strike" flagdescr:"Strike price of the option being sold" flagrequired:"true" flaggroup:"contract"`
	Call        bool            `flag:"call" flagdescr:"Call spread" flaggroup:"contract"`
	Put         bool            `flag:"put" flagdescr:"Put spread" flaggroup:"contract"`
	Open        bool            `flag:"open" flagdescr:"Opening position" flaggroup:"execution"`
	Close       bool            `flag:"close" flagdescr:"Closing position" flaggroup:"execution"`
	Quantity    float64         `flag:"quantity" flagdescr:"Number of contracts" flagrequired:"true" flaggroup:"execution"`
	Price       float64         `flag:"price" flagdescr:"Net debit or credit amount" flagrequired:"true" flaggroup:"pricing"`
	Duration    models.Duration `flag:"duration" flagdescr:"Order duration" flaggroup:"order"`
	Session     models.Session  `flag:"session" flagdescr:"Trading session" flaggroup:"order"`
}

// ironCondorBuildOpts holds flags for iron condor build flows.
type ironCondorBuildOpts struct {
	Underlying      string          `flag:"underlying" flagdescr:"Underlying symbol" flagrequired:"true" flaggroup:"contract"`
	Expiration      string          `flag:"expiration" flagdescr:"Expiration date (YYYY-MM-DD)" flagrequired:"true" flaggroup:"contract"`
	PutLongStrike   float64         `flag:"put-long-strike" flagdescr:"Lowest strike: put being bought (protection)" flagrequired:"true" flaggroup:"contract"`
	PutShortStrike  float64         `flag:"put-short-strike" flagdescr:"Put being sold (premium)" flagrequired:"true" flaggroup:"contract"`
	CallShortStrike float64         `flag:"call-short-strike" flagdescr:"Call being sold (premium)" flagrequired:"true" flaggroup:"contract"`
	CallLongStrike  float64         `flag:"call-long-strike" flagdescr:"Highest strike: call being bought (protection)" flagrequired:"true" flaggroup:"contract"`
	Open            bool            `flag:"open" flagdescr:"Opening position" flaggroup:"execution"`
	Close           bool            `flag:"close" flagdescr:"Closing position" flaggroup:"execution"`
	Quantity        float64         `flag:"quantity" flagdescr:"Number of contracts" flagrequired:"true" flaggroup:"execution"`
	Price           float64         `flag:"price" flagdescr:"Net credit or debit amount" flagrequired:"true" flaggroup:"pricing"`
	Duration        models.Duration `flag:"duration" flagdescr:"Order duration" flaggroup:"order"`
	Session         models.Session  `flag:"session" flagdescr:"Trading session" flaggroup:"order"`
}

// strangleBuildOpts holds flags for strangle build flows.
type strangleBuildOpts struct {
	Underlying string          `flag:"underlying" flagdescr:"Underlying symbol" flagrequired:"true" flaggroup:"contract"`
	Expiration string          `flag:"expiration" flagdescr:"Expiration date (YYYY-MM-DD)" flagrequired:"true" flaggroup:"contract"`
	CallStrike float64         `flag:"call-strike" flagdescr:"Strike price for the call leg" flagrequired:"true" flaggroup:"contract"`
	PutStrike  float64         `flag:"put-strike" flagdescr:"Strike price for the put leg" flagrequired:"true" flaggroup:"contract"`
	Buy        bool            `flag:"buy" flagdescr:"Buy the strangle (long, net debit)" flaggroup:"execution"`
	Sell       bool            `flag:"sell" flagdescr:"Sell the strangle (short, net credit)" flaggroup:"execution"`
	Open       bool            `flag:"open" flagdescr:"Opening position" flaggroup:"execution"`
	Close      bool            `flag:"close" flagdescr:"Closing position" flaggroup:"execution"`
	Quantity   float64         `flag:"quantity" flagdescr:"Number of contracts" flagrequired:"true" flaggroup:"execution"`
	Price      float64         `flag:"price" flagdescr:"Net debit or credit amount" flagrequired:"true" flaggroup:"pricing"`
	Duration   models.Duration `flag:"duration" flagdescr:"Order duration" flaggroup:"order"`
	Session    models.Session  `flag:"session" flagdescr:"Trading session" flaggroup:"order"`
}

// straddleBuildOpts holds flags for straddle build flows.
type straddleBuildOpts struct {
	Underlying string          `flag:"underlying" flagdescr:"Underlying symbol" flagrequired:"true" flaggroup:"contract"`
	Expiration string          `flag:"expiration" flagdescr:"Expiration date (YYYY-MM-DD)" flagrequired:"true" flaggroup:"contract"`
	Strike     float64         `flag:"strike" flagdescr:"Strike price (shared by call and put legs)" flagrequired:"true" flaggroup:"contract"`
	Buy        bool            `flag:"buy" flagdescr:"Buy the straddle (long, net debit)" flaggroup:"execution"`
	Sell       bool            `flag:"sell" flagdescr:"Sell the straddle (short, net credit)" flaggroup:"execution"`
	Open       bool            `flag:"open" flagdescr:"Opening position" flaggroup:"execution"`
	Close      bool            `flag:"close" flagdescr:"Closing position" flaggroup:"execution"`
	Quantity   float64         `flag:"quantity" flagdescr:"Number of contracts" flagrequired:"true" flaggroup:"execution"`
	Price      float64         `flag:"price" flagdescr:"Net debit or credit amount" flagrequired:"true" flaggroup:"pricing"`
	Duration   models.Duration `flag:"duration" flagdescr:"Order duration" flaggroup:"order"`
	Session    models.Session  `flag:"session" flagdescr:"Trading session" flaggroup:"order"`
}

// coveredCallBuildOpts holds flags for covered call build flows.
type coveredCallBuildOpts struct {
	Underlying string          `flag:"underlying" flagdescr:"Underlying symbol" flagrequired:"true" flaggroup:"contract"`
	Expiration string          `flag:"expiration" flagdescr:"Expiration date (YYYY-MM-DD)" flagrequired:"true" flaggroup:"contract"`
	Strike     float64         `flag:"strike" flagdescr:"Call strike price" flagrequired:"true" flaggroup:"contract"`
	Quantity   float64         `flag:"quantity" flagdescr:"Number of contracts (1 contract = 100 shares)" flagrequired:"true" flaggroup:"execution"`
	Price      float64         `flag:"price" flagdescr:"Net debit amount" flagrequired:"true" flaggroup:"pricing"`
	Duration   models.Duration `flag:"duration" flagdescr:"Order duration" flaggroup:"order"`
	Session    models.Session  `flag:"session" flagdescr:"Trading session" flaggroup:"order"`
}

// collarBuildOpts holds flags for collar-with-stock build flows.
type collarBuildOpts struct {
	Underlying string          `flag:"underlying" flagdescr:"Underlying symbol" flagrequired:"true" flaggroup:"contract"`
	PutStrike  float64         `flag:"put-strike" flagdescr:"Protective put strike price" flagrequired:"true" flaggroup:"contract"`
	CallStrike float64         `flag:"call-strike" flagdescr:"Covered call strike price" flagrequired:"true" flaggroup:"contract"`
	Expiration string          `flag:"expiration" flagdescr:"Expiration date for both options (YYYY-MM-DD)" flagrequired:"true" flaggroup:"contract"`
	Quantity   float64         `flag:"quantity" flagdescr:"Number of contracts (1 contract = 100 shares)" flagrequired:"true" flaggroup:"execution"`
	Open       bool            `flag:"open" flagdescr:"Opening position" flaggroup:"execution"`
	Close      bool            `flag:"close" flagdescr:"Closing position" flaggroup:"execution"`
	Price      float64         `flag:"price" flagdescr:"Net debit amount" flagrequired:"true" flaggroup:"pricing"`
	Duration   models.Duration `flag:"duration" flagdescr:"Order duration" flaggroup:"order"`
	Session    models.Session  `flag:"session" flagdescr:"Trading session" flaggroup:"order"`
}

// calendarBuildOpts holds flags for calendar spread build flows.
type calendarBuildOpts struct {
	Underlying     string          `flag:"underlying" flagdescr:"Underlying symbol" flagrequired:"true" flaggroup:"contract"`
	NearExpiration string          `flag:"near-expiration" flagdescr:"Near-term expiration date (YYYY-MM-DD)" flagrequired:"true" flaggroup:"contract"`
	FarExpiration  string          `flag:"far-expiration" flagdescr:"Far-term expiration date (YYYY-MM-DD)" flagrequired:"true" flaggroup:"contract"`
	Strike         float64         `flag:"strike" flagdescr:"Strike price (shared by both legs)" flagrequired:"true" flaggroup:"contract"`
	Call           bool            `flag:"call" flagdescr:"Call calendar spread" flaggroup:"contract"`
	Put            bool            `flag:"put" flagdescr:"Put calendar spread" flaggroup:"contract"`
	Open           bool            `flag:"open" flagdescr:"Opening position" flaggroup:"execution"`
	Close          bool            `flag:"close" flagdescr:"Closing position" flaggroup:"execution"`
	Quantity       float64         `flag:"quantity" flagdescr:"Number of contracts" flagrequired:"true" flaggroup:"execution"`
	Price          float64         `flag:"price" flagdescr:"Net debit amount" flagrequired:"true" flaggroup:"pricing"`
	Duration       models.Duration `flag:"duration" flagdescr:"Order duration" flaggroup:"order"`
	Session        models.Session  `flag:"session" flagdescr:"Trading session" flaggroup:"order"`
}

// diagonalBuildOpts holds flags for diagonal spread build flows.
type diagonalBuildOpts struct {
	Underlying     string          `flag:"underlying" flagdescr:"Underlying symbol" flagrequired:"true" flaggroup:"contract"`
	NearExpiration string          `flag:"near-expiration" flagdescr:"Near-term expiration date (YYYY-MM-DD)" flagrequired:"true" flaggroup:"contract"`
	FarExpiration  string          `flag:"far-expiration" flagdescr:"Far-term expiration date (YYYY-MM-DD)" flagrequired:"true" flaggroup:"contract"`
	NearStrike     float64         `flag:"near-strike" flagdescr:"Strike price for the near-term (sold) leg" flagrequired:"true" flaggroup:"contract"`
	FarStrike      float64         `flag:"far-strike" flagdescr:"Strike price for the far-term (bought) leg" flagrequired:"true" flaggroup:"contract"`
	Call           bool            `flag:"call" flagdescr:"Call diagonal spread" flaggroup:"contract"`
	Put            bool            `flag:"put" flagdescr:"Put diagonal spread" flaggroup:"contract"`
	Open           bool            `flag:"open" flagdescr:"Opening position" flaggroup:"execution"`
	Close          bool            `flag:"close" flagdescr:"Closing position" flaggroup:"execution"`
	Quantity       float64         `flag:"quantity" flagdescr:"Number of contracts" flagrequired:"true" flaggroup:"execution"`
	Price          float64         `flag:"price" flagdescr:"Net debit amount" flagrequired:"true" flaggroup:"pricing"`
	Duration       models.Duration `flag:"duration" flagdescr:"Order duration" flaggroup:"order"`
	Session        models.Session  `flag:"session" flagdescr:"Trading session" flaggroup:"order"`
}

// butterflyBuildOpts holds flags for butterfly spread build flows.
type butterflyBuildOpts struct {
	Underlying   string          `flag:"underlying" flagdescr:"Underlying symbol" flagrequired:"true" flaggroup:"contract"`
	Expiration   string          `flag:"expiration" flagdescr:"Expiration date (YYYY-MM-DD)" flagrequired:"true" flaggroup:"contract"`
	LowerStrike  float64         `flag:"lower-strike" flagdescr:"Lower wing strike" flagrequired:"true" flaggroup:"contract"`
	MiddleStrike float64         `flag:"middle-strike" flagdescr:"Middle body strike" flagrequired:"true" flaggroup:"contract"`
	UpperStrike  float64         `flag:"upper-strike" flagdescr:"Upper wing strike" flagrequired:"true" flaggroup:"contract"`
	Call         bool            `flag:"call" flagdescr:"Call butterfly" flaggroup:"contract"`
	Put          bool            `flag:"put" flagdescr:"Put butterfly" flaggroup:"contract"`
	Buy          bool            `flag:"buy" flagdescr:"Buy a long butterfly" flaggroup:"execution"`
	Sell         bool            `flag:"sell" flagdescr:"Sell a short butterfly" flaggroup:"execution"`
	Open         bool            `flag:"open" flagdescr:"Opening position" flaggroup:"execution"`
	Close        bool            `flag:"close" flagdescr:"Closing position" flaggroup:"execution"`
	Quantity     float64         `flag:"quantity" flagdescr:"Wing contract quantity" flagrequired:"true" flaggroup:"execution"`
	Price        float64         `flag:"price" flagdescr:"Net debit or credit amount" flagrequired:"true" flaggroup:"pricing"`
	Duration     models.Duration `flag:"duration" flagdescr:"Order duration" flaggroup:"order"`
	Session      models.Session  `flag:"session" flagdescr:"Trading session" flaggroup:"order"`
}

// condorBuildOpts holds flags for condor spread build flows.
type condorBuildOpts struct {
	Underlying        string          `flag:"underlying" flagdescr:"Underlying symbol" flagrequired:"true" flaggroup:"contract"`
	Expiration        string          `flag:"expiration" flagdescr:"Expiration date (YYYY-MM-DD)" flagrequired:"true" flaggroup:"contract"`
	LowerStrike       float64         `flag:"lower-strike" flagdescr:"Lower wing strike" flagrequired:"true" flaggroup:"contract"`
	LowerMiddleStrike float64         `flag:"lower-middle-strike" flagdescr:"Lower middle strike" flagrequired:"true" flaggroup:"contract"`
	UpperMiddleStrike float64         `flag:"upper-middle-strike" flagdescr:"Upper middle strike" flagrequired:"true" flaggroup:"contract"`
	UpperStrike       float64         `flag:"upper-strike" flagdescr:"Upper wing strike" flagrequired:"true" flaggroup:"contract"`
	Call              bool            `flag:"call" flagdescr:"Call condor" flaggroup:"contract"`
	Put               bool            `flag:"put" flagdescr:"Put condor" flaggroup:"contract"`
	Buy               bool            `flag:"buy" flagdescr:"Buy a long condor" flaggroup:"execution"`
	Sell              bool            `flag:"sell" flagdescr:"Sell a short condor" flaggroup:"execution"`
	Open              bool            `flag:"open" flagdescr:"Opening position" flaggroup:"execution"`
	Close             bool            `flag:"close" flagdescr:"Closing position" flaggroup:"execution"`
	Quantity          float64         `flag:"quantity" flagdescr:"Contract quantity per strike" flagrequired:"true" flaggroup:"execution"`
	Price             float64         `flag:"price" flagdescr:"Net debit or credit amount" flagrequired:"true" flaggroup:"pricing"`
	Duration          models.Duration `flag:"duration" flagdescr:"Order duration" flaggroup:"order"`
	Session           models.Session  `flag:"session" flagdescr:"Trading session" flaggroup:"order"`
}

// backRatioBuildOpts holds flags for back-ratio spread build flows.
type backRatioBuildOpts struct {
	Underlying  string          `flag:"underlying" flagdescr:"Underlying symbol" flagrequired:"true" flaggroup:"contract"`
	Expiration  string          `flag:"expiration" flagdescr:"Expiration date (YYYY-MM-DD)" flagrequired:"true" flaggroup:"contract"`
	ShortStrike float64         `flag:"short-strike" flagdescr:"Short option strike" flagrequired:"true" flaggroup:"contract"`
	LongStrike  float64         `flag:"long-strike" flagdescr:"Long option strike" flagrequired:"true" flaggroup:"contract"`
	Call        bool            `flag:"call" flagdescr:"Call back-ratio" flaggroup:"contract"`
	Put         bool            `flag:"put" flagdescr:"Put back-ratio" flaggroup:"contract"`
	Open        bool            `flag:"open" flagdescr:"Opening position" flaggroup:"execution"`
	Close       bool            `flag:"close" flagdescr:"Closing position" flaggroup:"execution"`
	Quantity    float64         `flag:"quantity" flagdescr:"Short-leg contract quantity" flagrequired:"true" flaggroup:"execution"`
	LongRatio   float64         `flag:"long-ratio" flagdescr:"Long contracts per short contract" default:"2" flaggroup:"execution"`
	Debit       bool            `flag:"debit" flagdescr:"Build as NET_DEBIT" flaggroup:"pricing"`
	Credit      bool            `flag:"credit" flagdescr:"Build as NET_CREDIT" flaggroup:"pricing"`
	Price       float64         `flag:"price" flagdescr:"Net debit or credit amount" flagrequired:"true" flaggroup:"pricing"`
	Duration    models.Duration `flag:"duration" flagdescr:"Order duration" flaggroup:"order"`
	Session     models.Session  `flag:"session" flagdescr:"Trading session" flaggroup:"order"`
}

// verticalRollBuildOpts holds flags for vertical roll build flows.
type verticalRollBuildOpts struct {
	Underlying       string          `flag:"underlying" flagdescr:"Underlying symbol" flagrequired:"true" flaggroup:"contract"`
	CloseExpiration  string          `flag:"close-expiration" flagdescr:"Expiration of the vertical being closed (YYYY-MM-DD)" flagrequired:"true" flaggroup:"contract"`
	OpenExpiration   string          `flag:"open-expiration" flagdescr:"Expiration of the vertical being opened (YYYY-MM-DD)" flagrequired:"true" flaggroup:"contract"`
	CloseLongStrike  float64         `flag:"close-long-strike" flagdescr:"Long strike of the vertical being closed" flagrequired:"true" flaggroup:"contract"`
	CloseShortStrike float64         `flag:"close-short-strike" flagdescr:"Short strike of the vertical being closed" flagrequired:"true" flaggroup:"contract"`
	OpenLongStrike   float64         `flag:"open-long-strike" flagdescr:"Long strike of the vertical being opened" flagrequired:"true" flaggroup:"contract"`
	OpenShortStrike  float64         `flag:"open-short-strike" flagdescr:"Short strike of the vertical being opened" flagrequired:"true" flaggroup:"contract"`
	Call             bool            `flag:"call" flagdescr:"Call vertical roll" flaggroup:"contract"`
	Put              bool            `flag:"put" flagdescr:"Put vertical roll" flaggroup:"contract"`
	Debit            bool            `flag:"debit" flagdescr:"Build as NET_DEBIT" flaggroup:"pricing"`
	Credit           bool            `flag:"credit" flagdescr:"Build as NET_CREDIT" flaggroup:"pricing"`
	Quantity         float64         `flag:"quantity" flagdescr:"Contract quantity" flagrequired:"true" flaggroup:"execution"`
	Price            float64         `flag:"price" flagdescr:"Net debit or credit amount" flagrequired:"true" flaggroup:"pricing"`
	Duration         models.Duration `flag:"duration" flagdescr:"Order duration" flaggroup:"order"`
	Session          models.Session  `flag:"session" flagdescr:"Trading session" flaggroup:"order"`
}

// doubleDiagonalBuildOpts holds flags for double diagonal spread build flows.
type doubleDiagonalBuildOpts struct {
	Underlying     string          `flag:"underlying" flagdescr:"Underlying symbol" flagrequired:"true" flaggroup:"contract"`
	NearExpiration string          `flag:"near-expiration" flagdescr:"Near expiration for short legs (YYYY-MM-DD)" flagrequired:"true" flaggroup:"contract"`
	FarExpiration  string          `flag:"far-expiration" flagdescr:"Far expiration for long legs (YYYY-MM-DD)" flagrequired:"true" flaggroup:"contract"`
	PutFarStrike   float64         `flag:"put-far-strike" flagdescr:"Far put long strike" flagrequired:"true" flaggroup:"contract"`
	PutNearStrike  float64         `flag:"put-near-strike" flagdescr:"Near put short strike" flagrequired:"true" flaggroup:"contract"`
	CallNearStrike float64         `flag:"call-near-strike" flagdescr:"Near call short strike" flagrequired:"true" flaggroup:"contract"`
	CallFarStrike  float64         `flag:"call-far-strike" flagdescr:"Far call long strike" flagrequired:"true" flaggroup:"contract"`
	Open           bool            `flag:"open" flagdescr:"Opening position" flaggroup:"execution"`
	Close          bool            `flag:"close" flagdescr:"Closing position" flaggroup:"execution"`
	Quantity       float64         `flag:"quantity" flagdescr:"Contract quantity" flagrequired:"true" flaggroup:"execution"`
	Price          float64         `flag:"price" flagdescr:"Net debit or credit amount" flagrequired:"true" flaggroup:"pricing"`
	Duration       models.Duration `flag:"duration" flagdescr:"Order duration" flaggroup:"order"`
	Session        models.Session  `flag:"session" flagdescr:"Trading session" flaggroup:"order"`
}

// Valid enum values for CLI flag parsing. Each slice corresponds to one enum
// type in the models package and is used by the generic parseEnum/requireEnum
// helpers in helpers.go.
var (
	validInstructions = []models.Instruction{
		models.InstructionBuy,
		models.InstructionSell,
		models.InstructionBuyToCover,
		models.InstructionSellShort,
		models.InstructionBuyToOpen,
		models.InstructionBuyToClose,
		models.InstructionSellToOpen,
		models.InstructionSellToClose,
		models.InstructionExchange,
		models.InstructionSellShortExempt,
	}

	validOrderTypes = []models.OrderType{
		models.OrderTypeMarket,
		models.OrderTypeLimit,
		models.OrderTypeStop,
		models.OrderTypeStopLimit,
		models.OrderTypeTrailingStop,
		models.OrderTypeTrailingStopLimit,
		models.OrderTypeMarketOnClose,
		models.OrderTypeLimitOnClose,
		models.OrderTypeNetDebit,
		models.OrderTypeNetCredit,
		models.OrderTypeNetZero,
	}

	validDurations = []models.Duration{
		models.DurationDay,
		models.DurationGoodTillCancel,
		models.DurationFillOrKill,
		models.DurationImmediateOrCancel,
		models.DurationEndOfWeek,
		models.DurationEndOfMonth,
		models.DurationNextEndOfMonth,
	}

	validSessions = []models.Session{
		models.SessionNormal,
		models.SessionAM,
		models.SessionPM,
		models.SessionSeamless,
	}

	validStopPriceLinkBases = []models.StopPriceLinkBasis{
		models.StopPriceLinkBasisManual,
		models.StopPriceLinkBasisBase,
		models.StopPriceLinkBasisTrigger,
		models.StopPriceLinkBasisLast,
		models.StopPriceLinkBasisBid,
		models.StopPriceLinkBasisAsk,
		models.StopPriceLinkBasisAskBid,
		models.StopPriceLinkBasisMark,
		models.StopPriceLinkBasisAverage,
	}

	validStopPriceLinkTypes = []models.StopPriceLinkType{
		models.StopPriceLinkTypeValue,
		models.StopPriceLinkTypePercent,
		models.StopPriceLinkTypeTick,
	}

	validStopTypes = []models.StopType{
		models.StopTypeStandard,
		models.StopTypeBid,
		models.StopTypeAsk,
		models.StopTypeLast,
		models.StopTypeMark,
	}

	validSpecialInstructions = []models.SpecialInstruction{
		models.SpecialInstructionAllOrNone,
		models.SpecialInstructionDoNotReduce,
		models.SpecialInstructionAllOrNoneDoNotReduce,
	}

	validDestinations = []models.RequestedDestination{
		models.RequestedDestinationINET,
		models.RequestedDestinationECNArca,
		models.RequestedDestinationCBOE,
		models.RequestedDestinationAMEX,
		models.RequestedDestinationPHLX,
		models.RequestedDestinationISE,
		models.RequestedDestinationBOX,
		models.RequestedDestinationNYSE,
		models.RequestedDestinationNASDAQ,
		models.RequestedDestinationBATS,
		models.RequestedDestinationC2,
		models.RequestedDestinationAUTO,
	}

	validPriceLinkBases = []models.PriceLinkBasis{
		models.PriceLinkBasisManual,
		models.PriceLinkBasisBase,
		models.PriceLinkBasisTrigger,
		models.PriceLinkBasisLast,
		models.PriceLinkBasisBid,
		models.PriceLinkBasisAsk,
		models.PriceLinkBasisAskBid,
		models.PriceLinkBasisMark,
		models.PriceLinkBasisAverage,
	}

	validPriceLinkTypes = []models.PriceLinkType{
		models.PriceLinkTypeValue,
		models.PriceLinkTypePercent,
		models.PriceLinkTypeTick,
	}

	validOrderStatusFilters = []orderStatusFilter{
		orderStatusFilterAll,
		orderStatusFilter(models.OrderStatusAwaitingParentOrder),
		orderStatusFilter(models.OrderStatusAwaitingCondition),
		orderStatusFilter(models.OrderStatusAwaitingStopCondition),
		orderStatusFilter(models.OrderStatusAwaitingManualReview),
		orderStatusFilter(models.OrderStatusAccepted),
		orderStatusFilter(models.OrderStatusAwaitingUROut),
		orderStatusFilter(models.OrderStatusPendingActivation),
		orderStatusFilter(models.OrderStatusQueued),
		orderStatusFilter(models.OrderStatusWorking),
		orderStatusFilter(models.OrderStatusRejected),
		orderStatusFilter(models.OrderStatusPendingCancel),
		orderStatusFilter(models.OrderStatusCanceled),
		orderStatusFilter(models.OrderStatusPendingReplace),
		orderStatusFilter(models.OrderStatusReplaced),
		orderStatusFilter(models.OrderStatusFilled),
		orderStatusFilter(models.OrderStatusExpired),
		orderStatusFilter(models.OrderStatusNew),
		orderStatusFilter(models.OrderStatusAwaitingReleaseTime),
		orderStatusFilter(models.OrderStatusPendingAcknowledgement),
		orderStatusFilter(models.OrderStatusPendingRecall),
		orderStatusFilter(models.OrderStatusUnknown),
	}
)

// normalizeOrderType preserves legacy CLI aliases after structcli has already
// validated the typed enum flag value.
func normalizeOrderType(orderType, fallback models.OrderType) models.OrderType {
	switch orderType {
	case "":
		return fallback
	case models.OrderType("MOC"):
		return models.OrderTypeMarketOnClose
	case models.OrderType("LOC"):
		return models.OrderTypeLimitOnClose
	default:
		return orderType
	}
}

// normalizeDuration preserves common trading abbreviations after structcli
// validation so order builders still receive canonical API enum values.
func normalizeDuration(duration models.Duration) models.Duration {
	switch duration {
	case models.Duration("GTC"):
		return models.DurationGoodTillCancel
	case models.Duration("FOK"):
		return models.DurationFillOrKill
	case models.Duration("IOC"):
		return models.DurationImmediateOrCancel
	default:
		return duration
	}
}

func defaultStopPriceLinkBasis(value models.StopPriceLinkBasis) models.StopPriceLinkBasis {
	if value == "" {
		return models.StopPriceLinkBasisLast
	}
	return value
}

func defaultStopPriceLinkType(value models.StopPriceLinkType) models.StopPriceLinkType {
	if value == "" {
		return models.StopPriceLinkTypeValue
	}
	return value
}

func defaultStopType(value models.StopType) models.StopType {
	if value == "" {
		return models.StopTypeStandard
	}
	return value
}

func validateOrderStatusFilter(raw string) error {
	for _, valid := range validOrderStatusFilters {
		if strings.EqualFold(raw, string(valid)) {
			return nil
		}
	}

	return newValidationError("invalid status")
}

// parseInstruction converts CLI input to an instruction enum.
func parseInstruction(raw string) (models.Instruction, error) {
	return requireEnum(raw, validInstructions, "action")
}

// parseOrderType converts CLI input to an order type enum.
// Supports aliases MOC (MARKET_ON_CLOSE) and LOC (LIMIT_ON_CLOSE).
func parseOrderType(raw string, fallback models.OrderType) (models.OrderType, error) {
	upper := strings.ToUpper(strings.TrimSpace(raw))

	// Resolve aliases before standard enum validation.
	switch upper {
	case "MOC":
		return models.OrderTypeMarketOnClose, nil
	case "LOC":
		return models.OrderTypeLimitOnClose, nil
	}

	return parseEnum(raw, validOrderTypes, fallback, "type")
}

// parseDuration converts CLI input to a duration enum for focused unit tests and
// legacy helper coverage. Runtime flag decoding is handled by structcli enums.
func parseDuration(raw string) (models.Duration, error) {
	upper := strings.ToUpper(strings.TrimSpace(raw))

	switch upper {
	case "GTC":
		return models.DurationGoodTillCancel, nil
	case "FOK":
		return models.DurationFillOrKill, nil
	case "IOC":
		return models.DurationImmediateOrCancel, nil
	}

	return parseEnum(raw, validDurations, "", "duration")
}

// requireMutableEnabled checks that mutable operations are explicitly enabled in config.
func requireMutableEnabled(configPath string) error {
	cfg, err := auth.LoadConfig(configPath)
	if err != nil {
		return apperr.NewValidationError(mutableDisabledMessage, nil)
	}

	if !cfg.IAlsoLikeToLiveDangerously {
		return apperr.NewValidationError(mutableDisabledMessage, nil)
	}

	return nil
}

// parseRequiredOrderID parses the --order-id flag or first positional argument as an order ID.
func parseRequiredOrderID(orderIDValue string, args []string) (int64, error) {
	// Flag takes priority over positional arg, matching resolveAccount() convention.
	value := strings.TrimSpace(orderIDValue)
	if value == "" && len(args) > 0 {
		value = strings.TrimSpace(args[0])
	}

	if value == "" {
		return 0, newValidationError("order-id is required (provide as positional arg or --order-id flag)")
	}

	orderID, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, newValidationError("order-id must be a valid integer")
	}

	if orderID <= 0 {
		return 0, newValidationError("order-id must be a positive integer")
	}

	return orderID, nil
}

// parseSpecOrder loads and validates spec mode JSON into an order request.
func parseSpecOrder(cmd *cobra.Command, spec string) (*models.OrderRequest, error) {
	raw, err := readSpecSource(cmd, spec)
	if err != nil {
		return nil, err
	}

	var order models.OrderRequest
	if err := json.Unmarshal(raw, &order); err != nil {
		return nil, newValidationError("spec must contain valid JSON")
	}

	return &order, nil
}

// readSpecSource resolves inline, file, and stdin JSON inputs.
// All three source types (stdin, @file, inline) share a single json.Valid check
// after the raw bytes are resolved.
func readSpecSource(cmd any, spec string) ([]byte, error) {
	trimmed := strings.TrimSpace(spec)
	if trimmed == "" {
		return nil, newValidationError("spec is required")
	}

	var payload []byte

	switch {
	case trimmed == "-":
		reader := specInputReader(cmd)
		if reader == nil {
			reader = strings.NewReader("")
		}

		var err error

		payload, err = io.ReadAll(reader)
		if err != nil {
			return nil, fmt.Errorf("read spec from stdin: %w", err)
		}

	case strings.HasPrefix(trimmed, "@"):
		filePath, _ := strings.CutPrefix(trimmed, "@")

		var err error

		payload, err = os.ReadFile(strings.TrimSpace(filePath))
		if err != nil {
			return nil, fmt.Errorf("read spec file: %w", err)
		}

	case trimmed[0] == '{' || trimmed[0] == '[':
		payload = []byte(trimmed)

	default:
		return nil, newValidationError("spec must be inline JSON, @file, or -")
	}

	if !json.Valid(payload) {
		return nil, newValidationError("spec must contain valid JSON")
	}

	return payload, nil
}

// specInputReader returns the command stdin reader.
func specInputReader(cmd any) io.Reader {
	if cobraCmd, ok := cmd.(interface{ InOrStdin() io.Reader }); ok {
		return cobraCmd.InOrStdin()
	}

	return nil
}

// parseEquityParams converts command flags into equity order builder params.
func parseEquityParams(opts *equityPlaceOpts, _ []string) (*orderbuilder.EquityParams, error) {
	if err := requireTypedEnum(opts.Action, "action"); err != nil {
		return nil, err
	}

	return &orderbuilder.EquityParams{
		Symbol:             strings.TrimSpace(opts.Symbol),
		Action:             opts.Action,
		Quantity:           opts.Quantity,
		OrderType:          normalizeOrderType(opts.Type, models.OrderTypeMarket),
		Price:              opts.Price,
		StopPrice:          opts.StopPrice,
		StopPriceOffset:    opts.StopOffset,
		StopPriceLinkBasis: defaultStopPriceLinkBasis(opts.StopLinkBasis),
		StopPriceLinkType:  defaultStopPriceLinkType(opts.StopLinkType),
		StopType:           defaultStopType(opts.StopType),
		ActivationPrice:    opts.ActivationPrice,
		SpecialInstruction: opts.SpecialInstruction,
		Destination:        opts.Destination,
		PriceLinkBasis:     opts.PriceLinkBasis,
		PriceLinkType:      opts.PriceLinkType,
		Duration:           normalizeDuration(opts.Duration),
		Session:            opts.Session,
	}, nil
}

// parseOptionParams converts command flags into option order builder params.
func parseOptionParams(opts *optionPlaceOpts, _ []string) (*orderbuilder.OptionParams, error) {
	if err := requireTypedEnum(opts.Action, "action"); err != nil {
		return nil, err
	}

	putCall, err := parsePutCall(opts.Call, opts.Put)
	if err != nil {
		return nil, err
	}

	expiration, err := parseExpiration(opts.Expiration)
	if err != nil {
		return nil, err
	}

	return &orderbuilder.OptionParams{
		Underlying:         strings.TrimSpace(opts.Underlying),
		Expiration:         expiration,
		Strike:             opts.Strike,
		PutCall:            putCall,
		Action:             opts.Action,
		Quantity:           opts.Quantity,
		OrderType:          normalizeOrderType(opts.Type, models.OrderTypeMarket),
		Price:              opts.Price,
		SpecialInstruction: opts.SpecialInstruction,
		Destination:        opts.Destination,
		PriceLinkBasis:     opts.PriceLinkBasis,
		PriceLinkType:      opts.PriceLinkType,
		Duration:           normalizeDuration(opts.Duration),
		Session:            opts.Session,
	}, nil
}

// parseBracketParams converts command flags into bracket order builder params.
func parseBracketParams(opts *bracketPlaceOpts, _ []string) (*orderbuilder.BracketParams, error) {
	if err := requireTypedEnum(opts.Action, "action"); err != nil {
		return nil, err
	}

	return &orderbuilder.BracketParams{
		Symbol:     strings.TrimSpace(opts.Symbol),
		Action:     opts.Action,
		Quantity:   opts.Quantity,
		OrderType:  normalizeOrderType(opts.Type, models.OrderTypeMarket),
		Price:      opts.Price,
		TakeProfit: opts.TakeProfit,
		StopLoss:   opts.StopLoss,
		Duration:   normalizeDuration(opts.Duration),
		Session:    opts.Session,
	}, nil
}

// parseOCOParams converts command flags into standalone OCO builder params.
func parseOCOParams(opts *ocoPlaceOpts, _ []string) (*orderbuilder.OCOParams, error) {
	if err := requireTypedEnum(opts.Action, "action"); err != nil {
		return nil, err
	}

	return &orderbuilder.OCOParams{
		Symbol:     strings.TrimSpace(opts.Symbol),
		Action:     opts.Action,
		Quantity:   opts.Quantity,
		TakeProfit: opts.TakeProfit,
		StopLoss:   opts.StopLoss,
		Duration:   normalizeDuration(opts.Duration),
		Session:    opts.Session,
	}, nil
}

// parseIronCondorParams converts command flags into iron condor builder params.
func parseIronCondorParams(opts *ironCondorBuildOpts, _ []string) (*orderbuilder.IronCondorParams, error) {
	isOpen, err := parseOpenClose(opts.Open, opts.Close)
	if err != nil {
		return nil, err
	}

	expiration, err := parseExpiration(opts.Expiration)
	if err != nil {
		return nil, err
	}

	return &orderbuilder.IronCondorParams{
		Underlying:      strings.TrimSpace(opts.Underlying),
		Expiration:      expiration,
		PutLongStrike:   opts.PutLongStrike,
		PutShortStrike:  opts.PutShortStrike,
		CallShortStrike: opts.CallShortStrike,
		CallLongStrike:  opts.CallLongStrike,
		Open:            isOpen,
		Quantity:        opts.Quantity,
		Price:           opts.Price,
		Duration:        normalizeDuration(opts.Duration),
		Session:         opts.Session,
	}, nil
}

// parseVerticalParams converts command flags into vertical spread builder params.
func parseVerticalParams(opts *verticalBuildOpts, _ []string) (*orderbuilder.VerticalParams, error) {
	putCall, err := parsePutCall(opts.Call, opts.Put)
	if err != nil {
		return nil, err
	}

	isOpen, err := parseOpenClose(opts.Open, opts.Close)
	if err != nil {
		return nil, err
	}

	expiration, err := parseExpiration(opts.Expiration)
	if err != nil {
		return nil, err
	}

	return &orderbuilder.VerticalParams{
		Underlying:  strings.TrimSpace(opts.Underlying),
		Expiration:  expiration,
		LongStrike:  opts.LongStrike,
		ShortStrike: opts.ShortStrike,
		PutCall:     putCall,
		Open:        isOpen,
		Quantity:    opts.Quantity,
		Price:       opts.Price,
		Duration:    normalizeDuration(opts.Duration),
		Session:     opts.Session,
	}, nil
}

// parseStrangleParams converts command flags into strangle builder params.
func parseStrangleParams(opts *strangleBuildOpts, _ []string) (*orderbuilder.StrangleParams, error) {
	isBuy, err := parseBuySell(opts.Buy, opts.Sell)
	if err != nil {
		return nil, err
	}

	isOpen, err := parseOpenClose(opts.Open, opts.Close)
	if err != nil {
		return nil, err
	}

	expiration, err := parseExpiration(opts.Expiration)
	if err != nil {
		return nil, err
	}

	return &orderbuilder.StrangleParams{
		Underlying: strings.TrimSpace(opts.Underlying),
		Expiration: expiration,
		CallStrike: opts.CallStrike,
		PutStrike:  opts.PutStrike,
		Buy:        isBuy,
		Open:       isOpen,
		Quantity:   opts.Quantity,
		Price:      opts.Price,
		Duration:   normalizeDuration(opts.Duration),
		Session:    opts.Session,
	}, nil
}

// parseStraddleParams converts command flags into straddle builder params.
func parseStraddleParams(opts *straddleBuildOpts, _ []string) (*orderbuilder.StraddleParams, error) {
	isBuy, err := parseBuySell(opts.Buy, opts.Sell)
	if err != nil {
		return nil, err
	}

	isOpen, err := parseOpenClose(opts.Open, opts.Close)
	if err != nil {
		return nil, err
	}

	expiration, err := parseExpiration(opts.Expiration)
	if err != nil {
		return nil, err
	}

	return &orderbuilder.StraddleParams{
		Underlying: strings.TrimSpace(opts.Underlying),
		Expiration: expiration,
		Strike:     opts.Strike,
		Buy:        isBuy,
		Open:       isOpen,
		Quantity:   opts.Quantity,
		Price:      opts.Price,
		Duration:   normalizeDuration(opts.Duration),
		Session:    opts.Session,
	}, nil
}

// parseCoveredCallParams converts command flags into covered call builder params.
func parseCoveredCallParams(opts *coveredCallBuildOpts, _ []string) (*orderbuilder.CoveredCallParams, error) {
	expiration, err := parseExpiration(opts.Expiration)
	if err != nil {
		return nil, err
	}

	return &orderbuilder.CoveredCallParams{
		Underlying: strings.TrimSpace(opts.Underlying),
		Expiration: expiration,
		Strike:     opts.Strike,
		Quantity:   opts.Quantity,
		Price:      opts.Price,
		Duration:   normalizeDuration(opts.Duration),
		Session:    opts.Session,
	}, nil
}

// parseCollarParams converts command flags into collar-with-stock builder params.
func parseCollarParams(opts *collarBuildOpts, _ []string) (*orderbuilder.CollarParams, error) {
	isOpen, err := parseOpenClose(opts.Open, opts.Close)
	if err != nil {
		return nil, err
	}

	expiration, err := parseExpiration(opts.Expiration)
	if err != nil {
		return nil, err
	}

	return &orderbuilder.CollarParams{
		Underlying: strings.TrimSpace(opts.Underlying),
		PutStrike:  opts.PutStrike,
		CallStrike: opts.CallStrike,
		Expiration: expiration,
		Quantity:   opts.Quantity,
		Open:       isOpen,
		Price:      opts.Price,
		Duration:   normalizeDuration(opts.Duration),
		Session:    opts.Session,
	}, nil
}

// parseCalendarParams converts command flags into calendar spread builder params.
func parseCalendarParams(opts *calendarBuildOpts, _ []string) (*orderbuilder.CalendarParams, error) {
	putCall, err := parsePutCall(opts.Call, opts.Put)
	if err != nil {
		return nil, err
	}

	isOpen, err := parseOpenClose(opts.Open, opts.Close)
	if err != nil {
		return nil, err
	}

	nearExpiration, err := parseDateFlag(opts.NearExpiration, "near-expiration")
	if err != nil {
		return nil, err
	}

	farExpiration, err := parseDateFlag(opts.FarExpiration, "far-expiration")
	if err != nil {
		return nil, err
	}

	return &orderbuilder.CalendarParams{
		Underlying:     strings.TrimSpace(opts.Underlying),
		NearExpiration: nearExpiration,
		FarExpiration:  farExpiration,
		Strike:         opts.Strike,
		PutCall:        putCall,
		Open:           isOpen,
		Quantity:       opts.Quantity,
		Price:          opts.Price,
		Duration:       normalizeDuration(opts.Duration),
		Session:        opts.Session,
	}, nil
}

// parseDiagonalParams converts command flags into diagonal spread builder params.
func parseDiagonalParams(opts *diagonalBuildOpts, _ []string) (*orderbuilder.DiagonalParams, error) {
	putCall, err := parsePutCall(opts.Call, opts.Put)
	if err != nil {
		return nil, err
	}

	isOpen, err := parseOpenClose(opts.Open, opts.Close)
	if err != nil {
		return nil, err
	}

	nearExpiration, err := parseDateFlag(opts.NearExpiration, "near-expiration")
	if err != nil {
		return nil, err
	}

	farExpiration, err := parseDateFlag(opts.FarExpiration, "far-expiration")
	if err != nil {
		return nil, err
	}

	return &orderbuilder.DiagonalParams{
		Underlying:     strings.TrimSpace(opts.Underlying),
		NearExpiration: nearExpiration,
		FarExpiration:  farExpiration,
		NearStrike:     opts.NearStrike,
		FarStrike:      opts.FarStrike,
		PutCall:        putCall,
		Open:           isOpen,
		Quantity:       opts.Quantity,
		Price:          opts.Price,
		Duration:       normalizeDuration(opts.Duration),
		Session:        opts.Session,
	}, nil
}

// parseButterflyParams converts command flags into butterfly builder params.
func parseButterflyParams(opts *butterflyBuildOpts, _ []string) (*orderbuilder.ButterflyParams, error) {
	putCall, err := parsePutCall(opts.Call, opts.Put)
	if err != nil {
		return nil, err
	}

	isBuy, err := parseBuySell(opts.Buy, opts.Sell)
	if err != nil {
		return nil, err
	}

	isOpen, err := parseOpenClose(opts.Open, opts.Close)
	if err != nil {
		return nil, err
	}

	expiration, err := parseExpiration(opts.Expiration)
	if err != nil {
		return nil, err
	}

	return &orderbuilder.ButterflyParams{
		Underlying:   strings.TrimSpace(opts.Underlying),
		Expiration:   expiration,
		LowerStrike:  opts.LowerStrike,
		MiddleStrike: opts.MiddleStrike,
		UpperStrike:  opts.UpperStrike,
		PutCall:      putCall,
		Buy:          isBuy,
		Open:         isOpen,
		Quantity:     opts.Quantity,
		Price:        opts.Price,
		Duration:     normalizeDuration(opts.Duration),
		Session:      opts.Session,
	}, nil
}

// parseCondorParams converts command flags into condor builder params.
func parseCondorParams(opts *condorBuildOpts, _ []string) (*orderbuilder.CondorParams, error) {
	putCall, err := parsePutCall(opts.Call, opts.Put)
	if err != nil {
		return nil, err
	}

	isBuy, err := parseBuySell(opts.Buy, opts.Sell)
	if err != nil {
		return nil, err
	}

	isOpen, err := parseOpenClose(opts.Open, opts.Close)
	if err != nil {
		return nil, err
	}

	expiration, err := parseExpiration(opts.Expiration)
	if err != nil {
		return nil, err
	}

	return &orderbuilder.CondorParams{
		Underlying:        strings.TrimSpace(opts.Underlying),
		Expiration:        expiration,
		LowerStrike:       opts.LowerStrike,
		LowerMiddleStrike: opts.LowerMiddleStrike,
		UpperMiddleStrike: opts.UpperMiddleStrike,
		UpperStrike:       opts.UpperStrike,
		PutCall:           putCall,
		Buy:               isBuy,
		Open:              isOpen,
		Quantity:          opts.Quantity,
		Price:             opts.Price,
		Duration:          normalizeDuration(opts.Duration),
		Session:           opts.Session,
	}, nil
}

// parseBackRatioParams converts command flags into back-ratio builder params.
func parseBackRatioParams(opts *backRatioBuildOpts, _ []string) (*orderbuilder.BackRatioParams, error) {
	putCall, err := parsePutCall(opts.Call, opts.Put)
	if err != nil {
		return nil, err
	}

	isOpen, err := parseOpenClose(opts.Open, opts.Close)
	if err != nil {
		return nil, err
	}

	isCredit, err := parseDebitCredit(opts.Debit, opts.Credit)
	if err != nil {
		return nil, err
	}

	expiration, err := parseExpiration(opts.Expiration)
	if err != nil {
		return nil, err
	}

	return &orderbuilder.BackRatioParams{
		Underlying:  strings.TrimSpace(opts.Underlying),
		Expiration:  expiration,
		ShortStrike: opts.ShortStrike,
		LongStrike:  opts.LongStrike,
		PutCall:     putCall,
		Open:        isOpen,
		Quantity:    opts.Quantity,
		LongRatio:   opts.LongRatio,
		Credit:      isCredit,
		Price:       opts.Price,
		Duration:    normalizeDuration(opts.Duration),
		Session:     opts.Session,
	}, nil
}

// parseVerticalRollParams converts command flags into vertical roll builder params.
func parseVerticalRollParams(opts *verticalRollBuildOpts, _ []string) (*orderbuilder.VerticalRollParams, error) {
	putCall, err := parsePutCall(opts.Call, opts.Put)
	if err != nil {
		return nil, err
	}

	isCredit, err := parseDebitCredit(opts.Debit, opts.Credit)
	if err != nil {
		return nil, err
	}

	closeExpiration, err := parseDateFlag(opts.CloseExpiration, "close-expiration")
	if err != nil {
		return nil, err
	}

	openExpiration, err := parseDateFlag(opts.OpenExpiration, "open-expiration")
	if err != nil {
		return nil, err
	}

	return &orderbuilder.VerticalRollParams{
		Underlying:       strings.TrimSpace(opts.Underlying),
		CloseExpiration:  closeExpiration,
		OpenExpiration:   openExpiration,
		CloseLongStrike:  opts.CloseLongStrike,
		CloseShortStrike: opts.CloseShortStrike,
		OpenLongStrike:   opts.OpenLongStrike,
		OpenShortStrike:  opts.OpenShortStrike,
		PutCall:          putCall,
		Credit:           isCredit,
		Quantity:         opts.Quantity,
		Price:            opts.Price,
		Duration:         normalizeDuration(opts.Duration),
		Session:          opts.Session,
	}, nil
}

// parseDoubleDiagonalParams converts command flags into double diagonal builder params.
func parseDoubleDiagonalParams(opts *doubleDiagonalBuildOpts, _ []string) (*orderbuilder.DoubleDiagonalParams, error) {
	isOpen, err := parseOpenClose(opts.Open, opts.Close)
	if err != nil {
		return nil, err
	}

	nearExpiration, err := parseDateFlag(opts.NearExpiration, "near-expiration")
	if err != nil {
		return nil, err
	}

	farExpiration, err := parseDateFlag(opts.FarExpiration, "far-expiration")
	if err != nil {
		return nil, err
	}

	return &orderbuilder.DoubleDiagonalParams{
		Underlying:     strings.TrimSpace(opts.Underlying),
		NearExpiration: nearExpiration,
		FarExpiration:  farExpiration,
		PutFarStrike:   opts.PutFarStrike,
		PutNearStrike:  opts.PutNearStrike,
		CallNearStrike: opts.CallNearStrike,
		CallFarStrike:  opts.CallFarStrike,
		Open:           isOpen,
		Quantity:       opts.Quantity,
		Price:          opts.Price,
		Duration:       normalizeDuration(opts.Duration),
		Session:        opts.Session,
	}, nil
}

// parseDateFlag parses a named YYYY-MM-DD flag value into a time.Time.
// Used by calendar/diagonal spreads which have two expiration flags instead
// of the single --expiration flag used by other spread types.
func parseDateFlag(raw, flagName string) (time.Time, error) {
	parsed, err := time.Parse("2006-01-02", strings.TrimSpace(raw))
	if err != nil {
		return time.Time{}, newValidationError(flagName + " must use YYYY-MM-DD format")
	}

	return parsed, nil
}

// parseExpiration parses the --expiration flag as a YYYY-MM-DD date.
func parseExpiration(raw string) (time.Time, error) {
	expiration, err := time.Parse("2006-01-02", strings.TrimSpace(raw))
	if err != nil {
		return time.Time{}, newValidationError("expiration must use YYYY-MM-DD format")
	}

	return expiration, nil
}

// parseBuySell validates mutually exclusive buy/sell flags.
func parseBuySell(buy, sell bool) (bool, error) {
	if buy == sell {
		return false, newValidationError("exactly one of --buy or --sell is required")
	}

	return buy, nil
}

// parseOpenClose validates mutually exclusive open/close flags.
func parseOpenClose(open, closeLeg bool) (bool, error) {
	if open == closeLeg {
		return false, newValidationError("exactly one of --open or --close is required")
	}

	return open, nil
}

// parseDebitCredit validates mutually exclusive debit/credit flags.
func parseDebitCredit(debit, credit bool) (bool, error) {
	if debit == credit {
		return false, newValidationError("exactly one of --debit or --credit is required")
	}

	return credit, nil
}

// parsePutCall validates mutually exclusive put/call flags.
func parsePutCall(call, put bool) (models.PutCall, error) {
	if call == put {
		return "", newValidationError("exactly one of --call or --put is required")
	}

	if call {
		return models.PutCallCall, nil
	}

	return models.PutCallPut, nil
}
