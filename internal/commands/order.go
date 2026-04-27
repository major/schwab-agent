package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/urfave/cli/v3"

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

// orderRepeatPlaceData wraps a successful order repeat placement response.
type orderRepeatPlaceData struct {
	OrderID         int64 `json:"orderId"`
	OriginalOrderID int64 `json:"originalOrderId"`
}

// orderRepeatPreviewData wraps an order repeat preview response.
type orderRepeatPreviewData struct {
	Preview         *models.PreviewOrder `json:"preview"`
	OriginalOrderID int64                `json:"originalOrderId"`
}

const confirmOrderMessage = "Add --confirm to execute this order"

const mutableDisabledMessage = `Mutable operations are disabled by default. ` +
	`Set "i-also-like-to-live-dangerously": true in your config file to enable order placement, cancellation, and replacement.`

// OrderCommand returns the parent order command and all nested order workflows.
func OrderCommand(c *client.Ref, configPath string, w io.Writer) *cli.Command {
	return &cli.Command{
		Name:   "order",
		Usage:  "List, build, preview, place, cancel, and replace orders",
		Action: requireSubcommand(),
		Commands: []*cli.Command{
			orderListCommand(c, configPath, w),
			orderGetCommand(c, configPath, w),
			orderPlaceCommand(c, configPath, w),
			orderPreviewCommand(c, configPath, w),
			orderBuildCommand(w),
			orderCancelCommand(c, configPath, w),
			orderReplaceCommand(c, configPath, w),
			orderRepeatCommand(c, configPath, w),
		},
	}
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

// orderListCommand lists orders for a specific account or all accounts.
// By default, terminal statuses (FILLED, CANCELED, REJECTED, EXPIRED, REPLACED)
// are filtered out to show only actionable orders. Use --status all to see
// everything, or --status <STATUS> to filter by specific statuses.
func orderListCommand(c *client.Ref, _ string, w io.Writer) *cli.Command {
	return &cli.Command{
		Name:  "list",
		Usage: "List orders (defaults to non-terminal statuses)",
		UsageText: `schwab-agent order list
schwab-agent order list --status all
schwab-agent order list --status FILLED
schwab-agent order list --status WORKING --status PENDING_ACTIVATION
schwab-agent order list --status WORKING,PENDING_ACTIVATION
schwab-agent order list --from 2025-01-01 --to 2025-01-31`,
		Flags: []cli.Flag{
			&cli.StringSliceFlag{Name: "status", Usage: "Filter by order status (repeatable, use 'all' for unfiltered): WORKING, PENDING_ACTIVATION, FILLED, EXPIRED, CANCELED, REJECTED, etc."},
			&cli.StringFlag{Name: "from", Usage: "Filter by entered time lower bound"},
			&cli.StringFlag{Name: "to", Usage: "Filter by entered time upper bound"},
			&cli.StringFlag{Name: "account", Usage: "Account hash value"},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			// Split comma-separated values and trim whitespace, matching the
			// --fields pattern in quote.go. Supports both repeatable flags
			// (--status WORKING --status FILLED) and comma-separated
			// (--status WORKING,FILLED).
			var statuses []string
			for _, raw := range cmd.StringSlice("status") {
				for part := range strings.SplitSeq(raw, ",") {
					trimmed := strings.TrimSpace(part)
					if trimmed != "" {
						statuses = append(statuses, trimmed)
					}
				}
			}

			// "all" is a pseudo-status that disables default filtering.
			// When present, fetch everything without any status filter.
			showAll := false
			for _, s := range statuses {
				if strings.EqualFold(s, "all") {
					showAll = true
					break
				}
			}

			// When specific statuses are requested (not "all"), pass them
			// through to the API. When "all" or no statuses, fetch unfiltered.
			var apiStatuses []string
			if !showAll {
				apiStatuses = statuses
			}

			params := client.OrderListParams{
				Statuses:        apiStatuses,
				FromEnteredTime: strings.TrimSpace(cmd.String("from")),
				ToEnteredTime:   strings.TrimSpace(cmd.String("to")),
			}

			account := strings.TrimSpace(cmd.String("account"))

			var orders []models.Order
			var err error
			if account == "" {
				orders, err = c.AllOrders(ctx, params)
			} else {
				orders, err = c.ListOrders(ctx, account, params)
			}
			if err != nil {
				return err
			}

			// When no statuses were explicitly requested, filter out terminal
			// orders client-side. This avoids N API calls (one per non-terminal
			// status) since the Schwab API only accepts one status per request.
			if len(statuses) == 0 {
				orders = filterNonTerminalOrders(orders)
			}

			return output.WriteSuccess(w, orderListData{Orders: orders}, output.NewMetadata())
		},
	}
}

// orderGetCommand returns a single order by account and ID.
func orderGetCommand(c *client.Ref, configPath string, w io.Writer) *cli.Command {
	return &cli.Command{
		Name:  "get",
		Usage: "Get an order by ID",
		UsageText: `schwab-agent order get 1234567890
schwab-agent order get --order-id 1234567890`,
		ArgsUsage: "<order-id>",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "order-id", Usage: "Order ID"},
			&cli.StringFlag{Name: "account", Usage: "Account hash value"},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			orderID, err := parseRequiredOrderID(cmd)
			if err != nil {
				return err
			}

			account, err := resolveAccount(cmd.String("account"), configPath, nil)
			if err != nil {
				return err
			}

			order, err := c.GetOrder(ctx, account, orderID)
			if err != nil {
				return err
			}

			return output.WriteSuccess(w, orderGetData{Order: order}, output.NewMetadata())
		},
	}
}

