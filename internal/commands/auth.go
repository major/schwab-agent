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

// authDeps holds injectable dependencies for auth commands. Production code
// uses defaultAuthDeps(); tests provide custom implementations for the
// specific dependencies they need to control.
type authDeps struct {
	configPath         func() string
	oauthTokenEndpoint func() string
	refreshAccessToken func(*auth.Config, *auth.TokenFile, string) (*auth.TokenFile, error)
	runLogin           func(*auth.Config, string, string, bool, io.Writer) error
	newAccountClient   func(string, *auth.Config) accountNumbersClient
}

// defaultAuthDeps returns the production dependency set.
func defaultAuthDeps() authDeps {
	return authDeps{
		configPath:         defaultAuthConfigPath,
		oauthTokenEndpoint: func() string { return "" },
		refreshAccessToken: auth.RefreshAccessToken,
		runLogin:           auth.RunLogin,
		newAccountClient: func(token string, cfg *auth.Config) accountNumbersClient {
			clientOptions := []client.Option{
				client.WithBaseURL(cfg.APIBaseURL()),
				client.WithTLSConfig(cfg.TLSConfig()),
			}

			return client.NewClient(token, clientOptions...)
		},
	}
}

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

// authDefaultAccountData is the success payload for set-default responses.
type authDefaultAccountData struct {
	DefaultAccount string `json:"default_account"`
}

// AuthCommand returns the parent auth command with setup, login, status, and refresh subcommands.
func AuthCommand(cfg *auth.Config, tokenPath string, w io.Writer) *cli.Command {
	return newAuthCommand(cfg, tokenPath, w, defaultAuthDeps())
}

// newAuthCommand builds the auth command tree with the given dependencies.
// Tests call this directly with custom deps; production goes through AuthCommand.
func newAuthCommand(cfg *auth.Config, tokenPath string, w io.Writer, deps authDeps) *cli.Command {
	return &cli.Command{
		Name:   "auth",
		Usage:  "Authentication commands",
		Action: requireSubcommand(),
		Commands: []*cli.Command{
			authLoginCommand(cfg, tokenPath, w, deps),
			authStatusCommand(cfg, tokenPath, w, deps),
			authRefreshCommand(cfg, tokenPath, w, deps),
		},
	}
}

