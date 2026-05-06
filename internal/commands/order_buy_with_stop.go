package commands

import (
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/major/schwab-agent/internal/client"
	"github.com/major/schwab-agent/internal/models"
	"github.com/major/schwab-agent/internal/orderbuilder"
)

// buyWithStopPlaceOpts holds flags shared by buy-with-stop place, build, and
// preview flows. The entry action is intentionally not exposed because this
// workflow is a BUY-only shortcut for opening stock with protective exits.
type buyWithStopPlaceOpts struct {
	Symbol     string           `flag:"symbol"      flagdescr:"Stock symbol (e.g., AAPL)"                                      flagrequired:"true" flagshort:"s"`
	Quantity   float64          `flag:"quantity"    flagdescr:"Share quantity"                                                 flagrequired:"true" flagshort:"q" flaggroup:"execution"`
	Price      float64          `flag:"price"       flagdescr:"Entry limit price (required for LIMIT orders, omit for MARKET)"                     flagshort:"p"`
	StopLoss   float64          `flag:"stop-loss"   flagdescr:"Stop trigger price - becomes market sell when hit"              flagrequired:"true"`
	TakeProfit float64          `flag:"take-profit" flagdescr:"Optional take-profit limit price"`
	Type       models.OrderType `flag:"type"        flagdescr:"Entry order type (LIMIT or MARKET, default LIMIT)"                                  flagshort:"t" flaggroup:"order"`
	Duration   models.Duration  `flag:"duration"    flagdescr:"Entry duration (exit legs are always GTC)"                                          flagshort:"d" flaggroup:"order"`
	Session    models.Session   `flag:"session"     flagdescr:"Trading session"                                                                                  flaggroup:"order"`
}

// parseBuyWithStopParams converts command flags into BUY-with-stop builder params.
func parseBuyWithStopParams(opts *buyWithStopPlaceOpts, _ []string) (*orderbuilder.BuyWithStopParams, error) {
	return &orderbuilder.BuyWithStopParams{
		Symbol:     strings.TrimSpace(opts.Symbol),
		Quantity:   opts.Quantity,
		OrderType:  normalizeOrderType(opts.Type, models.OrderTypeLimit),
		Price:      opts.Price,
		StopLoss:   opts.StopLoss,
		TakeProfit: opts.TakeProfit,
		Duration:   normalizeDuration(opts.Duration),
		Session:    opts.Session,
	}, nil
}

// newBuyWithStopPlaceCmd creates the typed place subcommand for BUY-with-stop orders.
func newBuyWithStopPlaceCmd(c *client.Ref, configPath string, w io.Writer) *cobra.Command {
	cmd := makeCobraPlaceOrderCommand(
		c,
		configPath,
		w,
		"buy-with-stop",
		"Place a buy order with automatic stop-loss protection",
		func() *buyWithStopPlaceOpts { return &buyWithStopPlaceOpts{} },
		func(cmd *cobra.Command, opts *buyWithStopPlaceOpts) { defineAndConstrain(cmd, opts) },
		parseBuyWithStopParams,
		orderbuilder.ValidateBuyWithStopOrder,
		orderbuilder.BuildBuyWithStopOrder,
	)
	applyBuyWithStopHelp(cmd, "place")

	return cmd
}

// newBuyWithStopBuildCmd creates the offline build subcommand for BUY-with-stop orders.
func newBuyWithStopBuildCmd(w io.Writer) *cobra.Command {
	cmd := makeCobraBuildOrderCommand(
		w,
		"buy-with-stop",
		"Build a buy-with-stop order request",
		func() *buyWithStopPlaceOpts { return &buyWithStopPlaceOpts{} },
		func(cmd *cobra.Command, opts *buyWithStopPlaceOpts) { defineAndConstrain(cmd, opts) },
		parseBuyWithStopParams,
		orderbuilder.ValidateBuyWithStopOrder,
		orderbuilder.BuildBuyWithStopOrder,
	)
	applyBuyWithStopHelp(cmd, "build")

	return cmd
}

// newBuyWithStopPreviewCmd creates the typed preview subcommand for BUY-with-stop orders.
func newBuyWithStopPreviewCmd(c *client.Ref, configPath string, w io.Writer) *cobra.Command {
	cmd := makeCobraPreviewOrderCommand(
		c,
		configPath,
		w,
		"buy-with-stop",
		"Preview a buy-with-stop order",
		func() *buyWithStopPlaceOpts { return &buyWithStopPlaceOpts{} },
		func(cmd *cobra.Command, opts *buyWithStopPlaceOpts) { defineAndConstrain(cmd, opts) },
		parseBuyWithStopParams,
		orderbuilder.ValidateBuyWithStopOrder,
		orderbuilder.BuildBuyWithStopOrder,
	)
	applyBuyWithStopHelp(cmd, "preview")

	return cmd
}

// applyBuyWithStopHelp keeps the three buy-with-stop command variants aligned.
func applyBuyWithStopHelp(cmd *cobra.Command, verb string) {
	description := "Preview a buy-with-stop bracket order through the Schwab API without placing it."
	switch verb {
	case "build":
		description = "Build a buy-with-stop bracket order request JSON locally with automatic stop-loss protection."
	case "place":
		description = "Place a buy-with-stop bracket order through the Schwab API with automatic stop-loss protection."
	}

	cmd.Long = description + `
The entry fills first as a TRIGGER parent, then the stop-loss activates automatically.
Optional --take-profit adds a second exit leg, creating an OCO structure where one exit fill cancels the other.
Exit legs are always GOOD_TILL_CANCEL regardless of --duration, which only controls the entry order.
IMPORTANT: Bracket orders cannot be modified via order replace. To adjust, cancel the entire order and re-place with new parameters.
MARKET entry (--type MARKET) skips price validation for stop-loss placement because there is no fixed entry price.`

	cmd.Example = "  # Buy 10 shares of AAPL at $150 with stop-loss at $140\n" +
		"  schwab-agent order " + verb + " buy-with-stop --symbol AAPL --quantity 10 --price 150 --stop-loss 140\n\n" +
		"  # Market entry with stop-loss (no price validation)\n" +
		"  schwab-agent order " + verb + " buy-with-stop --symbol AAPL --quantity 10 --type MARKET --stop-loss 140\n\n" +
		"  # With take-profit (creates OCO exit structure)\n" +
		"  schwab-agent order " + verb + " buy-with-stop --symbol AAPL --quantity 10 --price 150 --stop-loss 140 --take-profit 170"
}
