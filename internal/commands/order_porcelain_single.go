package commands

import (
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/major/schwab-agent/internal/client"
	"github.com/major/schwab-agent/internal/models"
	"github.com/major/schwab-agent/internal/orderbuilder"
)

// singleLegOpts holds flags for porcelain single-leg option commands.
// Direction (action) and contract type (call/put) are hardcoded by the command
// name, so no --action, --call, or --put flags are exposed.
type singleLegOpts struct {
	Underlying string           `flag:"underlying" flagdescr:"Underlying symbol"            flagrequired:"true" flaggroup:"contract"`
	Expiration string           `flag:"expiration" flagdescr:"Expiration date (YYYY-MM-DD)" flagrequired:"true" flaggroup:"contract"`
	Strike     float64          `flag:"strike"     flagdescr:"Strike price"                 flagrequired:"true" flaggroup:"contract"`
	Quantity   float64          `flag:"quantity"   flagdescr:"Contract quantity"            flagrequired:"true" flaggroup:"execution"`
	Type       models.OrderType `flag:"type"       flagdescr:"Order type"                                       flaggroup:"order"`
	Price      float64          `flag:"price"      flagdescr:"Limit price"                                      flaggroup:"pricing"`
	Duration   models.Duration  `flag:"duration"   flagdescr:"Order duration"                                   flaggroup:"order"`
	Session    models.Session   `flag:"session"    flagdescr:"Trading session"                                  flaggroup:"order"`
}

// singleLegConfig defines a porcelain single-leg command variant. Each config
// hardcodes the put/call type and instruction so the caller cannot accidentally
// pick the wrong direction.
type singleLegConfig struct {
	name    string
	short   string
	long    string
	example string
	putCall models.PutCall
	action  models.Instruction
}

// parseSingleLegFactory returns a parse function that bakes in the put/call type
// and instruction determined by the command name.
func parseSingleLegFactory(
	putCall models.PutCall,
	action models.Instruction,
) func(*singleLegOpts, []string) (*orderbuilder.OptionParams, error) {
	return func(opts *singleLegOpts, _ []string) (*orderbuilder.OptionParams, error) {
		expiration, err := parseExpiration(opts.Expiration)
		if err != nil {
			return nil, err
		}

		return &orderbuilder.OptionParams{
			Underlying: strings.TrimSpace(opts.Underlying),
			Expiration: expiration,
			Strike:     opts.Strike,
			PutCall:    putCall,
			Action:     action,
			Quantity:   opts.Quantity,
			OrderType:  normalizeOrderType(opts.Type, models.OrderTypeMarket),
			Price:      opts.Price,
			Duration:   normalizeDuration(opts.Duration),
			Session:    opts.Session,
		}, nil
	}
}

// singleLegConfigs returns all porcelain single-leg command variants.
func singleLegConfigs() []singleLegConfig {
	return []singleLegConfig{
		{
			name:    "long-call",
			short:   "Buy a call option (long call)",
			putCall: models.PutCallCall,
			action:  models.InstructionBuyToOpen,
			long: `Buy a call option to open a long call position. The command hardcodes
BUY_TO_OPEN and CALL so only contract details and quantity are needed.
Use this when you are bullish on the underlying and want leveraged
upside exposure with limited downside risk (premium paid).`,
			example: `  schwab-agent order build long-call --underlying AAPL --expiration 2026-06-19 --strike 200 --quantity 1
  schwab-agent order build long-call --underlying AAPL --expiration 2026-06-19 --strike 200 --quantity 1 --type LIMIT --price 5.00
  schwab-agent order place long-call --underlying AAPL --expiration 2026-06-19 --strike 200 --quantity 1`,
		},
		{
			name:    "long-put",
			short:   "Buy a put option (long put)",
			putCall: models.PutCallPut,
			action:  models.InstructionBuyToOpen,
			long: `Buy a put option to open a long put position. The command hardcodes
BUY_TO_OPEN and PUT so only contract details and quantity are needed.
Use this when you are bearish on the underlying or want downside
protection for an existing stock position.`,
			example: `  schwab-agent order build long-put --underlying AAPL --expiration 2026-06-19 --strike 190 --quantity 1
  schwab-agent order build long-put --underlying AAPL --expiration 2026-06-19 --strike 190 --quantity 1 --type LIMIT --price 3.00
  schwab-agent order place long-put --underlying AAPL --expiration 2026-06-19 --strike 190 --quantity 1`,
		},
		{
			name:    "cash-secured-put",
			short:   "Sell a put option (cash-secured put)",
			putCall: models.PutCallPut,
			action:  models.InstructionSellToOpen,
			long: `Sell a put option to open a cash-secured put position. The command hardcodes
SELL_TO_OPEN and PUT so only contract details and quantity are needed.
Use this when you are neutral-to-bullish and willing to buy the underlying
at the strike price. Requires cash collateral equal to the assignment value.`,
			example: `  schwab-agent order build cash-secured-put --underlying AAPL --expiration 2026-06-19 --strike 180 --quantity 1
  schwab-agent order build cash-secured-put --underlying AAPL --expiration 2026-06-19 --strike 180 --quantity 1 --type LIMIT --price 2.50
  schwab-agent order place cash-secured-put --underlying AAPL --expiration 2026-06-19 --strike 180 --quantity 1`,
		},
		{
			name:    "naked-call",
			short:   "Sell a call option (naked call)",
			putCall: models.PutCallCall,
			action:  models.InstructionSellToOpen,
			long: `Sell a call option to open a naked (uncovered) call position. The command
hardcodes SELL_TO_OPEN and CALL so only contract details and quantity are
needed. Use this when you are bearish or neutral and want to collect premium.
Naked calls carry unlimited upside risk and require margin approval.`,
			example: `  schwab-agent order build naked-call --underlying AAPL --expiration 2026-06-19 --strike 220 --quantity 1
  schwab-agent order build naked-call --underlying AAPL --expiration 2026-06-19 --strike 220 --quantity 1 --type LIMIT --price 4.00
  schwab-agent order place naked-call --underlying AAPL --expiration 2026-06-19 --strike 220 --quantity 1`,
		},
	}
}

