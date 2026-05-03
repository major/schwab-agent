package commands

import (
	"io"
	"strings"

	"github.com/leodido/structcli"
	"github.com/spf13/cobra"

	"github.com/major/schwab-agent/internal/client"
	"github.com/major/schwab-agent/internal/models"
	"github.com/major/schwab-agent/internal/orderbuilder"
	"github.com/major/schwab-agent/internal/output"
)

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

// orderPlaceOpts holds local flags for top-level spec-based order placement.
type orderPlaceOpts struct {
	Spec string `flag:"spec" flagdescr:"Inline JSON, @file, or - for stdin" flagrequired:"true"`
}

// Attach implements structcli.Options interface.
func (o *orderPlaceOpts) Attach(_ *cobra.Command) error { return nil }

// orderPreviewOpts holds local flags for order preview.
type orderPreviewOpts struct {
	Spec string `flag:"spec" flagdescr:"Inline JSON, @file, or - for stdin" flagrequired:"true"`
}

// Attach implements structcli.Options interface.
func (o *orderPreviewOpts) Attach(_ *cobra.Command) error { return nil }

// orderCancelOpts holds local flags for order cancellation.
type orderCancelOpts struct {
	OrderID string `flag:"order-id" flagdescr:"Order ID"`
}

// Attach implements structcli.Options interface.
func (o *orderCancelOpts) Attach(_ *cobra.Command) error { return nil }

// orderReplaceOpts holds local flags for order replacement.
type orderReplaceOpts struct {
	OrderID string `flag:"order-id" flagdescr:"Order ID"`
}

// Attach implements structcli.Options interface.
func (o *orderReplaceOpts) Attach(_ *cobra.Command) error { return nil }

// newOrderPlaceCmd places new orders from either flags or a JSON spec.
func newOrderPlaceCmd(c *client.Ref, configPath string, w io.Writer) *cobra.Command {
	opts := &orderPlaceOpts{}
	cmd := &cobra.Command{
		Use:   "place",
		Short: "Place an order",
		Long: `Place an order via subcommand (equity, option, bracket, oco) or from a JSON spec
with --spec. Requires "i-also-like-to-live-dangerously" set to true in config.json.
Recommended workflow: check the price with quote get, build the order JSON with
order build, preview it with order preview, then place.`,
		Example: `  # Place from a JSON file
   schwab-agent order place --spec @order.json
   # Place from stdin (piped from order build)
   schwab-agent order build equity --symbol AAPL --action BUY --quantity 10 --type LIMIT --price 200 | schwab-agent order place --spec -
   # Place from inline JSON
   schwab-agent order place --spec '{"orderType":"LIMIT",...}'`,
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

	equityCmd := makeCobraPlaceOrderCommand(c, configPath, w, "equity", "Place an equity order", func() *equityPlaceOpts { return &equityPlaceOpts{} }, func(cmd *cobra.Command, opts *equityPlaceOpts) { defineAndConstrain(cmd, opts) }, parseEquityParams, orderbuilder.ValidateEquityOrder, orderbuilder.BuildEquityOrder)
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

	optionCmd := makeCobraPlaceOrderCommand(c, configPath, w, "option", "Place an option order", func() *optionPlaceOpts { return &optionPlaceOpts{} }, func(cmd *cobra.Command, opts *optionPlaceOpts) {
		defineAndConstrain(cmd, opts, []string{"call", "put"})
	}, parseOptionParams, orderbuilder.ValidateOptionOrder, orderbuilder.BuildOptionOrder)
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

	bracketCmd := makeCobraPlaceOrderCommand(c, configPath, w, "bracket", "Place a bracket order", func() *bracketPlaceOpts { return &bracketPlaceOpts{} }, func(cmd *cobra.Command, opts *bracketPlaceOpts) { defineAndConstrain(cmd, opts) }, parseBracketParams, orderbuilder.ValidateBracketOrder, orderbuilder.BuildBracketOrder)
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

	ocoCmd := makeCobraPlaceOrderCommand(c, configPath, w, "oco", "Place a one-cancels-other order for an existing position", func() *ocoPlaceOpts { return &ocoPlaceOpts{} }, func(cmd *cobra.Command, opts *ocoPlaceOpts) { defineAndConstrain(cmd, opts) }, parseOCOParams, orderbuilder.ValidateOCOOrder, orderbuilder.BuildOCOOrder)
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
	cmd.SetFlagErrorFunc(normalizeFlagValidationErrorFunc)

	if flagSetup != nil {
		flagSetup(cmd, opts)
	}

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
		Long: `Cancel an existing order by ID. Requires the "i-also-like-to-live-dangerously"
config flag. The order ID can be passed as a positional argument or with
--order-id (flag takes priority).`,
		Example: `  schwab-agent order cancel 1234567890
   schwab-agent order cancel --order-id 1234567890`,
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
accepted. Requires the "i-also-like-to-live-dangerously" config flag. The
original order status becomes REPLACED after the new order is created.`,
		Example: `  schwab-agent order replace 1234567890 --symbol AAPL --action BUY --quantity 10 --type LIMIT --price 155.00 --duration DAY
   schwab-agent order replace --order-id 1234567890 --symbol AAPL --action BUY --quantity 10 --type LIMIT --price 155.00 --duration DAY`,
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
	cmd.SetFlagErrorFunc(normalizeFlagValidationErrorFunc)

	defineAndConstrain(cmd, equityOpts)
	if err := structcli.Define(cmd, opts); err != nil {
		panic(err)
	}

	return cmd
}
