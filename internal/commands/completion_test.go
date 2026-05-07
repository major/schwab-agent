package commands

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/major/schwab-agent/internal/apperr"
)

func TestCompletionParentRequiresSubcommand(t *testing.T) {
	var out bytes.Buffer
	cmd := NewCompletionCmd(&out)

	_, err := runTestCommand(t, cmd)

	require.Error(t, err)
	var validationErr *apperr.ValidationError
	require.ErrorAs(t, err, &validationErr)
	assert.Contains(t, err.Error(), "requires a subcommand")
}

func TestCompletionShellOutputs(t *testing.T) {
	tests := []struct {
		name     string
		shell    string
		contains string
	}{
		{name: "bash", shell: "bash", contains: "bash completion V2 for schwab-agent"},
		{name: "zsh", shell: "zsh", contains: "#compdef schwab-agent"},
		{name: "fish", shell: "fish", contains: "complete -c schwab-agent"},
		{name: "powershell", shell: "powershell", contains: "Register-ArgumentCompleter"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			root := &cobra.Command{Use: "schwab-agent"}
			root.AddGroup(&cobra.Group{ID: groupIDTools, Title: "Tool Commands"})
			root.AddCommand(NewCompletionCmd(&out))

			_, err := runTestCommand(t, root, "completion", tt.shell)

			require.NoError(t, err)
			assert.Contains(t, out.String(), tt.contains)
		})
	}
}
