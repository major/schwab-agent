package commands

import (
	"context"
	"io"
	"sort"
	"strings"

	"github.com/leodido/structcli"
	"github.com/spf13/cobra"

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

// positionFilters captures normalized position-list filters after structcli
// has unmarshaled flags. Optional numeric filters stay as pointers so callers
// can distinguish an omitted bound from an explicit zero-dollar threshold.
type positionFilters struct {
	Symbols    map[string]struct{}
	LosersOnly bool
	MinPnL     *float64
	MaxPnL     *float64
	Sort       positionSort
}

// listAllAccountPositions fetches positions from all linked accounts and
// returns a flat list with account identifiers and computed P&L fields.
func listAllAccountPositions(ctx context.Context, c *client.Ref, filters positionFilters, w io.Writer) error {
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

	entries := filterAndSortPositionEntries(flattenAccountPositions(accounts, numberToHash), filters)

	meta := output.NewMetadata()
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

// NewPositionCmd returns the Cobra command for position operations.
func NewPositionCmd(c *client.Ref, configPath string, w io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "position",
		Short:   "View positions across accounts",
		GroupID: "account-mgmt",
		RunE:    requireSubcommand,
	}

	cmd.SetFlagErrorFunc(suggestSubcommands)
	cmd.AddCommand(newPositionListCmd(c, configPath, w))

	return cmd
}

// positionListOpts holds the options for the position list subcommand.
type positionListOpts struct {
	AllAccounts bool         `flag:"all-accounts" flagdescr:"Show positions across all linked accounts"`
	Symbols     []string     `flag:"symbol" flagdescr:"Filter by symbol (repeatable, comma-separated values allowed)"`
	LosersOnly  bool         `flag:"losers-only" flagdescr:"Show only positions with negative unrealized P&L"`
	MinPnL      float64      `flag:"min-pnl" flagdescr:"Minimum unrealized P&L to include when set"`
	MaxPnL      float64      `flag:"max-pnl" flagdescr:"Maximum unrealized P&L to include when set"`
	Sort        positionSort `flag:"sort" flagdescr:"Sort positions by pnl-desc, pnl-asc, or value-desc"`
}

// Attach implements structcli.Options interface.
func (o *positionListOpts) Attach(_ *cobra.Command) error { return nil }

// newPositionListCmd returns the Cobra subcommand for listing positions.
// Default: single account (resolved via --account flag or config default).
// With --all-accounts: flattens positions from all linked accounts into a
// single list with account identifiers on each entry.
func newPositionListCmd(c *client.Ref, configPath string, w io.Writer) *cobra.Command {
	opts := &positionListOpts{}
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List positions for one or all accounts",
		Long: `List positions as a flat list with account identifiers and computed cost basis
and P&L fields that Schwab's API does not provide directly. Uses the default
account unless --account or --all-accounts is specified. Computed fields
include totalCostBasis, unrealizedPnL, and unrealizedPnLPct. Apply local symbol,
P&L, and sort filters to shrink output for portfolio triage workflows.`,
		Example: `  schwab-agent position list
	  schwab-agent position list --account ABCDEF1234567890
	  schwab-agent position list --all-accounts
	  schwab-agent position list --symbol AAPL --symbol MSFT
	  schwab-agent position list --losers-only --sort pnl-asc
	  schwab-agent position list --min-pnl -100 --max-pnl 500 --sort value-desc`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := structcli.Unmarshal(cmd, opts); err != nil {
				return err
			}

			filters, err := newPositionFilters(cmd, opts)
			if err != nil {
				return err
			}

			account, err := cmd.Flags().GetString("account")
			if err != nil {
				return err
			}

			if opts.AllAccounts && account != "" {
				return apperr.NewValidationError("--all-accounts and --account are mutually exclusive", nil)
			}

			if opts.AllAccounts {
				return listAllAccountPositions(cmd.Context(), c, filters, w)
			}

			return cobraListSingleAccountPositions(cmd.Context(), c, configPath, account, filters, w)
		},
	}

	if err := structcli.Define(cmd, opts); err != nil {
		panic(err)
	}

	cmd.PersistentFlags().String("account", "", "Account hash, account number, or nickname (overrides config default)")

	return cmd
}

// cobraListSingleAccountPositions fetches positions for a single resolved
// account. Cobra-specific variant that accepts the account flag value directly.
func cobraListSingleAccountPositions(
	ctx context.Context,
	c *client.Ref,
	configPath, accountFlag string,
	filters positionFilters,
	w io.Writer,
) error {
	hash, err := resolveAccount(c, accountFlag, configPath, nil)
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
		accounts := []models.Account{*account}
		enrichAccountsWithPreferences(accounts, prefs)
		account = &accounts[0]
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
	entries = filterAndSortPositionEntries(entries, filters)

	meta := output.NewMetadata()
	meta.Account = hash
	meta.Returned = len(entries)

	return output.WriteSuccess(w, positionListData{Positions: entries}, meta)
}

