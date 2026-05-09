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
	"runtime/debug"

	"github.com/spf13/cobra"

	"github.com/major/schwab-agent/internal/commands"
	"github.com/major/schwab-agent/internal/output"
)

// version is set via ldflags at build time.
var version = "dev"

func main() {
	root := buildApp(os.Stdout)

	cmd, err := root.ExecuteC()
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

	root := commands.BuildCommandTree(w, configPath, tokenPath, displayVersion(), deps, authDeps)

	// Register --instruction/--order-type flag aliases on qualifying order
	// commands after the command tree is fully built.
	commands.RegisterOrderFlagAliasesOnTree(root)

	return root
}

// displayVersion returns release versions unchanged, but annotates local dev
// builds with the Git revision embedded by the Go toolchain. That keeps release
// ldflags authoritative while making ad-hoc binaries traceable.
func displayVersion() string {
	if version != "dev" {
		return version
	}

	if revision := buildRevision(); revision != "" {
		return "dev-" + shortRevision(revision)
	}
	return version
}

func buildRevision() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	for _, setting := range info.Settings {
		if setting.Key == "vcs.revision" {
			return setting.Value
		}
	}
	return ""
}

func shortRevision(revision string) string {
	const shortSHAChars = 7
	if len(revision) <= shortSHAChars {
		return revision
	}
	return revision[:shortSHAChars]
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
