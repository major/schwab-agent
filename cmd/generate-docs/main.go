// generate-docs produces LLM-readable documentation from the CLI's Cobra
// command tree. It outputs SKILL.md (Anthropic skill format) and llms.txt (web
// standard) to the project root. The existing hand-crafted AGENTS.md is
// intentionally preserved since it contains architecture context that the
// generator cannot produce.
//
// Usage:
//
//	go run ./cmd/generate-docs/
//	make docs
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/major/schwab-agent/internal/commands"
	"github.com/spf13/cobra"
)

// version is set via ldflags at build time (same as the main binary).
var version = "dev"

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "generate-docs: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	outDir, err := projectRoot()
	if err != nil {
		return fmt.Errorf("finding project root: %w", err)
	}

	// Build the same command tree the production CLI uses. Empty config/token
	// paths are fine since the generator never runs any commands.
	root := commands.BuildCommandTree(
		os.Stdout, "", "", version,
		commands.DefaultRootDeps(),
		commands.DefaultAuthDeps(),
	)
	commands.InstallDebugOptions(root)
	commands.RegisterOrderFlagAliasesOnTree(root)

	commandList := visibleCommands(root)

	skill := generateSkill(root, commandList)
	//nolint:gosec // G306: public documentation files should be world-readable
	if err := os.WriteFile(filepath.Join(outDir, "SKILL.md"), []byte(skill), 0o644); err != nil {
		return fmt.Errorf("writing SKILL.md: %w", err)
	}
	fmt.Fprintf(os.Stderr, "wrote %s\n", filepath.Join(outDir, "SKILL.md"))

	llmstxt := generateLLMsTxt(root, commandList)
	//nolint:gosec // G306: public documentation files should be world-readable
	if err := os.WriteFile(filepath.Join(outDir, "llms.txt"), []byte(llmstxt), 0o644); err != nil {
		return fmt.Errorf("writing llms.txt: %w", err)
	}
	fmt.Fprintf(os.Stderr, "wrote %s\n", filepath.Join(outDir, "llms.txt"))

	return nil
}

func generateSkill(root *cobra.Command, commandList []*cobra.Command) string {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("name: schwab-agent\n")
	b.WriteString("description: Trade and inspect Charles Schwab accounts through the schwab-agent CLI. Use when an agent needs account lookup, market data, technical indicators, option chains, or order preview/build/place/cancel/replace workflows.\n")
	b.WriteString("---\n\n")
	b.WriteString("# schwab-agent\n\n")
	b.WriteString(root.Short + "\n\n")
	b.WriteString("## Usage\n\n")
	b.WriteString("```bash\n")
	b.WriteString(strings.TrimSpace(root.UsageString()) + "\n")
	b.WriteString("```\n\n")
	b.WriteString("## Commands\n\n")
	for _, cmd := range commandList {
		writeCommandMarkdown(&b, cmd)
	}

	return b.String()
}

func generateLLMsTxt(root *cobra.Command, commandList []*cobra.Command) string {
	var b strings.Builder
	b.WriteString("# schwab-agent\n\n")
	b.WriteString("> " + root.Short + "\n\n")
	b.WriteString("Module: github.com/major/schwab-agent\n\n")
	b.WriteString("## Commands\n\n")
	for _, cmd := range commandList {
		b.WriteString("- `" + cmd.CommandPath() + "`: " + oneLine(cmd.Short) + "\n")
	}
	b.WriteString("\n## Command Reference\n\n")
	for _, cmd := range commandList {
		writeCommandMarkdown(&b, cmd)
	}

	return b.String()
}

func writeCommandMarkdown(b *strings.Builder, cmd *cobra.Command) {
	b.WriteString("### `" + cmd.CommandPath() + "`\n\n")
	// Cobra's rendered usage is the canonical runtime contract. Reusing it here
	// keeps generated agent docs aligned with the live binary's help output,
	// including positional arguments, aliases, examples, and flag groups.
	b.WriteString("```bash\n")
	b.WriteString(strings.TrimSpace(cmd.UsageString()) + "\n")
	b.WriteString("```\n\n")
}

func visibleCommands(root *cobra.Command) []*cobra.Command {
	oldCommandSorting := cobra.EnableCommandSorting
	cobra.EnableCommandSorting = false
	defer func() { cobra.EnableCommandSorting = oldCommandSorting }()

	commandList := make([]*cobra.Command, 0)
	var walk func(*cobra.Command)
	walk = func(cmd *cobra.Command) {
		if cmd.Hidden {
			return
		}
		commandList = append(commandList, cmd)
		children := append([]*cobra.Command(nil), cmd.Commands()...)
		sort.Slice(children, func(i, j int) bool { return children[i].Name() < children[j].Name() })
		for _, child := range children {
			walk(child)
		}
	}
	walk(root)
	return commandList
}

func oneLine(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

// projectRoot returns the repository root by walking up from this source
// file's location. This works regardless of the working directory.
func projectRoot() (string, error) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("could not determine source file path")
	}

	// thisFile is cmd/generate-docs/main.go, so root is two levels up.
	return filepath.Abs(filepath.Join(filepath.Dir(thisFile), "..", ".."))
}
