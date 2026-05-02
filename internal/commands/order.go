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

const confirmOrderMessage = "Add --confirm to execute this order"

const mutableDisabledMessage = `Mutable operations are disabled by default. ` +
	`Set "i-also-like-to-live-dangerously": true in your config file to enable order placement, cancellation, and replacement.`

// NewOrderCmd returns the Cobra command for order operations.
func NewOrderCmd(c *client.Ref, configPath string, w io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "order",
		Short:   "List, build, preview, place, cancel, and replace orders",
		GroupID: "trading",
		RunE:    requireSubcommand,
	}

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
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List orders (defaults to non-terminal statuses)",
		Example: `schwab-agent order list
schwab-agent order list --status all
schwab-agent order list --status FILLED
schwab-agent order list --status WORKING --status PENDING_ACTIVATION
schwab-agent order list --status WORKING,PENDING_ACTIVATION
schwab-agent order list --from 2025-01-01 --to 2025-01-31`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			var statuses []string
			for _, raw := range flagStringSlice(cmd, "status") {
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
				FromEnteredTime: strings.TrimSpace(flagString(cmd, "from")),
				ToEnteredTime:   strings.TrimSpace(flagString(cmd, "to")),
			}

			account := strings.TrimSpace(flagString(cmd, "account"))

			var orders []models.Order
			var err error
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

	cmd.Flags().StringSlice("status", nil, "Filter by order status (repeatable, use 'all' for unfiltered): WORKING, PENDING_ACTIVATION, FILLED, EXPIRED, CANCELED, REJECTED, etc.")
	cmd.Flags().String("from", "", "Filter by entered time lower bound")
	cmd.Flags().String("to", "", "Filter by entered time upper bound")
	cmd.Flags().String("account", "", "Account hash value")

	return cmd
}

// newOrderGetCmd returns a single order by account and ID.
func newOrderGetCmd(c *client.Ref, configPath string, w io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get [order-id]",
		Short: "Get an order by ID",
		Example: `schwab-agent order get 1234567890
schwab-agent order get --order-id 1234567890`,
		RunE: func(cmd *cobra.Command, args []string) error {
			orderID, err := parseRequiredOrderID(cmd, args)
			if err != nil {
				return err
			}

			account, err := resolveAccount(flagString(cmd, "account"), configPath, nil)
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

	cmd.Flags().String("order-id", "", "Order ID")
	cmd.Flags().String("account", "", "Account hash value")

	return cmd
}

// newOrderPlaceCmd places new orders from either flags or a JSON spec.
func newOrderPlaceCmd(c *client.Ref, configPath string, w io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "place",
		Short: "Place an order",
		Example: `schwab-agent order place --spec @order.json --confirm
schwab-agent order place --spec - --confirm`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if strings.TrimSpace(flagString(cmd, "spec")) == "" {
				return newValidationError("spec is required for `order place` without a subcommand")
			}

			if err := requireMutableEnabled(configPath); err != nil {
				return err
			}

			if err := requireConfirm(flagBool(cmd, "confirm")); err != nil {
				return err
			}

			account, err := resolveAccount(flagString(cmd, "account"), configPath, nil)
			if err != nil {
				return err
			}

			order, err := parseSpecOrder(cmd, flagString(cmd, "spec"))
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
	cmd.Flags().String("spec", "", "Inline JSON, @file, or - for stdin")
	cmd.Flags().Bool("confirm", false, "Confirm order placement")
	cmd.Flags().String("account", "", "Account hash value")
	cmd.AddCommand(
		makeCobraPlaceOrderCommand(c, configPath, w, "equity", "Place an equity order", equityOrderFlagSetup, parseEquityParams, orderbuilder.ValidateEquityOrder, orderbuilder.BuildEquityOrder),
		makeCobraPlaceOrderCommand(c, configPath, w, "option", "Place an option order", optionOrderFlagSetup, parseOptionParams, orderbuilder.ValidateOptionOrder, orderbuilder.BuildOptionOrder),
		makeCobraPlaceOrderCommand(c, configPath, w, "bracket", "Place a bracket order", bracketOrderFlagSetup, parseBracketParams, orderbuilder.ValidateBracketOrder, orderbuilder.BuildBracketOrder),
		makeCobraPlaceOrderCommand(c, configPath, w, "oco", "Place a one-cancels-other order for an existing position", ocoOrderFlagSetup, parseOCOParams, orderbuilder.ValidateOCOOrder, orderbuilder.BuildOCOOrder),
	)

	return cmd
}

// makeCobraPlaceOrderCommand creates a Cobra place subcommand with the same
// parse/validate/build/place pipeline as the legacy generic factory.
func makeCobraPlaceOrderCommand[P any](
	c *client.Ref,
	configPath string,
	w io.Writer,
	name, usage string,
	flagSetup func(*cobra.Command),
	parse func(*cobra.Command, []string) (P, error),
	validate func(*P) error,
	build func(*P) (*models.OrderRequest, error),
) *cobra.Command {
	cmd := &cobra.Command{
		Use:   name,
		Short: usage,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireMutableEnabled(configPath); err != nil {
				return err
			}

			if err := requireConfirm(flagBool(cmd, "confirm")); err != nil {
				return err
			}

			account, err := resolveAccount(flagString(cmd, "account"), configPath, nil)
			if err != nil {
				return err
			}

			params, err := parse(cmd, args)
			if err != nil {
				return err
			}

			if err := validate(&params); err != nil {
				return err
			}

			order, err := build(&params)
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
		flagSetup(cmd)
	}
	cmd.Flags().Bool("confirm", false, "Confirm order placement")
	cmd.Flags().String("account", "", "Account hash value")

	return cmd
}

// newOrderPreviewCmd previews an order from a JSON spec.
func newOrderPreviewCmd(c *client.Ref, configPath string, w io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "preview",
		Short:   "Preview an order from JSON spec",
		Example: "schwab-agent order preview --spec @order.json",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if strings.TrimSpace(flagString(cmd, "spec")) == "" {
				return newValidationError("spec is required")
			}

			account, err := resolveAccount(flagString(cmd, "account"), configPath, nil)
			if err != nil {
				return err
			}

			order, err := parseSpecOrder(cmd, flagString(cmd, "spec"))
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

	cmd.Flags().String("spec", "", "Inline JSON, @file, or - for stdin")
	cmd.Flags().String("account", "", "Account hash value")

	return cmd
}

// newOrderCancelCmd cancels an existing order.
func newOrderCancelCmd(c *client.Ref, configPath string, w io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cancel [order-id]",
		Short: "Cancel an order",
		Example: `schwab-agent order cancel 1234567890 --confirm
schwab-agent order cancel --order-id 1234567890 --confirm`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireMutableEnabled(configPath); err != nil {
				return err
			}

			if err := requireConfirm(flagBool(cmd, "confirm")); err != nil {
				return err
			}

			orderID, err := parseRequiredOrderID(cmd, args)
			if err != nil {
				return err
			}

			account, err := resolveAccount(flagString(cmd, "account"), configPath, nil)
			if err != nil {
				return err
			}

			if err := c.CancelOrder(cmd.Context(), account, orderID); err != nil {
				return err
			}

			return output.WriteSuccess(w, orderCancelData{OrderID: orderID, Canceled: true}, output.NewMetadata())
		},
	}

	cmd.Flags().String("order-id", "", "Order ID")
	cmd.Flags().Bool("confirm", false, "Confirm cancellation")
	cmd.Flags().String("account", "", "Account hash value")

	return cmd
}