// orderPlaceCommand places new orders from either flags or a JSON spec.
func orderPlaceCommand(c *client.Ref, configPath string, w io.Writer) *cli.Command {
	return &cli.Command{
		Name:  "place",
		Usage: "Place an order",
		UsageText: `schwab-agent order place --spec @order.json --confirm
schwab-agent order place --spec - --confirm`,
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "spec", Usage: "Inline JSON, @file, or - for stdin"},
			&cli.BoolFlag{Name: "confirm", Usage: "Confirm order placement"},
			&cli.StringFlag{Name: "account", Usage: "Account hash value"},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			if strings.TrimSpace(cmd.String("spec")) == "" {
				return newValidationError("spec is required for `order place` without a subcommand")
			}

			if err := requireMutableEnabled(configPath); err != nil {
				return err
			}

			if err := requireConfirm(cmd.Bool("confirm")); err != nil {
				return err
			}

			account, err := resolveAccount(cmd.String("account"), configPath, nil)
			if err != nil {
				return err
			}

			order, err := parseSpecOrder(cmd, cmd.String("spec"))
			if err != nil {
				return err
			}

			response, err := c.PlaceOrder(ctx, account, order)
			if err != nil {
				return err
			}

			return output.WriteSuccess(w, orderPlaceData{OrderID: response.OrderID}, output.NewMetadata())
		},
		Commands: []*cli.Command{
			makePlaceOrderCommand(c, configPath, w, "equity", "Place an equity order",
				"schwab-agent order place equity --symbol AAPL --action BUY --quantity 10 --type LIMIT --price 150.00 --duration DAY --confirm",
				equityOrderFlags(), parseEquityParams,
				orderbuilder.ValidateEquityOrder, orderbuilder.BuildEquityOrder),
			makePlaceOrderCommand(c, configPath, w, "option", "Place an option order",
				`schwab-agent order place option --underlying AAPL --expiration 2025-06-20 --strike 200 --call --action BUY_TO_OPEN --quantity 1 --type LIMIT --price 5.00 --duration DAY --confirm`,
				optionOrderFlags(), parseOptionParams,
				orderbuilder.ValidateOptionOrder, orderbuilder.BuildOptionOrder),
			makePlaceOrderCommand(c, configPath, w, "bracket", "Place a bracket order",
				`schwab-agent order place bracket --symbol AAPL --action BUY --quantity 10 --type LIMIT --price 150.00 --take-profit 170.00 --stop-loss 140.00 --duration DAY --confirm`,
				bracketOrderFlags(), parseBracketParams,
				orderbuilder.ValidateBracketOrder, orderbuilder.BuildBracketOrder),
			makePlaceOrderCommand(c, configPath, w, "oco", "Place a one-cancels-other order for an existing position",
				`schwab-agent order place oco --symbol AAPL --action SELL --quantity 10 --take-profit 170.00 --stop-loss 140.00 --duration DAY --confirm`,
				ocoOrderFlags(), parseOCOParams,
				orderbuilder.ValidateOCOOrder, orderbuilder.BuildOCOOrder),
		},
	}
}

// makePlaceOrderCommand creates a place subcommand that enforces safety guards,
// resolves the account, then runs the parse/validate/build/place pipeline.
// Same generic pattern as makeBuildOrderCommand but adds mutable + confirm gates
// and the actual API call.
func makePlaceOrderCommand[P any](
	c *client.Ref,
	configPath string,
	w io.Writer,
	name, usage, usageText string,
	flags []cli.Flag,
	parse func(*cli.Command) (P, error),
	validate func(*P) error,
	build func(*P) (*models.OrderRequest, error),
) *cli.Command {
	return &cli.Command{
		Name:      name,
		Usage:     usage,
		UsageText: usageText,
		Flags: append(flags,
			&cli.BoolFlag{Name: "confirm", Usage: "Confirm order placement"},
			&cli.StringFlag{Name: "account", Usage: "Account hash value"},
		),
		Action: func(ctx context.Context, cmd *cli.Command) error {
			if err := requireMutableEnabled(configPath); err != nil {
				return err
			}

			if err := requireConfirm(cmd.Bool("confirm")); err != nil {
				return err
			}

			account, err := resolveAccount(cmd.String("account"), configPath, nil)
			if err != nil {
				return err
			}

			params, err := parse(cmd)
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

			response, err := c.PlaceOrder(ctx, account, order)
			if err != nil {
				return err
			}

			return output.WriteSuccess(w, orderPlaceData{OrderID: response.OrderID}, output.NewMetadata())
		},
	}
}

// orderPreviewCommand previews an order from a JSON spec.
func orderPreviewCommand(c *client.Ref, configPath string, w io.Writer) *cli.Command {
	return &cli.Command{
		Name:      "preview",
		Usage:     "Preview an order from JSON spec",
		UsageText: "schwab-agent order preview --spec @order.json",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "spec", Usage: "Inline JSON, @file, or - for stdin"},
			&cli.StringFlag{Name: "account", Usage: "Account hash value"},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			if strings.TrimSpace(cmd.String("spec")) == "" {
				return newValidationError("spec is required")
			}

			account, err := resolveAccount(cmd.String("account"), configPath, nil)
			if err != nil {
				return err
			}

			order, err := parseSpecOrder(cmd, cmd.String("spec"))
			if err != nil {
				return err
			}

			preview, err := c.PreviewOrder(ctx, account, order)
			if err != nil {
				return err
			}

			return output.WriteSuccess(w, orderPreviewData{
				Preview: preview,
				OrderID: preview.OrderID,
			}, output.NewMetadata())
		},
	}
}

