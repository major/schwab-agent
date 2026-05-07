package commands

import (
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/major/schwab-agent/internal/apperr"
	"github.com/major/schwab-agent/internal/client"
	"github.com/major/schwab-agent/internal/output"
)

// newOptionExpirationsCmd returns the command that lists available option
// expiration dates for an underlying symbol in compact porcelain format.
func newOptionExpirationsCmd(c *client.Ref, w io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "expirations <symbol>",
		Short: "List option expiration dates for a symbol",
		Long: `List available option expiration dates for an underlying symbol. Returns a
compact table with expiration date, days to expiration, expiration type
(R=regular monthly, S=weekly/special), and whether the date falls on a
standard third Friday.`,
		Example: `  schwab-agent option expirations AAPL
  schwab-agent option expirations TSLA`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			symbol := strings.ToUpper(strings.TrimSpace(args[0]))
			if err := requireArg(symbol, "symbol"); err != nil {
				return err
			}

			chain, err := c.ExpirationChainForSymbol(cmd.Context(), symbol)
			if err != nil {
				return err
			}

			if chain == nil || len(chain.ExpirationList) == 0 {
				return apperr.NewSymbolNotFoundError("no expirations found for "+symbol, nil)
			}

			cols, rows, err := flattenExpirationRows(chain, time.Now())
			if err != nil {
				return err
			}

			meta := output.NewMetadata()
			meta.Returned = len(rows)

			return output.WriteSuccess(w, map[string]any{
				"underlying": symbol,
				"columns":    cols,
				"rows":       rows,
				"rowCount":   len(rows),
			}, meta)
		},
	}

	return cmd
}
