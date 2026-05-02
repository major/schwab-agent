// schwab-agent is a CLI tool for AI agents to trade via Charles Schwab's APIs.
//
// This project is not affiliated with, endorsed by, or sponsored by
// Charles Schwab & Co., Inc. or any of its subsidiaries.
// "Schwab" and "thinkorswim" are trademarks of Charles Schwab & Co., Inc.
package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/leodido/structcli"
	structclimcp "github.com/leodido/structcli/mcp"
	"github.com/spf13/cobra"

	"github.com/major/schwab-agent/internal/apperr"
	"github.com/major/schwab-agent/internal/commands"
	"github.com/major/schwab-agent/internal/output"
)

// version is set via ldflags at build time.
var version = "dev"

func main() {
	root := buildApp(os.Stdout)

	if err := root.Execute(); err != nil {
		// Cobra returns plain fmt.Errorf errors for unknown commands. Wrap them
		// in ValidationError so the JSON output uses VALIDATION_ERROR instead of
		// UNKNOWN, which is more useful for agents parsing the output.
		if strings.HasPrefix(err.Error(), "unknown command") {
			err = apperr.NewValidationError(err.Error(), err)
		}
		_ = output.WriteError(os.Stdout, err)
		os.Exit(apperr.ExitCodeFor(err))
	}
}

// buildApp constructs the CLI root command with production defaults.
func buildApp(w io.Writer) *cobra.Command {
	return buildAppWithDeps(w, commands.DefaultRootDeps())
}

// buildAppWithDeps constructs the CLI root command with the given dependencies.
func buildAppWithDeps(w io.Writer, deps commands.RootDeps) *cobra.Command {
	configPath := defaultConfigPath()
	tokenPath := defaultTokenPath()
	authDeps := commands.DefaultAuthDeps()

	root := commands.BuildCommandTree(w, configPath, tokenPath, version, deps, authDeps)

	if err := structcli.Setup(root,
		structcli.WithJSONSchema(),
		structcli.WithHelpTopics(),
		structcli.WithFlagErrors(),
		structcli.WithAppName("schwab-agent"),
		structcli.WithMCP(structclimcp.Options{
			Name:    "schwab-agent",
			Version: version,
			Exclude: []string{
				// Interactive browser-based OAuth flow, not usable from MCP clients.
				"auth-login",
				// Shell completion generators are irrelevant in MCP mode.
				"completion-bash",
				"completion-zsh",
				"completion-fish",
				"completion-powershell",
			},
			// CommandFactory builds a fresh command tree per MCP tool call so that
			// PersistentPreRunE (auth lifecycle) runs for each invocation. Without
			// this, MCP mode skips PersistentPreRunE entirely and commands that
			// need an authenticated API client would fail.
			CommandFactory: func(_ []string, stdout io.Writer, _ io.Writer) (*cobra.Command, error) {
				return commands.BuildCommandTree(stdout, configPath, tokenPath, version, deps, authDeps), nil
			},
		}),
	); err != nil {
		panic(err)
	}

	return root
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
