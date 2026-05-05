package commands

import (
	"fmt"
	"io"
	"strings"

	"github.com/leodido/structcli"
	"github.com/spf13/cobra"

	"github.com/major/schwab-agent/internal/client"
	"github.com/major/schwab-agent/internal/models"
	"github.com/major/schwab-agent/internal/orderbuilder"
	"github.com/major/schwab-agent/internal/output"
)

// orderActionData wraps a successful mutable order response.
//
// Schwab's mutation endpoints return little useful body data, so the CLI follows
// up with a GET and returns the canonical order shape agents already consume
// from `order get`. OrderID stays in the wrapper as a small convenience and as a
// fallback for rare Schwab/proxy responses that accept the mutation without an
// order Location header.
type orderActionData struct {
	Action               string               `json:"action"`
	OrderID              int64                `json:"orderId"`
	OriginalOrderID      *int64               `json:"originalOrderId,omitempty"`
	PreviewDigest        string               `json:"previewDigest,omitempty"`
	SubmittedOrder       *models.OrderRequest `json:"submittedOrder,omitempty"`
	Canceled             bool                 `json:"canceled,omitempty"`
	Replaced             bool                 `json:"replaced,omitempty"`
	OrderStatus          *models.OrderStatus  `json:"orderStatus,omitempty"`
	OriginalOrderStatus  *models.OrderStatus  `json:"originalOrderStatus,omitempty"`
	VerificationState    string               `json:"verificationState"`
	VerificationFailures []string             `json:"verificationFailures,omitempty"`
	Order                *models.Order        `json:"order,omitempty"`
	OriginalOrder        *models.Order        `json:"originalOrder,omitempty"`
}

// orderPreviewData wraps an order preview response.
type orderPreviewData struct {
	BuiltOrder    *models.OrderRequest `json:"builtOrder,omitempty"`
	Preview       *models.PreviewOrder `json:"preview"`
	OrderID       *int64               `json:"orderId,omitempty"`
	PreviewDigest *previewDigestData   `json:"previewDigest,omitempty"`
}

// orderPlaceOpts holds local flags for top-level spec-based order placement.
type orderPlaceOpts struct {
	Spec        string `flag:"spec" flagdescr:"Inline JSON, @file, or - for stdin"`
	FromPreview string `flag:"from-preview" flagdescr:"Place the exact order payload saved by order preview --save-preview"`
}

// Attach implements structcli.Options interface.
func (o *orderPlaceOpts) Attach(_ *cobra.Command) error { return nil }

