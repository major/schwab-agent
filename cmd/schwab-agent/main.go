// schwab-agent is a CLI tool for AI agents to trade via Charles Schwab's APIs.
//
// This project is not affiliated with, endorsed by, or sponsored by
// Charles Schwab & Co., Inc. or any of its subsidiaries.
// "Schwab" and "thinkorswim" are trademarks of Charles Schwab & Co., Inc.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/urfave/cli/v3"

	"github.com/major/schwab-agent/internal/apperr"
	"github.com/major/schwab-agent/internal/auth"
	"github.com/major/schwab-agent/internal/client"
	"github.com/major/schwab-agent/internal/commands"
	"github.com/major/schwab-agent/internal/output"
)

// version is set via ldflags at build time.
var version = "dev"

// appDeps holds injectable dependencies for the Before hook.
// Tests override individual fields; production uses defaultAppDeps().
type appDeps struct {
	newClient            func(token string, opts ...client.Option) *client.Client
	tokenRefreshEndpoint func(*auth.Config) string
}

func defaultAppDeps() appDeps {
	return appDeps{
		newClient:            client.NewClient,
		tokenRefreshEndpoint: func(cfg *auth.Config) string { return cfg.OAuthTokenURL() },
	}
}

func main() {
	app := buildApp(os.Stdout)
	ctx := context.Background()

	if err := app.Run(ctx, os.Args); err != nil {
		_ = output.WriteError(os.Stdout, err)
		os.Exit(apperr.ExitCodeFor(err))
	}
}

// buildApp constructs the CLI root command with production defaults.
func buildApp(w io.Writer) *cli.Command {
	return buildAppWithDeps(w, defaultAppDeps())
}

// buildAppWithDeps constructs the CLI root command with the given dependencies.
func buildAppWithDeps(w io.Writer, deps appDeps) *cli.Command {
	configPath := defaultConfigPath()
	tokenPath := defaultTokenPath()
	loadedConfig, err := auth.LoadConfig(configPath)
	if err != nil {
		loadedConfig = &auth.Config{CallbackURL: "https://127.0.0.1:8182"}
	}

	// Pre-allocate the client ref so command closures always share the live
	// client. The Before hook populates ref.Client after authentication.
	apiClient := &client.Ref{}

	app := &cli.Command{
		Name:      "schwab-agent",
		Usage:     "CLI tool for AI agents to trade via Schwab APIs",
		Version:   version,
		Suggest:   true,
		Writer:    w,
		ErrWriter: os.Stderr,
		ExitErrHandler: func(_ context.Context, _ *cli.Command, _ error) {
			// Let main() render the JSON error envelope and set the exit code.
		},
		// OnUsageError intercepts flag-parsing errors. When an unknown command
		// is used (e.g. "price-history" instead of "history"), urfave/cli treats
		// the unknown name as arguments and then chokes on subcommand flags
		// like --period-type that aren't defined on the root command. This
		// detects that case and returns a clear "unknown command" error with a
		// fuzzy-match suggestion instead of the misleading flag error.
		OnUsageError: func(_ context.Context, cmd *cli.Command, err error, _ bool) error {
			if name := cmd.Args().First(); name != "" && !strings.HasPrefix(name, "-") {
				if cmd.Command(name) == nil {
					return unknownCommandError(cmd, name)
				}
			}

			return err
		},
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "account",
				Usage: "Override default account hash",
			},
			&cli.BoolFlag{
				Name:  "verbose",
				Usage: "Enable debug logging to stderr",
			},
			&cli.StringFlag{
				Name:  "config",
				Usage: "Path to config file",
				Value: configPath,
			},
			&cli.StringFlag{
				Name:  "token",
				Usage: "Path to token file",
				Value: tokenPath,
			},
		},
		Before: func(ctx context.Context, cmd *cli.Command) (context.Context, error) {
			subcommand := cmd.Args().First()

			// Catch unknown commands before attempting auth. Without this,
			// "schwab-agent price-history" (no conflicting flags) would fall
			// through to auth loading and produce a misleading auth error
			// instead of telling the user the command doesn't exist.
			if subcommand != "" && cmd.Command(subcommand) == nil {
				return ctx, unknownCommandError(cmd, subcommand)
			}

			if subcommand == "auth" || subcommand == "schema" || subcommand == "symbol" {
				return ctx, nil
			}

			if cmd.Bool("verbose") {
				logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
				slog.SetDefault(logger)
			}

			resolvedConfigPath := cmd.String("config")
			if resolvedConfigPath == "" {
				resolvedConfigPath = configPath
			}

			resolvedTokenPath := cmd.String("token")
			if resolvedTokenPath == "" {
				resolvedTokenPath = tokenPath
			}

			cfg, err := auth.LoadConfig(resolvedConfigPath)
			if err != nil {
				// Non-credential validation errors (e.g. invalid base_url) are
				// returned directly. Missing credentials get wrapped as
				// AuthRequiredError so the CLI reports exit code 3.
				if !errors.Is(err, auth.ErrMissingCredentials) {
					return ctx, err
				}

				return ctx, apperr.NewAuthRequiredError(
					"Missing required credentials: set SCHWAB_CLIENT_ID and SCHWAB_CLIENT_SECRET env vars, or add client_id and client_secret to the config file",
					err,
					apperr.WithDetails("Run `schwab-agent auth login` to authenticate"),
				)
			}

			token, err := auth.LoadToken(resolvedTokenPath)
			if err != nil {
				return ctx, apperr.NewAuthRequiredError(
					"No authentication token found",
					err,
					apperr.WithDetails("Run `schwab-agent auth login` to authenticate"),
				)
			}

			if auth.IsRefreshTokenStale(token) {
				return ctx, apperr.NewAuthExpiredError(
					"refresh token expired",
					nil,
					apperr.WithDetails("Run `schwab-agent auth login` to re-authenticate"),
				)
			}

			if auth.IsAccessTokenExpired(token) {
				refreshed, err := auth.RefreshAccessToken(cfg, token, deps.tokenRefreshEndpoint(cfg))
				if err != nil {
					return ctx, err
				}
				if err := auth.SaveToken(resolvedTokenPath, refreshed); err != nil {
					return ctx, err
				}
				token = refreshed
			}

			clientOptions := []client.Option{
				client.WithUserAgent("schwab-agent/" + version),
				client.WithBaseURL(cfg.APIBaseURL()),
				client.WithHTTPClient(cfg.HTTPClient(30 * time.Second)),
			}

			apiClient.Client = deps.newClient(token.Token.AccessToken, clientOptions...)
			return ctx, nil
		},
		Commands: []*cli.Command{
			commands.AuthCommand(loadedConfig, tokenPath, w),
			commands.AccountCommand(apiClient, configPath, w),
			commands.OrderCommand(apiClient, configPath, w),
			commands.QuoteCommand(apiClient, w),
			commands.ChainCommand(apiClient, w),
			commands.HistoryCommand(apiClient, w),
			commands.MarketCommand(apiClient, w),
			commands.InstrumentCommand(apiClient, w),
			commands.TACommand(apiClient, w),
			commands.SymbolCommand(w),
		},
	}

	app.Commands = append(app.Commands, commands.SchemaCommand(app, w))

	return app
}