// orderCancelCommand cancels an existing order.
func orderCancelCommand(c *client.Ref, configPath string, w io.Writer) *cli.Command {
	return &cli.Command{
		Name:  "cancel",
		Usage: "Cancel an order",
		UsageText: `schwab-agent order cancel 1234567890 --confirm
schwab-agent order cancel --order-id 1234567890 --confirm`,
		ArgsUsage: "<order-id>",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "order-id", Usage: "Order ID"},
			&cli.BoolFlag{Name: "confirm", Usage: "Confirm cancellation"},
			&cli.StringFlag{Name: "account", Usage: "Account hash value"},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			if err := requireMutableEnabled(configPath); err != nil {
				return err
			}

			if err := requireConfirm(cmd.Bool("confirm")); err != nil {
				return err
			}

			orderID, err := parseRequiredOrderID(cmd)
			if err != nil {
				return err
			}

			account, err := resolveAccount(cmd.String("account"), configPath, nil)
			if err != nil {
				return err
			}

			if err := c.CancelOrder(ctx, account, orderID); err != nil {
				return err
			}

			return output.WriteSuccess(w, orderCancelData{OrderID: orderID, Canceled: true}, output.NewMetadata())
		},
	}
}

// orderReplaceCommand replaces an existing order with an equity order payload.
func orderReplaceCommand(c *client.Ref, configPath string, w io.Writer) *cli.Command {
	return &cli.Command{
		Name:  "replace",
		Usage: "Replace an order with a new equity order spec",
		UsageText: `schwab-agent order replace 1234567890 --symbol AAPL --action BUY --quantity 10 --type LIMIT --price 155.00 --duration DAY --confirm
schwab-agent order replace --order-id 1234567890 --symbol AAPL --action BUY --quantity 10 --type LIMIT --price 155.00 --duration DAY --confirm`,
		ArgsUsage: "<order-id>",
		Flags: append(equityOrderFlags(),
			&cli.StringFlag{Name: "order-id", Usage: "Order ID"},
			&cli.BoolFlag{Name: "confirm", Usage: "Confirm replacement"},
			&cli.StringFlag{Name: "account", Usage: "Account hash value"},
		),
		Action: func(ctx context.Context, cmd *cli.Command) error {
			if err := requireMutableEnabled(configPath); err != nil {
				return err
			}

			if err := requireConfirm(cmd.Bool("confirm")); err != nil {
				return err
			}

			orderID, err := parseRequiredOrderID(cmd)
			if err != nil {
				return err
			}

			account, err := resolveAccount(cmd.String("account"), configPath, nil)
			if err != nil {
				return err
			}

			params, err := parseEquityParams(cmd)
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

			if err := c.ReplaceOrder(ctx, account, orderID, order); err != nil {
				return err
			}

			return output.WriteSuccess(w, orderReplaceData{OrderID: orderID, Replaced: true}, output.NewMetadata())
		},
	}
}

// orderRepeatCommand fetches an existing order, converts it to a submittable request,
// and either outputs the JSON (--build/default), previews it, or places it.
func orderRepeatCommand(c *client.Ref, configPath string, w io.Writer) *cli.Command {
	return &cli.Command{
		Name:  "repeat",
		Usage: "Repeat a previous order (fetch, convert, and optionally place)",
		UsageText: `schwab-agent order repeat 1234567890
schwab-agent order repeat --order-id 1234567890
schwab-agent order repeat 1234567890 --preview
schwab-agent order repeat 1234567890 --confirm`,
		ArgsUsage: "<order-id>",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "order-id", Usage: "Order ID"},
			&cli.BoolFlag{Name: "build", Usage: "Output reconstructed order request JSON (default)"},
			&cli.BoolFlag{Name: "preview", Usage: "Preview the order without placing it"},
			&cli.BoolFlag{Name: "confirm", Usage: "Place the order (requires safety guards)"},
			&cli.StringFlag{Name: "account", Usage: "Account hash value"},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			buildMode := cmd.Bool("build")
			previewMode := cmd.Bool("preview")
			confirmMode := cmd.Bool("confirm")

			// Reject multiple mode flags to avoid ambiguity.
			modeCount := 0
			if buildMode {
				modeCount++
			}
			if previewMode {
				modeCount++
			}
			if confirmMode {
				modeCount++
			}
			if modeCount > 1 {
				return newValidationError("specify only one of --build, --preview, or --confirm")
			}

			// Enforce safety guards early for --confirm so we fail fast
			// before making any API calls. Note: --confirm doubles as
			// both the mode selector and the safety gate for this command,
			// so a separate requireConfirm check would be tautological.
			if confirmMode {
				if err := requireMutableEnabled(configPath); err != nil {
					return err
				}
			}

			orderID, err := parseRequiredOrderID(cmd)
			if err != nil {
				return err
			}

			account, err := resolveAccount(cmd.String("account"), configPath, nil)
			if err != nil {
				return err
			}

			// Fetch the original order from the API.
			order, err := c.GetOrder(ctx, account, orderID)
			if err != nil {
				return err
			}

			// Convert to a submittable request (strips response-only fields).
			request := models.OrderToRequest(order)

			switch {
			case previewMode:
				preview, previewErr := c.PreviewOrder(ctx, account, &request)
				if previewErr != nil {
					return previewErr
				}
				return output.WriteSuccess(w, orderRepeatPreviewData{
					Preview:         preview,
					OriginalOrderID: orderID,
				}, output.NewMetadata())

			case confirmMode:
				response, placeErr := c.PlaceOrder(ctx, account, &request)
				if placeErr != nil {
					return placeErr
				}
				return output.WriteSuccess(w, orderRepeatPlaceData{
					OrderID:         response.OrderID,
					OriginalOrderID: orderID,
				}, output.NewMetadata())

			default:
				// --build or no flag: output raw order request JSON.
				return writeOrderRequestJSON(w, request)
			}
		},
	}
}

