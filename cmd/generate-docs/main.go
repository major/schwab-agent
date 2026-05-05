// generate-docs produces LLM-readable documentation from the CLI's command
// tree. It outputs SKILL.md (Anthropic skill format) and llms.txt (web
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
	"strings"

	"github.com/leodido/structcli"
	"github.com/leodido/structcli/generate"

	"github.com/major/schwab-agent/internal/commands"
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

	// Setup is required so JSONSchema (used internally by the generators)
	// can read flag metadata, enum values, and help topics.
	if err := structcli.Setup(root,
		structcli.WithJSONSchema(),
		structcli.WithHelpTopics(),
		structcli.WithFlagErrors(),
	); err != nil {
		return fmt.Errorf("structcli setup: %w", err)
	}

	// Generate SKILL.md (Anthropic skill file format with YAML frontmatter,
	// per-command flag tables, and trigger phrases).
	skill, err := generate.Skill(root, generate.SkillOptions{
		Name:    "schwab-agent",
		Author:  "major",
		Version: version,
	})
	if err != nil {
		return fmt.Errorf("generating SKILL.md: %w", err)
	}

	skill = labelSkillCodeFences(skill)

	//nolint:gosec // G306: public documentation files should be world-readable
	if err := os.WriteFile(filepath.Join(outDir, "SKILL.md"), skill, 0o644); err != nil {
		return fmt.Errorf("writing SKILL.md: %w", err)
	}

	fmt.Fprintf(os.Stderr, "wrote %s\n", filepath.Join(outDir, "SKILL.md"))

	// Generate llms.txt (llmstxt.org standard with commands index and flag
	// definitions).
	llmstxt, err := generate.LLMsTxt(root, generate.LLMsTxtOptions{
		ModulePath: "github.com/major/schwab-agent",
	})
	if err != nil {
		return fmt.Errorf("generating llms.txt: %w", err)
	}

	//nolint:gosec // G306: public documentation files should be world-readable
	if err := os.WriteFile(filepath.Join(outDir, "llms.txt"), llmstxt, 0o644); err != nil {
		return fmt.Errorf("writing llms.txt: %w", err)
	}

	fmt.Fprintf(os.Stderr, "wrote %s\n", filepath.Join(outDir, "llms.txt"))

	return nil
}

func labelSkillCodeFences(content []byte) []byte {
	lines := strings.Split(string(content), "\n")
	inFence := false
	for i, line := range lines {
		if line != "```" {
			continue
		}

		if inFence {
			inFence = false
			continue
		}

		// structcli generates shell examples in SKILL.md with bare opening
		// fences. Add a language tag here so generated docs satisfy markdownlint
		// without forking or post-editing the generated file by hand.
		lines[i] = "```bash"
		inFence = true
	}

	return []byte(strings.Join(lines, "\n"))
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
