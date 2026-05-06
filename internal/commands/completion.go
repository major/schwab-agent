package commands

import (
	"io"

	"github.com/spf13/cobra"
)

// NewCompletionCmd returns the shell completion command.
// It generates shell completion scripts for bash, zsh, fish, and powershell.
func NewCompletionCmd(w io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "completion",
		Short:       "Generate shell completion scripts",
		Long:        "Generate shell completion scripts for bash, zsh, fish, or powershell.",
		Annotations: map[string]string{annotationSkipAuth: annotationValueTrue},
		GroupID:     groupIDTools,
		RunE:        requireSubcommand,
	}

	cmd.SetFlagErrorFunc(suggestSubcommands)

	cmd.AddCommand(newBashCompletionCmd(w))
	cmd.AddCommand(newZshCompletionCmd(w))
	cmd.AddCommand(newFishCompletionCmd(w))
	cmd.AddCommand(newPowershellCompletionCmd(w))

	return cmd
}

// newBashCompletionCmd returns the bash completion subcommand.
func newBashCompletionCmd(w io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "bash",
		Short: "Generate bash completion script",
		Long: `Generate bash completion script.

To load completions in your current shell session:

	source <(schwab-agent completion bash)

To load completions for every new session, execute once:

	schwab-agent completion bash | sudo tee /etc/bash_completion.d/schwab-agent

or

	schwab-agent completion bash >> ~/.bashrc
`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Root().GenBashCompletionV2(w, true)
		},
	}
}

// newZshCompletionCmd returns the zsh completion subcommand.
func newZshCompletionCmd(w io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "zsh",
		Short: "Generate zsh completion script",
		Long: `Generate zsh completion script.

To load completions in your current shell session:

	source <(schwab-agent completion zsh)

To load completions for every new session, execute once:

	schwab-agent completion zsh > "${fpath[1]}/_schwab-agent"
`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Root().GenZshCompletion(w)
		},
	}
}

// newFishCompletionCmd returns the fish completion subcommand.
func newFishCompletionCmd(w io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "fish",
		Short: "Generate fish completion script",
		Long: `Generate fish completion script.

To load completions in your current shell session:

	schwab-agent completion fish | source

To load completions for every new session, execute once:

	schwab-agent completion fish > ~/.config/fish/completions/schwab-agent.fish
`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Root().GenFishCompletion(w, true)
		},
	}
}

// newPowershellCompletionCmd returns the powershell completion subcommand.
func newPowershellCompletionCmd(w io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "powershell",
		Short: "Generate powershell completion script",
		Long: `Generate powershell completion script.

To load completions in your current shell session:

	schwab-agent completion powershell | Out-String | Invoke-Expression

To load completions for every new session, add the output of the above command
to your powershell profile.
`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Root().GenPowerShellCompletionWithDesc(w)
		},
	}
}