// newOrderReplaceCmd replaces an existing order with a new equity order payload.
func newOrderReplaceCmd(c *client.Ref, configPath string, w io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "replace [order-id]",
		Short: "Replace an order with a new equity order spec",
		Example: `schwab-agent order replace 1234567890 --symbol AAPL --action BUY --quantity 10 --type LIMIT --price 155.00 --duration DAY --confirm
schwab-agent order replace --order-id 1234567890 --symbol AAPL --action BUY --quantity 10 --type LIMIT --price 155.00 --duration DAY --confirm`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireMutableEnabled(configPath); err != nil {
				return err
			}

			if err := requireConfirm(flagBool(cmd, "confirm")); err != nil {
				return err
			}

			orderID, err := parseRequiredOrderID(cmd, args)
			if err != nil {
				return err
			}

			account, err := resolveAccount(flagString(cmd, "account"), configPath, nil)
			if err != nil {
				return err
			}

			params, err := parseEquityParams(cmd, args)
			if err != nil {
				return err
			}

			if err := orderbuilder.ValidateEquityOrder(&params); err != nil {
				return err
			}

			order, err := orderbuilder.BuildEquityOrder(&params)
			if err != nil {
				return err
			}

			if err := c.ReplaceOrder(cmd.Context(), account, orderID, order); err != nil {
				return err
			}

			return output.WriteSuccess(w, orderReplaceData{OrderID: orderID, Replaced: true}, output.NewMetadata())
		},
	}

	equityOrderFlagSetup(cmd)
	cmd.Flags().String("order-id", "", "Order ID")
	cmd.Flags().Bool("confirm", false, "Confirm replacement")
	cmd.Flags().String("account", "", "Account hash value")

	return cmd
}

