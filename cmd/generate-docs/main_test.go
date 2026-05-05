package main

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRemoveLLMsJSONSchemaReferenceDropsEmptyOptionalSection(t *testing.T) {
	input := []byte(strings.Join([]string{
		"# schwab-agent",
		"",
		"## Optional",
		"",
		"- [JSON Schema](#json-schema): Machine-readable schema available via `schwab-agent --jsonschema`",
		"",
	}, "\n"))

	content := string(removeLLMsJSONSchemaReference(input))

	require.NotContains(t, content, "## Optional")
	require.NotContains(t, content, "--jsonschema")
}

func TestRemoveLLMsJSONSchemaReferencePreservesOtherOptionalItems(t *testing.T) {
	input := []byte(strings.Join([]string{
		"# schwab-agent",
		"",
		"## Optional",
		"",
		"- [JSON Schema](#json-schema): Machine-readable schema available via `schwab-agent --jsonschema`",
		"- [MCP Server](#mcp): Available as MCP server via `schwab-agent --mcp`",
		"",
		"## Next",
		"content",
	}, "\n"))

	content := string(removeLLMsJSONSchemaReference(input))

	require.Contains(t, content, "## Optional")
	require.Contains(t, content, "--mcp")
	require.Contains(t, content, "## Next")
	require.NotContains(t, content, "--jsonschema")
}
