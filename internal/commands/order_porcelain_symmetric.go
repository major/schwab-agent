package commands

import (
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/major/schwab-agent/internal/client"
	"github.com/major/schwab-agent/internal/models"
	"github.com/major/schwab-agent/internal/orderbuilder"
)

// porcelainStraddleOpts holds flags for porcelain straddle commands.
// Direction (buy/sell) and position opening are hardcoded by the command name,
// so no --buy, --sell, --open, or --close flags are exposed.
type porcelainStraddleOpts struct {
	Underlying string          `flag:"underlying" flagdescr:"Underlying symbol"                          flagrequired:"true" flaggroup:"contract"`
	Expiration string          `flag:"expiration" flagdescr:"Expiration date (YYYY-MM-DD)"               flagrequired:"true" flaggroup:"contract"`
	Strike     float64         `flag:"strike"     flagdescr:"Strike price (shared by call and put legs)" flagrequired:"true" flaggroup:"contract"`
	Quantity   float64         `flag:"quantity"   flagdescr:"Number of contracts"                        flagrequired:"true" flaggroup:"execution"`
	Price      float64         `flag:"price"      flagdescr:"Net debit or credit amount"                 flagrequired:"true" flaggroup:"pricing"`
	Duration   models.Duration `flag:"duration"   flagdescr:"Order duration"                                                 flaggroup:"order"`
	Session    models.Session  `flag:"session"    flagdescr:"Trading session"                                                flaggroup:"order"`
}

// porcelainStrangleOpts holds flags for porcelain strangle commands.
// Direction (buy/sell) and position opening are hardcoded by the command name,
// so no --buy, --sell, --open, or --close flags are exposed.
type porcelainStrangleOpts struct {
	Underlying string          `flag:"underlying"  flagdescr:"Underlying symbol"             flagrequired:"true" flaggroup:"contract"`
	Expiration string          `flag:"expiration"  flagdescr:"Expiration date (YYYY-MM-DD)"  flagrequired:"true" flaggroup:"contract"`
	CallStrike float64         `flag:"call-strike" flagdescr:"Strike price for the call leg" flagrequired:"true" flaggroup:"contract"`
	PutStrike  float64         `flag:"put-strike"  flagdescr:"Strike price for the put leg"  flagrequired:"true" flaggroup:"contract"`
	Quantity   float64         `flag:"quantity"    flagdescr:"Number of contracts"           flagrequired:"true" flaggroup:"execution"`
	Price      float64         `flag:"price"       flagdescr:"Net debit or credit amount"    flagrequired:"true" flaggroup:"pricing"`
	Duration   models.Duration `flag:"duration"    flagdescr:"Order duration"                                    flaggroup:"order"`
	Session    models.Session  `flag:"session"     flagdescr:"Trading session"                                   flaggroup:"order"`
}

// parsePorcelainStraddleFactory returns a parse function that bakes in the
// buy/sell direction determined by the command name. All porcelain straddle
// commands open positions (Open=true).
func parsePorcelainStraddleFactory(
	isBuy bool,
) func(*porcelainStraddleOpts, []string) (*orderbuilder.StraddleParams, error) {
	return func(opts *porcelainStraddleOpts, _ []string) (*orderbuilder.StraddleParams, error) {
		expiration, err := parseExpiration(opts.Expiration)
		if err != nil {
			return nil, err
		}

		return &orderbuilder.StraddleParams{
			Underlying: strings.TrimSpace(opts.Underlying),
			Expiration: expiration,
			Strike:     opts.Strike,
			Buy:        isBuy,
			Open:       true, // Porcelain straddles always open.
			Quantity:   opts.Quantity,
			Price:      opts.Price,
			Duration:   normalizeDuration(opts.Duration),
			Session:    opts.Session,
		}, nil
	}
}

// parsePorcelainStrangleFactory returns a parse function that bakes in the
// buy/sell direction determined by the command name. All porcelain strangle
// commands open positions (Open=true).
func parsePorcelainStrangleFactory(
	isBuy bool,
) func(*porcelainStrangleOpts, []string) (*orderbuilder.StrangleParams, error) {
	return func(opts *porcelainStrangleOpts, _ []string) (*orderbuilder.StrangleParams, error) {
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
			Open:       true, // Porcelain strangles always open.
			Quantity:   opts.Quantity,
			Price:      opts.Price,
			Duration:   normalizeDuration(opts.Duration),
			Session:    opts.Session,
		}, nil
	}
}