// equityOrderFlagSetup registers equity order flags on cmd.
func equityOrderFlagSetup(cmd *cobra.Command) {
	cmd.Flags().String("symbol", "", "Equity symbol")
	cmd.Flags().String("action", "", "Order action")
	cmd.Flags().Float64("quantity", 0, "Share quantity")
	cmd.Flags().String("type", "", "Order type")
	cmd.Flags().Float64("price", 0, "Limit price")
	cmd.Flags().Float64("stop-price", 0, "Stop price")
	cmd.Flags().Float64("stop-offset", 0, "Trailing stop offset amount")
	cmd.Flags().String("stop-link-basis", "", "Trailing stop reference price (LAST, BID, ASK, MARK)")
	cmd.Flags().String("stop-link-type", "", "Trailing stop offset type (VALUE, PERCENT, TICK)")
	cmd.Flags().String("stop-type", "", "Trailing stop trigger type (STANDARD, BID, ASK, LAST, MARK)")
	cmd.Flags().Float64("activation-price", 0, "Price that activates the trailing stop")
	cmd.Flags().String("duration", "", "Order duration")
	cmd.Flags().String("session", "", "Trading session")
	cmd.Flags().String("special-instruction", "", "Special instruction (ALL_OR_NONE, DO_NOT_REDUCE, ALL_OR_NONE_DO_NOT_REDUCE)")
	cmd.Flags().String("destination", "", "Order routing destination (INET, ECN_ARCA, CBOE, AMEX, PHLX, ISE, BOX, NYSE, NASDAQ, BATS, C2, AUTO)")
	cmd.Flags().String("price-link-basis", "", "Price link reference price (MANUAL, BASE, TRIGGER, LAST, BID, ASK, ASK_BID, MARK, AVERAGE)")
	cmd.Flags().String("price-link-type", "", "Price link offset type (VALUE, PERCENT, TICK)")
}

// optionOrderFlagSetup registers option order flags on cmd.
func optionOrderFlagSetup(cmd *cobra.Command) {
	cmd.Flags().String("underlying", "", "Underlying symbol")
	cmd.Flags().String("expiration", "", "Expiration date (YYYY-MM-DD)")
	cmd.Flags().Float64("strike", 0, "Strike price")
	cmd.Flags().Bool("call", false, "Call option")
	cmd.Flags().Bool("put", false, "Put option")
	cmd.Flags().String("action", "", "Order action")
	cmd.Flags().Float64("quantity", 0, "Contract quantity")
	cmd.Flags().String("type", "", "Order type")
	cmd.Flags().Float64("price", 0, "Limit price")
	cmd.Flags().String("duration", "", "Order duration")
	cmd.Flags().String("session", "", "Trading session")
	cmd.Flags().String("special-instruction", "", "Special instruction (ALL_OR_NONE, DO_NOT_REDUCE, ALL_OR_NONE_DO_NOT_REDUCE)")
	cmd.Flags().String("destination", "", "Order routing destination (INET, ECN_ARCA, CBOE, AMEX, PHLX, ISE, BOX, NYSE, NASDAQ, BATS, C2, AUTO)")
	cmd.Flags().String("price-link-basis", "", "Price link reference price (MANUAL, BASE, TRIGGER, LAST, BID, ASK, ASK_BID, MARK, AVERAGE)")
	cmd.Flags().String("price-link-type", "", "Price link offset type (VALUE, PERCENT, TICK)")
}

// bracketOrderFlagSetup registers bracket order flags on cmd.
func bracketOrderFlagSetup(cmd *cobra.Command) {
	cmd.Flags().String("symbol", "", "Equity symbol")
	cmd.Flags().String("action", "", "Order action")
	cmd.Flags().Float64("quantity", 0, "Share quantity")
	cmd.Flags().String("type", "", "Entry order type")
	cmd.Flags().Float64("price", 0, "Entry price")
	cmd.Flags().Float64("take-profit", 0, "Take-profit exit price")
	cmd.Flags().Float64("stop-loss", 0, "Stop-loss exit price")
	cmd.Flags().String("duration", "", "Order duration")
	cmd.Flags().String("session", "", "Trading session")
}

// ocoOrderFlagSetup registers standalone OCO order flags on cmd.
func ocoOrderFlagSetup(cmd *cobra.Command) {
	cmd.Flags().String("symbol", "", "Equity symbol")
	cmd.Flags().String("action", "", "Exit action (SELL to close long, BUY to close short)")
	cmd.Flags().Float64("quantity", 0, "Share quantity")
	cmd.Flags().Float64("take-profit", 0, "Take-profit exit price (limit order)")
	cmd.Flags().Float64("stop-loss", 0, "Stop-loss exit price (stop order)")
	cmd.Flags().String("duration", "", "Order duration")
	cmd.Flags().String("session", "", "Trading session")
}

// parseOCOParams converts command flags into standalone OCO builder params.
func parseOCOParams(cmd *cobra.Command, _ []string) (orderbuilder.OCOParams, error) {
	action, err := parseInstruction(flagString(cmd, "action"))
	if err != nil {
		return orderbuilder.OCOParams{}, err
	}

	duration, session, err := parseDurationSession(cmd)
	if err != nil {
		return orderbuilder.OCOParams{}, err
	}

	return orderbuilder.OCOParams{
		Symbol:     strings.TrimSpace(flagString(cmd, "symbol")),
		Action:     action,
		Quantity:   flagFloat64(cmd, "quantity"),
		TakeProfit: flagFloat64(cmd, "take-profit"),
		StopLoss:   flagFloat64(cmd, "stop-loss"),
		Duration:   duration,
		Session:    session,
	}, nil
}

