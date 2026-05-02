package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/leodido/structcli"
	"github.com/spf13/cobra"

	"github.com/major/schwab-agent/internal/apperr"
	"github.com/major/schwab-agent/internal/auth"
	"github.com/major/schwab-agent/internal/client"
	"github.com/major/schwab-agent/internal/models"
	"github.com/major/schwab-agent/internal/orderbuilder"
	"github.com/major/schwab-agent/internal/output"
)

// orderListData wraps the order list response.
type orderListData struct {
	Orders []models.Order `json:"orders"`
}

// orderGetData wraps a single order response.
type orderGetData struct {
	Order *models.Order `json:"order"`
}

// orderPlaceData wraps a successful order placement response.
type orderPlaceData struct {
	OrderID int64 `json:"orderId"`
}

// orderPreviewData wraps an order preview response.
type orderPreviewData struct {
	Preview *models.PreviewOrder `json:"preview"`
	OrderID *int64               `json:"orderId,omitempty"`
}

// orderCancelData wraps a successful order cancellation response.
type orderCancelData struct {
	OrderID  int64 `json:"orderId"`
	Canceled bool  `json:"canceled"`
}

// orderReplaceData wraps a successful order replacement response.
type orderReplaceData struct {
	OrderID  int64 `json:"orderId"`
	Replaced bool  `json:"replaced"`
}

// orderListOpts holds local flags for the order list subcommand.
type orderListOpts struct {
	Status []string `flag:"status" flagdescr:"Filter by order status (repeatable, use 'all' for unfiltered): WORKING, PENDING_ACTIVATION, FILLED, EXPIRED, CANCELED, REJECTED, etc."`
	From   string   `flag:"from" flagdescr:"Filter by entered time lower bound"`
	To     string   `flag:"to" flagdescr:"Filter by entered time upper bound"`
}

// Attach implements structcli.Options interface.
func (o *orderListOpts) Attach(_ *cobra.Command) error { return nil }

// orderGetOpts holds local flags for the order get subcommand.
type orderGetOpts struct {
	OrderID string `flag:"order-id" flagdescr:"Order ID"`
}

// Attach implements structcli.Options interface.
func (o *orderGetOpts) Attach(_ *cobra.Command) error { return nil }

// orderPlaceOpts holds local flags for top-level spec-based order placement.
type orderPlaceOpts struct {
	Spec    string `flag:"spec" flagdescr:"Inline JSON, @file, or - for stdin"`
	Confirm bool   `flag:"confirm" flagdescr:"Confirm order placement"`
}

// Attach implements structcli.Options interface.
func (o *orderPlaceOpts) Attach(_ *cobra.Command) error { return nil }

// orderPreviewOpts holds local flags for order preview.
type orderPreviewOpts struct {
	Spec string `flag:"spec" flagdescr:"Inline JSON, @file, or - for stdin"`
}

// Attach implements structcli.Options interface.
func (o *orderPreviewOpts) Attach(_ *cobra.Command) error { return nil }

// orderCancelOpts holds local flags for order cancellation.
type orderCancelOpts struct {
	OrderID string `flag:"order-id" flagdescr:"Order ID"`
	Confirm bool   `flag:"confirm" flagdescr:"Confirm cancellation"`
}

// Attach implements structcli.Options interface.
func (o *orderCancelOpts) Attach(_ *cobra.Command) error { return nil }

// orderReplaceOpts holds local flags for order replacement.
type orderReplaceOpts struct {
	OrderID string `flag:"order-id" flagdescr:"Order ID"`
	Confirm bool   `flag:"confirm" flagdescr:"Confirm replacement"`
}

// Attach implements structcli.Options interface.
func (o *orderReplaceOpts) Attach(_ *cobra.Command) error { return nil }

// equityPlaceOpts holds flags shared by equity place, build, and replace flows.
type equityPlaceOpts struct {
	Symbol             string  `flag:"symbol" flagdescr:"Equity symbol"`
	Action             string  `flag:"action" flagdescr:"Order action"`
	Quantity           float64 `flag:"quantity" flagdescr:"Share quantity"`
	Type               string  `flag:"type" flagdescr:"Order type"`
	Price              float64 `flag:"price" flagdescr:"Limit price"`
	StopPrice          float64 `flag:"stop-price" flagdescr:"Stop price"`
	StopOffset         float64 `flag:"stop-offset" flagdescr:"Trailing stop offset amount"`
	StopLinkBasis      string  `flag:"stop-link-basis" flagdescr:"Trailing stop reference price (LAST, BID, ASK, MARK)"`
	StopLinkType       string  `flag:"stop-link-type" flagdescr:"Trailing stop offset type (VALUE, PERCENT, TICK)"`
	StopType           string  `flag:"stop-type" flagdescr:"Trailing stop trigger type (STANDARD, BID, ASK, LAST, MARK)"`
	ActivationPrice    float64 `flag:"activation-price" flagdescr:"Price that activates the trailing stop"`
	Duration           string  `flag:"duration" flagdescr:"Order duration"`
	Session            string  `flag:"session" flagdescr:"Trading session"`
	SpecialInstruction string  `flag:"special-instruction" flagdescr:"Special instruction (ALL_OR_NONE, DO_NOT_REDUCE, ALL_OR_NONE_DO_NOT_REDUCE)"`
	Destination        string  `flag:"destination" flagdescr:"Order routing destination (INET, ECN_ARCA, CBOE, AMEX, PHLX, ISE, BOX, NYSE, NASDAQ, BATS, C2, AUTO)"`
	PriceLinkBasis     string  `flag:"price-link-basis" flagdescr:"Price link reference price (MANUAL, BASE, TRIGGER, LAST, BID, ASK, ASK_BID, MARK, AVERAGE)"`
	PriceLinkType      string  `flag:"price-link-type" flagdescr:"Price link offset type (VALUE, PERCENT, TICK)"`
}

// Attach implements structcli.Options interface.
func (o *equityPlaceOpts) Attach(_ *cobra.Command) error { return nil }

// optionPlaceOpts holds flags shared by option place and build flows.
type optionPlaceOpts struct {
	Underlying         string  `flag:"underlying" flagdescr:"Underlying symbol"`
	Expiration         string  `flag:"expiration" flagdescr:"Expiration date (YYYY-MM-DD)"`
	Strike             float64 `flag:"strike" flagdescr:"Strike price"`
	Call               bool    `flag:"call" flagdescr:"Call option"`
	Put                bool    `flag:"put" flagdescr:"Put option"`
	Action             string  `flag:"action" flagdescr:"Order action"`
	Quantity           float64 `flag:"quantity" flagdescr:"Contract quantity"`
	Type               string  `flag:"type" flagdescr:"Order type"`
	Price              float64 `flag:"price" flagdescr:"Limit price"`
	Duration           string  `flag:"duration" flagdescr:"Order duration"`
	Session            string  `flag:"session" flagdescr:"Trading session"`
	SpecialInstruction string  `flag:"special-instruction" flagdescr:"Special instruction (ALL_OR_NONE, DO_NOT_REDUCE, ALL_OR_NONE_DO_NOT_REDUCE)"`
	Destination        string  `flag:"destination" flagdescr:"Order routing destination (INET, ECN_ARCA, CBOE, AMEX, PHLX, ISE, BOX, NYSE, NASDAQ, BATS, C2, AUTO)"`
	PriceLinkBasis     string  `flag:"price-link-basis" flagdescr:"Price link reference price (MANUAL, BASE, TRIGGER, LAST, BID, ASK, ASK_BID, MARK, AVERAGE)"`
	PriceLinkType      string  `flag:"price-link-type" flagdescr:"Price link offset type (VALUE, PERCENT, TICK)"`
}