// porcelainStraddleBuildCmds returns build commands for long and short straddle.
func porcelainStraddleBuildCmds(w io.Writer) []*cobra.Command {
	longStraddle := makeCobraBuildOrderCommand(
		w,
		"long-straddle",
		"Buy a straddle (long call + long put at same strike)",
		func() *porcelainStraddleOpts { return &porcelainStraddleOpts{} },
		func(cmd *cobra.Command, opts *porcelainStraddleOpts) { defineAndConstrain(cmd, opts) },
		parsePorcelainStraddleFactory(true),
		orderbuilder.ValidateStraddleOrder,
		orderbuilder.BuildStraddleOrder,
	)
	longStraddle.Long = `Buy both a call and a put at the same strike to open a long straddle.
The command hardcodes BUY and OPEN so only the strike and quantity are needed.
Use this when you expect a large price move but are unsure of direction.
Maximum loss is the debit paid; profit is unlimited on the upside.`
	longStraddle.Example = `  schwab-agent order build long-straddle --underlying AAPL --expiration 2026-06-19 --strike 200 --quantity 1 --price 8.00
  schwab-agent order place long-straddle --underlying AAPL --expiration 2026-06-19 --strike 200 --quantity 1 --price 8.00`

	shortStraddle := makeCobraBuildOrderCommand(
		w,
		"short-straddle",
		"Sell a straddle (short call + short put at same strike)",
		func() *porcelainStraddleOpts { return &porcelainStraddleOpts{} },
		func(cmd *cobra.Command, opts *porcelainStraddleOpts) { defineAndConstrain(cmd, opts) },
		parsePorcelainStraddleFactory(false),
		orderbuilder.ValidateStraddleOrder,
		orderbuilder.BuildStraddleOrder,
	)
	shortStraddle.Long = `Sell both a call and a put at the same strike to open a short straddle.
The command hardcodes SELL and OPEN so only the strike and quantity are needed.
Use this when you expect the underlying to stay near the strike through
expiration. Maximum profit is the credit received; risk is unlimited.`
	shortStraddle.Example = `  schwab-agent order build short-straddle --underlying AAPL --expiration 2026-06-19 --strike 200 --quantity 1 --price 8.00
  schwab-agent order place short-straddle --underlying AAPL --expiration 2026-06-19 --strike 200 --quantity 1 --price 8.00`

	return []*cobra.Command{longStraddle, shortStraddle}
}

// porcelainStrangleBuildCmds returns build commands for long and short strangle.
func porcelainStrangleBuildCmds(w io.Writer) []*cobra.Command {
	longStrangle := makeCobraBuildOrderCommand(
		w,
		"long-strangle",
		"Buy a strangle (long call + long put at different strikes)",
		func() *porcelainStrangleOpts { return &porcelainStrangleOpts{} },
		func(cmd *cobra.Command, opts *porcelainStrangleOpts) { defineAndConstrain(cmd, opts) },
		parsePorcelainStrangleFactory(true),
		orderbuilder.ValidateStrangleOrder,
		orderbuilder.BuildStrangleOrder,
	)
	longStrangle.Long = `Buy a call and a put at different strikes to open a long strangle.
The command hardcodes BUY and OPEN so only the strikes and quantity are needed.
Use this when you expect a large price move but are unsure of direction.
Cheaper than a straddle but requires a bigger move to be profitable.`
	longStrangle.Example = `  schwab-agent order build long-strangle --underlying AAPL --expiration 2026-06-19 --call-strike 210 --put-strike 190 --quantity 1 --price 5.00
  schwab-agent order place long-strangle --underlying AAPL --expiration 2026-06-19 --call-strike 210 --put-strike 190 --quantity 1 --price 5.00`

	shortStrangle := makeCobraBuildOrderCommand(
		w,
		"short-strangle",
		"Sell a strangle (short call + short put at different strikes)",
		func() *porcelainStrangleOpts { return &porcelainStrangleOpts{} },
		func(cmd *cobra.Command, opts *porcelainStrangleOpts) { defineAndConstrain(cmd, opts) },
		parsePorcelainStrangleFactory(false),
		orderbuilder.ValidateStrangleOrder,
		orderbuilder.BuildStrangleOrder,
	)
	shortStrangle.Long = `Sell a call and a put at different strikes to open a short strangle.
The command hardcodes SELL and OPEN so only the strikes and quantity are needed.
Use this when you expect the underlying to stay between the two strikes
through expiration. Maximum profit is the credit received; risk is unlimited.`
	shortStrangle.Example = `  schwab-agent order build short-strangle --underlying AAPL --expiration 2026-06-19 --call-strike 210 --put-strike 190 --quantity 1 --price 5.00
  schwab-agent order place short-strangle --underlying AAPL --expiration 2026-06-19 --call-strike 210 --put-strike 190 --quantity 1 --price 5.00`

	return []*cobra.Command{longStrangle, shortStrangle}
}

