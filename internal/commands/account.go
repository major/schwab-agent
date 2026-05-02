// Package commands provides CLI command builders for schwab-agent.
package commands

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/leodido/structcli"
	"github.com/spf13/cobra"

	"github.com/major/schwab-agent/internal/apperr"
	"github.com/major/schwab-agent/internal/auth"
	"github.com/major/schwab-agent/internal/client"
	"github.com/major/schwab-agent/internal/models"
	"github.com/major/schwab-agent/internal/output"
)

// accountListData wraps the account list response.
type accountListData struct {
	Accounts []models.Account `json:"accounts"`
}

// accountNumbersData wraps the account numbers response.
type accountNumbersData struct {
	Accounts []models.AccountNumber `json:"accounts"`
}

// transactionListData wraps the transaction list response.
type transactionListData struct {
	Transactions []models.Transaction `json:"transactions"`
}

// transactionGetData wraps a single transaction response.
type transactionGetData struct {
	Transaction *models.Transaction `json:"transaction"`
}

// accountListOpts holds the options for the account list subcommand.
type accountListOpts struct {
	Positions bool
}

// accountGetOpts holds the options for the account get subcommand.
type accountGetOpts struct {
	Positions bool
}

// accountTransactionListOpts holds the options for the account transaction list subcommand.
type accountTransactionListOpts struct {
	Types  string
	From   string
	To     string
	Symbol string
}

// resolveAccount determines the account hash from multiple sources.
// Priority: flag > positional args (if provided) > config default > error.
// Pass nil for positionalArgs when the command doesn't accept positional account arguments.
func resolveAccount(accountFlag, configPath string, positionalArgs []string) (string, error) {
	if strings.TrimSpace(accountFlag) != "" {
		return strings.TrimSpace(accountFlag), nil
	}

	if len(positionalArgs) > 0 && strings.TrimSpace(positionalArgs[0]) != "" {
		return strings.TrimSpace(positionalArgs[0]), nil
	}

	if configPath != "" {
		cfg, err := auth.LoadConfig(configPath)
		if err == nil && strings.TrimSpace(cfg.DefaultAccount) != "" {
			return strings.TrimSpace(cfg.DefaultAccount), nil
		}
	}

	return "", apperr.NewAccountNotFoundError(
		"no account specified: use --account flag or set default_account in config",
		nil,
		apperr.WithDetails("Run `schwab-agent account numbers` to list accounts, then `schwab-agent account set-default` to set one"),
	)
}

// enrichAccountWithPreferences adds nickname and primary account flag from
// user preferences to a single account. Matches by account number.
func enrichAccountWithPreferences(account *models.Account, prefs *models.UserPreference) {
	if prefs == nil || account.SecuritiesAccount == nil || account.SecuritiesAccount.AccountNumber == nil {
		return
	}

	for i := range prefs.Accounts {
		if prefs.Accounts[i].AccountNumber != nil && *prefs.Accounts[i].AccountNumber == *account.SecuritiesAccount.AccountNumber {
			account.NickName = prefs.Accounts[i].NickName
			account.PrimaryAccount = prefs.Accounts[i].PrimaryAccount

			return
		}
	}
}

// enrichAccountsWithPreferences adds nickname and primary account flag from
// user preferences to each account in the slice. Builds a lookup map for O(n) matching.
func enrichAccountsWithPreferences(accounts []models.Account, prefs *models.UserPreference) {
	if prefs == nil {
		return
	}

	prefMap := make(map[string]*models.UserPreferenceAccount, len(prefs.Accounts))
	for i := range prefs.Accounts {
		if prefs.Accounts[i].AccountNumber != nil {
			prefMap[*prefs.Accounts[i].AccountNumber] = &prefs.Accounts[i]
		}
	}

	for i := range accounts {
		if accounts[i].SecuritiesAccount == nil || accounts[i].SecuritiesAccount.AccountNumber == nil {
			continue
		}

		if pref, ok := prefMap[*accounts[i].SecuritiesAccount.AccountNumber]; ok {
			accounts[i].NickName = pref.NickName
			accounts[i].PrimaryAccount = pref.PrimaryAccount
		}
	}
}