// Attach implements structcli.Options interface.
func (o *optionPlaceOpts) Attach(_ *cobra.Command) error { return nil }

// bracketPlaceOpts holds flags shared by bracket place and build flows.
type bracketPlaceOpts struct {
	Symbol     string  `flag:"symbol" flagdescr:"Equity symbol"`
	Action     string  `flag:"action" flagdescr:"Order action"`
	Quantity   float64 `flag:"quantity" flagdescr:"Share quantity"`
	Type       string  `flag:"type" flagdescr:"Entry order type"`
	Price      float64 `flag:"price" flagdescr:"Entry price"`
	TakeProfit float64 `flag:"take-profit" flagdescr:"Take-profit exit price"`
	StopLoss   float64 `flag:"stop-loss" flagdescr:"Stop-loss exit price"`
	Duration   string  `flag:"duration" flagdescr:"Order duration"`
	Session    string  `flag:"session" flagdescr:"Trading session"`
}

// Attach implements structcli.Options interface.
func (o *bracketPlaceOpts) Attach(_ *cobra.Command) error { return nil }

// ocoPlaceOpts holds flags shared by OCO place and build flows.
type ocoPlaceOpts struct {
	Symbol     string  `flag:"symbol" flagdescr:"Equity symbol"`
	Action     string  `flag:"action" flagdescr:"Exit action (SELL to close long, BUY to close short)"`
	Quantity   float64 `flag:"quantity" flagdescr:"Share quantity"`
	TakeProfit float64 `flag:"take-profit" flagdescr:"Take-profit exit price (limit order)"`
	StopLoss   float64 `flag:"stop-loss" flagdescr:"Stop-loss exit price (stop order)"`
	Duration   string  `flag:"duration" flagdescr:"Order duration"`
	Session    string  `flag:"session" flagdescr:"Trading session"`
}

// Attach implements structcli.Options interface.
func (o *ocoPlaceOpts) Attach(_ *cobra.Command) error { return nil }

// verticalBuildOpts holds flags for vertical spread build flows.
type verticalBuildOpts struct {
	Underlying  string  `flag:"underlying" flagdescr:"Underlying symbol"`
	Expiration  string  `flag:"expiration" flagdescr:"Expiration date (YYYY-MM-DD)"`
	LongStrike  float64 `flag:"long-strike" flagdescr:"Strike price of the option being bought"`
	ShortStrike float64 `flag:"short-strike" flagdescr:"Strike price of the option being sold"`
	Call        bool    `flag:"call" flagdescr:"Call spread"`
	Put         bool    `flag:"put" flagdescr:"Put spread"`
	Open        bool    `flag:"open" flagdescr:"Opening position"`
	Close       bool    `flag:"close" flagdescr:"Closing position"`
	Quantity    float64 `flag:"quantity" flagdescr:"Number of contracts"`
	Price       float64 `flag:"price" flagdescr:"Net debit or credit amount"`
	Duration    string  `flag:"duration" flagdescr:"Order duration"`
	Session     string  `flag:"session" flagdescr:"Trading session"`
}

// Attach implements structcli.Options interface.
func (o *verticalBuildOpts) Attach(_ *cobra.Command) error { return nil }

// ironCondorBuildOpts holds flags for iron condor build flows.
type ironCondorBuildOpts struct {
	Underlying      string  `flag:"underlying" flagdescr:"Underlying symbol"`
	Expiration      string  `flag:"expiration" flagdescr:"Expiration date (YYYY-MM-DD)"`
	PutLongStrike   float64 `flag:"put-long-strike" flagdescr:"Lowest strike: put being bought (protection)"`
	PutShortStrike  float64 `flag:"put-short-strike" flagdescr:"Put being sold (premium)"`
	CallShortStrike float64 `flag:"call-short-strike" flagdescr:"Call being sold (premium)"`
	CallLongStrike  float64 `flag:"call-long-strike" flagdescr:"Highest strike: call being bought (protection)"`
	Open            bool    `flag:"open" flagdescr:"Opening position"`
	Close           bool    `flag:"close" flagdescr:"Closing position"`
	Quantity        float64 `flag:"quantity" flagdescr:"Number of contracts"`
	Price           float64 `flag:"price" flagdescr:"Net credit or debit amount"`
	Duration        string  `flag:"duration" flagdescr:"Order duration"`
	Session         string  `flag:"session" flagdescr:"Trading session"`
}

// Attach implements structcli.Options interface.
func (o *ironCondorBuildOpts) Attach(_ *cobra.Command) error { return nil }

// strangleBuildOpts holds flags for strangle build flows.
type strangleBuildOpts struct {
	Underlying string  `flag:"underlying" flagdescr:"Underlying symbol"`
	Expiration string  `flag:"expiration" flagdescr:"Expiration date (YYYY-MM-DD)"`
	CallStrike float64 `flag:"call-strike" flagdescr:"Strike price for the call leg"`
	PutStrike  float64 `flag:"put-strike" flagdescr:"Strike price for the put leg"`
	Buy        bool    `flag:"buy" flagdescr:"Buy the strangle (long, net debit)"`
	Sell       bool    `flag:"sell" flagdescr:"Sell the strangle (short, net credit)"`
	Open       bool    `flag:"open" flagdescr:"Opening position"`
	Close      bool    `flag:"close" flagdescr:"Closing position"`
	Quantity   float64 `flag:"quantity" flagdescr:"Number of contracts"`
	Price      float64 `flag:"price" flagdescr:"Net debit or credit amount"`
	Duration   string  `flag:"duration" flagdescr:"Order duration"`
	Session    string  `flag:"session" flagdescr:"Trading session"`
}

// Attach implements structcli.Options interface.
func (o *strangleBuildOpts) Attach(_ *cobra.Command) error { return nil }