// verticalOrderFlagSetup registers vertical spread flags on cmd.
func verticalOrderFlagSetup(cmd *cobra.Command) {
	cmd.Flags().String("underlying", "", "Underlying symbol")
	cmd.Flags().String("expiration", "", "Expiration date (YYYY-MM-DD)")
	cmd.Flags().Float64("long-strike", 0, "Strike price of the option being bought")
	cmd.Flags().Float64("short-strike", 0, "Strike price of the option being sold")
	cmd.Flags().Bool("call", false, "Call spread")
	cmd.Flags().Bool("put", false, "Put spread")
	cmd.Flags().Bool("open", false, "Opening position")
	cmd.Flags().Bool("close", false, "Closing position")
	cmd.Flags().Float64("quantity", 0, "Number of contracts")
	cmd.Flags().Float64("price", 0, "Net debit or credit amount")
	cmd.Flags().String("duration", "", "Order duration")
	cmd.Flags().String("session", "", "Trading session")
}

// ironCondorOrderFlagSetup registers iron condor flags on cmd.
func ironCondorOrderFlagSetup(cmd *cobra.Command) {
	cmd.Flags().String("underlying", "", "Underlying symbol")
	cmd.Flags().String("expiration", "", "Expiration date (YYYY-MM-DD)")
	cmd.Flags().Float64("put-long-strike", 0, "Lowest strike: put being bought (protection)")
	cmd.Flags().Float64("put-short-strike", 0, "Put being sold (premium)")
	cmd.Flags().Float64("call-short-strike", 0, "Call being sold (premium)")
	cmd.Flags().Float64("call-long-strike", 0, "Highest strike: call being bought (protection)")
	cmd.Flags().Bool("open", false, "Opening position")
	cmd.Flags().Bool("close", false, "Closing position")
	cmd.Flags().Float64("quantity", 0, "Number of contracts")
	cmd.Flags().Float64("price", 0, "Net credit or debit amount")
	cmd.Flags().String("duration", "", "Order duration")
	cmd.Flags().String("session", "", "Trading session")
}

// parseIronCondorParams converts command flags into iron condor builder params.
func parseIronCondorParams(cmd *cobra.Command, _ []string) (orderbuilder.IronCondorParams, error) {
	isOpen, err := parseOpenClose(flagBool(cmd, "open"), flagBool(cmd, "close"))
	if err != nil {
		return orderbuilder.IronCondorParams{}, err
	}

	expiration, err := parseExpiration(cmd)
	if err != nil {
		return orderbuilder.IronCondorParams{}, err
	}

	duration, session, err := parseDurationSession(cmd)
	if err != nil {
		return orderbuilder.IronCondorParams{}, err
	}

	return orderbuilder.IronCondorParams{
		Underlying:      strings.TrimSpace(flagString(cmd, "underlying")),
		Expiration:      expiration,
		PutLongStrike:   flagFloat64(cmd, "put-long-strike"),
		PutShortStrike:  flagFloat64(cmd, "put-short-strike"),
		CallShortStrike: flagFloat64(cmd, "call-short-strike"),
		CallLongStrike:  flagFloat64(cmd, "call-long-strike"),
		Open:            isOpen,
		Quantity:        flagFloat64(cmd, "quantity"),
		Price:           flagFloat64(cmd, "price"),
		Duration:        duration,
		Session:         session,
	}, nil
}

// parseEquityParams converts command flags into equity order builder params.
func parseEquityParams(cmd *cobra.Command, _ []string) (orderbuilder.EquityParams, error) {
	action, err := parseInstruction(flagString(cmd, "action"))
	if err != nil {
		return orderbuilder.EquityParams{}, err
	}

	orderType, err := parseOrderType(flagString(cmd, "type"), models.OrderTypeMarket)
	if err != nil {
		return orderbuilder.EquityParams{}, err
	}

	duration, session, err := parseDurationSession(cmd)
	if err != nil {
		return orderbuilder.EquityParams{}, err
	}

	stopLinkBasis, err := parseStopPriceLinkBasis(flagString(cmd, "stop-link-basis"))
	if err != nil {
		return orderbuilder.EquityParams{}, err
	}

	stopLinkType, err := parseStopPriceLinkType(flagString(cmd, "stop-link-type"))
	if err != nil {
		return orderbuilder.EquityParams{}, err
	}

	stopType, err := parseStopType(flagString(cmd, "stop-type"))
	if err != nil {
		return orderbuilder.EquityParams{}, err
	}

	specialInstruction, err := parseSpecialInstruction(flagString(cmd, "special-instruction"))
	if err != nil {
		return orderbuilder.EquityParams{}, err
	}

	destination, err := parseDestination(flagString(cmd, "destination"))
	if err != nil {
		return orderbuilder.EquityParams{}, err
	}

	priceLinkBasis, err := parsePriceLinkBasis(flagString(cmd, "price-link-basis"))
	if err != nil {
		return orderbuilder.EquityParams{}, err
	}

	priceLinkType, err := parsePriceLinkType(flagString(cmd, "price-link-type"))
	if err != nil {
		return orderbuilder.EquityParams{}, err
	}

	return orderbuilder.EquityParams{
		Symbol:             strings.TrimSpace(flagString(cmd, "symbol")),
		Action:             action,
		Quantity:           flagFloat64(cmd, "quantity"),
		OrderType:          orderType,
		Price:              flagFloat64(cmd, "price"),
		StopPrice:          flagFloat64(cmd, "stop-price"),
		StopPriceOffset:    flagFloat64(cmd, "stop-offset"),
		StopPriceLinkBasis: stopLinkBasis,
		StopPriceLinkType:  stopLinkType,
		StopType:           stopType,
		ActivationPrice:    flagFloat64(cmd, "activation-price"),
		SpecialInstruction: specialInstruction,
		Destination:        destination,
		PriceLinkBasis:     priceLinkBasis,
		PriceLinkType:      priceLinkType,
		Duration:           duration,
		Session:            session,
	}, nil
}