// NewAccountCmd returns the Cobra parent command for account operations.
// Subcommands: list, get, numbers, set-default, transaction.
// The --account persistent flag overrides the default account hash for all subcommands.
func NewAccountCmd(c *client.Ref, configPath string, w io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "account",
		Short:   "Manage Schwab trading accounts",
		GroupID: "account-mgmt",
		RunE:    requireSubcommand,
	}
	cmd.SetFlagErrorFunc(suggestSubcommands)

	cmd.PersistentFlags().String("account", "", "Account hash to use (overrides config default)")

	cmd.AddCommand(
		newAccountListCmd(c, w),
		newAccountGetCmd(c, configPath, w),
		newAccountNumbersCmd(c, w),
		newAccountSetDefaultCmd(configPath, w),
		newAccountTransactionCmd(c, configPath, w),
	)

	return cmd
}

// newAccountListCmd returns the Cobra subcommand for listing all accounts.
func newAccountListCmd(c *client.Ref, w io.Writer) *cobra.Command {
	opts := &accountListOpts{}
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all accounts with nicknames and settings",
		Example: "schwab-agent account list\n" +
			"schwab-agent account list --positions",
		RunE: func(cmd *cobra.Command, _ []string) error {
			var fields []string
			if opts.Positions {
				fields = append(fields, "positions")
			}

			accounts, err := c.Accounts(cmd.Context(), fields...)
			if err != nil {
				return err
			}

			// Enrich accounts with nicknames from user preferences.
			// Best-effort: if preferences fail, return accounts without nicknames.
			prefs, prefsErr := c.UserPreference(cmd.Context())
			if prefsErr == nil {
				enrichAccountsWithPreferences(accounts, prefs)
			}

			return output.WriteSuccess(w, accountListData{Accounts: accounts}, output.NewMetadata())
		},
	}

	cmd.Flags().BoolVar(&opts.Positions, "positions", false, "Include current positions for each account")

	return cmd
}

// newAccountGetCmd returns the Cobra subcommand for getting a specific account.
func newAccountGetCmd(c *client.Ref, configPath string, w io.Writer) *cobra.Command {
	opts := &accountGetOpts{}
	cmd := &cobra.Command{
		Use:   "get [hash]",
		Short: "Get account details by hash value",
		Example: "schwab-agent account get\n" +
			"schwab-agent account get --positions",
		RunE: func(cmd *cobra.Command, args []string) error {
			accountFlag, err := cmd.Flags().GetString("account")
			if err != nil {
				return err
			}

			hash, err := resolveAccount(accountFlag, configPath, args)
			if err != nil {
				return err
			}

			var fields []string
			if opts.Positions {
				fields = append(fields, "positions")
			}

			account, err := c.Account(cmd.Context(), hash, fields...)
			if err != nil {
				return err
			}

			// Enrich with nickname from user preferences (best-effort).
			prefs, prefsErr := c.UserPreference(cmd.Context())
			if prefsErr == nil {
				enrichAccountWithPreferences(account, prefs)
			}

			meta := output.NewMetadata()
			meta.Account = hash

			return output.WriteSuccess(w, account, meta)
		},
	}

	cmd.Flags().BoolVar(&opts.Positions, "positions", false, "Include current positions in the account response")

	return cmd
}

// newAccountNumbersCmd returns the Cobra subcommand for listing account numbers and hash values.
func newAccountNumbersCmd(c *client.Ref, w io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:     "numbers",
		Short:   "List account numbers and hash values",
		Example: "schwab-agent account numbers",
		RunE: func(cmd *cobra.Command, _ []string) error {
			numbers, err := c.AccountNumbers(cmd.Context())
			if err != nil {
				return err
			}

			return output.WriteSuccess(w, accountNumbersData{Accounts: numbers}, output.NewMetadata())
		},
	}
}