// straddleBuildOpts holds flags for straddle build flows.
type straddleBuildOpts struct {
	Underlying string  `flag:"underlying" flagdescr:"Underlying symbol"`
	Expiration string  `flag:"expiration" flagdescr:"Expiration date (YYYY-MM-DD)"`
	Strike     float64 `flag:"strike" flagdescr:"Strike price (shared by call and put legs)"`
	Buy        bool    `flag:"buy" flagdescr:"Buy the straddle (long, net debit)"`
	Sell       bool    `flag:"sell" flagdescr:"Sell the straddle (short, net credit)"`
	Open       bool    `flag:"open" flagdescr:"Opening position"`
	Close      bool    `flag:"close" flagdescr:"Closing position"`
	Quantity   float64 `flag:"quantity" flagdescr:"Number of contracts"`
	Price      float64 `flag:"price" flagdescr:"Net debit or credit amount"`
	Duration   string  `flag:"duration" flagdescr:"Order duration"`
	Session    string  `flag:"session" flagdescr:"Trading session"`
}

// Attach implements structcli.Options interface.
func (o *straddleBuildOpts) Attach(_ *cobra.Command) error { return nil }

// coveredCallBuildOpts holds flags for covered call build flows.
type coveredCallBuildOpts struct {
	Underlying string  `flag:"underlying" flagdescr:"Underlying symbol"`
	Expiration string  `flag:"expiration" flagdescr:"Expiration date (YYYY-MM-DD)"`
	Strike     float64 `flag:"strike" flagdescr:"Call strike price"`
	Quantity   float64 `flag:"quantity" flagdescr:"Number of contracts (1 contract = 100 shares)"`
	Price      float64 `flag:"price" flagdescr:"Net debit amount"`
	Duration   string  `flag:"duration" flagdescr:"Order duration"`
	Session    string  `flag:"session" flagdescr:"Trading session"`
}

// Attach implements structcli.Options interface.
func (o *coveredCallBuildOpts) Attach(_ *cobra.Command) error { return nil }

// collarBuildOpts holds flags for collar-with-stock build flows.
type collarBuildOpts struct {
	Underlying string  `flag:"underlying" flagdescr:"Underlying symbol"`
	PutStrike  float64 `flag:"put-strike" flagdescr:"Protective put strike price"`
	CallStrike float64 `flag:"call-strike" flagdescr:"Covered call strike price"`
	Expiration string  `flag:"expiration" flagdescr:"Expiration date for both options (YYYY-MM-DD)"`
	Quantity   float64 `flag:"quantity" flagdescr:"Number of contracts (1 contract = 100 shares)"`
	Open       bool    `flag:"open" flagdescr:"Opening position"`
	Close      bool    `flag:"close" flagdescr:"Closing position"`
	Price      float64 `flag:"price" flagdescr:"Net debit amount"`
	Duration   string  `flag:"duration" flagdescr:"Order duration"`
	Session    string  `flag:"session" flagdescr:"Trading session"`
}

// Attach implements structcli.Options interface.
func (o *collarBuildOpts) Attach(_ *cobra.Command) error { return nil }

// calendarBuildOpts holds flags for calendar spread build flows.
type calendarBuildOpts struct {
	Underlying     string  `flag:"underlying" flagdescr:"Underlying symbol"`
	NearExpiration string  `flag:"near-expiration" flagdescr:"Near-term expiration date (YYYY-MM-DD)"`
	FarExpiration  string  `flag:"far-expiration" flagdescr:"Far-term expiration date (YYYY-MM-DD)"`
	Strike         float64 `flag:"strike" flagdescr:"Strike price (shared by both legs)"`
	Call           bool    `flag:"call" flagdescr:"Call calendar spread"`
	Put            bool    `flag:"put" flagdescr:"Put calendar spread"`
	Open           bool    `flag:"open" flagdescr:"Opening position"`
	Close          bool    `flag:"close" flagdescr:"Closing position"`
	Quantity       float64 `flag:"quantity" flagdescr:"Number of contracts"`
	Price          float64 `flag:"price" flagdescr:"Net debit amount"`
	Duration       string  `flag:"duration" flagdescr:"Order duration"`
	Session        string  `flag:"session" flagdescr:"Trading session"`
}

// Attach implements structcli.Options interface.
func (o *calendarBuildOpts) Attach(_ *cobra.Command) error { return nil }

// diagonalBuildOpts holds flags for diagonal spread build flows.
type diagonalBuildOpts struct {
	Underlying     string  `flag:"underlying" flagdescr:"Underlying symbol"`
	NearExpiration string  `flag:"near-expiration" flagdescr:"Near-term expiration date (YYYY-MM-DD)"`
	FarExpiration  string  `flag:"far-expiration" flagdescr:"Far-term expiration date (YYYY-MM-DD)"`
	NearStrike     float64 `flag:"near-strike" flagdescr:"Strike price for the near-term (sold) leg"`
	FarStrike      float64 `flag:"far-strike" flagdescr:"Strike price for the far-term (bought) leg"`
	Call           bool    `flag:"call" flagdescr:"Call diagonal spread"`
	Put            bool    `flag:"put" flagdescr:"Put diagonal spread"`
	Open           bool    `flag:"open" flagdescr:"Opening position"`
	Close          bool    `flag:"close" flagdescr:"Closing position"`
	Quantity       float64 `flag:"quantity" flagdescr:"Number of contracts"`
	Price          float64 `flag:"price" flagdescr:"Net debit amount"`
	Duration       string  `flag:"duration" flagdescr:"Order duration"`
	Session        string  `flag:"session" flagdescr:"Trading session"`
}

// Attach implements structcli.Options interface.
func (o *diagonalBuildOpts) Attach(_ *cobra.Command) error { return nil }

const confirmOrderMessage = "Add --confirm to execute this order"

const mutableDisabledMessage = `Mutable operations are disabled by default. ` +
	`Set "i-also-like-to-live-dangerously": true in your config file to enable order placement, cancellation, and replacement.`

// NewOrderCmd returns the Cobra command for order operations.
func NewOrderCmd(c *client.Ref, configPath string, w io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "order",
		Short: "List, build, preview, place, cancel, and replace orders",
		Long: `Manage orders across your Schwab accounts. Supports listing, viewing, placing,
previewing, building, canceling, and replacing orders. Placing, canceling, and
replacing orders require both the "i-also-like-to-live-dangerously" config flag
and --confirm on each command. Duration aliases GTC, FOK, and IOC are accepted.`,
		GroupID: "trading",
		RunE:    requireSubcommand,
	}

	cmd.SetFlagErrorFunc(suggestSubcommands)
	cmd.AddCommand(newOrderListCmd(c, configPath, w))
	cmd.AddCommand(newOrderGetCmd(c, configPath, w))
	cmd.AddCommand(newOrderPlaceCmd(c, configPath, w))
	cmd.AddCommand(newOrderPreviewCmd(c, configPath, w))
	cmd.AddCommand(newOrderBuildCmd(w))
	cmd.AddCommand(newOrderCancelCmd(c, configPath, w))
	cmd.AddCommand(newOrderReplaceCmd(c, configPath, w))

	return cmd
}

