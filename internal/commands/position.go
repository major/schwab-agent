package commands

import (
	"context"
	"io"

	"github.com/urfave/cli/v3"

	"github.com/major/schwab-agent/internal/apperr"
	"github.com/major/schwab-agent/internal/client"
	"github.com/major/schwab-agent/internal/models"
	"github.com/major/schwab-agent/internal/output"
)

// positionEntry enriches a position with account identifiers and computed
// cost basis / P&L fields that Schwab's API doesn't provide directly.
type positionEntry struct {
	AccountNumber   string `json:"accountNumber"`
	AccountHash     string `json:"accountHash,omitempty"`
	AccountNickName string `json:"accountNickName,omitempty"`

	models.Position

	// Computed fields derived from the raw position data.
	TotalCostBasis   *float64 `json:"totalCostBasis,omitempty"`
	UnrealizedPnL    *float64 `json:"unrealizedPnL,omitempty"`
	UnrealizedPnLPct *float64 `json:"unrealizedPnLPct,omitempty"`
}

// positionListData wraps the position list response.
type positionListData struct {
	Positions []positionEntry `json:"positions"`
}

// PositionCommand returns the parent CLI command for position operations.
func PositionCommand(c *client.Ref, configPath string, w io.Writer) *cli.Command {
	return &cli.Command{
		Name:  "position",
		Usage: "View positions across accounts",
		Commands: []*cli.Command{
			positionListCommand(c, configPath, w),
		},
	}
}

// positionListCommand returns the CLI command for listing positions.
// Default: single account (resolved via --account flag or config default).
// With --all-accounts: flattens positions from all linked accounts into a
// single list with account identifiers on each entry.
func positionListCommand(c *client.Ref, configPath string, w io.Writer) *cli.Command {
	return &cli.Command{
		Name:  "list",
		Usage: "List positions for one or all accounts",
		UsageText: `schwab-agent position list
schwab-agent position list --all-accounts
schwab-agent position list --account HASH`,
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "all-accounts",
				Usage: "Show positions across all linked accounts",
			},
			&cli.StringFlag{
				Name:  "account",
				Usage: "Account hash (overrides config default)",
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			if cmd.Bool("all-accounts") && cmd.String("account") != "" {
				return apperr.NewValidationError("--all-accounts and --account are mutually exclusive", nil)
			}

			if cmd.Bool("all-accounts") {
				return listAllAccountPositions(ctx, c, w)
			}

			return listSingleAccountPositions(ctx, c, configPath, cmd, w)
		},
	}
}

// listAllAccountPositions fetches positions from all linked accounts and
// returns a flat list with account identifiers and computed P&L fields.
func listAllAccountPositions(ctx context.Context, c *client.Ref, w io.Writer) error {
	accounts, err := c.Accounts(ctx, "positions")
	if err != nil {
		return err
	}

	// Map account numbers to hash values so we can include both in the output.
	// The accounts API returns account numbers but not hashes; the account
	// numbers API provides the mapping.
	numbers, err := c.AccountNumbers(ctx)
	if err != nil {
		return err
	}

	numberToHash := make(map[string]string, len(numbers))
	for _, n := range numbers {
		numberToHash[n.AccountNumber] = n.HashValue
	}

	// Enrich with nicknames from user preferences (best-effort).
	prefs, prefsErr := c.UserPreference(ctx)
	if prefsErr == nil {
		enrichAccountsWithPreferences(accounts, prefs)
	}

	entries := flattenAccountPositions(accounts, numberToHash)

	meta := output.NewMetadata()
	meta.Returned = len(entries)

	return output.WriteSuccess(w, positionListData{Positions: entries}, meta)
}