// equityOrderFlags returns the shared flag set for equity order workflows.
func equityOrderFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{Name: "symbol", Usage: "Equity symbol"},
		&cli.StringFlag{Name: "action", Usage: "Order action"},
		&cli.Float64Flag{Name: "quantity", Usage: "Share quantity"},
		&cli.StringFlag{Name: "type", Usage: "Order type"},
		&cli.Float64Flag{Name: "price", Usage: "Limit price"},
		&cli.Float64Flag{Name: "stop-price", Usage: "Stop price"},
		&cli.Float64Flag{Name: "stop-offset", Usage: "Trailing stop offset amount"},
		&cli.StringFlag{Name: "stop-link-basis", Usage: "Trailing stop reference price (LAST, BID, ASK, MARK)"},
		&cli.StringFlag{Name: "stop-link-type", Usage: "Trailing stop offset type (VALUE, PERCENT, TICK)"},
		&cli.StringFlag{Name: "stop-type", Usage: "Trailing stop trigger type (STANDARD, BID, ASK, LAST, MARK)"},
		&cli.Float64Flag{Name: "activation-price", Usage: "Price that activates the trailing stop"},
		&cli.StringFlag{Name: "duration", Usage: "Order duration"},
		&cli.StringFlag{Name: "session", Usage: "Trading session"},
		&cli.StringFlag{Name: "special-instruction", Usage: "Special instruction (ALL_OR_NONE, DO_NOT_REDUCE, ALL_OR_NONE_DO_NOT_REDUCE)"},
		&cli.StringFlag{Name: "destination", Usage: "Order routing destination (INET, ECN_ARCA, CBOE, AMEX, PHLX, ISE, BOX, NYSE, NASDAQ, BATS, C2, AUTO)"},
		&cli.StringFlag{Name: "price-link-basis", Usage: "Price link reference price (MANUAL, BASE, TRIGGER, LAST, BID, ASK, ASK_BID, MARK, AVERAGE)"},
		&cli.StringFlag{Name: "price-link-type", Usage: "Price link offset type (VALUE, PERCENT, TICK)"},
	}
}

// optionOrderFlags returns the shared flag set for option order workflows.
func optionOrderFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{Name: "underlying", Usage: "Underlying symbol"},
		&cli.StringFlag{Name: "expiration", Usage: "Expiration date (YYYY-MM-DD)"},
		&cli.Float64Flag{Name: "strike", Usage: "Strike price"},
		&cli.BoolFlag{Name: "call", Usage: "Call option"},
		&cli.BoolFlag{Name: "put", Usage: "Put option"},
		&cli.StringFlag{Name: "action", Usage: "Order action"},
		&cli.Float64Flag{Name: "quantity", Usage: "Contract quantity"},
		&cli.StringFlag{Name: "type", Usage: "Order type"},
		&cli.Float64Flag{Name: "price", Usage: "Limit price"},
		&cli.StringFlag{Name: "duration", Usage: "Order duration"},
		&cli.StringFlag{Name: "session", Usage: "Trading session"},
		&cli.StringFlag{Name: "special-instruction", Usage: "Special instruction (ALL_OR_NONE, DO_NOT_REDUCE, ALL_OR_NONE_DO_NOT_REDUCE)"},
		&cli.StringFlag{Name: "destination", Usage: "Order routing destination (INET, ECN_ARCA, CBOE, AMEX, PHLX, ISE, BOX, NYSE, NASDAQ, BATS, C2, AUTO)"},
		&cli.StringFlag{Name: "price-link-basis", Usage: "Price link reference price (MANUAL, BASE, TRIGGER, LAST, BID, ASK, ASK_BID, MARK, AVERAGE)"},
		&cli.StringFlag{Name: "price-link-type", Usage: "Price link offset type (VALUE, PERCENT, TICK)"},
	}
}

// bracketOrderFlags returns the shared flag set for bracket order workflows.
func bracketOrderFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{Name: "symbol", Usage: "Equity symbol"},
		&cli.StringFlag{Name: "action", Usage: "Order action"},
		&cli.Float64Flag{Name: "quantity", Usage: "Share quantity"},
		&cli.StringFlag{Name: "type", Usage: "Entry order type"},
		&cli.Float64Flag{Name: "price", Usage: "Entry price"},
		&cli.Float64Flag{Name: "take-profit", Usage: "Take-profit exit price"},
		&cli.Float64Flag{Name: "stop-loss", Usage: "Stop-loss exit price"},
		&cli.StringFlag{Name: "duration", Usage: "Order duration"},
		&cli.StringFlag{Name: "session", Usage: "Trading session"},
	}
}

// ocoOrderFlags returns the shared flag set for standalone OCO order workflows.
func ocoOrderFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{Name: "symbol", Usage: "Equity symbol"},
		&cli.StringFlag{Name: "action", Usage: "Exit action (SELL to close long, BUY to close short)"},
		&cli.Float64Flag{Name: "quantity", Usage: "Share quantity"},
		&cli.Float64Flag{Name: "take-profit", Usage: "Take-profit exit price (limit order)"},
		&cli.Float64Flag{Name: "stop-loss", Usage: "Stop-loss exit price (stop order)"},
		&cli.StringFlag{Name: "duration", Usage: "Order duration"},
		&cli.StringFlag{Name: "session", Usage: "Trading session"},
	}
}

// parseOCOParams converts command flags into standalone OCO builder params.
func parseOCOParams(cmd *cli.Command) (orderbuilder.OCOParams, error) {
	action, err := parseInstruction(cmd.String("action"))
	if err != nil {
		return orderbuilder.OCOParams{}, err
	}

	duration, session, err := parseDurationSession(cmd)
	if err != nil {
		return orderbuilder.OCOParams{}, err
	}

	return orderbuilder.OCOParams{
		Symbol:     strings.TrimSpace(cmd.String("symbol")),
		Action:     action,
		Quantity:   cmd.Float64("quantity"),
		TakeProfit: cmd.Float64("take-profit"),
		StopLoss:   cmd.Float64("stop-loss"),
		Duration:   duration,
		Session:    session,
	}, nil
}