// porcelainSymmetricBuildCommands returns all porcelain straddle and strangle build commands.
func porcelainSymmetricBuildCommands(w io.Writer) []*cobra.Command {
	cmds := porcelainStraddleBuildCmds(w)
	return append(cmds, porcelainStrangleBuildCmds(w)...)
}

// porcelainStraddlePlaceCmds returns place commands for long and short straddle.
//
//nolint:dupl // straddle/strangle place/preview functions are intentionally parallel in structure
func porcelainStraddlePlaceCmds(c *client.Ref, configPath string, w io.Writer) []*cobra.Command {
	longStraddle := makeCobraPlaceOrderCommand(
		c, configPath, w,
		"long-straddle",
		"Place a long straddle order",
		func() *porcelainStraddleOpts { return &porcelainStraddleOpts{} },
		func(cmd *cobra.Command, opts *porcelainStraddleOpts) { defineAndConstrain(cmd, opts) },
		parsePorcelainStraddleFactory(true),
		orderbuilder.ValidateStraddleOrder,
		orderbuilder.BuildStraddleOrder,
	)
	longStraddle.Long = `Place a long straddle order. Buys both a call and a put at the same strike.
The command hardcodes BUY and OPEN so only the strike and quantity are needed.`
	longStraddle.Example = `  schwab-agent order place long-straddle --underlying AAPL --expiration 2026-06-19 --strike 200 --quantity 1 --price 8.00`

	shortStraddle := makeCobraPlaceOrderCommand(
		c, configPath, w,
		"short-straddle",
		"Place a short straddle order",
		func() *porcelainStraddleOpts { return &porcelainStraddleOpts{} },
		func(cmd *cobra.Command, opts *porcelainStraddleOpts) { defineAndConstrain(cmd, opts) },
		parsePorcelainStraddleFactory(false),
		orderbuilder.ValidateStraddleOrder,
		orderbuilder.BuildStraddleOrder,
	)
	shortStraddle.Long = `Place a short straddle order. Sells both a call and a put at the same strike.
The command hardcodes SELL and OPEN so only the strike and quantity are needed.`
	shortStraddle.Example = `  schwab-agent order place short-straddle --underlying AAPL --expiration 2026-06-19 --strike 200 --quantity 1 --price 8.00`

	return []*cobra.Command{longStraddle, shortStraddle}
}

// porcelainStranglePlaceCmds returns place commands for long and short strangle.
//
//nolint:dupl // straddle/strangle place/preview functions are intentionally parallel in structure
func porcelainStranglePlaceCmds(c *client.Ref, configPath string, w io.Writer) []*cobra.Command {
	longStrangle := makeCobraPlaceOrderCommand(
		c, configPath, w,
		"long-strangle",
		"Place a long strangle order",
		func() *porcelainStrangleOpts { return &porcelainStrangleOpts{} },
		func(cmd *cobra.Command, opts *porcelainStrangleOpts) { defineAndConstrain(cmd, opts) },
		parsePorcelainStrangleFactory(true),
		orderbuilder.ValidateStrangleOrder,
		orderbuilder.BuildStrangleOrder,
	)
	longStrangle.Long = `Place a long strangle order. Buys a call and put at different strikes.
The command hardcodes BUY and OPEN so only the strikes and quantity are needed.`
	longStrangle.Example = `  schwab-agent order place long-strangle --underlying AAPL --expiration 2026-06-19 --call-strike 210 --put-strike 190 --quantity 1 --price 5.00`

	shortStrangle := makeCobraPlaceOrderCommand(
		c, configPath, w,
		"short-strangle",
		"Place a short strangle order",
		func() *porcelainStrangleOpts { return &porcelainStrangleOpts{} },
		func(cmd *cobra.Command, opts *porcelainStrangleOpts) { defineAndConstrain(cmd, opts) },
		parsePorcelainStrangleFactory(false),
		orderbuilder.ValidateStrangleOrder,
		orderbuilder.BuildStrangleOrder,
	)
	shortStrangle.Long = `Place a short strangle order. Sells a call and put at different strikes.
The command hardcodes SELL and OPEN so only the strikes and quantity are needed.`
	shortStrangle.Example = `  schwab-agent order place short-strangle --underlying AAPL --expiration 2026-06-19 --call-strike 210 --put-strike 190 --quantity 1 --price 5.00`

	return []*cobra.Command{longStrangle, shortStrangle}
}