// parseOptionParams converts command flags into option order builder params.
func parseOptionParams(cmd *cobra.Command, _ []string) (orderbuilder.OptionParams, error) {
	action, err := parseInstruction(flagString(cmd, "action"))
	if err != nil {
		return orderbuilder.OptionParams{}, err
	}

	orderType, err := parseOrderType(flagString(cmd, "type"), models.OrderTypeMarket)
	if err != nil {
		return orderbuilder.OptionParams{}, err
	}

	duration, session, err := parseDurationSession(cmd)
	if err != nil {
		return orderbuilder.OptionParams{}, err
	}

	putCall, err := parsePutCall(flagBool(cmd, "call"), flagBool(cmd, "put"))
	if err != nil {
		return orderbuilder.OptionParams{}, err
	}

	expiration, err := parseExpiration(cmd)
	if err != nil {
		return orderbuilder.OptionParams{}, err
	}

	specialInstruction, err := parseSpecialInstruction(flagString(cmd, "special-instruction"))
	if err != nil {
		return orderbuilder.OptionParams{}, err
	}

	destination, err := parseDestination(flagString(cmd, "destination"))
	if err != nil {
		return orderbuilder.OptionParams{}, err
	}

	priceLinkBasis, err := parsePriceLinkBasis(flagString(cmd, "price-link-basis"))
	if err != nil {
		return orderbuilder.OptionParams{}, err
	}

	priceLinkType, err := parsePriceLinkType(flagString(cmd, "price-link-type"))
	if err != nil {
		return orderbuilder.OptionParams{}, err
	}

	return orderbuilder.OptionParams{
		Underlying:         strings.TrimSpace(flagString(cmd, "underlying")),
		Expiration:         expiration,
		Strike:             flagFloat64(cmd, "strike"),
		PutCall:            putCall,
		Action:             action,
		Quantity:           flagFloat64(cmd, "quantity"),
		OrderType:          orderType,
		Price:              flagFloat64(cmd, "price"),
		SpecialInstruction: specialInstruction,
		Destination:        destination,
		PriceLinkBasis:     priceLinkBasis,
		PriceLinkType:      priceLinkType,
		Duration:           duration,
		Session:            session,
	}, nil
}

