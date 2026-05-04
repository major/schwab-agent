package commands

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/leodido/structcli"
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
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			// In MCP server mode the root command runs just to start the
			// JSON-RPC loop. Auth is handled per-tool-call via the
			// CommandFactory fresh trees, so skip it here.
			if isMCPMode(cmd) {
				return nil
			}

			if hasSkipAuthAnnotation(cmd) {
				return nil
			}

			// Debug mode prints flag source attribution and exits without
			// executing the command. Skip auth since introspection doesn't
			// need API access.
			if structcli.IsDebugActive(cmd) {
				structcli.UseDebug(cmd, cmd.OutOrStdout())
				return nil
			}

			verbose, err := cmd.Flags().GetBool("verbose")
			if err != nil {
				return err
			}
			if verbose {
				logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
				slog.SetDefault(logger)
			}

			resolvedConfigPath, err := cmd.Flags().GetString("config")
			if err != nil {
				return err
			}
			if resolvedConfigPath == "" {
				resolvedConfigPath = defaultConfigPath
			}

			resolvedTokenPath, err := cmd.Flags().GetString("token")
			if err != nil {
				return err
			}
			if resolvedTokenPath == "" {
				resolvedTokenPath = defaultTokenPath
			}

			cfg, err := auth.LoadConfig(resolvedConfigPath)
			if err != nil {
				// Preserve main.go's distinction between missing credentials and
				// other config validation failures so exit codes stay compatible.
				if !errors.Is(err, auth.ErrMissingCredentials) {
					return err
				}

				return apperr.NewAuthRequiredError(
					"Missing required credentials: set SCHWAB_CLIENT_ID and SCHWAB_CLIENT_SECRET env vars, or add client_id and client_secret to the config file",
					err,
					apperr.WithDetails("Run `schwab-agent auth login` to authenticate"),
				)
			}

			token, err := auth.LoadToken(resolvedTokenPath)
			if err != nil {
				return apperr.NewAuthRequiredError(
					"No authentication token found",
					err,
					apperr.WithDetails("Run `schwab-agent auth login` to authenticate"),
				)
			}

			if auth.IsRefreshTokenStale(token) {
				return apperr.NewAuthExpiredError(
					"refresh token expired",
					nil,
					apperr.WithDetails("Run `schwab-agent auth login` to re-authenticate"),
				)
			}

			if auth.IsAccessTokenExpired(token) {
				refreshed, err := auth.RefreshAccessToken(cfg, token, deps.TokenRefreshEndpoint(cfg))
				if err != nil {
					return fmt.Errorf("refreshing access token: %w", err)
				}
				if err := auth.SaveToken(resolvedTokenPath, refreshed); err != nil {
					return fmt.Errorf("saving refreshed token to %s: %w", resolvedTokenPath, err)
				}
				token = refreshed
			}

			clientOptions := []client.Option{
				client.WithUserAgent("schwab-agent/" + version),
				client.WithBaseURL(cfg.APIBaseURL()),
				client.WithTLSConfig(cfg.TLSConfig()),
			}

			ref.Client = deps.NewClient(token.Token.AccessToken, clientOptions...)
			return nil
		},
		PersistentPostRunE: func(_ *cobra.Command, _ []string) error {
			if ref.Client != nil {
				ref.Close()
			}
			return nil
		},
	}

	root.SetOut(w)
	root.SetErr(os.Stderr)
	root.SetFlagErrorFunc(suggestSubcommands)
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

// isMCPMode returns true when the root --mcp flag is set, meaning the CLI is
// running as an MCP JSON-RPC server rather than executing a single command.
func isMCPMode(cmd *cobra.Command) bool {
	if f := cmd.Root().PersistentFlags().Lookup("mcp"); f != nil {
		return f.Changed
	}

	return false
}

// hasSkipAuthAnnotation checks the current command and ancestors for auth bypass.
func hasSkipAuthAnnotation(cmd *cobra.Command) bool {
	for current := cmd; current != nil; current = current.Parent() {
		if current.Annotations["skipAuth"] == "true" {
			return true
		}
	}

	// structcli help topic commands (env-vars, config-keys) don't require auth
	if structcli.IsHelpTopicCommand(cmd) {
		return true
	}

	return false
}