// makeSingleLegBuildCmd creates a build subcommand for a single-leg porcelain config.
func makeSingleLegBuildCmd(w io.Writer, cfg singleLegConfig) *cobra.Command {
	cmd := makeCobraBuildOrderCommand(
		w,
		cfg.name,
		cfg.short,
		func() *singleLegOpts { return &singleLegOpts{} },
		func(cmd *cobra.Command, opts *singleLegOpts) { defineAndConstrain(cmd, opts) },
		parseSingleLegFactory(cfg.putCall, cfg.action),
		orderbuilder.ValidateOptionOrder,
		orderbuilder.BuildOptionOrder,
	)
	cmd.Long = cfg.long
	cmd.Example = cfg.example

	return cmd
}

// makeSingleLegPlaceCmd creates a place subcommand for a single-leg porcelain config.
func makeSingleLegPlaceCmd(
	c *client.Ref,
	configPath string,
	w io.Writer,
	cfg singleLegConfig,
) *cobra.Command {
	cmd := makeCobraPlaceOrderCommand(
		c,
		configPath,
		w,
		cfg.name,
		cfg.short,
		func() *singleLegOpts { return &singleLegOpts{} },
		func(cmd *cobra.Command, opts *singleLegOpts) { defineAndConstrain(cmd, opts) },
		parseSingleLegFactory(cfg.putCall, cfg.action),
		orderbuilder.ValidateOptionOrder,
		orderbuilder.BuildOptionOrder,
	)
	cmd.Long = cfg.long
	cmd.Example = cfg.example

	return cmd
}

// makeSingleLegPreviewCmd creates a preview subcommand for a single-leg porcelain config.
func makeSingleLegPreviewCmd(
	c *client.Ref,
	configPath string,
	w io.Writer,
	cfg singleLegConfig,
) *cobra.Command {
	cmd := makeCobraPreviewOrderCommand(
		c,
		configPath,
		w,
		cfg.name,
		cfg.short,
		func() *singleLegOpts { return &singleLegOpts{} },
		func(cmd *cobra.Command, opts *singleLegOpts) { defineAndConstrain(cmd, opts) },
		parseSingleLegFactory(cfg.putCall, cfg.action),
		orderbuilder.ValidateOptionOrder,
		orderbuilder.BuildOptionOrder,
	)
	cmd.Long = cfg.long
	cmd.Example = cfg.example

	return cmd
}

// singleLegBuildCommands returns build commands for all single-leg porcelain variants.
func singleLegBuildCommands(w io.Writer) []*cobra.Command {
	configs := singleLegConfigs()
	cmds := make([]*cobra.Command, 0, len(configs))
	for _, cfg := range configs {
		cmds = append(cmds, makeSingleLegBuildCmd(w, cfg))
	}

	return cmds
}

// singleLegPlaceCommands returns place commands for all single-leg porcelain variants.
func singleLegPlaceCommands(c *client.Ref, configPath string, w io.Writer) []*cobra.Command {
	configs := singleLegConfigs()
	cmds := make([]*cobra.Command, 0, len(configs))
	for _, cfg := range configs {
		cmds = append(cmds, makeSingleLegPlaceCmd(c, configPath, w, cfg))
	}

	return cmds
}

// singleLegPreviewCommands returns preview commands for all single-leg porcelain variants.
func singleLegPreviewCommands(c *client.Ref, configPath string, w io.Writer) []*cobra.Command {
	configs := singleLegConfigs()
	cmds := make([]*cobra.Command, 0, len(configs))
	for _, cfg := range configs {
		cmds = append(cmds, makeSingleLegPreviewCmd(c, configPath, w, cfg))
	}

	return cmds
}