// newAccountSetDefaultCmd returns the Cobra subcommand for setting the default account hash.
// This command has no safety guard (no requireMutableEnabled) - matches existing behavior.
func newAccountSetDefaultCmd(configPath string, w io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:     "set-default <hash>",
		Short:   "Set the default account hash",
		Example: "schwab-agent account set-default ABCDEF1234567890",
		RunE: func(_ *cobra.Command, args []string) error {
			hash := ""
			if len(args) > 0 {
				hash = strings.TrimSpace(args[0])
			}

			if err := requireArg(hash, "account hash"); err != nil {
				return err
			}

			if err := auth.SetDefaultAccount(configPath, hash); err != nil {
				return err
			}

			return output.WriteSuccess(w, authDefaultAccountData{
				DefaultAccount: hash,
			}, output.NewMetadata())
		},
	}
}

// newAccountTransactionCmd returns the Cobra subcommand for transaction queries.
// Transactions are account-scoped, so they live under the account parent command
// and inherit the --account persistent flag.
func newAccountTransactionCmd(c *client.Ref, configPath string, w io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "transaction",
		Short: "List and look up account transactions",
		RunE:  requireSubcommand,
	}
	cmd.SetFlagErrorFunc(suggestSubcommands)

	cmd.AddCommand(
		newAccountTransactionListCmd(c, configPath, w),
		newAccountTransactionGetCmd(c, configPath, w),
	)

	return cmd
}

// newAccountTransactionListCmd returns the Cobra subcommand for listing transactions.
func newAccountTransactionListCmd(c *client.Ref, configPath string, w io.Writer) *cobra.Command {
	opts := &accountTransactionListOpts{}
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List transactions for an account",
		Example: "schwab-agent account transaction list\n" +
			"schwab-agent account transaction list --types TRADE --symbol AAPL\n" +
			"schwab-agent account transaction list --from 2025-01-01T00:00:00Z --to 2025-01-31T23:59:59Z",
		RunE: func(cmd *cobra.Command, _ []string) error {
			accountFlag, err := cmd.Flags().GetString("account")
			if err != nil {
				return err
			}

			account, err := resolveAccount(accountFlag, configPath, nil)
			if err != nil {
				return err
			}

			params := client.TransactionParams{
				Types:     opts.Types,
				StartDate: opts.From,
				EndDate:   opts.To,
				Symbol:    opts.Symbol,
			}

			result, err := c.Transactions(cmd.Context(), account, params)
			if err != nil {
				return err
			}

			return output.WriteSuccess(w, transactionListData{Transactions: result}, output.NewMetadata())
		},
	}

	cmd.Flags().StringVar(&opts.Types, "types", "", "Transaction type filter (TRADE, DIVIDEND, etc.)")
	cmd.Flags().StringVar(&opts.From, "from", "", "Start date (YYYY-MM-DDTHH:MM:SSZ)")
	cmd.Flags().StringVar(&opts.To, "to", "", "End date (YYYY-MM-DDTHH:MM:SSZ)")
	cmd.Flags().StringVar(&opts.Symbol, "symbol", "", "Filter by symbol")

	return cmd
}

// newAccountTransactionGetCmd returns the Cobra subcommand for getting a specific transaction.
func newAccountTransactionGetCmd(c *client.Ref, configPath string, w io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:     "get <transaction-id>",
		Short:   "Get a specific transaction by ID",
		Example: "schwab-agent account transaction get 98765432101",
		RunE: func(cmd *cobra.Command, args []string) error {
			txnIDStr := ""
			if len(args) > 0 {
				txnIDStr = args[0]
			}

			if err := requireArg(txnIDStr, "transaction ID"); err != nil {
				return err
			}

			txnID, err := strconv.ParseInt(txnIDStr, 10, 64)
			if err != nil {
				return apperr.NewValidationError(
					fmt.Sprintf("transaction ID must be a number: %q", txnIDStr), err,
				)
			}

			accountFlag, err := cmd.Flags().GetString("account")
			if err != nil {
				return err
			}

			account, err := resolveAccount(accountFlag, configPath, nil)
			if err != nil {
				return err
			}

			result, err := c.Transaction(cmd.Context(), account, txnID)
			if err != nil {
				return err
			}

			return output.WriteSuccess(w, transactionGetData{Transaction: result}, output.NewMetadata())
		},
	}
}