// listSingleAccountPositions fetches positions for a single resolved account.
func listSingleAccountPositions(
	ctx context.Context,
	c *client.Ref,
	configPath string,
	cmd *cli.Command,
	w io.Writer,
) error {
	hash, err := resolveAccount(cmd.String("account"), configPath, nil)
	if err != nil {
		return err
	}

	account, err := c.Account(ctx, hash, "positions")
	if err != nil {
		return err
	}

	// Enrich with nickname from user preferences (best-effort).
	prefs, prefsErr := c.UserPreference(ctx)
	if prefsErr == nil {
		enrichAccountWithPreferences(account, prefs)
	}

	acctNum := ""
	acctNick := ""

	if account.SecuritiesAccount != nil && account.SecuritiesAccount.AccountNumber != nil {
		acctNum = *account.SecuritiesAccount.AccountNumber
	}

	if account.NickName != nil {
		acctNick = *account.NickName
	}

	var entries []positionEntry

	if account.SecuritiesAccount != nil {
		for i := range account.SecuritiesAccount.Positions {
			entries = append(entries, newPositionEntry(acctNum, hash, acctNick, &account.SecuritiesAccount.Positions[i]))
		}
	}

	if entries == nil {
		entries = []positionEntry{}
	}

	meta := output.NewMetadata()
	meta.Account = hash
	meta.Returned = len(entries)

	return output.WriteSuccess(w, positionListData{Positions: entries}, meta)
}

// flattenAccountPositions converts a slice of accounts with positions into
// a flat list of position entries with account identifiers.
func flattenAccountPositions(accounts []models.Account, numberToHash map[string]string) []positionEntry {
	var entries []positionEntry

	for _, acct := range accounts {
		if acct.SecuritiesAccount == nil {
			continue
		}

		acctNum := ""
		acctHash := ""
		acctNick := ""

		if acct.SecuritiesAccount.AccountNumber != nil {
			acctNum = *acct.SecuritiesAccount.AccountNumber
			acctHash = numberToHash[acctNum]
		}

		if acct.NickName != nil {
			acctNick = *acct.NickName
		}

		for i := range acct.SecuritiesAccount.Positions {
			entries = append(entries, newPositionEntry(acctNum, acctHash, acctNick, &acct.SecuritiesAccount.Positions[i]))
		}
	}

	if entries == nil {
		entries = []positionEntry{}
	}

	return entries
}

// newPositionEntry creates a position entry with account identifiers and
// computed cost basis / P&L fields.
func newPositionEntry(accountNumber, accountHash, nickName string, pos *models.Position) positionEntry {
	entry := positionEntry{
		AccountNumber:   accountNumber,
		AccountHash:     accountHash,
		AccountNickName: nickName,
		Position:        *pos,
	}

	computePositionFields(&entry)

	return entry
}

// computePositionFields calculates derived cost basis and P&L values from
// the raw position data returned by the Schwab API.
//
//	totalCostBasis   = averagePrice * (longQuantity + shortQuantity)
//	unrealizedPnL    = longOpenProfitLoss + shortOpenProfitLoss
//	unrealizedPnLPct = (unrealizedPnL / totalCostBasis) * 100
func computePositionFields(e *positionEntry) {
	// Cost basis: average price * total quantity (long + short).
	// Positions are typically one-sided (all long or all short), but the API
	// allows both to be set so we handle the general case.
	if e.AveragePrice != nil {
		var qty float64
		if e.LongQuantity != nil {
			qty += *e.LongQuantity
		}

		if e.ShortQuantity != nil {
			qty += *e.ShortQuantity
		}

		if qty != 0 {
			costBasis := *e.AveragePrice * qty
			e.TotalCostBasis = &costBasis
		}
	}

	// Unrealized P&L: sum of long and short open P&L from the API.
	var pnl float64
	var hasPnL bool

	if e.LongOpenProfitLoss != nil {
		pnl += *e.LongOpenProfitLoss
		hasPnL = true
	}

	if e.ShortOpenProfitLoss != nil {
		pnl += *e.ShortOpenProfitLoss
		hasPnL = true
	}

	if hasPnL {
		e.UnrealizedPnL = &pnl
	}

	// P&L percentage: avoid division by zero on zero-cost positions
	// (e.g., positions acquired via stock split or transfer).
	if e.UnrealizedPnL != nil && e.TotalCostBasis != nil && *e.TotalCostBasis != 0 {
		pct := *e.UnrealizedPnL / *e.TotalCostBasis * 100
		e.UnrealizedPnLPct = &pct
	}
}
