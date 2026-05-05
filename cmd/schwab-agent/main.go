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

	"github.com/leodido/structcli"
	"github.com/leodido/structcli/debug"
	"github.com/spf13/cobra"

	"github.com/major/schwab-agent/internal/commands"
	"github.com/major/schwab-agent/internal/output"
)

// version is set via ldflags at build time.
var version = "dev"

func main() {
	root := buildApp(os.Stdout)

	cmd, err := structcli.ExecuteC(root)
	if err != nil {
		exitCode, writeErr := output.WriteCommandError(os.Stdout, cmd, err)
		if writeErr != nil {
			os.Exit(1)
		}
		os.Exit(exitCode)
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
		structcli.WithDebug(debug.Options{Exit: true}),
	); err != nil {
		panic(err)
	}

	// Register --instruction/--order-type flag aliases on qualifying commands.
	// This must happen AFTER structcli.Setup() because adding non-structcli
	// flags before Setup interferes with structcli's JSON Schema generation.
	commands.RegisterOrderFlagAliasesOnTree(root)

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
