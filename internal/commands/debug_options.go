package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const debugOptionsFlagName = "debug-options"

// debugFlag describes one flag in --debug-options output. The structure stays
// intentionally small: agents mostly need to know the effective value and
// whether it came from argv or a default.
type debugFlag struct {
	Name    string `json:"name"`
	Value   string `json:"value"`
	Source  string `json:"source"`
	Changed bool   `json:"changed"`
}

// debugOptionsEnvelope is the JSON form for --debug-options=json.
type debugOptionsEnvelope struct {
	Command string      `json:"command"`
	Flags   []debugFlag `json:"flags"`
}

// InstallDebugOptions adds the Cobra-native --debug-options flag. It wraps
// runnable commands so debug mode can print flag attribution after Cobra has
// parsed argv but before any handler talks to the Schwab API.
func InstallDebugOptions(root *cobra.Command) {
	root.PersistentFlags().String(debugOptionsFlagName, "", "Print resolved flag values and exit (text or json)")
	flag := root.PersistentFlags().Lookup(debugOptionsFlagName)
	flag.NoOptDefVal = "text"
	flag.DefValue = ""

	wrapDebugOptions(root)
}

// IsDebugOptionsActive reports whether the current command was invoked in debug
// mode. Root auth hooks use this to skip config/token loading for pure
// introspection requests.
func IsDebugOptionsActive(cmd *cobra.Command) bool {
	flag := cmd.Flags().Lookup(debugOptionsFlagName)
	return flag != nil && flag.Changed
}

func wrapDebugOptions(cmd *cobra.Command) {
	originalRun := cmd.Run
	originalRunE := cmd.RunE

	if originalRun != nil || originalRunE != nil {
		cmd.Run = nil
		cmd.RunE = func(cmd *cobra.Command, args []string) error {
			if IsDebugOptionsActive(cmd) {
				return writeDebugOptions(cmd, cmd.OutOrStdout())
			}

			if originalRunE != nil {
				return originalRunE(cmd, args)
			}
			originalRun(cmd, args)
			return nil
		}
	}

	for _, child := range cmd.Commands() {
		wrapDebugOptions(child)
	}
}

func writeDebugOptions(cmd *cobra.Command, w io.Writer) error {
	format, err := cmd.Flags().GetString(debugOptionsFlagName)
	if err != nil {
		return err
	}
	if strings.EqualFold(format, "json") {
		encoder := json.NewEncoder(w)
		encoder.SetEscapeHTML(false)
		return encoder.Encode(debugOptionsEnvelope{Command: cmd.CommandPath(), Flags: debugFlags(cmd)})
	}

	_, err = fmt.Fprintf(w, "Command: %s\nFlags:\n", cmd.CommandPath())
	if err != nil {
		return err
	}
	for _, flag := range debugFlags(cmd) {
		if _, err := fmt.Fprintf(w, "  --%s=%s (%s)\n", flag.Name, flag.Value, flag.Source); err != nil {
			return err
		}
	}

	return nil
}

func debugFlags(cmd *cobra.Command) []debugFlag {
	flags := make([]debugFlag, 0)
	cmd.Flags().VisitAll(func(flag *pflag.Flag) {
		if flag.Hidden {
			return
		}
		source := "default"
		if flag.Changed {
			source = "argv"
		}
		flags = append(flags, debugFlag{
			Name:    flag.Name,
			Value:   flag.Value.String(),
			Source:  source,
			Changed: flag.Changed,
		})
	})

	sort.Slice(flags, func(i, j int) bool { return flags[i].Name < flags[j].Name })
	return flags
}
