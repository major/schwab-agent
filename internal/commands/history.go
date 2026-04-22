// Package commands provides CLI command builders for schwab-agent.
package commands

import (
	"context"
	"io"

	"github.com/urfave/cli/v3"

	"github.com/major/schwab-agent/internal/client"
	"github.com/major/schwab-agent/internal/models"
	"github.com/major/schwab-agent/internal/output"
)

// priceHistoryData wraps the price history response.
type priceHistoryData struct {
	PriceHistory *models.CandleList `json:"priceHistory"`
}

// HistoryCommand returns the CLI command for price history lookups.
func HistoryCommand(c *client.Ref, w io.Writer) *cli.Command {
	return &cli.Command{
		Name:  "history",
		Usage: "Retrieve price history for a symbol",
		Commands: []*cli.Command{
			{
				Name:  "get",
				Usage: "Get price history candles for a symbol",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "period-type", Usage: "Period type (day, month, year, ytd)"},
					&cli.StringFlag{Name: "period", Usage: "Number of periods"},
					&cli.StringFlag{Name: "frequency-type", Usage: "Frequency type (minute, daily, weekly, monthly)"},
					&cli.StringFlag{Name: "frequency", Usage: "Frequency value"},
					&cli.StringFlag{Name: "from", Usage: "Start date (milliseconds since epoch)"},
					&cli.StringFlag{Name: "to", Usage: "End date (milliseconds since epoch)"},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					symbol := cmd.Args().First()
				if err := requireArg(symbol, "symbol"); err != nil {
					return err
				}

				params := client.HistoryParams{
					PeriodType:    cmd.String("period-type"),
					Period:        cmd.String("period"),
					FrequencyType: cmd.String("frequency-type"),
					Frequency:     cmd.String("frequency"),
					StartDate:     cmd.String("from"),
					EndDate:       cmd.String("to"),
				}

				result, err := c.PriceHistory(ctx, symbol, &params)
					if err != nil {
						return err
					}

					return output.WriteSuccess(w, priceHistoryData{PriceHistory: result}, output.NewMetadata())
				},
			},
		},
	}
}
