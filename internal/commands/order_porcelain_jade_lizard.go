package commands

import (
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/major/schwab-agent/internal/client"
	"github.com/major/schwab-agent/internal/models"
	"github.com/major/schwab-agent/internal/orderbuilder"
)

// jadeLizardBuildOpts holds flags for the jade lizard porcelain command.
// All directions are hardcoded (SELL_TO_OPEN for the put and short call,
// BUY_TO_OPEN for the protective long call), so only strikes and quantity
// are needed from the caller.
type jadeLizardBuildOpts struct {
	Underlying      string          `flag:"underlying"        flagdescr:"Underlying symbol"                      flagrequired:"true" flaggroup:"contract"`
	Expiration      string          `flag:"expiration"        flagdescr:"Expiration date (YYYY-MM-DD)"           flagrequired:"true" flaggroup:"contract"`
	PutStrike       float64         `flag:"put-strike"        flagdescr:"Short put strike (lowest)"              flagrequired:"true" flaggroup:"contract"`
	ShortCallStrike float64         `flag:"short-call-strike" flagdescr:"Short call strike (middle)"             flagrequired:"true" flaggroup:"contract"`
	LongCallStrike  float64         `flag:"long-call-strike"  flagdescr:"Long call strike (highest, protection)" flagrequired:"true" flaggroup:"contract"`
	Quantity        float64         `flag:"quantity"          flagdescr:"Number of contracts"                    flagrequired:"true" flaggroup:"execution"`
	Price           float64         `flag:"price"             flagdescr:"Net credit amount"                      flagrequired:"true" flaggroup:"pricing"`
	Duration        models.Duration `flag:"duration"          flagdescr:"Order duration"                                             flaggroup:"order"`
	Session         models.Session  `flag:"session"           flagdescr:"Trading session"                                            flaggroup:"order"`
}

// parseJadeLizardParams converts command flags into jade lizard builder params.
func parseJadeLizardParams(opts *jadeLizardBuildOpts, _ []string) (*orderbuilder.JadeLizardParams, error) {
	expiration, err := parseExpiration(opts.Expiration)
	if err != nil {
		return nil, err
	}

	return &orderbuilder.JadeLizardParams{
		Underlying:      strings.TrimSpace(opts.Underlying),
		Expiration:      expiration,
		PutStrike:       opts.PutStrike,
		ShortCallStrike: opts.ShortCallStrike,
		LongCallStrike:  opts.LongCallStrike,
		Quantity:        opts.Quantity,
		Price:           opts.Price,
		Duration:        normalizeDuration(opts.Duration),
		Session:         opts.Session,
	}, nil
}

const (
	jadeLizardShort = "Build a jade lizard order (short put + bear call spread)"
	jadeLizardLong  = `Open a jade lizard position: sell a put, sell a call, and buy a higher-strike
call for upside protection. All three legs open simultaneously for a net credit.
The command hardcodes SELL_TO_OPEN for the put and short call and BUY_TO_OPEN
for the protective call, so only the three strikes and quantity are needed.
Strikes must be ordered: put-strike < short-call-strike < long-call-strike.`
	jadeLizardExample = `  schwab-agent order build jade-lizard --underlying AAPL --expiration 2026-06-19 --put-strike 180 --short-call-strike 210 --long-call-strike 220 --quantity 1 --price 3.50
  schwab-agent order place jade-lizard --underlying AAPL --expiration 2026-06-19 --put-strike 180 --short-call-strike 210 --long-call-strike 220 --quantity 1 --price 3.50`
)

// jadeLizardBuildCommand returns a single build command for jade lizard.
func jadeLizardBuildCommand(w io.Writer) *cobra.Command {
	cmd := makeCobraBuildOrderCommand(
		w,
		"jade-lizard",
		jadeLizardShort,
		func() *jadeLizardBuildOpts { return &jadeLizardBuildOpts{} },
		func(cmd *cobra.Command, opts *jadeLizardBuildOpts) { defineAndConstrain(cmd, opts) },
		parseJadeLizardParams,
		orderbuilder.ValidateJadeLizardOrder,
		orderbuilder.BuildJadeLizardOrder,
	)
	cmd.Long = jadeLizardLong
	cmd.Example = jadeLizardExample

	return cmd
}

// jadeLizardPlaceCommand returns a single place command for jade lizard.
func jadeLizardPlaceCommand(c *client.Ref, configPath string, w io.Writer) *cobra.Command {
	cmd := makeCobraPlaceOrderCommand(
		c,
		configPath,
		w,
		"jade-lizard",
		"Place a jade lizard order",
		func() *jadeLizardBuildOpts { return &jadeLizardBuildOpts{} },
		func(cmd *cobra.Command, opts *jadeLizardBuildOpts) { defineAndConstrain(cmd, opts) },
		parseJadeLizardParams,
		orderbuilder.ValidateJadeLizardOrder,
		orderbuilder.BuildJadeLizardOrder,
	)
	cmd.Long = `Place a jade lizard order. Sells a put and a short call spread (short call +
long call) for a net credit. The command hardcodes all directions so only the
three strikes and quantity are needed.`
	cmd.Example = `  schwab-agent order place jade-lizard --underlying AAPL --expiration 2026-06-19 --put-strike 180 --short-call-strike 210 --long-call-strike 220 --quantity 1 --price 3.50`

	return cmd
}

// jadeLizardPreviewCommand returns a single preview command for jade lizard.
func jadeLizardPreviewCommand(c *client.Ref, configPath string, w io.Writer) *cobra.Command {
	cmd := makeCobraPreviewOrderCommand(
		c,
		configPath,
		w,
		"jade-lizard",
		"Preview a jade lizard order",
		func() *jadeLizardBuildOpts { return &jadeLizardBuildOpts{} },
		func(cmd *cobra.Command, opts *jadeLizardBuildOpts) { defineAndConstrain(cmd, opts) },
		parseJadeLizardParams,
		orderbuilder.ValidateJadeLizardOrder,
		orderbuilder.BuildJadeLizardOrder,
	)
	cmd.Long = `Preview a jade lizard order without placing it. Shows the three-leg order
structure and Schwab's preview response including estimated commissions.`
	cmd.Example = `  schwab-agent order preview jade-lizard --underlying AAPL --expiration 2026-06-19 --put-strike 180 --short-call-strike 210 --long-call-strike 220 --quantity 1 --price 3.50`

	return cmd
}
