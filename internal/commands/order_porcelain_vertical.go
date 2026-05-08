package commands

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/major/schwab-agent/internal/client"
	"github.com/major/schwab-agent/internal/models"
	"github.com/major/schwab-agent/internal/orderbuilder"
)

// porcelainVerticalOpts holds flags for porcelain vertical spread commands.
// Uses --high-strike and --low-strike (factual position on the strike ladder)
// instead of --long-strike and --short-strike (directional, requiring the caller
// to know which leg is bought vs sold). The command name determines the mapping.
type porcelainVerticalOpts struct {
	Underlying string          `flag:"underlying"  flagdescr:"Underlying symbol"            flagrequired:"true" flaggroup:"contract"`
	Expiration string          `flag:"expiration"  flagdescr:"Expiration date (YYYY-MM-DD)" flagrequired:"true" flaggroup:"contract"`
	HighStrike float64         `flag:"high-strike" flagdescr:"Higher strike price"          flagrequired:"true" flaggroup:"contract"`
	LowStrike  float64         `flag:"low-strike"  flagdescr:"Lower strike price"           flagrequired:"true" flaggroup:"contract"`
	Quantity   float64         `flag:"quantity"    flagdescr:"Number of contracts"          flagrequired:"true" flaggroup:"execution"`
	Price      float64         `flag:"price"       flagdescr:"Net debit or credit amount"   flagrequired:"true" flaggroup:"pricing"`
	Duration   models.Duration `flag:"duration"    flagdescr:"Order duration"                                   flaggroup:"order"`
	Session    models.Session  `flag:"session"     flagdescr:"Trading session"                                  flaggroup:"order"`
}

// Validate is called by validateCobraOptions after Cobra decodes bound flags.
// Ensures the high strike is strictly greater than the low strike.
func (o *porcelainVerticalOpts) Validate(_ context.Context) []error {
	if o.HighStrike <= o.LowStrike {
		return []error{
			fmt.Errorf("--high-strike (%.2f) must be greater than --low-strike (%.2f)", o.HighStrike, o.LowStrike),
		}
	}
	return nil
}

// porcelainVerticalConfig defines a porcelain vertical spread command variant.
// Each config hardcodes the put/call type and the mapping from high/low strike
// to long/short strike, so the caller cannot accidentally sell the wrong leg.
type porcelainVerticalConfig struct {
	name       string
	short      string
	long       string
	example    string
	putCall    models.PutCall
	longIsHigh bool // true = longStrike is the high strike, false = longStrike is the low strike
}

// parsePorcelainVerticalFactory returns a parse function that maps high/low
// strikes to long/short strikes based on the spread type. All porcelain vertical
// spreads are opening positions (Open=true).
//
// Strike mapping rationale:
//
//	put-credit-spread (bull put):  SELL high put + BUY low put   -> longIsHigh=false
//	call-credit-spread (bear call): SELL low call + BUY high call -> longIsHigh=true
//	put-debit-spread (bear put):  BUY high put + SELL low put   -> longIsHigh=true
//	call-debit-spread (bull call): BUY low call + SELL high call -> longIsHigh=false
func parsePorcelainVerticalFactory(
	putCall models.PutCall,
	longIsHigh bool,
) func(*porcelainVerticalOpts, []string) (*orderbuilder.VerticalParams, error) {
	return func(opts *porcelainVerticalOpts, _ []string) (*orderbuilder.VerticalParams, error) {
		expiration, err := parseExpiration(opts.Expiration)
		if err != nil {
			return nil, err
		}

		longStrike, shortStrike := opts.LowStrike, opts.HighStrike
		if longIsHigh {
			longStrike, shortStrike = opts.HighStrike, opts.LowStrike
		}

		return &orderbuilder.VerticalParams{
			Underlying:  strings.TrimSpace(opts.Underlying),
			Expiration:  expiration,
			LongStrike:  longStrike,
			ShortStrike: shortStrike,
			PutCall:     putCall,
			Open:        true, // Porcelain verticals always open.
			Quantity:    opts.Quantity,
			Price:       opts.Price,
			Duration:    normalizeDuration(opts.Duration),
			Session:     opts.Session,
		}, nil
	}
}

