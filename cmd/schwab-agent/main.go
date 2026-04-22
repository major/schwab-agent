// schwab-agent is a CLI tool for AI agents to trade via Charles Schwab's APIs.
package main

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/urfave/cli/v3"

	"github.com/major/schwab-agent/internal/auth"
	"github.com/major/schwab-agent/internal/client"
	"github.com/major/schwab-agent/internal/commands"
	"github.com/major/schwab-agent/internal/errors"
	"github.com/major/schwab-agent/internal/output"
)

// version is set via ldflags at build time.
var version = "dev"

var (
	// loadConfigFunc wraps auth.LoadConfig for test overrides.
	loadConfigFunc = auth.LoadConfig
	// loadTokenFunc wraps auth.LoadToken for test overrides.
	loadTokenFunc = auth.LoadToken
	// saveTokenFunc wraps auth.SaveToken for test overrides.
	saveTokenFunc = auth.SaveToken
	// isRefreshTokenStaleFunc wraps auth.IsRefreshTokenStale for test overrides.
	isRefreshTokenStaleFunc = auth.IsRefreshTokenStale
	// isAccessTokenExpiredFunc wraps auth.IsAccessTokenExpired for test overrides.
	isAccessTokenExpiredFunc = auth.IsAccessTokenExpired
	// refreshAccessTokenFunc wraps auth.RefreshAccessToken for test overrides.
	refreshAccessTokenFunc = auth.RefreshAccessToken
	// newClientFunc wraps client.NewClient for test overrides.
	newClientFunc = client.NewClient
	// tokenRefreshEndpointFunc returns the OAuth token refresh endpoint.
	tokenRefreshEndpointFunc = func() string { return "https://api.schwabapi.com/v1/oauth/token" }
)

func main() {
	app := buildApp(os.Stdout)
	ctx := context.Background()

	if err := app.Run(ctx, os.Args); err != nil {
		_ = output.WriteError(os.Stdout, err)
		os.Exit(errors.ExitCodeFor(err))
	}
}

// buildApp constructs the CLI root command.
func buildApp(w io.Writer) *cli.Command {
	configPath := defaultConfigPath()
	tokenPath := defaultTokenPath()
	loadedConfig, err := loadConfigFunc(configPath)
	if err != nil {
		loadedConfig = &auth.Config{CallbackURL: "https://127.0.0.1:8182"}
	}

	// Pre-allocate the API client so command closures always share the live token.
	apiClient := &client.Client{}

	app := &cli.Command{
		Name:      "schwab-agent",
		Usage:     "CLI tool for AI agents to trade via Schwab APIs",
		Version:   version,
		Writer:    w,
		ErrWriter: os.Stderr,
		ExitErrHandler: func(_ context.Context, _ *cli.Command, _ error) {
			// Let main() render the JSON error envelope and set the exit code.
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

			cfg, err := loadConfigFunc(resolvedConfigPath)
			if err != nil {
			authErr := errors.NewAuthRequiredError("Missing required credentials: set SCHWAB_CLIENT_ID and SCHWAB_CLIENT_SECRET env vars, or add client_id and client_secret to the config file", err)
			authErr.Details = "Run `schwab-agent auth login` to authenticate"
				return ctx, authErr
			}

			token, err := loadTokenFunc(resolvedTokenPath)
			if err != nil {
				authErr := errors.NewAuthRequiredError("No authentication token found", err)
				authErr.Details = "Run `schwab-agent auth login` to authenticate"
				return ctx, authErr
			}

			if isRefreshTokenStaleFunc(token) {
				authErr := errors.NewAuthExpiredError("refresh token expired", nil)
				authErr.Details = "Run `schwab-agent auth login` to re-authenticate"
				return ctx, authErr
			}

			if isAccessTokenExpiredFunc(token) {
				refreshed, err := refreshAccessTokenFunc(cfg, token, tokenRefreshEndpointFunc())
				if err != nil {
					return ctx, err
				}
				if err := saveTokenFunc(resolvedTokenPath, refreshed); err != nil {
					return ctx, err
				}
				token = refreshed
			}

			*apiClient = *newClientFunc(token.Token.AccessToken)
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
