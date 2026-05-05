package main

import (
	"strings"
	"testing"

	"github.com/major/schwab-agent/internal/commands"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func buildTestCommandTree(t *testing.T) *cobra.Command {
	t.Helper()

	root := commands.BuildCommandTree(
		&strings.Builder{}, "", "", version,
		commands.DefaultRootDeps(),
		commands.DefaultAuthDeps(),
	)
	commands.InstallDebugOptions(root)
	commands.RegisterOrderFlagAliasesOnTree(root)
	root.AddCommand(&cobra.Command{Use: "__hidden", Hidden: true})

	return root
}

func TestGenerateDocsExcludeRuntimeOnlyJSONSchemaReferences(t *testing.T) {
	root := buildTestCommandTree(t)
	commandList := visibleCommands(root)

	content := generateLLMsTxt(root, commandList)

	require.NotContains(t, content, "--jsonschema")
	require.NotContains(t, content, "JSON Schema")
}

func TestGenerateSkillUsesTaggedCodeFences(t *testing.T) {
	root := buildTestCommandTree(t)
	commandList := visibleCommands(root)

	content := generateSkill(root, commandList)

	require.Contains(t, content, "```bash\n")
	require.NotContains(t, content, "Examples:\n\n```\n")
}

func TestVisibleCommandsSkipsHiddenCommands(t *testing.T) {
	root := buildTestCommandTree(t)
	commandList := visibleCommands(root)

	paths := make([]string, 0, len(commandList))
	for _, cmd := range commandList {
		paths = append(paths, cmd.CommandPath())
	}

	require.Contains(t, strings.Join(paths, "\n"), "schwab-agent quote")
	require.NotContains(t, strings.Join(paths, "\n"), "__hidden")
}

func TestVisibleCommandsDoesNotMutateCommandTreeOrder(t *testing.T) {
	oldCommandSorting := cobra.EnableCommandSorting
	cobra.EnableCommandSorting = false
	t.Cleanup(func() { cobra.EnableCommandSorting = oldCommandSorting })

	root := &cobra.Command{Use: "root"}
	beta := &cobra.Command{Use: "beta"}
	alpha := &cobra.Command{Use: "alpha"}
	root.AddCommand(beta, alpha)

	_ = visibleCommands(root)

	children := root.Commands()
	require.Equal(t, []*cobra.Command{beta, alpha}, children)
}