// porcelainSymmetricPlaceCommands returns all porcelain straddle and strangle place commands.
func porcelainSymmetricPlaceCommands(
	c *client.Ref,
	configPath string,
	w io.Writer,
) []*cobra.Command {
	cmds := porcelainStraddlePlaceCmds(c, configPath, w)
	return append(cmds, porcelainStranglePlaceCmds(c, configPath, w)...)
}

// porcelainStraddlePreviewCmds returns preview commands for long and short straddle.
//
//nolint:dupl // straddle/strangle place/preview functions are intentionally parallel in structure
func porcelainStraddlePreviewCmds(c *client.Ref, configPath string, w io.Writer) []*cobra.Command {
	longStraddle := makeCobraPreviewOrderCommand(
		c, configPath, w,
		"long-straddle",
		"Preview a long straddle order",
		func() *porcelainStraddleOpts { return &porcelainStraddleOpts{} },
		func(cmd *cobra.Command, opts *porcelainStraddleOpts) { defineAndConstrain(cmd, opts) },
		parsePorcelainStraddleFactory(true),
		orderbuilder.ValidateStraddleOrder,
		orderbuilder.BuildStraddleOrder,
	)
	longStraddle.Long = `Preview a long straddle order without placing it. Shows order details and
Schwab's preview response including estimated commissions and fees.`
	longStraddle.Example = `  schwab-agent order preview long-straddle --underlying AAPL --expiration 2026-06-19 --strike 200 --quantity 1 --price 8.00`

	shortStraddle := makeCobraPreviewOrderCommand(
		c, configPath, w,
		"short-straddle",
		"Preview a short straddle order",
		func() *porcelainStraddleOpts { return &porcelainStraddleOpts{} },
		func(cmd *cobra.Command, opts *porcelainStraddleOpts) { defineAndConstrain(cmd, opts) },
		parsePorcelainStraddleFactory(false),
		orderbuilder.ValidateStraddleOrder,
		orderbuilder.BuildStraddleOrder,
	)
	shortStraddle.Long = `Preview a short straddle order without placing it. Shows order details and
Schwab's preview response including estimated commissions and fees.`
	shortStraddle.Example = `  schwab-agent order preview short-straddle --underlying AAPL --expiration 2026-06-19 --strike 200 --quantity 1 --price 8.00`

	return []*cobra.Command{longStraddle, shortStraddle}
}

// porcelainStranglePreviewCmds returns preview commands for long and short strangle.
//
//nolint:dupl // straddle/strangle place/preview functions are intentionally parallel in structure
func porcelainStranglePreviewCmds(c *client.Ref, configPath string, w io.Writer) []*cobra.Command {
	longStrangle := makeCobraPreviewOrderCommand(
		c, configPath, w,
		"long-strangle",
		"Preview a long strangle order",
		func() *porcelainStrangleOpts { return &porcelainStrangleOpts{} },
		func(cmd *cobra.Command, opts *porcelainStrangleOpts) { defineAndConstrain(cmd, opts) },
		parsePorcelainStrangleFactory(true),
		orderbuilder.ValidateStrangleOrder,
		orderbuilder.BuildStrangleOrder,
	)
	longStrangle.Long = `Preview a long strangle order without placing it. Shows order details and
Schwab's preview response including estimated commissions and fees.`
	longStrangle.Example = `  schwab-agent order preview long-strangle --underlying AAPL --expiration 2026-06-19 --call-strike 210 --put-strike 190 --quantity 1 --price 5.00`

	shortStrangle := makeCobraPreviewOrderCommand(
		c, configPath, w,
		"short-strangle",
		"Preview a short strangle order",
		func() *porcelainStrangleOpts { return &porcelainStrangleOpts{} },
		func(cmd *cobra.Command, opts *porcelainStrangleOpts) { defineAndConstrain(cmd, opts) },
		parsePorcelainStrangleFactory(false),
		orderbuilder.ValidateStrangleOrder,
		orderbuilder.BuildStrangleOrder,
	)
	shortStrangle.Long = `Preview a short strangle order without placing it. Shows order details and
Schwab's preview response including estimated commissions and fees.`
	shortStrangle.Example = `  schwab-agent order preview short-strangle --underlying AAPL --expiration 2026-06-19 --call-strike 210 --put-strike 190 --quantity 1 --price 5.00`

	return []*cobra.Command{longStrangle, shortStrangle}
}

// porcelainSymmetricPreviewCommands returns all porcelain straddle and strangle preview commands.
func porcelainSymmetricPreviewCommands(
	c *client.Ref,
	configPath string,
	w io.Writer,
) []*cobra.Command {
	cmds := porcelainStraddlePreviewCmds(c, configPath, w)
	return append(cmds, porcelainStranglePreviewCmds(c, configPath, w)...)
}