// parseBracketParams converts command flags into bracket order builder params.
func parseBracketParams(cmd *cobra.Command, _ []string) (orderbuilder.BracketParams, error) {
	action, err := parseInstruction(flagString(cmd, "action"))
	if err != nil {
		return orderbuilder.BracketParams{}, err
	}

	orderType, err := parseOrderType(flagString(cmd, "type"), models.OrderTypeMarket)
	if err != nil {
		return orderbuilder.BracketParams{}, err
	}

	duration, session, err := parseDurationSession(cmd)
	if err != nil {
		return orderbuilder.BracketParams{}, err
	}

	return orderbuilder.BracketParams{
		Symbol:     strings.TrimSpace(flagString(cmd, "symbol")),
		Action:     action,
		Quantity:   flagFloat64(cmd, "quantity"),
		OrderType:  orderType,
		Price:      flagFloat64(cmd, "price"),
		TakeProfit: flagFloat64(cmd, "take-profit"),
		StopLoss:   flagFloat64(cmd, "stop-loss"),
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
func parseRequiredOrderID(cmd *cobra.Command, args []string) (int64, error) {
	// Flag takes priority over positional arg, matching resolveAccount() convention.
	value := strings.TrimSpace(flagString(cmd, "order-id"))
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
func parseVerticalParams(cmd *cobra.Command, _ []string) (orderbuilder.VerticalParams, error) {
	putCall, err := parsePutCall(flagBool(cmd, "call"), flagBool(cmd, "put"))
	if err != nil {
		return orderbuilder.VerticalParams{}, err
	}

	isOpen, err := parseOpenClose(flagBool(cmd, "open"), flagBool(cmd, "close"))
	if err != nil {
		return orderbuilder.VerticalParams{}, err
	}

	expiration, err := parseExpiration(cmd)
	if err != nil {
		return orderbuilder.VerticalParams{}, err
	}

	duration, session, err := parseDurationSession(cmd)
	if err != nil {
		return orderbuilder.VerticalParams{}, err
	}

	return orderbuilder.VerticalParams{
		Underlying:  strings.TrimSpace(flagString(cmd, "underlying")),
		Expiration:  expiration,
		LongStrike:  flagFloat64(cmd, "long-strike"),
		ShortStrike: flagFloat64(cmd, "short-strike"),
		PutCall:     putCall,
		Open:        isOpen,
		Quantity:    flagFloat64(cmd, "quantity"),
		Price:       flagFloat64(cmd, "price"),
		Duration:    duration,
		Session:     session,
	}, nil
}

// strangleOrderFlagSetup registers strangle flags on cmd.
func strangleOrderFlagSetup(cmd *cobra.Command) {
	cmd.Flags().String("underlying", "", "Underlying symbol")
	cmd.Flags().String("expiration", "", "Expiration date (YYYY-MM-DD)")
	cmd.Flags().Float64("call-strike", 0, "Strike price for the call leg")
	cmd.Flags().Float64("put-strike", 0, "Strike price for the put leg")
	cmd.Flags().Bool("buy", false, "Buy the strangle (long, net debit)")
	cmd.Flags().Bool("sell", false, "Sell the strangle (short, net credit)")
	cmd.Flags().Bool("open", false, "Opening position")
	cmd.Flags().Bool("close", false, "Closing position")
	cmd.Flags().Float64("quantity", 0, "Number of contracts")
	cmd.Flags().Float64("price", 0, "Net debit or credit amount")
	cmd.Flags().String("duration", "", "Order duration")
	cmd.Flags().String("session", "", "Trading session")
}

// parseStrangleParams converts command flags into strangle builder params.
func parseStrangleParams(cmd *cobra.Command, _ []string) (orderbuilder.StrangleParams, error) {
	isBuy, err := parseBuySell(flagBool(cmd, "buy"), flagBool(cmd, "sell"))
	if err != nil {
		return orderbuilder.StrangleParams{}, err
	}

	isOpen, err := parseOpenClose(flagBool(cmd, "open"), flagBool(cmd, "close"))
	if err != nil {
		return orderbuilder.StrangleParams{}, err
	}

	expiration, err := parseExpiration(cmd)
	if err != nil {
		return orderbuilder.StrangleParams{}, err
	}

	duration, session, err := parseDurationSession(cmd)
	if err != nil {
		return orderbuilder.StrangleParams{}, err
	}

	return orderbuilder.StrangleParams{
		Underlying: strings.TrimSpace(flagString(cmd, "underlying")),
		Expiration: expiration,
		CallStrike: flagFloat64(cmd, "call-strike"),
		PutStrike:  flagFloat64(cmd, "put-strike"),
		Buy:        isBuy,
		Open:       isOpen,
		Quantity:   flagFloat64(cmd, "quantity"),
		Price:      flagFloat64(cmd, "price"),
		Duration:   duration,
		Session:    session,
	}, nil
}

// straddleOrderFlagSetup registers straddle flags on cmd.
func straddleOrderFlagSetup(cmd *cobra.Command) {
	cmd.Flags().String("underlying", "", "Underlying symbol")
	cmd.Flags().String("expiration", "", "Expiration date (YYYY-MM-DD)")
	cmd.Flags().Float64("strike", 0, "Strike price (shared by call and put legs)")
	cmd.Flags().Bool("buy", false, "Buy the straddle (long, net debit)")
	cmd.Flags().Bool("sell", false, "Sell the straddle (short, net credit)")
	cmd.Flags().Bool("open", false, "Opening position")
	cmd.Flags().Bool("close", false, "Closing position")
	cmd.Flags().Float64("quantity", 0, "Number of contracts")
	cmd.Flags().Float64("price", 0, "Net debit or credit amount")
	cmd.Flags().String("duration", "", "Order duration")
	cmd.Flags().String("session", "", "Trading session")
}

// parseStraddleParams converts command flags into straddle builder params.
func parseStraddleParams(cmd *cobra.Command, _ []string) (orderbuilder.StraddleParams, error) {
	isBuy, err := parseBuySell(flagBool(cmd, "buy"), flagBool(cmd, "sell"))
	if err != nil {
		return orderbuilder.StraddleParams{}, err
	}

	isOpen, err := parseOpenClose(flagBool(cmd, "open"), flagBool(cmd, "close"))
	if err != nil {
		return orderbuilder.StraddleParams{}, err
	}

	expiration, err := parseExpiration(cmd)
	if err != nil {
		return orderbuilder.StraddleParams{}, err
	}

	duration, session, err := parseDurationSession(cmd)
	if err != nil {
		return orderbuilder.StraddleParams{}, err
	}

	return orderbuilder.StraddleParams{
		Underlying: strings.TrimSpace(flagString(cmd, "underlying")),
		Expiration: expiration,
		Strike:     flagFloat64(cmd, "strike"),
		Buy:        isBuy,
		Open:       isOpen,
		Quantity:   flagFloat64(cmd, "quantity"),
		Price:      flagFloat64(cmd, "price"),
		Duration:   duration,
		Session:    session,
	}, nil
}

// coveredCallOrderFlagSetup registers covered call flags on cmd.
func coveredCallOrderFlagSetup(cmd *cobra.Command) {
	cmd.Flags().String("underlying", "", "Underlying symbol")
	cmd.Flags().String("expiration", "", "Expiration date (YYYY-MM-DD)")
	cmd.Flags().Float64("strike", 0, "Call strike price")
	cmd.Flags().Float64("quantity", 0, "Number of contracts (1 contract = 100 shares)")
	cmd.Flags().Float64("price", 0, "Net debit amount")
	cmd.Flags().String("duration", "", "Order duration")
	cmd.Flags().String("session", "", "Trading session")
}

// parseCoveredCallParams converts command flags into covered call builder params.
func parseCoveredCallParams(cmd *cobra.Command, _ []string) (orderbuilder.CoveredCallParams, error) {
	expiration, err := parseExpiration(cmd)
	if err != nil {
		return orderbuilder.CoveredCallParams{}, err
	}

	duration, session, err := parseDurationSession(cmd)
	if err != nil {
		return orderbuilder.CoveredCallParams{}, err
	}

	return orderbuilder.CoveredCallParams{
		Underlying: strings.TrimSpace(flagString(cmd, "underlying")),
		Expiration: expiration,
		Strike:     flagFloat64(cmd, "strike"),
		Quantity:   flagFloat64(cmd, "quantity"),
		Price:      flagFloat64(cmd, "price"),
		Duration:   duration,
		Session:    session,
	}, nil
}

// collarOrderFlagSetup registers collar-with-stock flags on cmd.
func collarOrderFlagSetup(cmd *cobra.Command) {
	cmd.Flags().String("underlying", "", "Underlying symbol")
	cmd.Flags().Float64("put-strike", 0, "Protective put strike price")
	cmd.Flags().Float64("call-strike", 0, "Covered call strike price")
	cmd.Flags().String("expiration", "", "Expiration date for both options (YYYY-MM-DD)")
	cmd.Flags().Float64("quantity", 0, "Number of contracts (1 contract = 100 shares)")
	cmd.Flags().Bool("open", false, "Opening position")
	cmd.Flags().Bool("close", false, "Closing position")
	cmd.Flags().Float64("price", 0, "Net debit amount")
	cmd.Flags().String("duration", "", "Order duration")
	cmd.Flags().String("session", "", "Trading session")
}

// parseCollarParams converts command flags into collar-with-stock builder params.
func parseCollarParams(cmd *cobra.Command, _ []string) (orderbuilder.CollarParams, error) {
	isOpen, err := parseOpenClose(flagBool(cmd, "open"), flagBool(cmd, "close"))
	if err != nil {
		return orderbuilder.CollarParams{}, err
	}

	expiration, err := parseExpiration(cmd)
	if err != nil {
		return orderbuilder.CollarParams{}, err
	}

	duration, session, err := parseDurationSession(cmd)
	if err != nil {
		return orderbuilder.CollarParams{}, err
	}

	return orderbuilder.CollarParams{
		Underlying: strings.TrimSpace(flagString(cmd, "underlying")),
		PutStrike:  flagFloat64(cmd, "put-strike"),
		CallStrike: flagFloat64(cmd, "call-strike"),
		Expiration: expiration,
		Quantity:   flagFloat64(cmd, "quantity"),
		Open:       isOpen,
		Price:      flagFloat64(cmd, "price"),
		Duration:   duration,
		Session:    session,
	}, nil
}

// calendarOrderFlagSetup registers calendar spread flags on cmd.
func calendarOrderFlagSetup(cmd *cobra.Command) {
	cmd.Flags().String("underlying", "", "Underlying symbol")
	cmd.Flags().String("near-expiration", "", "Near-term expiration date (YYYY-MM-DD)")
	cmd.Flags().String("far-expiration", "", "Far-term expiration date (YYYY-MM-DD)")
	cmd.Flags().Float64("strike", 0, "Strike price (shared by both legs)")
	cmd.Flags().Bool("call", false, "Call calendar spread")
	cmd.Flags().Bool("put", false, "Put calendar spread")
	cmd.Flags().Bool("open", false, "Opening position")
	cmd.Flags().Bool("close", false, "Closing position")
	cmd.Flags().Float64("quantity", 0, "Number of contracts")
	cmd.Flags().Float64("price", 0, "Net debit amount")
	cmd.Flags().String("duration", "", "Order duration")
	cmd.Flags().String("session", "", "Trading session")
}

// parseCalendarParams converts command flags into calendar spread builder params.
func parseCalendarParams(cmd *cobra.Command, _ []string) (orderbuilder.CalendarParams, error) {
	putCall, err := parsePutCall(flagBool(cmd, "call"), flagBool(cmd, "put"))
	if err != nil {
		return orderbuilder.CalendarParams{}, err
	}

	isOpen, err := parseOpenClose(flagBool(cmd, "open"), flagBool(cmd, "close"))
	if err != nil {
		return orderbuilder.CalendarParams{}, err
	}

	nearExpiration, err := parseDateFlag(flagString(cmd, "near-expiration"), "near-expiration")
	if err != nil {
		return orderbuilder.CalendarParams{}, err
	}

	farExpiration, err := parseDateFlag(flagString(cmd, "far-expiration"), "far-expiration")
	if err != nil {
		return orderbuilder.CalendarParams{}, err
	}

	duration, session, err := parseDurationSession(cmd)
	if err != nil {
		return orderbuilder.CalendarParams{}, err
	}

	return orderbuilder.CalendarParams{
		Underlying:     strings.TrimSpace(flagString(cmd, "underlying")),
		NearExpiration: nearExpiration,
		FarExpiration:  farExpiration,
		Strike:         flagFloat64(cmd, "strike"),
		PutCall:        putCall,
		Open:           isOpen,
		Quantity:       flagFloat64(cmd, "quantity"),
		Price:          flagFloat64(cmd, "price"),
		Duration:       duration,
		Session:        session,
	}, nil
}

// diagonalOrderFlagSetup registers diagonal spread flags on cmd.
func diagonalOrderFlagSetup(cmd *cobra.Command) {
	cmd.Flags().String("underlying", "", "Underlying symbol")
	cmd.Flags().String("near-expiration", "", "Near-term expiration date (YYYY-MM-DD)")
	cmd.Flags().String("far-expiration", "", "Far-term expiration date (YYYY-MM-DD)")
	cmd.Flags().Float64("near-strike", 0, "Strike price for the near-term (sold) leg")
	cmd.Flags().Float64("far-strike", 0, "Strike price for the far-term (bought) leg")
	cmd.Flags().Bool("call", false, "Call diagonal spread")
	cmd.Flags().Bool("put", false, "Put diagonal spread")
	cmd.Flags().Bool("open", false, "Opening position")
	cmd.Flags().Bool("close", false, "Closing position")
	cmd.Flags().Float64("quantity", 0, "Number of contracts")
	cmd.Flags().Float64("price", 0, "Net debit amount")
	cmd.Flags().String("duration", "", "Order duration")
	cmd.Flags().String("session", "", "Trading session")
}

// parseDiagonalParams converts command flags into diagonal spread builder params.
func parseDiagonalParams(cmd *cobra.Command, _ []string) (orderbuilder.DiagonalParams, error) {
	putCall, err := parsePutCall(flagBool(cmd, "call"), flagBool(cmd, "put"))
	if err != nil {
		return orderbuilder.DiagonalParams{}, err
	}

	isOpen, err := parseOpenClose(flagBool(cmd, "open"), flagBool(cmd, "close"))
	if err != nil {
		return orderbuilder.DiagonalParams{}, err
	}

	nearExpiration, err := parseDateFlag(flagString(cmd, "near-expiration"), "near-expiration")
	if err != nil {
		return orderbuilder.DiagonalParams{}, err
	}

	farExpiration, err := parseDateFlag(flagString(cmd, "far-expiration"), "far-expiration")
	if err != nil {
		return orderbuilder.DiagonalParams{}, err
	}

	duration, session, err := parseDurationSession(cmd)
	if err != nil {
		return orderbuilder.DiagonalParams{}, err
	}

	return orderbuilder.DiagonalParams{
		Underlying:     strings.TrimSpace(flagString(cmd, "underlying")),
		NearExpiration: nearExpiration,
		FarExpiration:  farExpiration,
		NearStrike:     flagFloat64(cmd, "near-strike"),
		FarStrike:      flagFloat64(cmd, "far-strike"),
		PutCall:        putCall,
		Open:           isOpen,
		Quantity:       flagFloat64(cmd, "quantity"),
		Price:          flagFloat64(cmd, "price"),
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
func parseExpiration(cmd *cobra.Command) (time.Time, error) {
	expiration, err := time.Parse("2006-01-02", strings.TrimSpace(flagString(cmd, "expiration")))
	if err != nil {
		return time.Time{}, newValidationError("expiration must use YYYY-MM-DD format")
	}

	return expiration, nil
}

// parseDurationSession parses the --duration and --session flags together.
// Every order parse function needs both, so this eliminates the repeated pair.
func parseDurationSession(cmd *cobra.Command) (models.Duration, models.Session, error) {
	duration, err := parseDuration(flagString(cmd, "duration"))
	if err != nil {
		return "", "", err
	}

	session, err := parseSession(flagString(cmd, "session"))
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