// orderPreviewOpts holds local flags for order preview.
type orderPreviewOpts struct {
	Spec        string `flag:"spec" flagdescr:"Inline JSON, @file, or - for stdin" flagrequired:"true"`
	SavePreview bool   `flag:"save-preview" flagdescr:"Save this preview locally and return a digest for order place --from-preview"`
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

// fetchOrderActionData returns the order details after a successful mutable
// action. The follow-up GET is deliberately best-effort: once Schwab accepts a
// mutation, the CLI must not turn that successful trade action into a command
// failure just because the read-after-write lookup is delayed or unavailable.
func fetchOrderActionData(cmd *cobra.Command, c *client.Ref, account, action string, orderID int64, submittedOrder *models.OrderRequest) (data orderActionData, errs []string) {
	data = orderActionData{
		Action:            action,
		OrderID:           orderID,
		SubmittedOrder:    submittedOrder,
		VerificationState: "unverified",
	}
	if orderID == 0 {
		data.VerificationFailures = []string{"order details unavailable: Schwab accepted the order action but did not return an order ID"}
		return data, data.VerificationFailures
	}

	order, err := c.GetOrder(cmd.Context(), account, orderID)
	if err != nil {
		data.VerificationFailures = []string{fmt.Sprintf("order details unavailable after successful order action: %v", err)}
		return data, data.VerificationFailures
	}

	data.Order = order
	data.OrderStatus = order.Status
	data.VerificationState = "verified"
	return data, nil
}

// fetchReplaceActionData verifies both sides of Schwab's replace workflow when
// the API exposes a distinct replacement order ID. A replace creates a new order
// and marks the original as REPLACED, but Schwab sometimes omits the Location
// header. In that fallback case the client only knows the original ID, so we
// avoid a duplicate GET and report the original order as the best available
// verification target.
func fetchReplaceActionData(cmd *cobra.Command, c *client.Ref, account string, originalOrderID, replacementOrderID int64, submittedOrder *models.OrderRequest) (data orderActionData, errs []string) {
	data, errs = fetchOrderActionData(cmd, c, account, "replace", replacementOrderID, submittedOrder)
	data.Replaced = true
	data.OriginalOrderID = &originalOrderID

	if replacementOrderID == originalOrderID {
		data.OriginalOrder = data.Order
		data.OriginalOrderStatus = data.OrderStatus
		return data, errs
	}

	originalOrder, err := c.GetOrder(cmd.Context(), account, originalOrderID)
	if err != nil {
		failure := fmt.Sprintf("original order details unavailable after successful replace: %v", err)
		data.VerificationFailures = append(data.VerificationFailures, failure)
		data.VerificationState = "partial"
		return data, append(errs, failure)
	}

	data.OriginalOrder = originalOrder
	data.OriginalOrderStatus = originalOrder.Status
	if originalOrder.Status == nil || *originalOrder.Status != models.OrderStatusReplaced {
		status := "missing"
		if originalOrder.Status != nil {
			status = string(*originalOrder.Status)
		}
		failure := fmt.Sprintf("original order status is %s after replace, expected REPLACED", status)
		data.VerificationFailures = append(data.VerificationFailures, failure)
		data.VerificationState = "partial"
		return data, append(errs, failure)
	}

	if data.VerificationState == "verified" {
		return data, errs
	}

	data.VerificationState = "partial"
	return data, errs
}

// writeOrderActionResult emits a normal success envelope when the canonical
// order lookup succeeds, or a partial envelope when Schwab accepted the mutation
// but the follow-up details could not be fetched. That distinction lets agents
// trust the order action occurred while still seeing why `data.order` is absent.
func writeOrderActionResult(w io.Writer, data *orderActionData, errs []string) error {
	metadata := output.NewMetadata()
	if len(errs) > 0 {
		return output.WritePartial(w, data, errs, metadata)
	}

	return output.WriteSuccess(w, data, metadata)
}

type orderPlacePayload struct {
	Account       string
	Order         *models.OrderRequest
	PreviewDigest string
}

// resolveOrderPlacePayload returns the account and order payload for top-level
// placement. `--from-preview` deliberately bypasses spec parsing so the mutable
// submit path reuses the exact payload saved during preview instead of rebuilding
// an order from fresh flags or JSON.
func resolveOrderPlacePayload(cmd *cobra.Command, c *client.Ref, configPath string, opts *orderPlaceOpts) (*orderPlacePayload, error) {
	if strings.TrimSpace(opts.FromPreview) != "" {
		entry, err := loadOrderPreview(opts.FromPreview)
		if err != nil {
			return nil, err
		}

		accountFlag, err := cmd.Flags().GetString("account")
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(accountFlag) != "" {
			account, err := resolveAccount(c, accountFlag, configPath, nil)
			if err != nil {
				return nil, err
			}
			if account != entry.Account {
				return nil, newValidationError("--account does not match the account bound to the preview digest")
			}
		}

		return &orderPlacePayload{Account: entry.Account, Order: entry.Order, PreviewDigest: entry.Digest}, nil
	}

	accountFlag, err := cmd.Flags().GetString("account")
	if err != nil {
		return nil, err
	}
	account, err := resolveAccount(c, accountFlag, configPath, nil)
	if err != nil {
		return nil, err
	}

	order, err := parseSpecOrder(cmd, opts.Spec)
	if err != nil {
		return nil, err
	}
	return &orderPlacePayload{Account: account, Order: order}, nil
}

// writeOrderPreviewResult optionally saves the previewed order to the local
// digest ledger before emitting the standard preview envelope.
func writeOrderPreviewResult(w io.Writer, account string, order *models.OrderRequest, preview *models.PreviewOrder, savePreview bool) error {
	data := orderPreviewData{BuiltOrder: order, Preview: preview, OrderID: preview.OrderID}
	if savePreview {
		digestData, err := saveOrderPreview(account, order, preview)
		if err != nil {
			return err
		}
		data.PreviewDigest = digestData
	}

	return output.WriteSuccess(w, data, output.NewMetadata())
}

// newOrderPlaceCmd places new orders from either flags or a JSON spec.
func newOrderPlaceCmd(c *client.Ref, configPath string, w io.Writer) *cobra.Command {
	opts := &orderPlaceOpts{}
	cmd := &cobra.Command{
		Use:   "place",
		Short: "Place an order",
		Long: `Place an order via subcommand (equity, option, bracket, oco), from a JSON spec
with --spec, or from an exact saved preview with --from-preview. Requires
"i-also-like-to-live-dangerously" set to true in config.json. The safest workflow
is to run order preview --save-preview, inspect the response, then place with the
returned previewDigest.digest value.`,
		Example: `  # Place from a JSON file
  schwab-agent order place --spec @order.json
  # Place the exact payload saved by a previous preview
  schwab-agent order preview equity --account abc123 --symbol AAPL --action BUY --quantity 10 --type LIMIT --price 200 --save-preview
  schwab-agent order place --from-preview <digest>
  # Place from stdin (piped from order build)
  schwab-agent order build equity --symbol AAPL --action BUY --quantity 10 --type LIMIT --price 200 | schwab-agent order place --spec -
  # Place from inline JSON
  schwab-agent order place --spec '{"orderType":"LIMIT",...}'`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := structcli.Unmarshal(cmd, opts); err != nil {
				return err
			}

			specProvided := strings.TrimSpace(opts.Spec) != ""
			previewDigest := strings.TrimSpace(opts.FromPreview)
			fromPreviewProvided := previewDigest != ""
			if specProvided == fromPreviewProvided {
				return newValidationError("provide exactly one of --spec or --from-preview for `order place` without a subcommand")
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

			payload, err := resolveOrderPlacePayload(cmd, c, configFlag, opts)
			if err != nil {
				return err
			}
			if err := orderbuilder.ValidateOrderRequest(payload.Order); err != nil {
				return err
			}

			response, err := c.PlaceOrder(cmd.Context(), payload.Account, payload.Order)
			if err != nil {
				return err
			}

			data, errs := fetchOrderActionData(cmd, c, payload.Account, "place", response.OrderID, payload.Order)
			data.PreviewDigest = payload.PreviewDigest
			return writeOrderActionResult(w, &data, errs)
		},
	}

	cmd.SetFlagErrorFunc(suggestSubcommands)
	if err := structcli.Define(cmd, opts); err != nil {
		panic(err)
	}
	cmd.MarkFlagsMutuallyExclusive("spec", "from-preview")

	equityCmd := makeCobraPlaceOrderCommand(c, configPath, w, "equity", "Place an equity order", func() *equityPlaceOpts { return &equityPlaceOpts{} }, func(cmd *cobra.Command, opts *equityPlaceOpts) { defineAndConstrain(cmd, opts) }, parseEquityParams, orderbuilder.ValidateEquityOrder, orderbuilder.BuildEquityOrder)
	equityCmd.Long = `Place an equity (stock) order. Supports MARKET, LIMIT, STOP, STOP_LIMIT, and
TRAILING_STOP order types. Default type is MARKET if --type is omitted. Duration
aliases GTC, FOK, and IOC are accepted alongside their full names. Requires
i-also-like-to-live-dangerously in config for placement.`
	equityCmd.Example = `  # Buy 10 shares at market price
  schwab-agent order place equity --symbol AAPL --action BUY --quantity 10
  # Buy with a limit price, good till cancel
  schwab-agent order place equity --symbol AAPL --action BUY --quantity 10 --type LIMIT --price 150 --duration GTC
  # Sell with a trailing stop ($2.50 offset)
  schwab-agent order place equity --symbol AAPL --action SELL --quantity 10 --type TRAILING_STOP --stop-offset 2.50
  # Sell with a stop-limit order
  schwab-agent order place equity --symbol AAPL --action SELL --quantity 10 --type STOP_LIMIT --stop-price 145 --price 144`

	optionCmd := makeCobraPlaceOrderCommand(c, configPath, w, "option", "Place an option order", func() *optionPlaceOpts { return &optionPlaceOpts{} }, func(cmd *cobra.Command, opts *optionPlaceOpts) {
		defineAndConstrain(cmd, opts, []string{"call", "put"})
	}, parseOptionParams, orderbuilder.ValidateOptionOrder, orderbuilder.BuildOptionOrder)
	optionCmd.Long = `Place a single-leg option order. Requires --underlying, --expiration, --strike,
and exactly one of --call or --put. Use BUY_TO_OPEN/SELL_TO_CLOSE for new
positions and SELL_TO_OPEN/BUY_TO_CLOSE for existing ones. Requires
i-also-like-to-live-dangerously in config for placement.`
	optionCmd.Example = `  # Buy a call option to open
  schwab-agent order place option --underlying AAPL --expiration 2025-06-20 --strike 200 --call --action BUY_TO_OPEN --quantity 1
  # Sell a put at a limit price
  schwab-agent order place option --underlying AAPL --expiration 2025-06-20 --strike 190 --put --action SELL_TO_OPEN --quantity 1 --type LIMIT --price 3.50
  # Close an existing call position
  schwab-agent order place option --underlying AAPL --expiration 2025-06-20 --strike 200 --call --action SELL_TO_CLOSE --quantity 1`

	bracketCmd := makeCobraPlaceOrderCommand(c, configPath, w, "bracket", "Place a bracket order", func() *bracketPlaceOpts { return &bracketPlaceOpts{} }, func(cmd *cobra.Command, opts *bracketPlaceOpts) { defineAndConstrain(cmd, opts) }, parseBracketParams, orderbuilder.ValidateBracketOrder, orderbuilder.BuildBracketOrder)
	bracketCmd.Long = `Place a bracket order that combines an entry trade with automatic exit orders.
At least one of --take-profit or --stop-loss is required. Exit instructions are
auto-inverted from the entry action (BUY entry creates SELL exits). Canceling
the parent cascades to all child orders.`
	bracketCmd.Example = `  # Buy with both take-profit and stop-loss
  schwab-agent order place bracket --symbol NVDA --action BUY --quantity 10 --type MARKET --take-profit 150 --stop-loss 120
  # Buy with only a stop-loss safety net
  schwab-agent order place bracket --symbol AAPL --action BUY --quantity 10 --type LIMIT --price 180 --stop-loss 170
  # Buy with only a take-profit target
  schwab-agent order place bracket --symbol TSLA --action BUY --quantity 5 --type MARKET --take-profit 300`

	ocoCmd := makeCobraPlaceOrderCommand(c, configPath, w, "oco", "Place a one-cancels-other order for an existing position", func() *ocoPlaceOpts { return &ocoPlaceOpts{} }, func(cmd *cobra.Command, opts *ocoPlaceOpts) { defineAndConstrain(cmd, opts) }, parseOCOParams, orderbuilder.ValidateOCOOrder, orderbuilder.BuildOCOOrder)
	ocoCmd.Long = `Place a one-cancels-other order for an existing position. When one exit fills,
the other is automatically canceled. At least one of --take-profit or --stop-loss
is required. For long positions use SELL; for short positions use BUY. Unlike
bracket orders, OCO has no entry leg.`
	ocoCmd.Example = `  # Set take-profit and stop-loss for a long position
  schwab-agent order place oco --symbol AAPL --action SELL --quantity 100 --take-profit 160 --stop-loss 140
  # Protect a position with only a stop-loss
  schwab-agent order place oco --symbol AAPL --action SELL --quantity 50 --stop-loss 140
  # Close a short position with exits
  schwab-agent order place oco --symbol TSLA --action BUY --quantity 10 --take-profit 200 --stop-loss 250`

	cmd.AddCommand(equityCmd, optionCmd, bracketCmd, ocoCmd, newBuyWithStopPlaceCmd(c, configPath, w))

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
			// Resolve --instruction/--order-type aliases before Unmarshal
			// picks up flag values into the opts struct.
			if err := resolveOrderFlagAliasesViaFlags(cmd); err != nil {
				return err
			}

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

			account, err := resolveAccount(c, accountFlag, configFlag, nil)
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

			data, errs := fetchOrderActionData(cmd, c, account, "place", response.OrderID, order)
			return writeOrderActionResult(w, &data, errs)
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
		Short: "Preview an order from JSON spec or typed flags",
		Long: `Preview an order from a JSON spec or typed subcommand flags without placing it.
Typed preview subcommands reuse the same local builders as order place, then
return both the built order request and Schwab preview response in one envelope.
This removes the build-then-preview round trip while keeping placement explicit.
Use --save-preview to store the exact reviewed payload locally and return a
previewDigest.digest value for order place --from-preview. Does not require
safety guards since no order is actually placed.`,
		Example: `  schwab-agent order preview --spec @order.json
  schwab-agent order preview equity --symbol AAPL --action BUY --quantity 10 --type LIMIT --price 200 --save-preview
  schwab-agent order preview option --underlying AAPL --expiration 2026-06-20 --strike 200 --call --action BUY_TO_OPEN --quantity 1 --type LIMIT --price 5.00
  schwab-agent order build equity --symbol AAPL --action BUY --quantity 10 --type LIMIT --price 200 | schwab-agent order preview --spec -`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := structcli.Unmarshal(cmd, opts); err != nil {
				return err
			}

			if strings.TrimSpace(opts.Spec) == "" {
				return newValidationError("spec is required")
			}

			configFlag, err := cmd.Flags().GetString("config")
			if err != nil {
				return err
			}
			if configFlag == "" {
				configFlag = configPath
			}

			accountFlag, err := cmd.Flags().GetString("account")
			if err != nil {
				return err
			}

			account, err := resolveAccount(c, accountFlag, configFlag, nil)
			if err != nil {
				return err
			}

			order, err := parseSpecOrder(cmd, opts.Spec)
			if err != nil {
				return err
			}
			if err := orderbuilder.ValidateOrderRequest(order); err != nil {
				return err
			}

			preview, err := c.PreviewOrder(cmd.Context(), account, order)
			if err != nil {
				return err
			}

			return writeOrderPreviewResult(w, account, order, preview, opts.SavePreview)
		},
	}

	if err := structcli.Define(cmd, opts); err != nil {
		panic(err)
	}

	equityCmd := makeCobraPreviewOrderCommand(c, configPath, w, "equity", "Preview an equity order", func() *equityPlaceOpts { return &equityPlaceOpts{} }, func(cmd *cobra.Command, opts *equityPlaceOpts) { defineAndConstrain(cmd, opts) }, parseEquityParams, orderbuilder.ValidateEquityOrder, orderbuilder.BuildEquityOrder)
	equityCmd.Long = `Preview an equity (stock) order without placing it. Supports the same flags
as order place equity, but skips the mutable-operation safety gate because no
order is submitted. The response includes the built order request plus Schwab's
preview details so agents can inspect both in one call. Add --save-preview to
return a digest for exact-payload placement.`
	equityCmd.Example = `  schwab-agent order preview equity --symbol AAPL --action BUY --quantity 10
	  schwab-agent order preview equity --symbol AAPL --action BUY --quantity 10 --type LIMIT --price 150 --duration GTC`

	optionCmd := makeCobraPreviewOrderCommand(c, configPath, w, "option", "Preview an option order", func() *optionPlaceOpts { return &optionPlaceOpts{} }, func(cmd *cobra.Command, opts *optionPlaceOpts) {
		defineAndConstrain(cmd, opts, []string{"call", "put"})
	}, parseOptionParams, orderbuilder.ValidateOptionOrder, orderbuilder.BuildOptionOrder)
	optionCmd.Long = `Preview a single-leg option order without placing it. Requires --underlying,
--expiration, --strike, and exactly one of --call or --put. The response includes
the locally built OCC order request and Schwab's preview response. Add
--save-preview to return a digest for exact-payload placement.`
	optionCmd.Example = `  schwab-agent order preview option --underlying AAPL --expiration 2025-06-20 --strike 200 --call --action BUY_TO_OPEN --quantity 1
	  schwab-agent order preview option --underlying AAPL --expiration 2025-06-20 --strike 190 --put --action SELL_TO_OPEN --quantity 1 --type LIMIT --price 3.50`

	bracketCmd := makeCobraPreviewOrderCommand(c, configPath, w, "bracket", "Preview a bracket order", func() *bracketPlaceOpts { return &bracketPlaceOpts{} }, func(cmd *cobra.Command, opts *bracketPlaceOpts) { defineAndConstrain(cmd, opts) }, parseBracketParams, orderbuilder.ValidateBracketOrder, orderbuilder.BuildBracketOrder)
	bracketCmd.Long = `Preview a bracket order without placing it. At least one of --take-profit or
--stop-loss is required. The preview response includes the locally built trigger
order and Schwab's validation, fee, and commission details.`
	bracketCmd.Example = `  schwab-agent order preview bracket --symbol NVDA --action BUY --quantity 10 --type MARKET --take-profit 150 --stop-loss 120
	  schwab-agent order preview bracket --symbol AAPL --action BUY --quantity 10 --type LIMIT --price 180 --stop-loss 170`

	ocoCmd := makeCobraPreviewOrderCommand(c, configPath, w, "oco", "Preview a one-cancels-other order", func() *ocoPlaceOpts { return &ocoPlaceOpts{} }, func(cmd *cobra.Command, opts *ocoPlaceOpts) { defineAndConstrain(cmd, opts) }, parseOCOParams, orderbuilder.ValidateOCOOrder, orderbuilder.BuildOCOOrder)
	ocoCmd.Long = `Preview a one-cancels-other order for an existing position without placing it.
When both exits are present, the built order shows the OCO relationship Schwab
will validate during preview.`
	ocoCmd.Example = `  schwab-agent order preview oco --symbol AAPL --action SELL --quantity 100 --take-profit 160 --stop-loss 140
	  schwab-agent order preview oco --symbol TSLA --action BUY --quantity 10 --stop-loss 250`

	cmd.AddCommand(equityCmd, optionCmd, bracketCmd, ocoCmd, newBuyWithStopPreviewCmd(c, configPath, w))

	return cmd
}