// terminalOrderStatuses are order statuses that represent completed/final states.
// Orders in these statuses are filtered out by default to show only actionable
// orders. Use --status all to include them.
var terminalOrderStatuses = map[models.OrderStatus]bool{
	models.OrderStatusFilled:   true,
	models.OrderStatusCanceled: true,
	models.OrderStatusRejected: true,
	models.OrderStatusExpired:  true,
	models.OrderStatusReplaced: true,
}

// filterNonTerminalOrders returns only orders whose status is not terminal.
// Orders with a nil Status are included (conservative: don't hide unknowns).
func filterNonTerminalOrders(orders []models.Order) []models.Order {
	filtered := make([]models.Order, 0, len(orders))
	for i := range orders {
		if orders[i].Status == nil || !terminalOrderStatuses[*orders[i].Status] {
			filtered = append(filtered, orders[i])
		}
	}
	return filtered
}

// newOrderListCmd lists orders for a specific account or all accounts.
// By default, terminal statuses are filtered out to show only actionable orders.
func newOrderListCmd(c *client.Ref, _ string, w io.Writer) *cobra.Command {
	opts := &orderListOpts{}
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List orders (defaults to non-terminal statuses)",
		Long: `List orders for the current account, or all accounts when no --account is set.
By default, terminal statuses (FILLED, CANCELED, REJECTED, EXPIRED, REPLACED)
are filtered out to show only actionable orders. Use --status all to see
everything. Multiple --status values fan out into separate API calls with
merged, deduplicated results.`,
		Example: `  schwab-agent order list
  schwab-agent order list --status all
  schwab-agent order list --status WORKING --status PENDING_ACTIVATION
  schwab-agent order list --status WORKING,FILLED,EXPIRED
  schwab-agent order list --from 2025-01-01 --to 2025-01-31`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := structcli.Unmarshal(cmd, opts); err != nil {
				return err
			}

			var statuses []string
			for _, raw := range opts.Status {
				for part := range strings.SplitSeq(raw, ",") {
					trimmed := strings.TrimSpace(part)
					if trimmed != "" {
						statuses = append(statuses, trimmed)
					}
				}
			}

			showAll := false
			for _, s := range statuses {
				if strings.EqualFold(s, "all") {
					showAll = true
					break
				}
			}

			var apiStatuses []string
			if !showAll {
				apiStatuses = statuses
			}

			params := client.OrderListParams{
				Statuses:        apiStatuses,
				FromEnteredTime: strings.TrimSpace(opts.From),
				ToEnteredTime:   strings.TrimSpace(opts.To),
			}

			account, err := cmd.Flags().GetString("account")
			if err != nil {
				return err
			}
			account = strings.TrimSpace(account)

			var orders []models.Order
			if account == "" {
				orders, err = c.AllOrders(cmd.Context(), params)
			} else {
				orders, err = c.ListOrders(cmd.Context(), account, params)
			}
			if err != nil {
				return err
			}

			if len(statuses) == 0 {
				orders = filterNonTerminalOrders(orders)
			}

			return output.WriteSuccess(w, orderListData{Orders: orders}, output.NewMetadata())
		},
	}

	if err := structcli.Define(cmd, opts); err != nil {
		panic(err)
	}

	return cmd
}

// newOrderGetCmd returns a single order by account and ID.
func newOrderGetCmd(c *client.Ref, configPath string, w io.Writer) *cobra.Command {
	opts := &orderGetOpts{}
	cmd := &cobra.Command{
		Use:   "get [order-id]",
		Short: "Get an order by ID",
		Long: `Retrieve a single order by its ID. The order ID can be passed as a positional
argument or with the --order-id flag. When both are provided, the flag takes
priority. Requires a default account or --account flag.`,
		Example: `  schwab-agent order get 1234567890
  schwab-agent order get --order-id 1234567890`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := structcli.Unmarshal(cmd, opts); err != nil {
				return err
			}

			orderID, err := parseRequiredOrderID(opts.OrderID, args)
			if err != nil {
				return err
			}

			accountFlag, err := cmd.Flags().GetString("account")
			if err != nil {
				return err
			}

			account, err := resolveAccount(accountFlag, configPath, nil)
			if err != nil {
				return err
			}

			order, err := c.GetOrder(cmd.Context(), account, orderID)
			if err != nil {
				return err
			}

			return output.WriteSuccess(w, orderGetData{Order: order}, output.NewMetadata())
		},
	}

	if err := structcli.Define(cmd, opts); err != nil {
		panic(err)
	}

	return cmd
}

