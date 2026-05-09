package commands

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/major/schwab-agent/internal/client"
	"github.com/major/schwab-agent/internal/models"
	"github.com/major/schwab-agent/internal/orderbuilder"
)

const (
	shortIronCondorName  = "short-iron-condor"
	shortIronCondorShort = "Sell an opening iron condor"
	shortIronCondorLong  = `Sell an opening iron condor with the safer, factual strike names agents
already use for the generic iron-condor builder. This porcelain hardcodes the
opening, net-credit direction so callers cannot accidentally close a position or
flip the credit/debit semantics. Strikes must be ordered put-long < put-short <
call-short < call-long.`
	shortIronCondorExample = `  schwab-agent order build short-iron-condor --underlying F --expiration 2026-06-18 --put-long-strike 9 --put-short-strike 10 --call-short-strike 14 --call-long-strike 15 --quantity 1 --price 0.50
  schwab-agent order preview short-iron-condor --underlying F --expiration 2026-06-18 --put-long-strike 9 --put-short-strike 10 --call-short-strike 14 --call-long-strike 15 --quantity 1 --price 0.50 --save-preview
  schwab-agent order place short-iron-condor --underlying F --expiration 2026-06-18 --put-long-strike 9 --put-short-strike 10 --call-short-strike 14 --call-long-strike 15 --quantity 1 --price 0.50`
)

type shortIronCondorOpts struct {
	Underlying      string          `flag:"underlying"        flagdescr:"Underlying symbol"                              flagrequired:"true" flaggroup:"contract"`
	Expiration      string          `flag:"expiration"        flagdescr:"Expiration date (YYYY-MM-DD)"                   flagrequired:"true" flaggroup:"contract"`
	PutLongStrike   float64         `flag:"put-long-strike"   flagdescr:"Lowest strike: put being bought (protection)"   flagrequired:"true" flaggroup:"contract"`
	PutShortStrike  float64         `flag:"put-short-strike"  flagdescr:"Put being sold (premium)"                       flagrequired:"true" flaggroup:"contract"`
	CallShortStrike float64         `flag:"call-short-strike" flagdescr:"Call being sold (premium)"                      flagrequired:"true" flaggroup:"contract"`
	CallLongStrike  float64         `flag:"call-long-strike"  flagdescr:"Highest strike: call being bought (protection)" flagrequired:"true" flaggroup:"contract"`
	Quantity        float64         `flag:"quantity"          flagdescr:"Number of contracts"                            flagrequired:"true" flaggroup:"execution"`
	Price           float64         `flag:"price"             flagdescr:"Net credit amount"                              flagrequired:"true" flaggroup:"pricing"`
	Duration        models.Duration `flag:"duration"          flagdescr:"Order duration"                                                     flaggroup:"order"`
	Session         models.Session  `flag:"session"           flagdescr:"Trading session"                                                    flaggroup:"order"`
}

func shortIronCondorBuildCommand(w io.Writer) *cobra.Command {
	cmd := makeCobraBuildOrderCommand(
		w,
		shortIronCondorName,
		shortIronCondorShort,
		func() *shortIronCondorOpts { return &shortIronCondorOpts{} },
		func(cmd *cobra.Command, opts *shortIronCondorOpts) { defineCobraFlags(cmd, opts) },
		parseShortIronCondorParams,
		orderbuilder.ValidateIronCondorOrder,
		orderbuilder.BuildIronCondorOrder,
	)
	cmd.Long = shortIronCondorLong
	cmd.Example = shortIronCondorExample

	return cmd
}

func shortIronCondorPlaceCommand(c *client.Ref, configPath string, w io.Writer) *cobra.Command {
	cmd := makeCobraPlaceOrderCommand(
		c,
		configPath,
		w,
		shortIronCondorName,
		shortIronCondorShort,
		func() *shortIronCondorOpts { return &shortIronCondorOpts{} },
		func(cmd *cobra.Command, opts *shortIronCondorOpts) { defineCobraFlags(cmd, opts) },
		parseShortIronCondorParams,
		orderbuilder.ValidateIronCondorOrder,
		orderbuilder.BuildIronCondorOrder,
	)
	cmd.Long = shortIronCondorLong
	cmd.Example = shortIronCondorExample

	return cmd
}

func shortIronCondorPreviewCommand(c *client.Ref, configPath string, w io.Writer) *cobra.Command {
	cmd := makeCobraPreviewOrderCommand(
		c,
		configPath,
		w,
		shortIronCondorName,
		shortIronCondorShort,
		func() *shortIronCondorOpts { return &shortIronCondorOpts{} },
		func(cmd *cobra.Command, opts *shortIronCondorOpts) { defineCobraFlags(cmd, opts) },
		parseShortIronCondorParams,
		orderbuilder.ValidateIronCondorOrder,
		orderbuilder.BuildIronCondorOrder,
	)
	cmd.Long = shortIronCondorLong
	cmd.Example = shortIronCondorExample

	return cmd
}

func parseShortIronCondorParams(opts *shortIronCondorOpts, _ []string) (*orderbuilder.IronCondorParams, error) {
	expiration, err := parseExpiration(opts.Expiration)
	if err != nil {
		return nil, fmt.Errorf("parsing expiration %q: %w", opts.Expiration, err)
	}

	return &orderbuilder.IronCondorParams{
		Underlying:      strings.TrimSpace(opts.Underlying),
		Expiration:      expiration,
		PutLongStrike:   opts.PutLongStrike,
		PutShortStrike:  opts.PutShortStrike,
		CallShortStrike: opts.CallShortStrike,
		CallLongStrike:  opts.CallLongStrike,
		Open:            true,
		Quantity:        opts.Quantity,
		Price:           opts.Price,
		Duration:        normalizeDuration(opts.Duration),
		Session:         opts.Session,
	}, nil
}
