// Package commands provides urfave/cli command builders for schwab-agent.
package commands

import (
	"context"
	"io"
	"strconv"
	"strings"

	"github.com/urfave/cli/v3"

	"github.com/major/schwab-agent/internal/auth"
	"github.com/major/schwab-agent/internal/client"
	"github.com/major/schwab-agent/internal/apperr"
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

// AccountCommand returns the parent CLI command for account operations.
// Subcommands: list, get, numbers, set-default, transaction.
// The --account flag overrides the default account hash for all subcommands.
func AccountCommand(c *client.Ref, configPath string, w io.Writer) *cli.Command {
	return &cli.Command{
		Name:   "account",
		Usage:  "Manage Schwab trading accounts",
		Action: requireSubcommand(),
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "account",
				Usage: "Account hash to use (overrides config default)",
			},
		},
		Commands: []*cli.Command{
			accountListCommand(c, w),
			accountGetCommand(c, configPath, w),
			accountNumbersCommand(c, w),
			AccountSetDefaultCommand(configPath, w),
			accountTransactionCommand(c, configPath, w),
		},
	}
}

// accountListCommand returns the CLI command for listing all accounts.
func accountListCommand(c *client.Ref, w io.Writer) *cli.Command {
	return &cli.Command{
		Name:  "list",
		Usage: "List all accounts with nicknames and settings",
		UsageText: `schwab-agent account list
schwab-agent account list --positions`,
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "positions",
				Usage: "Include current positions for each account",
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			var fields []string
			if cmd.Bool("positions") {
				fields = append(fields, "positions")
			}

			accounts, err := c.Accounts(ctx, fields...)
			if err != nil {
				return err
			}

			// Enrich accounts with nicknames from user preferences.
			// Best-effort: if preferences fail, return accounts without nicknames.
			prefs, prefsErr := c.UserPreference(ctx)
			if prefsErr == nil {
				enrichAccountsWithPreferences(accounts, prefs)
			}

			return output.WriteSuccess(w, accountListData{Accounts: accounts}, output.NewMetadata())
		},
	}
}

// accountGetCommand returns the CLI command for getting a specific account.
func accountGetCommand(c *client.Ref, configPath string, w io.Writer) *cli.Command {
	return &cli.Command{
		Name:  "get",
		Usage: "Get account details by hash value",
		UsageText: `schwab-agent account get
schwab-agent account get --positions`,
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "positions",
				Usage: "Include current positions in the account response",
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			hash, err := resolveAccount(cmd.String("account"), configPath, cmd.Args().Slice())
			if err != nil {
				return err
			}

			var fields []string
			if cmd.Bool("positions") {
				fields = append(fields, "positions")
			}

			account, err := c.Account(ctx, hash, fields...)
			if err != nil {
				return err
			}

			// Enrich with nickname from user preferences (best-effort).
			prefs, prefsErr := c.UserPreference(ctx)
			if prefsErr == nil {
				enrichAccountWithPreferences(account, prefs)
			}

			meta := output.NewMetadata()
			meta.Account = hash

			return output.WriteSuccess(w, account, meta)
		},
	}
}

// accountNumbersCommand returns the CLI command for listing account numbers and hash values.
func accountNumbersCommand(c *client.Ref, w io.Writer) *cli.Command {
	return &cli.Command{
		Name:      "numbers",
		Usage:     "List account numbers and hash values",
		UsageText: "schwab-agent account numbers",
		Action: func(ctx context.Context, _ *cli.Command) error {
			numbers, err := c.AccountNumbers(ctx)
			if err != nil {
				return err
			}

			return output.WriteSuccess(w, accountNumbersData{Accounts: numbers}, output.NewMetadata())
		},
	}
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

// accountTransactionCommand returns the CLI subcommand for transaction queries.
// Transactions are account-scoped, so they live under the account parent command
// and inherit the --account flag.
func accountTransactionCommand(c *client.Ref, configPath string, w io.Writer) *cli.Command {
	return &cli.Command{
		Name:   "transaction",
		Usage:  "List and look up account transactions",
		Action: requireSubcommand(),
		Commands: []*cli.Command{
			{
				Name:  "list",
				Usage: "List transactions for an account",
				UsageText: `schwab-agent account transaction list
schwab-agent account transaction list --types TRADE --symbol AAPL
schwab-agent account transaction list --from 2025-01-01T00:00:00Z --to 2025-01-31T23:59:59Z`,
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "types", Usage: "Transaction type filter (TRADE, DIVIDEND, etc.)"},
					&cli.StringFlag{Name: "from", Usage: "Start date (YYYY-MM-DDTHH:MM:SSZ)"},
					&cli.StringFlag{Name: "to", Usage: "End date (YYYY-MM-DDTHH:MM:SSZ)"},
					&cli.StringFlag{Name: "symbol", Usage: "Filter by symbol"},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					account, err := resolveAccount(cmd.String("account"), configPath, nil)
					if err != nil {
						return err
					}

					params := client.TransactionParams{
						Types:     cmd.String("types"),
						StartDate: cmd.String("from"),
						EndDate:   cmd.String("to"),
						Symbol:    cmd.String("symbol"),
					}

					result, err := c.Transactions(ctx, account, params)
					if err != nil {
						return err
					}

					return output.WriteSuccess(w, transactionListData{Transactions: result}, output.NewMetadata())
				},
			},
			{
				Name:      "get",
				Usage:     "Get a specific transaction by ID",
				UsageText: "schwab-agent account transaction get 98765432101",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					txnIDStr := cmd.Args().First()
					if err := requireArg(txnIDStr, "transaction ID"); err != nil {
						return err
					}

					txnID, err := strconv.ParseInt(txnIDStr, 10, 64)
					if err != nil {
						return apperr.NewValidationError("transaction ID must be a number", nil)
					}

					account, err := resolveAccount(cmd.String("account"), configPath, nil)
					if err != nil {
						return err
					}

					result, err := c.Transaction(ctx, account, txnID)
					if err != nil {
						return err
					}

					return output.WriteSuccess(w, transactionGetData{Transaction: result}, output.NewMetadata())
				},
			},
		},
	}
}