// porcelainVerticalConfigs returns all porcelain vertical spread command variants.
func porcelainVerticalConfigs() []porcelainVerticalConfig {
	return []porcelainVerticalConfig{
		{
			name:       "put-credit-spread",
			short:      "Open a bull put spread (credit)",
			putCall:    models.PutCallPut,
			longIsHigh: false, // Buy low put, sell high put.
			long: `Open a bull put spread for a net credit. Sells the higher-strike put and buys
the lower-strike put as protection. The command hardcodes PUT, OPEN, and the
correct long/short strike mapping so you only need to specify the two strikes.
Maximum profit is the credit received; maximum loss is the spread width minus
the credit.`,
			example: `  schwab-agent order build put-credit-spread --underlying AAPL --expiration 2026-06-19 --high-strike 195 --low-strike 190 --quantity 1 --price 1.50
  schwab-agent order place put-credit-spread --underlying AAPL --expiration 2026-06-19 --high-strike 195 --low-strike 190 --quantity 1 --price 1.50`,
		},
		{
			name:       "call-credit-spread",
			short:      "Open a bear call spread (credit)",
			putCall:    models.PutCallCall,
			longIsHigh: true, // Buy high call, sell low call.
			long: `Open a bear call spread for a net credit. Sells the lower-strike call and buys
the higher-strike call as protection. The command hardcodes CALL, OPEN, and the
correct long/short strike mapping so you only need to specify the two strikes.
Maximum profit is the credit received; maximum loss is the spread width minus
the credit.`,
			example: `  schwab-agent order build call-credit-spread --underlying AAPL --expiration 2026-06-19 --high-strike 210 --low-strike 200 --quantity 1 --price 2.00
  schwab-agent order place call-credit-spread --underlying AAPL --expiration 2026-06-19 --high-strike 210 --low-strike 200 --quantity 1 --price 2.00`,
		},
		{
			name:       "put-debit-spread",
			short:      "Open a bear put spread (debit)",
			putCall:    models.PutCallPut,
			longIsHigh: true, // Buy high put, sell low put.
			long: `Open a bear put spread for a net debit. Buys the higher-strike put and sells
the lower-strike put to offset the cost. The command hardcodes PUT, OPEN, and
the correct long/short strike mapping so you only need to specify the two
strikes. Maximum profit is the spread width minus the debit paid; maximum loss
is the debit paid.`,
			example: `  schwab-agent order build put-debit-spread --underlying AAPL --expiration 2026-06-19 --high-strike 200 --low-strike 190 --quantity 1 --price 3.00
  schwab-agent order place put-debit-spread --underlying AAPL --expiration 2026-06-19 --high-strike 200 --low-strike 190 --quantity 1 --price 3.00`,
		},
		{
			name:       "call-debit-spread",
			short:      "Open a bull call spread (debit)",
			putCall:    models.PutCallCall,
			longIsHigh: false, // Buy low call, sell high call.
			long: `Open a bull call spread for a net debit. Buys the lower-strike call and sells
the higher-strike call to offset the cost. The command hardcodes CALL, OPEN,
and the correct long/short strike mapping so you only need to specify the two
strikes. Maximum profit is the spread width minus the debit paid; maximum loss
is the debit paid.`,
			example: `  schwab-agent order build call-debit-spread --underlying AAPL --expiration 2026-06-19 --high-strike 210 --low-strike 200 --quantity 1 --price 3.50
  schwab-agent order place call-debit-spread --underlying AAPL --expiration 2026-06-19 --high-strike 210 --low-strike 200 --quantity 1 --price 3.50`,
		},
	}
}