// verticalOrderFlags returns the shared flag set for vertical spread workflows.
func verticalOrderFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{Name: "underlying", Usage: "Underlying symbol"},
		&cli.StringFlag{Name: "expiration", Usage: "Expiration date (YYYY-MM-DD)"},
		&cli.Float64Flag{Name: "long-strike", Usage: "Strike price of the option being bought"},
		&cli.Float64Flag{Name: "short-strike", Usage: "Strike price of the option being sold"},
		&cli.BoolFlag{Name: "call", Usage: "Call spread"},
		&cli.BoolFlag{Name: "put", Usage: "Put spread"},
		&cli.BoolFlag{Name: "open", Usage: "Opening position"},
		&cli.BoolFlag{Name: "close", Usage: "Closing position"},
		&cli.Float64Flag{Name: "quantity", Usage: "Number of contracts"},
		&cli.Float64Flag{Name: "price", Usage: "Net debit or credit amount"},
		&cli.StringFlag{Name: "duration", Usage: "Order duration"},
		&cli.StringFlag{Name: "session", Usage: "Trading session"},
	}
}

// ironCondorOrderFlags returns the CLI flags for the iron-condor build command.
func ironCondorOrderFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{Name: "underlying", Usage: "Underlying symbol"},
		&cli.StringFlag{Name: "expiration", Usage: "Expiration date (YYYY-MM-DD)"},
		&cli.Float64Flag{Name: "put-long-strike", Usage: "Lowest strike: put being bought (protection)"},
		&cli.Float64Flag{Name: "put-short-strike", Usage: "Put being sold (premium)"},
		&cli.Float64Flag{Name: "call-short-strike", Usage: "Call being sold (premium)"},
		&cli.Float64Flag{Name: "call-long-strike", Usage: "Highest strike: call being bought (protection)"},
		&cli.BoolFlag{Name: "open", Usage: "Opening position"},
		&cli.BoolFlag{Name: "close", Usage: "Closing position"},
		&cli.Float64Flag{Name: "quantity", Usage: "Number of contracts"},
		&cli.Float64Flag{Name: "price", Usage: "Net credit or debit amount"},
		&cli.StringFlag{Name: "duration", Usage: "Order duration"},
		&cli.StringFlag{Name: "session", Usage: "Trading session"},
	}
}

// parseIronCondorParams converts command flags into iron condor builder params.
func parseIronCondorParams(cmd *cli.Command) (orderbuilder.IronCondorParams, error) {
	isOpen, err := parseOpenClose(cmd.Bool("open"), cmd.Bool("close"))
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
		Underlying:      strings.TrimSpace(cmd.String("underlying")),
		Expiration:      expiration,
		PutLongStrike:   cmd.Float64("put-long-strike"),
		PutShortStrike:  cmd.Float64("put-short-strike"),
		CallShortStrike: cmd.Float64("call-short-strike"),
		CallLongStrike:  cmd.Float64("call-long-strike"),
		Open:            isOpen,
		Quantity:        cmd.Float64("quantity"),
		Price:           cmd.Float64("price"),
		Duration:        duration,
		Session:         session,
	}, nil
}

// parseEquityParams converts command flags into equity order builder params.
func parseEquityParams(cmd *cli.Command) (orderbuilder.EquityParams, error) {
	action, err := parseInstruction(cmd.String("action"))
	if err != nil {
		return orderbuilder.EquityParams{}, err
	}

	orderType, err := parseOrderType(cmd.String("type"), models.OrderTypeMarket)
	if err != nil {
		return orderbuilder.EquityParams{}, err
	}

	duration, session, err := parseDurationSession(cmd)
	if err != nil {
		return orderbuilder.EquityParams{}, err
	}

	stopLinkBasis, err := parseStopPriceLinkBasis(cmd.String("stop-link-basis"))
	if err != nil {
		return orderbuilder.EquityParams{}, err
	}

	stopLinkType, err := parseStopPriceLinkType(cmd.String("stop-link-type"))
	if err != nil {
		return orderbuilder.EquityParams{}, err
	}

	stopType, err := parseStopType(cmd.String("stop-type"))
	if err != nil {
		return orderbuilder.EquityParams{}, err
	}

	specialInstruction, err := parseSpecialInstruction(cmd.String("special-instruction"))
	if err != nil {
		return orderbuilder.EquityParams{}, err
	}

	destination, err := parseDestination(cmd.String("destination"))
	if err != nil {
		return orderbuilder.EquityParams{}, err
	}

	priceLinkBasis, err := parsePriceLinkBasis(cmd.String("price-link-basis"))
	if err != nil {
		return orderbuilder.EquityParams{}, err
	}

	priceLinkType, err := parsePriceLinkType(cmd.String("price-link-type"))
	if err != nil {
		return orderbuilder.EquityParams{}, err
	}

	return orderbuilder.EquityParams{
		Symbol:             strings.TrimSpace(cmd.String("symbol")),
		Action:             action,
		Quantity:           cmd.Float64("quantity"),
		OrderType:          orderType,
		Price:              cmd.Float64("price"),
		StopPrice:          cmd.Float64("stop-price"),
		StopPriceOffset:    cmd.Float64("stop-offset"),
		StopPriceLinkBasis: stopLinkBasis,
		StopPriceLinkType:  stopLinkType,
		StopType:           stopType,
		ActivationPrice:    cmd.Float64("activation-price"),
		SpecialInstruction: specialInstruction,
		Destination:        destination,
		PriceLinkBasis:     priceLinkBasis,
		PriceLinkType:      priceLinkType,
		Duration:           duration,
		Session:            session,
	}, nil
}