// unknownCommandError builds a ValidationError for an unrecognized command,
// including a fuzzy-match suggestion when a close match exists.
func unknownCommandError(cmd *cli.Command, name string) error {
	msg := fmt.Sprintf("unknown command %q for %q", name, cmd.Name)

	if suggestion := suggestClosestCommand(cmd.Commands, name); suggestion != "" {
		msg += fmt.Sprintf(". Did you mean %q?", suggestion)
	}

	return apperr.NewValidationError(msg, nil)
}

// suggestClosestCommand finds the best matching command name for a given input.
// It first checks for substring containment (handles "price-history" -> "history"),
// then falls back to Jaro-Winkler fuzzy matching for typos (handles "ordr" -> "order").
func suggestClosestCommand(cmds []*cli.Command, provided string) string {
	lower := strings.ToLower(provided)

	// Prefer the longest substring match so "price-history" picks "history"
	// over shorter accidental matches.
	best := ""
	for _, cmd := range cmds {
		for _, name := range cmd.Names() {
			nameLower := strings.ToLower(name)
			if strings.Contains(lower, nameLower) || strings.Contains(nameLower, lower) {
				if len(name) > len(best) {
					best = name
				}
			}
		}
	}

	if best != "" {
		return best
	}

	// Fall back to the built-in Jaro-Winkler fuzzy matcher for genuine typos.
	return cli.SuggestCommand(cmds, provided)
}

// defaultConfigPath returns the default config file location.
func defaultConfigPath() string {
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

// defaultTokenPath returns the default token file location.
func defaultTokenPath() string {
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return filepath.Join(".config", "schwab-agent", "token.json")
		}
		configHome = filepath.Join(homeDir, ".config")
	}

	return filepath.Join(configHome, "schwab-agent", "token.json")
}