// makeCobraPreviewOrderCommand creates a typed preview subcommand that mirrors
// the place subcommand parse/validate/build pipeline without crossing the
// mutable-operation safety boundary. Preview still calls Schwab, but it never
// submits an order, so agents can collapse build + preview into one CLI call.
func makeCobraPreviewOrderCommand[O any, P any](
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
			// Resolve --instruction/--order-type aliases before Unmarshal
			// picks up flag values into the opts struct.
			if err := resolveOrderFlagAliasesViaFlags(cmd); err != nil {
				return err
			}

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

			accountFlag, err := cmd.Flags().GetString("account")
			if err != nil {
				return err
			}

			account, err := resolveAccount(c, accountFlag, configFlag, nil)
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

			preview, err := c.PreviewOrder(cmd.Context(), account, order)
			if err != nil {
				return err
			}

			savePreview, err := cmd.Flags().GetBool("save-preview")
			if err != nil {
				return err
			}
			return writeOrderPreviewResult(w, account, order, preview, savePreview)
		},
	}
	cmd.SetFlagErrorFunc(normalizeFlagValidationErrorFunc)

	if flagSetup != nil {
		flagSetup(cmd, opts)
	}
	cmd.Flags().Bool("save-preview", false, "Save this preview locally and return a digest for order place --from-preview")

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

			account, err := resolveAccount(c, accountFlag, configFlag, nil)
			if err != nil {
				return err
			}

			if err := c.CancelOrder(cmd.Context(), account, orderID); err != nil {
				return err
			}

			data, errs := fetchOrderActionData(cmd, c, account, "cancel", orderID, nil)
			data.Canceled = true
			return writeOrderActionResult(w, &data, errs)
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
			// Resolve --instruction/--order-type aliases before Unmarshal
			// picks up flag values into the opts structs.
			if err := resolveOrderFlagAliasesViaFlags(cmd); err != nil {
				return err
			}

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

			account, err := resolveAccount(c, accountFlag, configFlag, nil)
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

			response, err := c.ReplaceOrder(cmd.Context(), account, orderID, order)
			if err != nil {
				return err
			}

			data, errs := fetchReplaceActionData(cmd, c, account, orderID, response.OrderID, order)
			return writeOrderActionResult(w, &data, errs)
		},
	}
	cmd.SetFlagErrorFunc(normalizeFlagValidationErrorFunc)

	defineAndConstrain(cmd, equityOpts)
	if err := structcli.Define(cmd, opts); err != nil {
		panic(err)
	}

	cmd.AddCommand(newOrderReplaceOptionCmd(c, configPath, w))

	return cmd
}