// parseOptionParams converts command flags into option order builder params.
func parseOptionParams(cmd *cli.Command) (orderbuilder.OptionParams, error) {
	action, err := parseInstruction(cmd.String("action"))
	if err != nil {
		return orderbuilder.OptionParams{}, err
	}

	orderType, err := parseOrderType(cmd.String("type"), models.OrderTypeMarket)
	if err != nil {
		return orderbuilder.OptionParams{}, err
	}

	duration, session, err := parseDurationSession(cmd)
	if err != nil {
		return orderbuilder.OptionParams{}, err
	}

	putCall, err := parsePutCall(cmd.Bool("call"), cmd.Bool("put"))
	if err != nil {
		return orderbuilder.OptionParams{}, err
	}

	expiration, err := parseExpiration(cmd)
	if err != nil {
		return orderbuilder.OptionParams{}, err
	}

	specialInstruction, err := parseSpecialInstruction(cmd.String("special-instruction"))
	if err != nil {
		return orderbuilder.OptionParams{}, err
	}

	destination, err := parseDestination(cmd.String("destination"))
	if err != nil {
		return orderbuilder.OptionParams{}, err
	}

	priceLinkBasis, err := parsePriceLinkBasis(cmd.String("price-link-basis"))
	if err != nil {
		return orderbuilder.OptionParams{}, err
	}

	priceLinkType, err := parsePriceLinkType(cmd.String("price-link-type"))
	if err != nil {
		return orderbuilder.OptionParams{}, err
	}

	return orderbuilder.OptionParams{
		Underlying:         strings.TrimSpace(cmd.String("underlying")),
		Expiration:         expiration,
		Strike:             cmd.Float64("strike"),
		PutCall:            putCall,
		Action:             action,
		Quantity:           cmd.Float64("quantity"),
		OrderType:          orderType,
		Price:              cmd.Float64("price"),
		SpecialInstruction: specialInstruction,
		Destination:        destination,
		PriceLinkBasis:     priceLinkBasis,
		PriceLinkType:      priceLinkType,
		Duration:           duration,
		Session:            session,
	}, nil
}

// parseBracketParams converts command flags into bracket order builder params.
func parseBracketParams(cmd *cli.Command) (orderbuilder.BracketParams, error) {
	action, err := parseInstruction(cmd.String("action"))
	if err != nil {
		return orderbuilder.BracketParams{}, err
	}

	orderType, err := parseOrderType(cmd.String("type"), models.OrderTypeMarket)
	if err != nil {
		return orderbuilder.BracketParams{}, err
	}

	duration, session, err := parseDurationSession(cmd)
	if err != nil {
		return orderbuilder.BracketParams{}, err
	}

	return orderbuilder.BracketParams{
		Symbol:     strings.TrimSpace(cmd.String("symbol")),
		Action:     action,
		Quantity:   cmd.Float64("quantity"),
		OrderType:  orderType,
		Price:      cmd.Float64("price"),
		TakeProfit: cmd.Float64("take-profit"),
		StopLoss:   cmd.Float64("stop-loss"),
		Duration:   duration,
		Session:    session,
	}, nil
}

// parseSpecOrder loads and validates spec mode JSON into an order request.
func parseSpecOrder(cmd *cli.Command, spec string) (*models.OrderRequest, error) {
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
func readSpecSource(cmd *cli.Command, spec string) ([]byte, error) {
	trimmed := strings.TrimSpace(spec)
	if trimmed == "" {
		return nil, newValidationError("spec is required")
	}

	var payload []byte

	switch {
	case trimmed == "-":
		reader := cmd.Root().Reader
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
func parseRequiredOrderID(cmd *cli.Command) (int64, error) {
	// Flag takes priority over positional arg, matching resolveAccount() convention.
	value := strings.TrimSpace(cmd.String("order-id"))
	if value == "" {
		value = strings.TrimSpace(cmd.Args().First())
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
func parseVerticalParams(cmd *cli.Command) (orderbuilder.VerticalParams, error) {
	putCall, err := parsePutCall(cmd.Bool("call"), cmd.Bool("put"))
	if err != nil {
		return orderbuilder.VerticalParams{}, err
	}

	isOpen, err := parseOpenClose(cmd.Bool("open"), cmd.Bool("close"))
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
		Underlying:  strings.TrimSpace(cmd.String("underlying")),
		Expiration:  expiration,
		LongStrike:  cmd.Float64("long-strike"),
		ShortStrike: cmd.Float64("short-strike"),
		PutCall:     putCall,
		Open:        isOpen,
		Quantity:    cmd.Float64("quantity"),
		Price:       cmd.Float64("price"),
		Duration:    duration,
		Session:     session,
	}, nil
}

// strangleOrderFlags returns the CLI flags for the strangle build command.
func strangleOrderFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{Name: "underlying", Usage: "Underlying symbol"},
		&cli.StringFlag{Name: "expiration", Usage: "Expiration date (YYYY-MM-DD)"},
		&cli.Float64Flag{Name: "call-strike", Usage: "Strike price for the call leg"},
		&cli.Float64Flag{Name: "put-strike", Usage: "Strike price for the put leg"},
		&cli.BoolFlag{Name: "buy", Usage: "Buy the strangle (long, net debit)"},
		&cli.BoolFlag{Name: "sell", Usage: "Sell the strangle (short, net credit)"},
		&cli.BoolFlag{Name: "open", Usage: "Opening position"},
		&cli.BoolFlag{Name: "close", Usage: "Closing position"},
		&cli.Float64Flag{Name: "quantity", Usage: "Number of contracts"},
		&cli.Float64Flag{Name: "price", Usage: "Net debit or credit amount"},
		&cli.StringFlag{Name: "duration", Usage: "Order duration"},
		&cli.StringFlag{Name: "session", Usage: "Trading session"},
	}
}

// parseStrangleParams converts command flags into strangle builder params.
func parseStrangleParams(cmd *cli.Command) (orderbuilder.StrangleParams, error) {
	isBuy, err := parseBuySell(cmd.Bool("buy"), cmd.Bool("sell"))
	if err != nil {
		return orderbuilder.StrangleParams{}, err
	}

	isOpen, err := parseOpenClose(cmd.Bool("open"), cmd.Bool("close"))
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
		Underlying: strings.TrimSpace(cmd.String("underlying")),
		Expiration: expiration,
		CallStrike: cmd.Float64("call-strike"),
		PutStrike:  cmd.Float64("put-strike"),
		Buy:        isBuy,
		Open:       isOpen,
		Quantity:   cmd.Float64("quantity"),
		Price:      cmd.Float64("price"),
		Duration:   duration,
		Session:    session,
	}, nil
}

// straddleOrderFlags returns the CLI flags for the straddle build command.
func straddleOrderFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{Name: "underlying", Usage: "Underlying symbol"},
		&cli.StringFlag{Name: "expiration", Usage: "Expiration date (YYYY-MM-DD)"},
		&cli.Float64Flag{Name: "strike", Usage: "Strike price (shared by call and put legs)"},
		&cli.BoolFlag{Name: "buy", Usage: "Buy the straddle (long, net debit)"},
		&cli.BoolFlag{Name: "sell", Usage: "Sell the straddle (short, net credit)"},
		&cli.BoolFlag{Name: "open", Usage: "Opening position"},
		&cli.BoolFlag{Name: "close", Usage: "Closing position"},
		&cli.Float64Flag{Name: "quantity", Usage: "Number of contracts"},
		&cli.Float64Flag{Name: "price", Usage: "Net debit or credit amount"},
		&cli.StringFlag{Name: "duration", Usage: "Order duration"},
		&cli.StringFlag{Name: "session", Usage: "Trading session"},
	}
}

