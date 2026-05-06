package commands

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/spf13/cobra"

	"github.com/major/schwab-agent/internal/apperr"
	"github.com/major/schwab-agent/internal/auth"
	"github.com/major/schwab-agent/internal/client"
)

// RootDeps holds injectable dependencies for the root command auth lifecycle.
type RootDeps struct {
	NewClient            func(token string, opts ...client.Option) *client.Client
	TokenRefreshEndpoint func(*auth.Config) string
}

type rootAuthConfig struct {
	DefaultConfigPath string
	DefaultTokenPath  string
	Version           string
	Deps              RootDeps
	Ref               *client.Ref
}

// DefaultRootDeps returns the production dependency set for root auth setup.
func DefaultRootDeps() RootDeps {
	return RootDeps{
		NewClient:            client.NewClient,
		TokenRefreshEndpoint: func(cfg *auth.Config) string { return cfg.OAuthTokenURL() },
	}
}

// NewRootCmd constructs the Cobra root command and owns auth lifecycle hooks.
func NewRootCmd(
	ref *client.Ref,
	defaultConfigPath string,
	defaultTokenPath string,
	version string,
	w io.Writer,
	deps RootDeps,
) *cobra.Command {
	deps = completeRootDeps(deps)
	if ref == nil {
		ref = &client.Ref{}
	}

	root := &cobra.Command{
		Use:                        "schwab-agent",
		Short:                      "CLI tool for AI agents to trade via Schwab APIs",
		Version:                    version,
		SilenceErrors:              true,
		SilenceUsage:               true,
		SuggestionsMinimumDistance: 2,
		PersistentPreRunE: rootAuthConfig{
			DefaultConfigPath: defaultConfigPath,
			DefaultTokenPath:  defaultTokenPath,
			Version:           version,
			Deps:              deps,
			Ref:               ref,
		}.preRun,
		PersistentPostRunE: func(_ *cobra.Command, _ []string) error {
			if ref.Client != nil {
				ref.Close()
			}
			return nil
		},
	}

	root.SetOut(w)
	root.SetErr(os.Stderr)
	root.SetFlagErrorFunc(apperr.NormalizeFlagError)
	root.PersistentFlags().StringP("account", "a", "", "Override default account by hash, account number, or nickname")
	root.PersistentFlags().BoolP("verbose", "v", false, "Enable debug logging to stderr")
	root.PersistentFlags().String("config", defaultConfigPath, "Path to config file")
	root.PersistentFlags().String("token", defaultTokenPath, "Path to token file")
	root.AddGroup(
		&cobra.Group{ID: "trading", Title: "Trading Commands"},
		&cobra.Group{ID: "market-data", Title: "Market Data Commands"},
		&cobra.Group{ID: "account-mgmt", Title: "Account Management Commands"},
		&cobra.Group{ID: "tools", Title: "Tool Commands"},
	)

	return root
}

func (cfg rootAuthConfig) preRun(cmd *cobra.Command, _ []string) error {
	if hasSkipAuthAnnotation(cmd) {
		return nil
	}

	if err := enableVerboseLogging(cmd); err != nil {
		return err
	}

	resolvedConfigPath, resolvedTokenPath, err := cfg.authPaths(cmd)
	if err != nil {
		return err
	}

	authConfig, token, err := loadAuthFiles(resolvedConfigPath, resolvedTokenPath)
	if err != nil {
		return err
	}

	if auth.IsRefreshTokenStale(token) {
		return apperr.NewAuthExpiredError(
			"refresh token expired",
			nil,
			apperr.WithDetails("Run `schwab-agent auth login` to re-authenticate"),
		)
	}

	if auth.IsAccessTokenExpired(token) {
		refreshed, err := auth.RefreshAccessToken(authConfig, token, cfg.Deps.TokenRefreshEndpoint(authConfig))
		if err != nil {
			return fmt.Errorf("refreshing access token: %w", err)
		}
		if err := auth.SaveToken(resolvedTokenPath, refreshed); err != nil {
			return fmt.Errorf("saving refreshed token to %s: %w", resolvedTokenPath, err)
		}
		token = refreshed
	}

	cfg.Ref.Client = cfg.Deps.NewClient(token.Token.AccessToken, rootClientOptions(authConfig, cfg.Version)...)
	return nil
}

func enableVerboseLogging(cmd *cobra.Command) error {
	verbose, err := cmd.Flags().GetBool("verbose")
	if err != nil {
		return err
	}
	if verbose {
		logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
		slog.SetDefault(logger)
	}

	return nil
}

func (cfg rootAuthConfig) authPaths(cmd *cobra.Command) (string, string, error) {
	configPath, err := cmd.Flags().GetString("config")
	if err != nil {
		return "", "", err
	}
	if configPath == "" {
		configPath = cfg.DefaultConfigPath
	}

	tokenPath, err := cmd.Flags().GetString("token")
	if err != nil {
		return "", "", err
	}
	if tokenPath == "" {
		tokenPath = cfg.DefaultTokenPath
	}

	return configPath, tokenPath, nil
}

func loadAuthFiles(configPath, tokenPath string) (*auth.Config, *auth.TokenFile, error) {
	cfg, err := auth.LoadConfig(configPath)
	if err != nil {
		// Preserve main.go's distinction between missing credentials and other config
		// validation failures so exit codes stay compatible.
		if !errors.Is(err, auth.ErrMissingCredentials) {
			return nil, nil, err
		}

		return nil, nil, apperr.NewAuthRequiredError(
			"Missing required credentials: set SCHWAB_CLIENT_ID and SCHWAB_CLIENT_SECRET env vars, or add client_id and client_secret to the config file",
			err,
			apperr.WithDetails("Run `schwab-agent auth login` to authenticate"),
		)
	}

	token, err := auth.LoadToken(tokenPath)
	if err != nil {
		return nil, nil, apperr.NewAuthRequiredError(
			"No authentication token found",
			err,
			apperr.WithDetails("Run `schwab-agent auth login` to authenticate"),
		)
	}

	return cfg, token, nil
}

func rootClientOptions(cfg *auth.Config, version string) []client.Option {
	return []client.Option{
		client.WithUserAgent("schwab-agent/" + version),
		client.WithBaseURL(cfg.APIBaseURL()),
		client.WithTLSConfig(cfg.TLSConfig()),
	}
}

// completeRootDeps fills omitted dependencies with production defaults.
func completeRootDeps(deps RootDeps) RootDeps {
	defaults := DefaultRootDeps()
	if deps.NewClient == nil {
		deps.NewClient = defaults.NewClient
	}
	if deps.TokenRefreshEndpoint == nil {
		deps.TokenRefreshEndpoint = defaults.TokenRefreshEndpoint
	}

	return deps
}

// hasSkipAuthAnnotation checks the current command and ancestors for auth bypass.
func hasSkipAuthAnnotation(cmd *cobra.Command) bool {
	for current := cmd; current != nil; current = current.Parent() {
		if current.Annotations["skipAuth"] == "true" {
			return true
		}
	}

	return false
}
