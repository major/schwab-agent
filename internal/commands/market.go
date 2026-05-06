package commands

import (
	"io"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/major/schwab-go/schwab/marketdata"

	"github.com/major/schwab-agent/internal/apperr"
	"github.com/major/schwab-agent/internal/client"
	"github.com/major/schwab-agent/internal/models"
	"github.com/major/schwab-agent/internal/output"
)

// allMarkets returns the full list of Schwab market types, used as the default
// when no specific markets are requested.
func allMarkets() []string {
	return []string{commandUseEquity, commandUseOption, "bond", "future", "forex"}
}

// moversData wraps the movers response.
type moversData struct {
	Movers *marketdata.MoverResponse `json:"movers"`
}

// marketHoursData is the market-hours output shape historically returned by
// schwab-agent. schwab-go intentionally uses value fields, so the command adapts
// those values back to the existing pointer-based model before writing JSON.
type marketHoursData map[string]map[string]models.MarketHours

// marketMoversOpts holds the options for the market movers subcommand.
type marketMoversOpts struct {
	Sort      moversSort      `flag:"sort"      flagdescr:"Sort order (VOLUME, TRADES, PERCENT_CHANGE_UP, PERCENT_CHANGE_DOWN)"`
	Frequency moversFrequency `flag:"frequency" flagdescr:"Minimum percent change magnitude (0, 1, 5, 10, 30, 60)"`
}

// NewMarketCmd returns the Cobra command for market hours and movers lookups.
func NewMarketCmd(c *client.Ref, w io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "market",
		Short:   "Market hours and top movers",
		GroupID: groupIDMarketData,
		RunE:    requireSubcommand,
	}
	cmd.SetFlagErrorFunc(suggestSubcommands)

	cmd.AddCommand(
		newMarketHoursCmd(c, w),
		newMarketMoversCmd(c, w),
	)

	return cmd
}

// newMarketHoursCmd returns the Cobra subcommand for market hours.
func newMarketHoursCmd(c *client.Ref, w io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "hours [market ...]",
		Short: "Get market hours for one or more markets (equity, option, bond, future, forex)",
		Long: `Get current market hours for one or more markets. With no arguments, returns
hours for all markets (equity, option, bond, future, forex). Specify one or
more market names to filter.`,
		Example: `  schwab-agent market hours
  schwab-agent market hours equity
  schwab-agent market hours equity option`,
		RunE: func(cmd *cobra.Command, args []string) error {
			markets := args
			if len(markets) == 0 {
				markets = allMarkets()
			}

			// Use the single-market endpoint when exactly one market is requested,
			// matching Schwab's two market-hours routes while keeping the CLI output
			// shape identical for both paths.
			if len(markets) == 1 {
				result, err := c.MarketData.GetMarketHoursSingle(cmd.Context(), markets[0], "")
				if err != nil {
					return mapSchwabGoError(err)
				}
				return output.WriteSuccess(w, convertMarketHours(result), output.NewMetadata())
			}

			result, err := c.MarketData.GetMarketHours(cmd.Context(), markets, "")
			if err != nil {
				return mapSchwabGoError(err)
			}
			return output.WriteSuccess(w, convertMarketHours(result), output.NewMetadata())
		},
	}
}

func convertMarketHours(hours marketdata.MarketHoursMap) marketHoursData {
	converted := make(marketHoursData, len(hours))
	for market, products := range hours {
		convertedProducts := make(map[string]models.MarketHours, len(products))
		for product, productHours := range products {
			convertedProducts[product] = convertMarketHoursEntry(productHours)
		}
		converted[market] = convertedProducts
	}
	return converted
}

func convertMarketHoursEntry(hours marketdata.MarketHours) models.MarketHours {
	isOpen := hours.IsOpen
	return models.MarketHours{
		MarketType:   stringPtr(hours.MarketType),
		Product:      stringPtr(hours.Product),
		ProductName:  stringPtr(hours.ProductName),
		IsOpen:       &isOpen,
		SessionHours: convertMarketSessionHours(hours.SessionHours),
		Exchange:     stringPtr(hours.Exchange),
		Category:     stringPtr(hours.Category),
		Date:         stringPtr(hours.Date),
	}
}

func convertMarketSessionHours(sessionHours map[string][]marketdata.SessionHours) *models.SessionHours {
	if len(sessionHours) == 0 {
		return nil
	}

	return &models.SessionHours{
		PreMarket:     convertMarketSessions(sessionHours["preMarket"]),
		RegularMarket: convertMarketSessions(sessionHours["regularMarket"]),
		PostMarket:    convertMarketSessions(sessionHours["postMarket"]),
	}
}

func convertMarketSessions(sessions []marketdata.SessionHours) []models.MarketSession {
	if len(sessions) == 0 {
		return nil
	}

	converted := make([]models.MarketSession, 0, len(sessions))
	for _, session := range sessions {
		converted = append(converted, models.MarketSession{
			Start: stringPtr(session.Start),
			End:   stringPtr(session.End),
		})
	}
	return converted
}

func stringPtr(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

// newMarketMoversCmd returns the Cobra subcommand for market movers.
func newMarketMoversCmd(c *client.Ref, w io.Writer) *cobra.Command {
	opts := &marketMoversOpts{}
	cmd := &cobra.Command{
		Use:   "movers <index>",
		Short: "Get top movers for an index ($SPX, $DJI, $COMPX, NYSE, NASDAQ, OTCBB, INDEX_ALL, EQUITY_ALL, OPTION_ALL, OPTION_PUT, OPTION_CALL)",
		Long: `Get top movers for a market index. Sort by VOLUME, TRADES, PERCENT_CHANGE_UP,
or PERCENT_CHANGE_DOWN. Use --frequency to filter by minimum percent change
magnitude (0, 1, 5, 10, 30, 60). Quote shell-special characters in index
names (e.g. '$SPX').`,
		Example: `  schwab-agent market movers '$SPX' --sort PERCENT_CHANGE_UP
  schwab-agent market movers '$DJI' --sort VOLUME --frequency 5
  schwab-agent market movers EQUITY_ALL --sort PERCENT_CHANGE_UP`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateCobraOptions(cmd.Context(), opts); err != nil {
				return err
			}

			index := ""
			if len(args) > 0 {
				index = args[0]
			}
			if err := requireArg(index, "index"); err != nil {
				return err
			}

			sort := marketdata.MoverSort(string(opts.Sort))

			// schwab-go accepts *int for frequency: nil omits it from the
			// query string, non-nil sends the value (including 0).
			var freq *int
			if cmd.Flags().Changed("frequency") {
				f, err := strconv.Atoi(string(opts.Frequency))
				if err != nil {
					return apperr.NewValidationError(
						"invalid frequency: "+string(opts.Frequency), err,
					)
				}
				freq = &f
			}

			result, err := c.MarketData.GetMovers(cmd.Context(), index, sort, freq)
			if err != nil {
				return mapSchwabGoError(err)
			}

			return output.WriteSuccess(w, moversData{Movers: result}, output.NewMetadata())
		},
	}

	defineCobraFlags(cmd, opts)
	cmd.SetFlagErrorFunc(normalizeFlagValidationErrorFunc)

	return cmd
}
