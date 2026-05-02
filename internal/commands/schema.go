// Package commands provides CLI command constructors for schwab-agent.
package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/major/schwab-agent/internal/apperr"
)

// SchemaOutput is the top-level schema introspection structure.
type SchemaOutput struct {
	Commands    map[string]CommandSchema `json:"commands"`
	GlobalFlags map[string]FlagSchema    `json:"global_flags"`
}

// CommandSchema describes a single CLI command in the schema.
type CommandSchema struct {
	Description string                `json:"description"`
	Flags       map[string]FlagSchema `json:"flags"`
	Args        map[string]any        `json:"args"`
	Examples    []string              `json:"examples"`
}

// FlagSchema describes a single CLI flag in the schema.
type FlagSchema struct {
	Type        string `json:"type"`
	Required    bool   `json:"required"`
	Default     any    `json:"default"`
	Description string `json:"description"`
}

// schemaOpts holds the options for the schema command.
type schemaOpts struct {
	Command string
}

// NewSchemaCmd returns the Cobra command for schema introspection.
// It walks the provided root command tree and emits raw JSON, not the standard
// success/error envelope, so agent tooling can consume the command contract directly.
func NewSchemaCmd(root *cobra.Command, w io.Writer) *cobra.Command {
	opts := &schemaOpts{}
	cmd := &cobra.Command{
		Use:         "schema",
		Short:       "Display JSON schema of all available commands",
		Annotations: map[string]string{"skipAuth": "true"},
		GroupID:     "tools",
		Example: `  schwab-agent schema
  schwab-agent schema --command "order place equity"`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			allCommands := make(map[string]CommandSchema)
			walkCommands(root, "", allCommands)
			// Match the legacy schema contract: report the application command tree,
			// not the schema command that is currently producing the introspection output.
			delete(allCommands, cmd.Name())
			globalFlags := extractFlags(root.PersistentFlags())

			if opts.Command != "" {
				cs, ok := allCommands[opts.Command]
				if !ok {
					return apperr.NewValidationError(fmt.Sprintf("command %q not found", opts.Command), nil)
				}
				allCommands = map[string]CommandSchema{opts.Command: cs}
			}

			schema := SchemaOutput{
				Commands:    allCommands,
				GlobalFlags: globalFlags,
			}
			enc := json.NewEncoder(w)
			enc.SetIndent("", "  ")
			enc.SetEscapeHTML(false)
			return enc.Encode(schema)
		},
	}
	cmd.Flags().StringVar(&opts.Command, "command", "", `Filter to a single command path (e.g., "order place equity")`)
	return cmd
}

// walkCommands recursively traverses the command tree and populates the commands
// map with space-separated command paths as keys.
func walkCommands(cmd *cobra.Command, prefix string, commands map[string]CommandSchema) {
	for _, sub := range cmd.Commands() {
		if sub.Hidden || sub.Name() == "help" {
			continue
		}

		// Cobra's Use string may contain positional args, so keep only the command name.
		fields := strings.Fields(sub.Use)
		if len(fields) == 0 {
			continue
		}
		name := fields[0]
		path := name
		if prefix != "" {
			path = prefix + " " + name
		}
		commands[path] = CommandSchema{
			Description: sub.Short,
			Flags:       extractFlags(sub.LocalFlags()),
			Args:        map[string]any{},
			Examples:    parseExamples(sub.Example),
		}
		walkCommands(sub, path, commands)
	}
}

// extractFlags converts pflag definitions to schema flag descriptions.
func extractFlags(flags *pflag.FlagSet) map[string]FlagSchema {
	result := make(map[string]FlagSchema)
	flags.VisitAll(func(f *pflag.Flag) {
		if f.Hidden {
			return
		}
		result["--"+f.Name] = classifyFlag(f)
	})
	return result
}

// classifyFlag determines the type, default value, and description for a pflag.
func classifyFlag(f *pflag.Flag) FlagSchema {
	typeName := f.Value.Type()
	schema := FlagSchema{
		Type:        typeName,
		Description: f.Usage,
	}

	// pflag stores defaults as strings. Parse common scalar types back to native
	// values so schema JSON preserves bool and number defaults for consumers.
	switch typeName {
	case "int", "int64", "int32":
		if v, err := strconv.ParseInt(f.DefValue, 10, 64); err == nil {
			schema.Default = v
		} else {
			schema.Default = f.DefValue
		}
	case "float64", "float32":
		if v, err := strconv.ParseFloat(f.DefValue, 64); err == nil {
			schema.Default = v
		} else {
			schema.Default = f.DefValue
		}
	case "bool":
		if v, err := strconv.ParseBool(f.DefValue); err == nil {
			schema.Default = v
		} else {
			schema.Default = f.DefValue
		}
	default:
		schema.Default = f.DefValue
	}
	return schema
}

// parseExamples splits a UsageText string into individual example lines,
// trimming whitespace and dropping blanks. Returns an empty slice (not nil)
// when there are no examples so JSON output stays consistent.
func parseExamples(usageText string) []string {
	if strings.TrimSpace(usageText) == "" {
		return []string{}
	}

	lines := strings.Split(usageText, "\n")
	examples := make([]string, 0, len(lines))

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			examples = append(examples, trimmed)
		}
	}

	if len(examples) == 0 {
		return []string{}
	}

	return examples
}