// AccountSetDefaultCommand returns the account set-default subcommand.
func AccountSetDefaultCommand(configPath string, w io.Writer) *cli.Command {
	return &cli.Command{
		Name:      "set-default",
		Usage:     "Set the default account hash",
		ArgsUsage: "<hash>",
		UsageText: "schwab-agent account set-default ABCDEF1234567890",
		Action: func(_ context.Context, cmd *cli.Command) error {
			hash := strings.TrimSpace(cmd.Args().First())
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

// authLoginCommand returns the auth login command.
func authLoginCommand(cfg *auth.Config, tokenPath string, w io.Writer, deps authDeps) *cli.Command {
	return &cli.Command{
		Name:  "login",
		Usage: "Run the OAuth login flow",
		UsageText: `schwab-agent auth login
schwab-agent auth login --no-browser`,
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "no-browser",
				Usage: "Print the authorization URL in the JSON response instead of opening a browser",
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			defaultConfigPath := deps.configPath()
			resolvedConfigPath := resolveAuthConfigPath(cmd, defaultConfigPath)
			resolvedTokenPath := resolveAuthTokenPath(cmd, tokenPath)
			effectiveConfig := runtimeAuthConfig(cfg, resolvedConfigPath, defaultConfigPath)

			loginConfig, err := requireAuthConfig(effectiveConfig, resolvedConfigPath)
			if err != nil {
				return err
			}

			var loginOutput strings.Builder
			openBrowser := !cmd.Bool("no-browser")
			if err := deps.runLogin(loginConfig, resolvedTokenPath, deps.oauthTokenEndpoint(), openBrowser, &loginOutput); err != nil {
				return err
			}

			tokenFile, err := auth.LoadToken(resolvedTokenPath)
			if err != nil {
				return err
			}

			defaultAccount := configDefaultAccount(loginConfig)
			autoSetDefault := false
			accounts, err := deps.newAccountClient(tokenFile.Token.AccessToken, loginConfig).AccountNumbers(ctx)
			if err != nil {
				return err
			}

			if len(accounts) == 1 {
				defaultAccount = accounts[0].HashValue
				if err := auth.SetDefaultAccount(resolvedConfigPath, defaultAccount); err != nil {
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

			return output.WriteSuccess(w, data, output.NewMetadata())
		},
	}
}

// authStatusCommand returns the auth status command.
func authStatusCommand(cfg *auth.Config, tokenPath string, w io.Writer, deps authDeps) *cli.Command {
	return &cli.Command{
		Name:      "status",
		Usage:     "Show token and config status",
		UsageText: "schwab-agent auth status",
		Action: func(_ context.Context, cmd *cli.Command) error {
			defaultConfigPath := deps.configPath()
			resolvedConfigPath := resolveAuthConfigPath(cmd, defaultConfigPath)
			resolvedTokenPath := resolveAuthTokenPath(cmd, tokenPath)
			effectiveConfig := runtimeAuthConfig(cfg, resolvedConfigPath, defaultConfigPath)

			tokenFile, err := auth.LoadToken(resolvedTokenPath)
			if err != nil {
				return err
			}

			statusConfig := optionalAuthConfig(effectiveConfig, resolvedConfigPath)
			data := authStatusData{
				Valid:            !auth.IsAccessTokenExpired(tokenFile),
				ExpiresAt:        unixSecondsToRFC3339(tokenFile.Token.ExpiresAt),
				RefreshExpiresAt: refreshExpiryRFC3339(tokenFile),
				DefaultAccount:   configDefaultAccount(statusConfig),
				ClientID:         redactClientID(statusConfig.ClientID),
			}

			return output.WriteSuccess(w, data, output.NewMetadata())
		},
	}
}

// authRefreshCommand returns the auth refresh command.
func authRefreshCommand(cfg *auth.Config, tokenPath string, w io.Writer, deps authDeps) *cli.Command {
	return &cli.Command{
		Name:      "refresh",
		Usage:     "Refresh the current access token",
		UsageText: "schwab-agent auth refresh",
		Action: func(_ context.Context, cmd *cli.Command) error {
			defaultConfigPath := deps.configPath()
			resolvedConfigPath := resolveAuthConfigPath(cmd, defaultConfigPath)
			resolvedTokenPath := resolveAuthTokenPath(cmd, tokenPath)
			effectiveConfig := runtimeAuthConfig(cfg, resolvedConfigPath, defaultConfigPath)

			refreshConfig, err := requireAuthConfig(effectiveConfig, resolvedConfigPath)
			if err != nil {
				return err
			}

			tokenFile, err := auth.LoadToken(resolvedTokenPath)
			if err != nil {
				return err
			}

			refreshedToken, err := deps.refreshAccessToken(refreshConfig, tokenFile, deps.oauthTokenEndpoint())
			if err != nil {
				return err
			}

			if err := auth.SaveToken(resolvedTokenPath, refreshedToken); err != nil {
				return err
			}

			return output.WriteSuccess(w, authRefreshData{
				ExpiresAt: unixSecondsToRFC3339(refreshedToken.Token.ExpiresAt),
			}, output.NewMetadata())
		},
	}
}

// resolveAuthConfigPath returns the runtime config path for auth subcommands.
func resolveAuthConfigPath(cmd *cli.Command, fallback string) string {
	if path := strings.TrimSpace(cmd.String("config")); path != "" {
		return path
	}

	return fallback
}

// resolveAuthTokenPath returns the runtime token path for auth subcommands.
func resolveAuthTokenPath(cmd *cli.Command, fallback string) string {
	if path := strings.TrimSpace(cmd.String("token")); path != "" {
		return path
	}

	return fallback
}

// runtimeAuthConfig returns the captured config only when the command is using
// the same config path that buildApp() used to construct the command tree.
// If the user supplies a different --config path at runtime, we must ignore the
// captured config and reload from disk so auth subcommands see the same proxy
// and credential settings as the rest of the CLI.
func runtimeAuthConfig(cfg *auth.Config, resolvedConfigPath, defaultConfigPath string) *auth.Config {
	if resolvedConfigPath != defaultConfigPath {
		return nil
	}

	return cfg
}

// requireAuthConfig returns a valid auth config or loads it from disk.
func requireAuthConfig(cfg *auth.Config, configPath string) (*auth.Config, error) {
	if cfg != nil && cfg.ClientID != "" && cfg.ClientSecret != "" {
		return cfg, nil
	}

	return auth.LoadConfig(configPath)
}

// optionalAuthConfig returns the provided config or best-effort loaded config.
func optionalAuthConfig(cfg *auth.Config, configPath string) *auth.Config {
	if cfg != nil {
		return cfg
	}

	loadedConfig, err := auth.LoadConfig(configPath)
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