// newOrderReplaceOptionCmd replaces an existing order with a new single-leg
// option payload built from structured contract flags.
func newOrderReplaceOptionCmd(c *client.Ref, configPath string, w io.Writer) *cobra.Command {
	opts := &orderReplaceOpts{}
	optionOpts := &optionPlaceOpts{}
	cmd := &cobra.Command{
		Use:   "option [order-id]",
		Short: "Replace an order with a new option order spec",
		Long: `Replace an existing order with a new single-leg option order. The option
contract is built from --underlying, --expiration, --strike, and exactly one of
--call or --put, then submitted through Schwab's replace endpoint. Requires the
"i-also-like-to-live-dangerously" config flag.`,
		Example: `  schwab-agent order replace option 1234567890 --underlying AAPL --expiration 2026-06-19 --strike 200 --call --action BUY_TO_OPEN --quantity 1 --type LIMIT --price 5.00
   schwab-agent order replace option --order-id 1234567890 --underlying AAPL --expiration 2026-06-19 --strike 190 --put --instruction SELL_TO_OPEN --quantity 1 --order-type LIMIT --price 3.50`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := structcli.Unmarshal(cmd, opts); err != nil {
				return err
			}
			if err := structcli.Unmarshal(cmd, optionOpts); err != nil {
				return err
			}
			if err := resolveOrderFlagAliases(cmd, &optionOpts.Action, &optionOpts.Type); err != nil {
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

			account, err := resolveAccount(c, accountFlag, configFlag, nil)
			if err != nil {
				return err
			}

			occSymbol, err := buildOCCSymbol(optionOpts.Underlying, optionOpts.Expiration, optionOpts.Strike, optionOpts.Call, optionOpts.Put)
			if err != nil {
				return err
			}

			params, err := parseOptionParams(optionOpts, args)
			if err != nil {
				return err
			}

			if err := orderbuilder.ValidateOptionOrder(params); err != nil {
				return err
			}

			order, err := orderbuilder.BuildOptionOrder(params)
			if err != nil {
				return err
			}
			// Keep the replacement path explicitly tied to the shared OCC builder so
			// future option-build changes cannot accidentally drift from symbol build/parse.
			order.OrderLegCollection[0].Instrument.Symbol = occSymbol

			response, err := c.ReplaceOrder(cmd.Context(), account, orderID, order)
			if err != nil {
				return err
			}

			data, errs := fetchReplaceActionData(cmd, c, account, orderID, response.OrderID, order)
			return writeOrderActionResult(w, &data, errs)
		},
	}
	cmd.SetFlagErrorFunc(normalizeFlagValidationErrorFunc)

	defineAndConstrain(cmd, optionOpts, []string{"call", "put"})
	if err := structcli.Define(cmd, opts); err != nil {
		panic(err)
	}

	return cmd
}
