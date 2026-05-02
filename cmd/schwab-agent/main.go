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
	"github.com/leodido/structcli/debug"
	structclimcp "github.com/leodido/structcli/mcp"
	"github.com/spf13/cobra"

	"github.com/major/schwab-agent/internal/apperr"
	"github.com/major/schwab-agent/internal/client"
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
	root := buildCommandTree(w, deps)

	if err := structcli.Setup(root,
		structcli.WithJSONSchema(),
		structcli.WithHelpTopics(),
		structcli.WithFlagErrors(),
		structcli.WithAppName("schwab-agent"),
		structcli.WithDebug(debug.Options{Exit: true}),
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
				return buildCommandTree(stdout, deps), nil
			},
		}),
	); err != nil {
		panic(err)
	}

	return root
}

// buildCommandTree constructs the full command tree without calling structcli.Setup.
// Used by buildAppWithDeps for the main CLI and by the MCP CommandFactory for
// per-tool-call fresh command trees (each with its own client.Ref and auth lifecycle).
func buildCommandTree(w io.Writer, deps commands.RootDeps) *cobra.Command {
	configPath := defaultConfigPath()
	tokenPath := defaultTokenPath()

	// Pre-allocate the client ref so command closures always share the live
	// client. The root command's PersistentPreRunE populates ref.Client after authentication.
	ref := &client.Ref{}

	root := commands.NewRootCmd(ref, configPath, tokenPath, version, w, deps)
	root.AddCommand(commands.NewSymbolCmd(w))
	root.AddCommand(commands.NewQuoteCmd(ref, w))
	root.AddCommand(commands.NewHistoryCmd(ref, w))
	root.AddCommand(commands.NewInstrumentCmd(ref, w))
	root.AddCommand(commands.NewChainCmd(ref, w))
	root.AddCommand(commands.NewMarketCmd(ref, w))
	root.AddCommand(commands.NewTACmd(ref, w))
	root.AddCommand(commands.NewAccountCmd(ref, configPath, w))
	root.AddCommand(commands.NewPositionCmd(ref, configPath, w))
	root.AddCommand(commands.NewAuthCmd(configPath, tokenPath, w, commands.DefaultAuthDeps()))
	root.AddCommand(commands.NewOrderCmd(ref, configPath, w))
	root.AddCommand(commands.NewCompletionCmd(w))

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