// newOrderPlaceCmd places new orders from either flags or a JSON spec.
func newOrderPlaceCmd(c *client.Ref, configPath string, w io.Writer) *cobra.Command {
	opts := &orderPlaceOpts{}
	cmd := &cobra.Command{
		Use:   "place",
		Short: "Place an order",
		Long: `Place an order via subcommand (equity, option, bracket, oco) or from a JSON spec
with --spec. Both safety guards are required: set "i-also-like-to-live-dangerously"
to true in config.json, and pass --confirm on every placement. Recommended workflow:
check the price with quote get, build the order JSON with order build, preview it
with order preview, then place with --confirm.`,
		Example: `  # Place from a JSON file
  schwab-agent order place --spec @order.json --confirm
  # Place from stdin (piped from order build)
  schwab-agent order build equity --symbol AAPL --action BUY --quantity 10 --type LIMIT --price 200 | schwab-agent order place --spec - --confirm
  # Place from inline JSON
  schwab-agent order place --spec '{"orderType":"LIMIT",...}' --confirm`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := structcli.Unmarshal(cmd, opts); err != nil {
				return err
			}

			if strings.TrimSpace(opts.Spec) == "" {
				return newValidationError("spec is required for `order place` without a subcommand")
			}

			configFlag, err := cmd.Flags().GetString("config")
			if err != nil {
				return err
			}
			if configFlag == "" {
				configFlag = configPath
			}

			if err := requireMutableEnabled(configFlag); err != nil {
				return err
			}

			if err := requireConfirm(opts.Confirm); err != nil {
				return err
			}

			accountFlag, err := cmd.Flags().GetString("account")
			if err != nil {
				return err
			}

			account, err := resolveAccount(accountFlag, configFlag, nil)
			if err != nil {
				return err
			}

			order, err := parseSpecOrder(cmd, opts.Spec)
			if err != nil {
				return err
			}

			response, err := c.PlaceOrder(cmd.Context(), account, order)
			if err != nil {
				return err
			}

			return output.WriteSuccess(w, orderPlaceData{OrderID: response.OrderID}, output.NewMetadata())
		},
	}

	cmd.SetFlagErrorFunc(suggestSubcommands)
	if err := structcli.Define(cmd, opts); err != nil {
		panic(err)
	}

	equityCmd := makeCobraPlaceOrderCommand(c, configPath, w, "equity", "Place an equity order", func() *equityPlaceOpts { return &equityPlaceOpts{} }, equityOrderFlagSetup, parseEquityParams, orderbuilder.ValidateEquityOrder, orderbuilder.BuildEquityOrder)
	equityCmd.Long = `Place an equity (stock) order. Supports MARKET, LIMIT, STOP, STOP_LIMIT, and
TRAILING_STOP order types. Default type is MARKET if --type is omitted. Duration
aliases GTC, FOK, and IOC are accepted alongside their full names. Both safety
guards are required for placement.`
	equityCmd.Example = `  # Buy 10 shares at market price
  schwab-agent order place equity --symbol AAPL --action BUY --quantity 10 --confirm
  # Buy with a limit price, good till cancel
  schwab-agent order place equity --symbol AAPL --action BUY --quantity 10 --type LIMIT --price 150 --duration GTC --confirm
  # Sell with a trailing stop ($2.50 offset)
  schwab-agent order place equity --symbol AAPL --action SELL --quantity 10 --type TRAILING_STOP --stop-offset 2.50 --confirm
  # Sell with a stop-limit order
  schwab-agent order place equity --symbol AAPL --action SELL --quantity 10 --type STOP_LIMIT --stop-price 145 --price 144 --confirm`

	optionCmd := makeCobraPlaceOrderCommand(c, configPath, w, "option", "Place an option order", func() *optionPlaceOpts { return &optionPlaceOpts{} }, optionOrderFlagSetup, parseOptionParams, orderbuilder.ValidateOptionOrder, orderbuilder.BuildOptionOrder)
	optionCmd.Long = `Place a single-leg option order. Requires --underlying, --expiration, --strike,
and exactly one of --call or --put. Use BUY_TO_OPEN/SELL_TO_CLOSE for new
positions and SELL_TO_OPEN/BUY_TO_CLOSE for existing ones. Both safety guards
are required for placement.`
	optionCmd.Example = `  # Buy a call option to open
  schwab-agent order place option --underlying AAPL --expiration 2025-06-20 --strike 200 --call --action BUY_TO_OPEN --quantity 1 --confirm
  # Sell a put at a limit price
  schwab-agent order place option --underlying AAPL --expiration 2025-06-20 --strike 190 --put --action SELL_TO_OPEN --quantity 1 --type LIMIT --price 3.50 --confirm
  # Close an existing call position
  schwab-agent order place option --underlying AAPL --expiration 2025-06-20 --strike 200 --call --action SELL_TO_CLOSE --quantity 1 --confirm`

	bracketCmd := makeCobraPlaceOrderCommand(c, configPath, w, "bracket", "Place a bracket order", func() *bracketPlaceOpts { return &bracketPlaceOpts{} }, bracketOrderFlagSetup, parseBracketParams, orderbuilder.ValidateBracketOrder, orderbuilder.BuildBracketOrder)
	bracketCmd.Long = `Place a bracket order that combines an entry trade with automatic exit orders.
At least one of --take-profit or --stop-loss is required. Exit instructions are
auto-inverted from the entry action (BUY entry creates SELL exits). Canceling
the parent cascades to all child orders.`
	bracketCmd.Example = `  # Buy with both take-profit and stop-loss
  schwab-agent order place bracket --symbol NVDA --action BUY --quantity 10 --type MARKET --take-profit 150 --stop-loss 120 --confirm
  # Buy with only a stop-loss safety net
  schwab-agent order place bracket --symbol AAPL --action BUY --quantity 10 --type LIMIT --price 180 --stop-loss 170 --confirm
  # Buy with only a take-profit target
  schwab-agent order place bracket --symbol TSLA --action BUY --quantity 5 --type MARKET --take-profit 300 --confirm`

	ocoCmd := makeCobraPlaceOrderCommand(c, configPath, w, "oco", "Place a one-cancels-other order for an existing position", func() *ocoPlaceOpts { return &ocoPlaceOpts{} }, ocoOrderFlagSetup, parseOCOParams, orderbuilder.ValidateOCOOrder, orderbuilder.BuildOCOOrder)
	ocoCmd.Long = `Place a one-cancels-other order for an existing position. When one exit fills,
the other is automatically canceled. At least one of --take-profit or --stop-loss
is required. For long positions use SELL; for short positions use BUY. Unlike
bracket orders, OCO has no entry leg.`
	ocoCmd.Example = `  # Set take-profit and stop-loss for a long position
  schwab-agent order place oco --symbol AAPL --action SELL --quantity 100 --take-profit 160 --stop-loss 140 --confirm
  # Protect a position with only a stop-loss
  schwab-agent order place oco --symbol AAPL --action SELL --quantity 50 --stop-loss 140 --confirm
  # Close a short position with exits
  schwab-agent order place oco --symbol TSLA --action BUY --quantity 10 --take-profit 200 --stop-loss 250 --confirm`

	cmd.AddCommand(equityCmd, optionCmd, bracketCmd, ocoCmd)

	return cmd
}

// makeCobraPlaceOrderCommand creates a Cobra place subcommand with the same
// parse/validate/build/place pipeline as the legacy generic factory.
func makeCobraPlaceOrderCommand[O any, P any](
	c *client.Ref,
	configPath string,
	w io.Writer,
	name, usage string,
	newOpts func() *O,
	flagSetup func(*cobra.Command, *O),
	parse func(*O, []string) (*P, error),
	validate func(*P) error,
	build func(*P) (*models.OrderRequest, error),
) *cobra.Command {
	opts := newOpts()
	var confirm bool
	cmd := &cobra.Command{
		Use:   name,
		Short: usage,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := structcli.Unmarshal(cmd, any(opts).(structcli.Options)); err != nil {
				return err
			}

			configFlag, err := cmd.Flags().GetString("config")
			if err != nil {
				return err
			}
			if configFlag == "" {
				configFlag = configPath
			}

			if err := requireMutableEnabled(configFlag); err != nil {
				return err
			}

			if err := requireConfirm(confirm); err != nil {
				return err
			}

			accountFlag, err := cmd.Flags().GetString("account")
			if err != nil {
				return err
			}

			account, err := resolveAccount(accountFlag, configFlag, nil)
			if err != nil {
				return err
			}

			params, err := parse(opts, args)
			if err != nil {
				return err
			}

			if err := validate(params); err != nil {
				return err
			}

			order, err := build(params)
			if err != nil {
				return err
			}

			response, err := c.PlaceOrder(cmd.Context(), account, order)
			if err != nil {
				return err
			}

			return output.WriteSuccess(w, orderPlaceData{OrderID: response.OrderID}, output.NewMetadata())
		},
	}

	if flagSetup != nil {
		flagSetup(cmd, opts)
	}
	cmd.Flags().BoolVar(&confirm, "confirm", false, "Confirm order placement")

	return cmd
}

