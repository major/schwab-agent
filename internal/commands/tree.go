package commands

import (
	"io"

	"github.com/spf13/cobra"

	"github.com/major/schwab-agent/internal/client"
)

// BuildCommandTree constructs the full command tree without calling
// structcli.Setup. It is shared by the main CLI, the MCP CommandFactory
// (which needs a fresh tree per tool call), and the doc generator.
func BuildCommandTree(
	w io.Writer,
	configPath, tokenPath, version string,
	deps RootDeps,
	authDeps AuthDeps,
) *cobra.Command {
	// Pre-allocate the client ref so command closures always share the live
	// client. The root command's PersistentPreRunE populates ref.Client
	// after authentication.
	ref := &client.Ref{}

	root := NewRootCmd(ref, configPath, tokenPath, version, w, deps)
	root.AddCommand(NewSymbolCmd(w))
	root.AddCommand(NewQuoteCmd(ref, w))
	root.AddCommand(NewHistoryCmd(ref, w))
	root.AddCommand(NewInstrumentCmd(ref, w))
	root.AddCommand(NewChainCmd(ref, w))
	root.AddCommand(NewOptionCmd(ref, w))
	root.AddCommand(NewMarketCmd(ref, w))
	root.AddCommand(NewTACmd(ref, w))
	root.AddCommand(NewAccountCmd(ref, configPath, w))
	root.AddCommand(NewPositionCmd(ref, configPath, w))
	root.AddCommand(NewAuthCmd(configPath, tokenPath, w, authDeps))
	root.AddCommand(NewOrderCmd(ref, configPath, w))
	root.AddCommand(NewCompletionCmd(w))

	return root
}