// parseStraddleParams converts command flags into straddle builder params.
func parseStraddleParams(cmd *cli.Command) (orderbuilder.StraddleParams, error) {
	isBuy, err := parseBuySell(cmd.Bool("buy"), cmd.Bool("sell"))
	if err != nil {
		return orderbuilder.StraddleParams{}, err
	}

	isOpen, err := parseOpenClose(cmd.Bool("open"), cmd.Bool("close"))
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
		Underlying: strings.TrimSpace(cmd.String("underlying")),
		Expiration: expiration,
		Strike:     cmd.Float64("strike"),
		Buy:        isBuy,
		Open:       isOpen,
		Quantity:   cmd.Float64("quantity"),
		Price:      cmd.Float64("price"),
		Duration:   duration,
		Session:    session,
	}, nil
}

// coveredCallOrderFlags returns the CLI flags for the covered-call build command.
func coveredCallOrderFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{Name: "underlying", Usage: "Underlying symbol"},
		&cli.StringFlag{Name: "expiration", Usage: "Expiration date (YYYY-MM-DD)"},
		&cli.Float64Flag{Name: "strike", Usage: "Call strike price"},
		&cli.Float64Flag{Name: "quantity", Usage: "Number of contracts (1 contract = 100 shares)"},
		&cli.Float64Flag{Name: "price", Usage: "Net debit amount"},
		&cli.StringFlag{Name: "duration", Usage: "Order duration"},
		&cli.StringFlag{Name: "session", Usage: "Trading session"},
	}
}

// parseCoveredCallParams converts command flags into covered call builder params.
func parseCoveredCallParams(cmd *cli.Command) (orderbuilder.CoveredCallParams, error) {
	expiration, err := parseExpiration(cmd)
	if err != nil {
		return orderbuilder.CoveredCallParams{}, err
	}

	duration, session, err := parseDurationSession(cmd)
	if err != nil {
		return orderbuilder.CoveredCallParams{}, err
	}

	return orderbuilder.CoveredCallParams{
		Underlying: strings.TrimSpace(cmd.String("underlying")),
		Expiration: expiration,
		Strike:     cmd.Float64("strike"),
		Quantity:   cmd.Float64("quantity"),
		Price:      cmd.Float64("price"),
		Duration:   duration,
		Session:    session,
	}, nil
}

// collarOrderFlags returns the CLI flags for the collar-with-stock build command.
func collarOrderFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{Name: "underlying", Usage: "Underlying symbol"},
		&cli.Float64Flag{Name: "put-strike", Usage: "Protective put strike price"},
		&cli.Float64Flag{Name: "call-strike", Usage: "Covered call strike price"},
		&cli.StringFlag{Name: "expiration", Usage: "Expiration date for both options (YYYY-MM-DD)"},
		&cli.Float64Flag{Name: "quantity", Usage: "Number of contracts (1 contract = 100 shares)"},
		&cli.BoolFlag{Name: "open", Usage: "Opening position"},
		&cli.BoolFlag{Name: "close", Usage: "Closing position"},
		&cli.Float64Flag{Name: "price", Usage: "Net debit amount"},
		&cli.StringFlag{Name: "duration", Usage: "Order duration"},
		&cli.StringFlag{Name: "session", Usage: "Trading session"},
	}
}

// parseCollarParams converts command flags into collar-with-stock builder params.
func parseCollarParams(cmd *cli.Command) (orderbuilder.CollarParams, error) {
	isOpen, err := parseOpenClose(cmd.Bool("open"), cmd.Bool("close"))
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
		Underlying: strings.TrimSpace(cmd.String("underlying")),
		PutStrike:  cmd.Float64("put-strike"),
		CallStrike: cmd.Float64("call-strike"),
		Expiration: expiration,
		Quantity:   cmd.Float64("quantity"),
		Open:       isOpen,
		Price:      cmd.Float64("price"),
		Duration:   duration,
		Session:    session,
	}, nil
}

