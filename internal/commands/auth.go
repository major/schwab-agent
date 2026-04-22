// Package commands provides urfave/cli command builders for schwab-agent.
package commands

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/urfave/cli/v3"

	"github.com/major/schwab-agent/internal/auth"
	"github.com/major/schwab-agent/internal/client"
	"github.com/major/schwab-agent/internal/output"
	"github.com/major/schwab-agent/internal/models"
)

const refreshTokenMaxAgeSeconds int64 = 561600

var (
	// authConfigPathFunc resolves the config file path for commands that need it.
	authConfigPathFunc = defaultAuthConfigPath
	// oauthTokenEndpointFunc resolves the OAuth token endpoint, mainly for tests.
	oauthTokenEndpointFunc = func() string { return "" }
	// loadConfigFunc wraps auth.LoadConfig for test overrides.
	loadConfigFunc = auth.LoadConfig
	// loadTokenFunc wraps auth.LoadToken for test overrides.
	loadTokenFunc = auth.LoadToken
	// saveTokenFunc wraps auth.SaveToken for test overrides.
	saveTokenFunc = auth.SaveToken
	// refreshAccessTokenFunc wraps auth.RefreshAccessToken for test overrides.
	refreshAccessTokenFunc = auth.RefreshAccessToken
	// runLoginFunc wraps auth.RunLogin for test overrides.
	runLoginFunc = auth.RunLogin
	// setDefaultAccountFunc wraps auth.SetDefaultAccount for test overrides.
	setDefaultAccountFunc = auth.SetDefaultAccount
	// newAccountClientFunc constructs account-capable API clients for login follow-up.
	newAccountClientFunc = func(token string) accountNumbersClient {
		return client.NewClient(token)
	}
)

// accountNumbersClient captures the client behavior needed by auth login.
type accountNumbersClient interface {
	AccountNumbers(ctx context.Context) ([]models.AccountNumber, error)
}

// authStatusData is the success payload for auth status responses.
type authStatusData struct {
	Valid            bool   `json:"valid"`
	ExpiresAt        string `json:"expires_at"`
	RefreshExpiresAt string `json:"refresh_expires_at"`
	DefaultAccount   string `json:"default_account"`
	ClientID         string `json:"client_id"`
}

// authLoginData is the success payload for auth login responses.
type authLoginData struct {
	Valid            bool   `json:"valid"`
	ExpiresAt        string `json:"expires_at"`
	RefreshExpiresAt string `json:"refresh_expires_at"`
	DefaultAccount   string `json:"default_account"`
	AuthorizationURL string `json:"authorization_url,omitempty"`
	AutoSetDefault   bool   `json:"auto_set_default"`
}

// authRefreshData is the success payload for auth refresh responses.
type authRefreshData struct {
	ExpiresAt string `json:"expires_at"`
}

// AuthCommand returns the parent auth command with setup, login, status, and refresh subcommands.
func AuthCommand(cfg *auth.Config, tokenPath string, w io.Writer) *cli.Command {
	return &cli.Command{
		Name:  "auth",
		Usage: "Authentication commands",
		Commands: []*cli.Command{
			authLoginCommand(cfg, tokenPath, w),
			authStatusCommand(cfg, tokenPath, w),
			authRefreshCommand(cfg, tokenPath, w),
		},
	}
}

// AccountSetDefaultCommand returns the account set-default subcommand.
func AccountSetDefaultCommand(configPath string, w io.Writer) *cli.Command {
	return &cli.Command{
		Name:      "set-default",
		Usage:     "Set the default account hash",
		ArgsUsage: "<hash>",
		Action: func(_ context.Context, cmd *cli.Command) error {
			hash := strings.TrimSpace(cmd.Args().First())
			if err := requireArg(hash, "account hash"); err != nil {
				return err
			}

			if err := setDefaultAccountFunc(configPath, hash); err != nil {
				return err
			}

			return output.WriteSuccess(w, map[string]any{
				"default_account": hash,
			}, output.TimestampMeta())
		},
	}
}

// authLoginCommand returns the auth login command.
func authLoginCommand(cfg *auth.Config, tokenPath string, w io.Writer) *cli.Command {
	return &cli.Command{
		Name:  "login",
		Usage: "Run the OAuth login flow",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "no-browser",
				Usage: "Print the authorization URL in the JSON response instead of opening a browser",
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			loginConfig, err := requireAuthConfig(cfg)
			if err != nil {
				return err
			}

			var loginOutput strings.Builder
			openBrowser := !cmd.Bool("no-browser")
			if err := runLoginFunc(loginConfig, tokenPath, oauthTokenEndpointFunc(), openBrowser, &loginOutput); err != nil {
				return err
			}

			tokenFile, err := loadTokenFunc(tokenPath)
			if err != nil {
				return err
			}

			defaultAccount := configDefaultAccount(loginConfig)
			autoSetDefault := false
			accounts, err := newAccountClientFunc(tokenFile.Token.AccessToken).AccountNumbers(ctx)
			if err != nil {
				return err
			}

			if len(accounts) == 1 {
				defaultAccount = accounts[0].HashValue
				if err := setDefaultAccountFunc(authConfigPathFunc(), defaultAccount); err != nil {
					return err
				}
				autoSetDefault = true
			}

			data := authLoginData{
				Valid:            !auth.IsAccessTokenExpired(tokenFile),
				ExpiresAt:        unixSecondsToRFC3339(tokenFile.Token.ExpiresAt),
				RefreshExpiresAt: refreshExpiryRFC3339(tokenFile),
				DefaultAccount:   defaultAccount,
				AuthorizationURL: strings.TrimSpace(loginOutput.String()),
				AutoSetDefault:   autoSetDefault,
			}

			return output.WriteSuccess(w, data, output.TimestampMeta())
		},
	}
}