// makePorcelainVerticalBuildCmd creates a build subcommand for a porcelain vertical config.
func makePorcelainVerticalBuildCmd(w io.Writer, cfg porcelainVerticalConfig) *cobra.Command {
	cmd := makeCobraBuildOrderCommand(
		w,
		cfg.name,
		cfg.short,
		func() *porcelainVerticalOpts { return &porcelainVerticalOpts{} },
		func(cmd *cobra.Command, opts *porcelainVerticalOpts) { defineAndConstrain(cmd, opts) },
		parsePorcelainVerticalFactory(cfg.putCall, cfg.longIsHigh),
		orderbuilder.ValidateVerticalOrder,
		orderbuilder.BuildVerticalOrder,
	)
	cmd.Long = cfg.long
	cmd.Example = cfg.example

	return cmd
}

// makePorcelainVerticalPlaceCmd creates a place subcommand for a porcelain vertical config.
func makePorcelainVerticalPlaceCmd(
	c *client.Ref,
	configPath string,
	w io.Writer,
	cfg porcelainVerticalConfig,
) *cobra.Command {
	cmd := makeCobraPlaceOrderCommand(
		c,
		configPath,
		w,
		cfg.name,
		cfg.short,
		func() *porcelainVerticalOpts { return &porcelainVerticalOpts{} },
		func(cmd *cobra.Command, opts *porcelainVerticalOpts) { defineAndConstrain(cmd, opts) },
		parsePorcelainVerticalFactory(cfg.putCall, cfg.longIsHigh),
		orderbuilder.ValidateVerticalOrder,
		orderbuilder.BuildVerticalOrder,
	)
	cmd.Long = cfg.long
	cmd.Example = cfg.example

	return cmd
}

// makePorcelainVerticalPreviewCmd creates a preview subcommand for a porcelain vertical config.
func makePorcelainVerticalPreviewCmd(
	c *client.Ref,
	configPath string,
	w io.Writer,
	cfg porcelainVerticalConfig,
) *cobra.Command {
	cmd := makeCobraPreviewOrderCommand(
		c,
		configPath,
		w,
		cfg.name,
		cfg.short,
		func() *porcelainVerticalOpts { return &porcelainVerticalOpts{} },
		func(cmd *cobra.Command, opts *porcelainVerticalOpts) { defineAndConstrain(cmd, opts) },
		parsePorcelainVerticalFactory(cfg.putCall, cfg.longIsHigh),
		orderbuilder.ValidateVerticalOrder,
		orderbuilder.BuildVerticalOrder,
	)
	cmd.Long = cfg.long
	cmd.Example = cfg.example

	return cmd
}

// porcelainVerticalBuildCommands returns build commands for all porcelain vertical variants.
func porcelainVerticalBuildCommands(w io.Writer) []*cobra.Command {
	configs := porcelainVerticalConfigs()
	cmds := make([]*cobra.Command, 0, len(configs))
	for _, cfg := range configs {
		cmds = append(cmds, makePorcelainVerticalBuildCmd(w, cfg))
	}

	return cmds
}

// porcelainVerticalPlaceCommands returns place commands for all porcelain vertical variants.
func porcelainVerticalPlaceCommands(
	c *client.Ref,
	configPath string,
	w io.Writer,
) []*cobra.Command {
	configs := porcelainVerticalConfigs()
	cmds := make([]*cobra.Command, 0, len(configs))
	for _, cfg := range configs {
		cmds = append(cmds, makePorcelainVerticalPlaceCmd(c, configPath, w, cfg))
	}

	return cmds
}

// porcelainVerticalPreviewCommands returns preview commands for all porcelain vertical variants.
func porcelainVerticalPreviewCommands(
	c *client.Ref,
	configPath string,
	w io.Writer,
) []*cobra.Command {
	configs := porcelainVerticalConfigs()
	cmds := make([]*cobra.Command, 0, len(configs))
	for _, cfg := range configs {
		cmds = append(cmds, makePorcelainVerticalPreviewCmd(c, configPath, w, cfg))
	}

	return cmds
}
