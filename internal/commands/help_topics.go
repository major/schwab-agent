package commands

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// NewHelpTopicCmds returns Cobra-native help topic commands (env-vars, config-keys).
// They are real commands so the normal Cobra auth-skip annotation path can handle them.
func NewHelpTopicCmds(w io.Writer) []*cobra.Command {
	return []*cobra.Command{
		newEnvVarsCmd(w),
		newConfigKeysCmd(w),
	}
}

// newEnvVarsCmd returns a static reference for environment variables consumed
// outside Cobra flag binding. This includes auth, config, and state settings.
func newEnvVarsCmd(w io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:         "env-vars",
		Short:       "Show supported environment variables",
		Long:        "Show environment variables that schwab-agent reads for authentication, API configuration, and state.",
		Annotations: map[string]string{annotationSkipAuth: annotationValueTrue},
		GroupID:     groupIDTools,
		Args:        cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			_, err := fmt.Fprint(w, `Environment Variables

  Authentication and API configuration:
    SCHWAB_CLIENT_ID            OAuth client ID; overrides config.json client_id.
    SCHWAB_CLIENT_SECRET        OAuth client secret; overrides config.json client_secret.
    SCHWAB_CALLBACK_URL         OAuth callback URL; overrides config.json callback_url.
    SCHWAB_BASE_URL             Schwab-compatible API base URL; overrides config.json base_url.
    SCHWAB_BASE_URL_INSECURE    Set true to skip TLS verification for local proxy development.

  Local state and diagnostics:
    SCHWAB_AGENT_STATE_DIR      Directory for preview digest ledger files.
`)
			return err
		},
	}
}

// newConfigKeysCmd prints config keys from the live Cobra command tree and their flags.
func newConfigKeysCmd(w io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:         "config-keys",
		Short:       "Show config keys and related flags",
		Long:        "Show config keys derived from the current Cobra command tree and their corresponding flags.",
		Annotations: map[string]string{annotationSkipAuth: annotationValueTrue},
		GroupID:     groupIDTools,
		Args:        cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return writeConfigKeys(w, cmd.Root())
		},
	}
}

func writeConfigKeys(w io.Writer, root *cobra.Command) error {
	if _, err := fmt.Fprintln(w, "Configuration Keys"); err != nil {
		return err
	}

	if err := writeFlagSection(w, root.CommandPath()+" (global)", root.PersistentFlags()); err != nil {
		return err
	}

	for _, cmd := range allRunnableCommands(root) {
		if err := writeFlagSection(w, cmd.CommandPath(), cmd.NonInheritedFlags()); err != nil {
			return err
		}
	}

	_, err := fmt.Fprintln(w, "\n  Keys can be nested under the command name in the config file.")
	return err
}

func allRunnableCommands(root *cobra.Command) []*cobra.Command {
	var commands []*cobra.Command
	for _, child := range root.Commands() {
		walkCommands(child, &commands)
	}

	return commands
}

func walkCommands(cmd *cobra.Command, commands *[]*cobra.Command) {
	if !cmd.Hidden && cmd.HasAvailableFlags() {
		*commands = append(*commands, cmd)
	}

	for _, child := range cmd.Commands() {
		walkCommands(child, commands)
	}
}

func writeFlagSection(w io.Writer, title string, flags *pflag.FlagSet) error {
	rows := flagRows(flags)
	if len(rows) == 0 {
		return nil
	}

	if _, err := fmt.Fprintf(w, "\n  %s:\n", title); err != nil {
		return err
	}
	for _, row := range rows {
		if _, err := fmt.Fprintf(
			w,
			"    %-22s  --%-22s  %-12s  %s\n",
			row.key,
			row.flag,
			row.valueType,
			row.defaultValue,
		); err != nil {
			return err
		}
	}

	return nil
}

type flagRow struct {
	key          string
	flag         string
	valueType    string
	defaultValue string
}

func flagRows(flags *pflag.FlagSet) []flagRow {
	var rows []flagRow
	flags.VisitAll(func(flag *pflag.Flag) {
		if flag.Hidden || flag.Name == "help" {
			return
		}

		rows = append(rows, flagRow{
			key:          strings.ReplaceAll(flag.Name, "-", ""),
			flag:         flag.Name,
			valueType:    flag.Value.Type(),
			defaultValue: flag.DefValue,
		})
	})

	return rows
}