// newOrderPreviewCmd previews an order from a JSON spec.
func newOrderPreviewCmd(c *client.Ref, configPath string, w io.Writer) *cobra.Command {
	opts := &orderPreviewOpts{}
	cmd := &cobra.Command{
		Use:   "preview",
		Short: "Preview an order from JSON spec",
		Long: `Preview an order from a JSON spec without placing it. Returns estimated
commissions, fees, and order details. Pipe output from order build for a
build-then-preview workflow. Does not require safety guards since no order
is actually placed.`,
		Example: `  schwab-agent order preview --spec @order.json
  schwab-agent order build equity --symbol AAPL --action BUY --quantity 10 --type LIMIT --price 200 | schwab-agent order preview --spec -`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := structcli.Unmarshal(cmd, opts); err != nil {
				return err
			}

			if strings.TrimSpace(opts.Spec) == "" {
				return newValidationError("spec is required")
			}

			accountFlag, err := cmd.Flags().GetString("account")
			if err != nil {
				return err
			}

			account, err := resolveAccount(accountFlag, configPath, nil)
			if err != nil {
				return err
			}

			order, err := parseSpecOrder(cmd, opts.Spec)
			if err != nil {
				return err
			}

			preview, err := c.PreviewOrder(cmd.Context(), account, order)
			if err != nil {
				return err
			}

			return output.WriteSuccess(w, orderPreviewData{Preview: preview, OrderID: preview.OrderID}, output.NewMetadata())
		},
	}

	if err := structcli.Define(cmd, opts); err != nil {
		panic(err)
	}

	return cmd
}

// newOrderCancelCmd cancels an existing order.
func newOrderCancelCmd(c *client.Ref, configPath string, w io.Writer) *cobra.Command {
	opts := &orderCancelOpts{}
	cmd := &cobra.Command{
		Use:   "cancel [order-id]",
		Short: "Cancel an order",
		Long: `Cancel an existing order by ID. Requires both safety guards: the
"i-also-like-to-live-dangerously" config flag and --confirm. The order ID can
be passed as a positional argument or with --order-id (flag takes priority).`,
		Example: `  schwab-agent order cancel 1234567890 --confirm
  schwab-agent order cancel --order-id 1234567890 --confirm`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := structcli.Unmarshal(cmd, opts); err != nil {
				return err
			}

			configFlag, err := cmd.Flags().GetString("config")
			if err != nil {
				return err
			}
			if configFlag == "" {
				configFlag = configPath
			}

			if err := requireMutableEnabled(configFlag); err != nil {
				return err
			}

			if err := requireConfirm(opts.Confirm); err != nil {
				return err
			}

			orderID, err := parseRequiredOrderID(opts.OrderID, args)
			if err != nil {
				return err
			}

			accountFlag, err := cmd.Flags().GetString("account")
			if err != nil {
				return err
			}

			account, err := resolveAccount(accountFlag, configFlag, nil)
			if err != nil {
				return err
			}

			if err := c.CancelOrder(cmd.Context(), account, orderID); err != nil {
				return err
			}

			return output.WriteSuccess(w, orderCancelData{OrderID: orderID, Canceled: true}, output.NewMetadata())
		},
	}

	if err := structcli.Define(cmd, opts); err != nil {
		panic(err)
	}

	return cmd
}

// newOrderReplaceCmd replaces an existing order with a new equity order payload.
func newOrderReplaceCmd(c *client.Ref, configPath string, w io.Writer) *cobra.Command {
	opts := &orderReplaceOpts{}
	equityOpts := &equityPlaceOpts{}
	cmd := &cobra.Command{
		Use:   "replace [order-id]",
		Short: "Replace an order with a new equity order spec",
		Long: `Replace an existing order with a new equity order. The original order is
canceled and a new one is created with a new ID. Only equity order flags are
accepted. Requires both safety guards. The original order status becomes
REPLACED after the new order is created.`,
		Example: `  schwab-agent order replace 1234567890 --symbol AAPL --action BUY --quantity 10 --type LIMIT --price 155.00 --duration DAY --confirm
  schwab-agent order replace --order-id 1234567890 --symbol AAPL --action BUY --quantity 10 --type LIMIT --price 155.00 --duration DAY --confirm`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := structcli.Unmarshal(cmd, opts); err != nil {
				return err
			}
			if err := structcli.Unmarshal(cmd, equityOpts); err != nil {
				return err
			}

			configFlag, err := cmd.Flags().GetString("config")
			if err != nil {
				return err
			}
			if configFlag == "" {
				configFlag = configPath
			}

			if err := requireMutableEnabled(configFlag); err != nil {
				return err
			}

			if err := requireConfirm(opts.Confirm); err != nil {
				return err
			}

			orderID, err := parseRequiredOrderID(opts.OrderID, args)
			if err != nil {
				return err
			}

			accountFlag, err := cmd.Flags().GetString("account")
			if err != nil {
				return err
			}

			account, err := resolveAccount(accountFlag, configFlag, nil)
			if err != nil {
				return err
			}

			params, err := parseEquityParams(equityOpts, args)
			if err != nil {
				return err
			}

			if err := orderbuilder.ValidateEquityOrder(params); err != nil {
				return err
			}

			order, err := orderbuilder.BuildEquityOrder(params)
			if err != nil {
				return err
			}

			if err := c.ReplaceOrder(cmd.Context(), account, orderID, order); err != nil {
				return err
			}

			return output.WriteSuccess(w, orderReplaceData{OrderID: orderID, Replaced: true}, output.NewMetadata())
		},
	}

	equityOrderFlagSetup(cmd, equityOpts)
	if err := structcli.Define(cmd, opts); err != nil {
		panic(err)
	}

	return cmd
}

// equityOrderFlagSetup registers equity order flags on cmd.
func equityOrderFlagSetup(cmd *cobra.Command, opts *equityPlaceOpts) {
	if err := structcli.Define(cmd, opts); err != nil {
		panic(err)
	}
}

// optionOrderFlagSetup registers option order flags on cmd.
func optionOrderFlagSetup(cmd *cobra.Command, opts *optionPlaceOpts) {
	if err := structcli.Define(cmd, opts); err != nil {
		panic(err)
	}
	cmd.MarkFlagsMutuallyExclusive("call", "put")
	cmd.MarkFlagsOneRequired("call", "put")
}

// bracketOrderFlagSetup registers bracket order flags on cmd.
func bracketOrderFlagSetup(cmd *cobra.Command, opts *bracketPlaceOpts) {
	if err := structcli.Define(cmd, opts); err != nil {
		panic(err)
	}
}

// ocoOrderFlagSetup registers standalone OCO order flags on cmd.
func ocoOrderFlagSetup(cmd *cobra.Command, opts *ocoPlaceOpts) {
	if err := structcli.Define(cmd, opts); err != nil {
		panic(err)
	}
}