// calendarOrderFlags returns the CLI flags for the calendar spread build command.
func calendarOrderFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{Name: "underlying", Usage: "Underlying symbol"},
		&cli.StringFlag{Name: "near-expiration", Usage: "Near-term expiration date (YYYY-MM-DD)"},
		&cli.StringFlag{Name: "far-expiration", Usage: "Far-term expiration date (YYYY-MM-DD)"},
		&cli.Float64Flag{Name: "strike", Usage: "Strike price (shared by both legs)"},
		&cli.BoolFlag{Name: "call", Usage: "Call calendar spread"},
		&cli.BoolFlag{Name: "put", Usage: "Put calendar spread"},
		&cli.BoolFlag{Name: "open", Usage: "Opening position"},
		&cli.BoolFlag{Name: "close", Usage: "Closing position"},
		&cli.Float64Flag{Name: "quantity", Usage: "Number of contracts"},
		&cli.Float64Flag{Name: "price", Usage: "Net debit amount"},
		&cli.StringFlag{Name: "duration", Usage: "Order duration"},
		&cli.StringFlag{Name: "session", Usage: "Trading session"},
	}
}

// parseCalendarParams converts command flags into calendar spread builder params.
func parseCalendarParams(cmd *cli.Command) (orderbuilder.CalendarParams, error) {
	putCall, err := parsePutCall(cmd.Bool("call"), cmd.Bool("put"))
	if err != nil {
		return orderbuilder.CalendarParams{}, err
	}

	isOpen, err := parseOpenClose(cmd.Bool("open"), cmd.Bool("close"))
	if err != nil {
		return orderbuilder.CalendarParams{}, err
	}

	nearExpiration, err := parseDateFlag(cmd.String("near-expiration"), "near-expiration")
	if err != nil {
		return orderbuilder.CalendarParams{}, err
	}

	farExpiration, err := parseDateFlag(cmd.String("far-expiration"), "far-expiration")
	if err != nil {
		return orderbuilder.CalendarParams{}, err
	}

	duration, session, err := parseDurationSession(cmd)
	if err != nil {
		return orderbuilder.CalendarParams{}, err
	}

	return orderbuilder.CalendarParams{
		Underlying:     strings.TrimSpace(cmd.String("underlying")),
		NearExpiration: nearExpiration,
		FarExpiration:  farExpiration,
		Strike:         cmd.Float64("strike"),
		PutCall:        putCall,
		Open:           isOpen,
		Quantity:       cmd.Float64("quantity"),
		Price:          cmd.Float64("price"),
		Duration:       duration,
		Session:        session,
	}, nil
}

// diagonalOrderFlags returns the CLI flags for the diagonal spread build command.
func diagonalOrderFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{Name: "underlying", Usage: "Underlying symbol"},
		&cli.StringFlag{Name: "near-expiration", Usage: "Near-term expiration date (YYYY-MM-DD)"},
		&cli.StringFlag{Name: "far-expiration", Usage: "Far-term expiration date (YYYY-MM-DD)"},
		&cli.Float64Flag{Name: "near-strike", Usage: "Strike price for the near-term (sold) leg"},
		&cli.Float64Flag{Name: "far-strike", Usage: "Strike price for the far-term (bought) leg"},
		&cli.BoolFlag{Name: "call", Usage: "Call diagonal spread"},
		&cli.BoolFlag{Name: "put", Usage: "Put diagonal spread"},
		&cli.BoolFlag{Name: "open", Usage: "Opening position"},
		&cli.BoolFlag{Name: "close", Usage: "Closing position"},
		&cli.Float64Flag{Name: "quantity", Usage: "Number of contracts"},
		&cli.Float64Flag{Name: "price", Usage: "Net debit amount"},
		&cli.StringFlag{Name: "duration", Usage: "Order duration"},
		&cli.StringFlag{Name: "session", Usage: "Trading session"},
	}
}

// parseDiagonalParams converts command flags into diagonal spread builder params.
func parseDiagonalParams(cmd *cli.Command) (orderbuilder.DiagonalParams, error) {
	putCall, err := parsePutCall(cmd.Bool("call"), cmd.Bool("put"))
	if err != nil {
		return orderbuilder.DiagonalParams{}, err
	}

	isOpen, err := parseOpenClose(cmd.Bool("open"), cmd.Bool("close"))
	if err != nil {
		return orderbuilder.DiagonalParams{}, err
	}

	nearExpiration, err := parseDateFlag(cmd.String("near-expiration"), "near-expiration")
	if err != nil {
		return orderbuilder.DiagonalParams{}, err
	}

	farExpiration, err := parseDateFlag(cmd.String("far-expiration"), "far-expiration")
	if err != nil {
		return orderbuilder.DiagonalParams{}, err
	}

	duration, session, err := parseDurationSession(cmd)
	if err != nil {
		return orderbuilder.DiagonalParams{}, err
	}

	return orderbuilder.DiagonalParams{
		Underlying:     strings.TrimSpace(cmd.String("underlying")),
		NearExpiration: nearExpiration,
		FarExpiration:  farExpiration,
		NearStrike:     cmd.Float64("near-strike"),
		FarStrike:      cmd.Float64("far-strike"),
		PutCall:        putCall,
		Open:           isOpen,
		Quantity:       cmd.Float64("quantity"),
		Price:          cmd.Float64("price"),
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
func parseExpiration(cmd *cli.Command) (time.Time, error) {
	expiration, err := time.Parse("2006-01-02", strings.TrimSpace(cmd.String("expiration")))
	if err != nil {
		return time.Time{}, newValidationError("expiration must use YYYY-MM-DD format")
	}

	return expiration, nil
}

// parseDurationSession parses the --duration and --session flags together.
// Every order parse function needs both, so this eliminates the repeated pair.
func parseDurationSession(cmd *cli.Command) (models.Duration, models.Session, error) {
	duration, err := parseDuration(cmd.String("duration"))
	if err != nil {
		return "", "", err
	}

	session, err := parseSession(cmd.String("session"))
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
