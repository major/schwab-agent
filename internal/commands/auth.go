// Package commands provides CLI command builders for schwab-agent.
package commands

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/leodido/structcli"
	"github.com/spf13/cobra"

	"github.com/major/schwab-agent/internal/auth"
	"github.com/major/schwab-agent/internal/client"
	"github.com/major/schwab-agent/internal/models"
	"github.com/major/schwab-agent/internal/output"
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

// authLoginOpts holds the options for the auth login subcommand.
type authLoginOpts struct {
	NoBrowser bool `flag:"no-browser" flagdescr:"Print the authorization URL in the JSON response instead of opening a browser"`
}

// Attach implements structcli.Options interface.
func (o *authLoginOpts) Attach(_ *cobra.Command) error { return nil }

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

// AuthDeps holds injectable dependencies for Cobra auth commands. Production
// code passes AuthDeps{} and relies on completeAuthDeps to fill defaults;
// tests override specific fields.
type AuthDeps struct {
	ConfigPath         func() string
	OAuthTokenEndpoint func() string
	RefreshAccessToken func(*auth.Config, *auth.TokenFile, string) (*auth.TokenFile, error)
	RunLogin           func(*auth.Config, string, string, bool, io.Writer) error
	NewAccountClient   func(string, *auth.Config) accountNumbersClient
}

// DefaultAuthDeps returns the production dependency set for Cobra auth commands.
func DefaultAuthDeps() AuthDeps {
	defaults := defaultAuthDeps()

	return AuthDeps{
		ConfigPath:         defaults.configPath,
		OAuthTokenEndpoint: defaults.oauthTokenEndpoint,
		RefreshAccessToken: defaults.refreshAccessToken,
		RunLogin:           defaults.runLogin,
		NewAccountClient:   defaults.newAccountClient,
	}
}

// NewAuthCmd returns the Cobra auth parent command with login, status, and
// refresh subcommands. Auth commands bypass PersistentPreRunE (via skipAuth
// annotation) and load config from disk in each subcommand's RunE.
func NewAuthCmd(configPath, tokenPath string, w io.Writer, deps AuthDeps) *cobra.Command {
	deps = completeAuthDeps(deps, configPath)

	cmd := &cobra.Command{
		Use:         "auth",
		Short:       "Authentication commands",
		Annotations: map[string]string{"skipAuth": "true"},
		GroupID:     "account-mgmt",
		RunE:        requireSubcommand,
	}
	cmd.SetFlagErrorFunc(suggestSubcommands)

	cmd.AddCommand(newAuthLoginCmd(tokenPath, w, deps))
	cmd.AddCommand(newAuthStatusCmd(tokenPath, w, deps))
	cmd.AddCommand(newAuthRefreshCmd(tokenPath, w, deps))

	return cmd
}

// newAuthLoginCmd returns the Cobra login subcommand.
func newAuthLoginCmd(tokenPath string, w io.Writer, deps AuthDeps) *cobra.Command {
	opts := &authLoginOpts{}
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Run the OAuth login flow",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := structcli.Unmarshal(cmd, opts); err != nil {
				return err
			}

			defaultConfigPath := deps.ConfigPath()
			resolvedConfigPath := cobraResolveConfigPath(cmd, defaultConfigPath)
			resolvedTokenPath := cobraResolveTokenPath(cmd, tokenPath)

			// Load config from disk since PersistentPreRunE is skipped.
			loginConfig, err := requireAuthConfig(nil, resolvedConfigPath)
			if err != nil {
				return err
			}

			var loginOutput strings.Builder
			openBrowser := !opts.NoBrowser

			if err := deps.RunLogin(loginConfig, resolvedTokenPath, deps.OAuthTokenEndpoint(), openBrowser, &loginOutput); err != nil {
				return err
			}

			tokenFile, err := auth.LoadToken(resolvedTokenPath)
			if err != nil {
				return err
			}

			defaultAccount := configDefaultAccount(loginConfig)
			autoSetDefault := false

			accounts, err := deps.NewAccountClient(tokenFile.Token.AccessToken, loginConfig).AccountNumbers(cmd.Context())
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

	if err := structcli.Define(cmd, opts); err != nil {
		panic(err)
	}

	return cmd
}

// newAuthStatusCmd returns the Cobra status subcommand.
func newAuthStatusCmd(tokenPath string, w io.Writer, deps AuthDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show token and config status",
		RunE: func(cmd *cobra.Command, _ []string) error {
			defaultConfigPath := deps.ConfigPath()
			resolvedConfigPath := cobraResolveConfigPath(cmd, defaultConfigPath)
			resolvedTokenPath := cobraResolveTokenPath(cmd, tokenPath)

			tokenFile, err := auth.LoadToken(resolvedTokenPath)
			if err != nil {
				return err
			}

			// Load config best-effort since PersistentPreRunE is skipped.
			statusConfig := optionalAuthConfig(nil, resolvedConfigPath)
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

// newAuthRefreshCmd returns the Cobra refresh subcommand.
func newAuthRefreshCmd(tokenPath string, w io.Writer, deps AuthDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "refresh",
		Short: "Refresh the current access token",
		RunE: func(cmd *cobra.Command, _ []string) error {
			defaultConfigPath := deps.ConfigPath()
			resolvedConfigPath := cobraResolveConfigPath(cmd, defaultConfigPath)
			resolvedTokenPath := cobraResolveTokenPath(cmd, tokenPath)

			// Load config from disk since PersistentPreRunE is skipped.
			refreshConfig, err := requireAuthConfig(nil, resolvedConfigPath)
			if err != nil {
				return err
			}

			tokenFile, err := auth.LoadToken(resolvedTokenPath)
			if err != nil {
				return err
			}

			refreshedToken, err := deps.RefreshAccessToken(refreshConfig, tokenFile, deps.OAuthTokenEndpoint())
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

// completeAuthDeps fills nil dependency fields with production defaults.
// The configPath parameter becomes the default ConfigPath when not overridden.
func completeAuthDeps(deps AuthDeps, configPath string) AuthDeps {
	if deps.ConfigPath == nil {
		deps.ConfigPath = func() string { return configPath }
	}

	if deps.OAuthTokenEndpoint == nil {
		deps.OAuthTokenEndpoint = func() string { return "" }
	}

	if deps.RefreshAccessToken == nil {
		deps.RefreshAccessToken = auth.RefreshAccessToken
	}

	if deps.RunLogin == nil {
		deps.RunLogin = auth.RunLogin
	}

	if deps.NewAccountClient == nil {
		defaults := defaultAuthDeps()
		deps.NewAccountClient = defaults.newAccountClient
	}

	return deps
}

// cobraResolveConfigPath resolves the config path for Cobra auth commands.
// It checks the --config persistent flag inherited from root and falls back
// to the default when the flag is absent (standalone test mode) or unchanged.
func cobraResolveConfigPath(cmd *cobra.Command, fallback string) string {
	if f := cmd.Flag("config"); f != nil && f.Changed {
		if path := strings.TrimSpace(f.Value.String()); path != "" {
			return path
		}
	}

	return fallback
}

// cobraResolveTokenPath resolves the token path for Cobra auth commands.
// It checks the --token persistent flag inherited from root and falls back
// to the default when the flag is absent (standalone test mode) or unchanged.
func cobraResolveTokenPath(cmd *cobra.Command, fallback string) string {
	if f := cmd.Flag("token"); f != nil && f.Changed {
		if path := strings.TrimSpace(f.Value.String()); path != "" {
			return path
		}
	}

	return fallback
}