// newPositionFilters normalizes local filtering flags once so the fetch paths
// can share identical behavior across single-account and all-account modes.
func newPositionFilters(cmd *cobra.Command, opts *positionListOpts) (positionFilters, error) {
	filters := positionFilters{
		Symbols:    make(map[string]struct{}),
		LosersOnly: opts.LosersOnly,
		Sort:       opts.Sort,
	}

	for _, raw := range opts.Symbols {
		for part := range strings.SplitSeq(raw, ",") {
			trimmed := strings.TrimSpace(part)
			if trimmed != "" {
				filters.Symbols[strings.ToUpper(trimmed)] = struct{}{}
			}
		}
	}

	minChanged := cmd.Flags().Changed("min-pnl")
	maxChanged := cmd.Flags().Changed("max-pnl")
	if minChanged {
		minPnL := opts.MinPnL
		filters.MinPnL = &minPnL
	}
	if maxChanged {
		maxPnL := opts.MaxPnL
		filters.MaxPnL = &maxPnL
	}
	if minChanged && maxChanged && opts.MinPnL > opts.MaxPnL {
		return positionFilters{}, apperr.NewValidationError("--min-pnl cannot be greater than --max-pnl", nil)
	}

	return filters, nil
}

// filterAndSortPositionEntries applies local filters and optional sorting while
// preserving a non-nil empty slice for stable JSON output.
func filterAndSortPositionEntries(entries []positionEntry, filters positionFilters) []positionEntry {
	filtered := make([]positionEntry, 0, len(entries))
	for i := range entries {
		if matchesPositionFilters(&entries[i], filters) {
			filtered = append(filtered, entries[i])
		}
	}

	sortPositionEntries(filtered, filters.Sort)

	if filtered == nil {
		return []positionEntry{}
	}

	return filtered
}

// matchesPositionFilters returns true when a position satisfies every active
// local filter. P&L filters exclude entries without computed P&L because those
// positions cannot be proven inside the requested numeric range.
func matchesPositionFilters(entry *positionEntry, filters positionFilters) bool {
	if len(filters.Symbols) > 0 && !matchesPositionSymbol(entry, filters.Symbols) {
		return false
	}

	if filters.LosersOnly || filters.MinPnL != nil || filters.MaxPnL != nil {
		if entry.UnrealizedPnL == nil {
			return false
		}

		pnl := *entry.UnrealizedPnL
		if filters.LosersOnly && pnl >= 0 {
			return false
		}
		if filters.MinPnL != nil && pnl < *filters.MinPnL {
			return false
		}
		if filters.MaxPnL != nil && pnl > *filters.MaxPnL {
			return false
		}
	}

	return true
}

// matchesPositionSymbol compares symbols case-insensitively because the Schwab
// API and human-entered CLI values may differ in casing for equities and ETFs.
func matchesPositionSymbol(entry *positionEntry, symbols map[string]struct{}) bool {
	if entry.Instrument == nil || entry.Instrument.Symbol == nil {
		return false
	}

	symbol := strings.ToUpper(strings.TrimSpace(*entry.Instrument.Symbol))
	_, ok := symbols[symbol]
	return ok
}

// sortPositionEntries applies stable sorting so equal or unknown values retain
// the Schwab response order. Unknown numeric fields always sort last.
func sortPositionEntries(entries []positionEntry, sortBy positionSort) {
	switch sortBy {
	case positionSortPnLDesc:
		sort.SliceStable(entries, func(i, j int) bool {
			return optionalFloatLess(entries[i].UnrealizedPnL, entries[j].UnrealizedPnL, true)
		})
	case positionSortPnLAsc:
		sort.SliceStable(entries, func(i, j int) bool {
			return optionalFloatLess(entries[i].UnrealizedPnL, entries[j].UnrealizedPnL, false)
		})
	case positionSortValueDesc:
		sort.SliceStable(entries, func(i, j int) bool {
			return optionalFloatLess(entries[i].MarketValue, entries[j].MarketValue, true)
		})
	}
}

// optionalFloatLess compares optional numeric values for sort.SliceStable.
// Nil values sort last in both directions because missing P&L or market value
// should not outrank known portfolio data.
func optionalFloatLess(left, right *float64, desc bool) bool {
	if left == nil && right == nil {
		return false
	}
	if left == nil {
		return false
	}
	if right == nil {
		return true
	}
	if *left == *right {
		return false
	}
	if desc {
		return *left > *right
	}
	return *left < *right
}
