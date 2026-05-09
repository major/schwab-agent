package commands

import (
	"io"

	"github.com/spf13/cobra"

	"github.com/major/schwab-agent/internal/client"
)

// orderBuildPorcelainCommands returns porcelain build commands for common option
// strategies. These commands hardcode direction and contract type so the caller
// only specifies contract details, quantity, and price. The generic commands
// (option, vertical, straddle, strangle) remain available for power users and
// closing positions.
func orderBuildPorcelainCommands(w io.Writer) []*cobra.Command {
	cmds := singleLegBuildCommands(w)
	cmds = append(cmds, sellCoveredCallBuildCommand(w))
	cmds = append(cmds, porcelainVerticalBuildCommands(w)...)
	cmds = append(cmds, porcelainSymmetricBuildCommands(w)...)
	cmds = append(cmds, shortIronCondorBuildCommand(w))
	cmds = append(cmds, jadeLizardBuildCommand(w))

	return cmds
}

// orderPlacePorcelainCommands returns porcelain place commands that mirror the
// build porcelain commands but include authentication, account resolution, and
// API submission.
func orderPlacePorcelainCommands(c *client.Ref, configPath string, w io.Writer) []*cobra.Command {
	cmds := singleLegPlaceCommands(c, configPath, w)
	cmds = append(cmds, sellCoveredCallPlaceCommand(c, configPath, w))
	cmds = append(cmds, porcelainVerticalPlaceCommands(c, configPath, w)...)
	cmds = append(cmds, porcelainSymmetricPlaceCommands(c, configPath, w)...)
	cmds = append(cmds, shortIronCondorPlaceCommand(c, configPath, w))
	cmds = append(cmds, jadeLizardPlaceCommand(c, configPath, w))

	return cmds
}

// orderPreviewPorcelainCommands returns porcelain preview commands that mirror
// the build porcelain commands but call the preview API instead of placing orders.
func orderPreviewPorcelainCommands(c *client.Ref, configPath string, w io.Writer) []*cobra.Command {
	cmds := singleLegPreviewCommands(c, configPath, w)
	cmds = append(cmds, sellCoveredCallPreviewCommand(c, configPath, w))
	cmds = append(cmds, porcelainVerticalPreviewCommands(c, configPath, w)...)
	cmds = append(cmds, porcelainSymmetricPreviewCommands(c, configPath, w)...)
	cmds = append(cmds, shortIronCondorPreviewCommand(c, configPath, w))
	cmds = append(cmds, jadeLizardPreviewCommand(c, configPath, w))

	return cmds
}
