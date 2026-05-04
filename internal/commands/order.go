package commands

import (
	"io"
	"strings"
	"time"

	"github.com/leodido/structcli"
	"github.com/spf13/cobra"

	"github.com/major/schwab-agent/internal/client"
	"github.com/major/schwab-agent/internal/models"
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

// orderListOpts holds local flags for the order list subcommand.
type orderListOpts struct {
	// Keep status as []string because structcli v0.17 does not support slices of
	// registered custom enum types. RunE still validates values against the same
	// registered enum set after expanding comma-separated repeatable input.
	Status []string `flag:"status" flagdescr:"Filter by order status (repeatable, use 'all' for unfiltered): WORKING, PENDING_ACTIVATION, FILLED, EXPIRED, CANCELED, REJECTED, etc."`
	From   string   `flag:"from" flagdescr:"Filter by entered time lower bound"`
	To     string   `flag:"to" flagdescr:"Filter by entered time upper bound"`
	Recent bool     `flag:"recent" flagdescr:"Show recent order activity, including terminal statuses, from the last 24 hours unless --from is set"`
}

// Attach implements structcli.Options interface.
func (o *orderListOpts) Attach(_ *cobra.Command) error { return nil }

// orderGetOpts holds local flags for the order get subcommand.
type orderGetOpts struct {
	OrderID string `flag:"order-id" flagdescr:"Order ID"`
}

// Attach implements structcli.Options interface.
func (o *orderGetOpts) Attach(_ *cobra.Command) error { return nil }

// NewOrderCmd returns the Cobra command for order operations.
func NewOrderCmd(c *client.Ref, configPath string, w io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "order",
		Short: "List, build, preview, place, cancel, and replace orders",
		Long: `Manage orders across your Schwab accounts. Supports listing, viewing, placing,
previewing, building, canceling, and replacing orders. Placing, canceling, and
replacing orders require the "i-also-like-to-live-dangerously" config flag.
Duration aliases GTC, FOK, and IOC are accepted.`,
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
everything. Use --recent for an agent-friendly recent activity view that keeps
terminal statuses and narrows the default lookback to 24 hours. Multiple
--status values fan out into separate API calls with merged, deduplicated
results.`,
		Example: `  schwab-agent order list
  schwab-agent order list --recent
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

			showAll := opts.Recent && len(statuses) == 0
			for _, s := range statuses {
				if err := validateOrderStatusFilter(s); err != nil {
					return err
				}

				if strings.EqualFold(s, "all") {
					showAll = true
					break
				}
			}

			var apiStatuses []string
			if !showAll {
				apiStatuses = statuses
			}

			from := strings.TrimSpace(opts.From)
			if opts.Recent && from == "" {
				// Recent activity is meant for post-mutation verification. A 24-hour
				// lookback keeps filled/canceled/replaced orders visible without the
				// noisier 60-day default used by the underlying Schwab endpoint.
				from = time.Now().UTC().Add(-24 * time.Hour).Format(time.RFC3339)
			}

			params := client.OrderListParams{
				Statuses:        apiStatuses,
				FromEnteredTime: from,
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

			if len(statuses) == 0 && !opts.Recent {
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

			account, err := resolveAccount(c, accountFlag, configPath, nil)
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