// authStatusCommand returns the auth status command.
func authStatusCommand(cfg *auth.Config, tokenPath string, w io.Writer) *cli.Command {
	return &cli.Command{
		Name:  "status",
		Usage: "Show token and config status",
		Action: func(_ context.Context, _ *cli.Command) error {
			tokenFile, err := loadTokenFunc(tokenPath)
			if err != nil {
				return err
			}

			statusConfig := optionalAuthConfig(cfg)
			data := authStatusData{
				Valid:            !auth.IsAccessTokenExpired(tokenFile),
				ExpiresAt:        unixSecondsToRFC3339(tokenFile.Token.ExpiresAt),
				RefreshExpiresAt: refreshExpiryRFC3339(tokenFile),
				DefaultAccount:   configDefaultAccount(statusConfig),
				ClientID:         redactClientID(statusConfig.ClientID),
			}

			return output.WriteSuccess(w, data, output.TimestampMeta())
		},
	}
}

// authRefreshCommand returns the auth refresh command.
func authRefreshCommand(cfg *auth.Config, tokenPath string, w io.Writer) *cli.Command {
	return &cli.Command{
		Name:  "refresh",
		Usage: "Refresh the current access token",
		Action: func(_ context.Context, _ *cli.Command) error {
			refreshConfig, err := requireAuthConfig(cfg)
			if err != nil {
				return err
			}

			tokenFile, err := loadTokenFunc(tokenPath)
			if err != nil {
				return err
			}

			refreshedToken, err := refreshAccessTokenFunc(refreshConfig, tokenFile, oauthTokenEndpointFunc())
			if err != nil {
				return err
			}

			if err := saveTokenFunc(tokenPath, refreshedToken); err != nil {
				return err
			}

			return output.WriteSuccess(w, authRefreshData{
				ExpiresAt: unixSecondsToRFC3339(refreshedToken.Token.ExpiresAt),
			}, output.TimestampMeta())
		},
	}
}

// requireAuthConfig returns a valid auth config or loads it from disk.
func requireAuthConfig(cfg *auth.Config) (*auth.Config, error) {
	if cfg != nil && cfg.ClientID != "" && cfg.ClientSecret != "" {
		return cfg, nil
	}

	return loadConfigFunc(authConfigPathFunc())
}

// optionalAuthConfig returns the provided config or best-effort loaded config.
func optionalAuthConfig(cfg *auth.Config) *auth.Config {
	if cfg != nil {
		return cfg
	}

	loadedConfig, err := loadConfigFunc(authConfigPathFunc())
	if err != nil {
		return &auth.Config{}
	}

	return loadedConfig
}

// defaultAuthConfigPath returns the default auth config path under XDG config home.
func defaultAuthConfigPath() string {
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return filepath.Join(".config", "schwab-agent", "config.json")
		}
		configHome = filepath.Join(homeDir, ".config")
	}

	return filepath.Join(configHome, "schwab-agent", "config.json")
}

// unixSecondsToRFC3339 converts Unix seconds into RFC3339, returning an empty string for zero.
func unixSecondsToRFC3339(seconds float64) string {
	if seconds <= 0 {
		return ""
	}

	return time.Unix(int64(seconds), 0).UTC().Format(time.RFC3339)
}

// refreshExpiryRFC3339 converts the refresh token lifetime into RFC3339.
func refreshExpiryRFC3339(tf *auth.TokenFile) string {
	if tf == nil || tf.CreationTimestamp <= 0 {
		return ""
	}

	return time.Unix(tf.CreationTimestamp+refreshTokenMaxAgeSeconds, 0).UTC().Format(time.RFC3339)
}

// redactClientID shortens the client ID to the first four characters plus ellipsis.
func redactClientID(clientID string) string {
	clientID = strings.TrimSpace(clientID)
	if clientID == "" {
		return ""
	}

	if len(clientID) <= 4 {
		return clientID + "..."
	}

	return clientID[:4] + "..."
}

// configDefaultAccount returns the configured default account, if any.
func configDefaultAccount(cfg *auth.Config) string {
	if cfg == nil {
		return ""
	}

	return strings.TrimSpace(cfg.DefaultAccount)
}