// parseOCOParams converts command flags into standalone OCO builder params.
func parseOCOParams(opts *ocoPlaceOpts, _ []string) (*orderbuilder.OCOParams, error) {
	action, err := parseInstruction(opts.Action)
	if err != nil {
		return nil, err
	}

	duration, session, err := parseDurationSession(opts.Duration, opts.Session)
	if err != nil {
		return nil, err
	}

	return &orderbuilder.OCOParams{
		Symbol:     strings.TrimSpace(opts.Symbol),
		Action:     action,
		Quantity:   opts.Quantity,
		TakeProfit: opts.TakeProfit,
		StopLoss:   opts.StopLoss,
		Duration:   duration,
		Session:    session,
	}, nil
}

// verticalOrderFlagSetup registers vertical spread flags on cmd.
func verticalOrderFlagSetup(cmd *cobra.Command, opts *verticalBuildOpts) {
	if err := structcli.Define(cmd, opts); err != nil {
		panic(err)
	}
	cmd.MarkFlagsMutuallyExclusive("call", "put")
	cmd.MarkFlagsOneRequired("call", "put")
	cmd.MarkFlagsMutuallyExclusive("open", "close")
	cmd.MarkFlagsOneRequired("open", "close")
}

// ironCondorOrderFlagSetup registers iron condor flags on cmd.
func ironCondorOrderFlagSetup(cmd *cobra.Command, opts *ironCondorBuildOpts) {
	if err := structcli.Define(cmd, opts); err != nil {
		panic(err)
	}
	cmd.MarkFlagsMutuallyExclusive("open", "close")
	cmd.MarkFlagsOneRequired("open", "close")
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

	duration, session, err := parseDurationSession(opts.Duration, opts.Session)
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
		Duration:        duration,
		Session:         session,
	}, nil
}

// parseEquityParams converts command flags into equity order builder params.
func parseEquityParams(opts *equityPlaceOpts, _ []string) (*orderbuilder.EquityParams, error) {
	action, err := parseInstruction(opts.Action)
	if err != nil {
		return nil, err
	}

	orderType, err := parseOrderType(opts.Type, models.OrderTypeMarket)
	if err != nil {
		return nil, err
	}

	duration, session, err := parseDurationSession(opts.Duration, opts.Session)
	if err != nil {
		return nil, err
	}

	stopLinkBasis, err := parseStopPriceLinkBasis(opts.StopLinkBasis)
	if err != nil {
		return nil, err
	}

	stopLinkType, err := parseStopPriceLinkType(opts.StopLinkType)
	if err != nil {
		return nil, err
	}

	stopType, err := parseStopType(opts.StopType)
	if err != nil {
		return nil, err
	}

	specialInstruction, err := parseSpecialInstruction(opts.SpecialInstruction)
	if err != nil {
		return nil, err
	}

	destination, err := parseDestination(opts.Destination)
	if err != nil {
		return nil, err
	}

	priceLinkBasis, err := parsePriceLinkBasis(opts.PriceLinkBasis)
	if err != nil {
		return nil, err
	}

	priceLinkType, err := parsePriceLinkType(opts.PriceLinkType)
	if err != nil {
		return nil, err
	}

	return &orderbuilder.EquityParams{
		Symbol:             strings.TrimSpace(opts.Symbol),
		Action:             action,
		Quantity:           opts.Quantity,
		OrderType:          orderType,
		Price:              opts.Price,
		StopPrice:          opts.StopPrice,
		StopPriceOffset:    opts.StopOffset,
		StopPriceLinkBasis: stopLinkBasis,
		StopPriceLinkType:  stopLinkType,
		StopType:           stopType,
		ActivationPrice:    opts.ActivationPrice,
		SpecialInstruction: specialInstruction,
		Destination:        destination,
		PriceLinkBasis:     priceLinkBasis,
		PriceLinkType:      priceLinkType,
		Duration:           duration,
		Session:            session,
	}, nil
}

// parseOptionParams converts command flags into option order builder params.
func parseOptionParams(opts *optionPlaceOpts, _ []string) (*orderbuilder.OptionParams, error) {
	action, err := parseInstruction(opts.Action)
	if err != nil {
		return nil, err
	}

	orderType, err := parseOrderType(opts.Type, models.OrderTypeMarket)
	if err != nil {
		return nil, err
	}

	duration, session, err := parseDurationSession(opts.Duration, opts.Session)
	if err != nil {
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

	specialInstruction, err := parseSpecialInstruction(opts.SpecialInstruction)
	if err != nil {
		return nil, err
	}

	destination, err := parseDestination(opts.Destination)
	if err != nil {
		return nil, err
	}

	priceLinkBasis, err := parsePriceLinkBasis(opts.PriceLinkBasis)
	if err != nil {
		return nil, err
	}

	priceLinkType, err := parsePriceLinkType(opts.PriceLinkType)
	if err != nil {
		return nil, err
	}

	return &orderbuilder.OptionParams{
		Underlying:         strings.TrimSpace(opts.Underlying),
		Expiration:         expiration,
		Strike:             opts.Strike,
		PutCall:            putCall,
		Action:             action,
		Quantity:           opts.Quantity,
		OrderType:          orderType,
		Price:              opts.Price,
		SpecialInstruction: specialInstruction,
		Destination:        destination,
		PriceLinkBasis:     priceLinkBasis,
		PriceLinkType:      priceLinkType,
		Duration:           duration,
		Session:            session,
	}, nil
}

// parseBracketParams converts command flags into bracket order builder params.
func parseBracketParams(opts *bracketPlaceOpts, _ []string) (*orderbuilder.BracketParams, error) {
	action, err := parseInstruction(opts.Action)
	if err != nil {
		return nil, err
	}

	orderType, err := parseOrderType(opts.Type, models.OrderTypeMarket)
	if err != nil {
		return nil, err
	}

	duration, session, err := parseDurationSession(opts.Duration, opts.Session)
	if err != nil {
		return nil, err
	}

	return &orderbuilder.BracketParams{
		Symbol:     strings.TrimSpace(opts.Symbol),
		Action:     action,
		Quantity:   opts.Quantity,
		OrderType:  orderType,
		Price:      opts.Price,
		TakeProfit: opts.TakeProfit,
		StopLoss:   opts.StopLoss,
		Duration:   duration,
		Session:    session,
	}, nil
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

// requireConfirm enforces the write-operation safety gate.
func requireConfirm(confirmed bool) error {
	if confirmed {
		return nil
	}

	return apperr.NewValidationError(confirmOrderMessage, nil)
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
)

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

// parseDuration converts CLI input to a duration enum.
// Supports standard trading abbreviations: GTC (GOOD_TILL_CANCEL),
// FOK (FILL_OR_KILL), and IOC (IMMEDIATE_OR_CANCEL).
func parseDuration(raw string) (models.Duration, error) {
	upper := strings.ToUpper(strings.TrimSpace(raw))

	// Resolve common trading abbreviations before standard enum validation.
	// These are universal across brokers and trading platforms.
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

// parseSession converts CLI input to a session enum.
func parseSession(raw string) (models.Session, error) {
	return parseEnum(raw, validSessions, "", "session")
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

	duration, session, err := parseDurationSession(opts.Duration, opts.Session)
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
		Duration:    duration,
		Session:     session,
	}, nil
}

// strangleOrderFlagSetup registers strangle flags on cmd.
func strangleOrderFlagSetup(cmd *cobra.Command, opts *strangleBuildOpts) {
	if err := structcli.Define(cmd, opts); err != nil {
		panic(err)
	}
	cmd.MarkFlagsMutuallyExclusive("buy", "sell")
	cmd.MarkFlagsOneRequired("buy", "sell")
	cmd.MarkFlagsMutuallyExclusive("open", "close")
	cmd.MarkFlagsOneRequired("open", "close")
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

	duration, session, err := parseDurationSession(opts.Duration, opts.Session)
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
		Duration:   duration,
		Session:    session,
	}, nil
}

// straddleOrderFlagSetup registers straddle flags on cmd.
func straddleOrderFlagSetup(cmd *cobra.Command, opts *straddleBuildOpts) {
	if err := structcli.Define(cmd, opts); err != nil {
		panic(err)
	}
	cmd.MarkFlagsMutuallyExclusive("buy", "sell")
	cmd.MarkFlagsOneRequired("buy", "sell")
	cmd.MarkFlagsMutuallyExclusive("open", "close")
	cmd.MarkFlagsOneRequired("open", "close")
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

	duration, session, err := parseDurationSession(opts.Duration, opts.Session)
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
		Duration:   duration,
		Session:    session,
	}, nil
}

// coveredCallOrderFlagSetup registers covered call flags on cmd.
func coveredCallOrderFlagSetup(cmd *cobra.Command, opts *coveredCallBuildOpts) {
	if err := structcli.Define(cmd, opts); err != nil {
		panic(err)
	}
}

// parseCoveredCallParams converts command flags into covered call builder params.
func parseCoveredCallParams(opts *coveredCallBuildOpts, _ []string) (*orderbuilder.CoveredCallParams, error) {
	expiration, err := parseExpiration(opts.Expiration)
	if err != nil {
		return nil, err
	}

	duration, session, err := parseDurationSession(opts.Duration, opts.Session)
	if err != nil {
		return nil, err
	}

	return &orderbuilder.CoveredCallParams{
		Underlying: strings.TrimSpace(opts.Underlying),
		Expiration: expiration,
		Strike:     opts.Strike,
		Quantity:   opts.Quantity,
		Price:      opts.Price,
		Duration:   duration,
		Session:    session,
	}, nil
}

// collarOrderFlagSetup registers collar-with-stock flags on cmd.
func collarOrderFlagSetup(cmd *cobra.Command, opts *collarBuildOpts) {
	if err := structcli.Define(cmd, opts); err != nil {
		panic(err)
	}
	cmd.MarkFlagsMutuallyExclusive("open", "close")
	cmd.MarkFlagsOneRequired("open", "close")
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

	duration, session, err := parseDurationSession(opts.Duration, opts.Session)
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
		Duration:   duration,
		Session:    session,
	}, nil
}

// calendarOrderFlagSetup registers calendar spread flags on cmd.
func calendarOrderFlagSetup(cmd *cobra.Command, opts *calendarBuildOpts) {
	if err := structcli.Define(cmd, opts); err != nil {
		panic(err)
	}
	cmd.MarkFlagsMutuallyExclusive("call", "put")
	cmd.MarkFlagsOneRequired("call", "put")
	cmd.MarkFlagsMutuallyExclusive("open", "close")
	cmd.MarkFlagsOneRequired("open", "close")
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

	duration, session, err := parseDurationSession(opts.Duration, opts.Session)
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
		Duration:       duration,
		Session:        session,
	}, nil
}

// diagonalOrderFlagSetup registers diagonal spread flags on cmd.
func diagonalOrderFlagSetup(cmd *cobra.Command, opts *diagonalBuildOpts) {
	if err := structcli.Define(cmd, opts); err != nil {
		panic(err)
	}
	cmd.MarkFlagsMutuallyExclusive("call", "put")
	cmd.MarkFlagsOneRequired("call", "put")
	cmd.MarkFlagsMutuallyExclusive("open", "close")
	cmd.MarkFlagsOneRequired("open", "close")
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

	duration, session, err := parseDurationSession(opts.Duration, opts.Session)
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
		Duration:       duration,
		Session:        session,
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

// parseDurationSession parses the --duration and --session flags together.
// Every order parse function needs both, so this eliminates the repeated pair.
func parseDurationSession(rawDuration, rawSession string) (models.Duration, models.Session, error) {
	duration, err := parseDuration(rawDuration)
	if err != nil {
		return "", "", err
	}

	session, err := parseSession(rawSession)
	if err != nil {
		return "", "", err
	}

	return duration, session, nil
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

// parseStopPriceLinkBasis converts CLI input to a stop price link basis enum.
// Defaults to LAST when empty, which is the most common trailing stop reference.
func parseStopPriceLinkBasis(raw string) (models.StopPriceLinkBasis, error) {
	return parseEnum(raw, validStopPriceLinkBases, models.StopPriceLinkBasisLast, "stop-link-basis")
}

// parseStopPriceLinkType converts CLI input to a stop price link type enum.
// Defaults to VALUE when empty, which means the offset is a dollar amount.
func parseStopPriceLinkType(raw string) (models.StopPriceLinkType, error) {
	return parseEnum(raw, validStopPriceLinkTypes, models.StopPriceLinkTypeValue, "stop-link-type")
}

// parseStopType converts CLI input to a stop type enum.
// Defaults to STANDARD when empty.
func parseStopType(raw string) (models.StopType, error) {
	return parseEnum(raw, validStopTypes, models.StopTypeStandard, "stop-type")
}

// parseSpecialInstruction converts a CLI flag value into a SpecialInstruction constant.
// Returns an empty value when the flag is not set.
func parseSpecialInstruction(raw string) (models.SpecialInstruction, error) {
	return parseEnum(raw, validSpecialInstructions, "", "special-instruction")
}

// parseDestination converts CLI input to a requested destination enum.
// Returns empty when not set (optional field).
func parseDestination(raw string) (models.RequestedDestination, error) {
	return parseEnum(raw, validDestinations, "", "destination")
}

// parsePriceLinkBasis converts CLI input to a price link basis enum.
// Returns empty when not set (optional field).
func parsePriceLinkBasis(raw string) (models.PriceLinkBasis, error) {
	return parseEnum(raw, validPriceLinkBases, "", "price-link-basis")
}

// parsePriceLinkType converts CLI input to a price link type enum.
// Returns empty when not set (optional field).
func parsePriceLinkType(raw string) (models.PriceLinkType, error) {
	return parseEnum(raw, validPriceLinkTypes, "", "price-link-type")
}
