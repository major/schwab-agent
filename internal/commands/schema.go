// Package commands provides CLI command constructors for schwab-agent.
package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/urfave/cli/v3"
)

// SchemaOutput is the top-level schema introspection structure.
type SchemaOutput struct {
	Commands    map[string]CommandSchema `json:"commands"`
	GlobalFlags map[string]FlagSchema   `json:"global_flags"`
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

// SchemaCommand returns the CLI command for schema introspection.
// It walks the provided app's command tree and emits a JSON description
// of all commands, flags, and their types. Output is raw JSON, not wrapped
// in the standard success/error envelope.
func SchemaCommand(app *cli.Command, w io.Writer) *cli.Command {
	return &cli.Command{
		Name:  "schema",
		Usage: "Display JSON schema of all available commands",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "command",
				Usage: "Filter to a single command path (e.g., \"order place equity\")",
			},
		},
		Action: func(_ context.Context, cmd *cli.Command) error {
			commands := make(map[string]CommandSchema)
			walkCommands(app, "", commands)

			globalFlags := extractFlags(app.Flags)

			// Filter to a single command when --command is set.
			filter := cmd.String("command")
			if filter != "" {
				cs, ok := commands[filter]
				if !ok {
					return fmt.Errorf("command %q not found", filter)
				}
				commands = map[string]CommandSchema{filter: cs}
			}

			schema := SchemaOutput{
				Commands:    commands,
				GlobalFlags: globalFlags,
			}

			enc := json.NewEncoder(w)
			enc.SetIndent("", "  ")
			enc.SetEscapeHTML(false)
			return enc.Encode(schema)
		},
	}
}

// walkCommands recursively traverses the command tree and populates the
// commands map with space-separated command paths as keys.
func walkCommands(cmd *cli.Command, prefix string, commands map[string]CommandSchema) {
	for _, sub := range cmd.Commands {
		path := sub.Name
		if prefix != "" {
			path = prefix + " " + sub.Name
		}

		commands[path] = CommandSchema{
			Description: sub.Usage,
			Flags:       extractFlags(sub.Flags),
			Args:        map[string]any{},
			Examples:    []string{},
		}

		walkCommands(sub, path, commands)
	}
}

// extractFlags converts CLI flag definitions to schema flag descriptions.
func extractFlags(flags []cli.Flag) map[string]FlagSchema {
	result := make(map[string]FlagSchema)
	for _, f := range flags {
		name, schema := classifyFlag(f)
		if name != "" {
			result["--"+name] = schema
		}
	}
	return result
}

// classifyFlag determines the type and properties of a single CLI flag
// via type assertion on concrete urfave/cli v3 flag types.
func classifyFlag(f cli.Flag) (string, FlagSchema) {
	switch tf := f.(type) {
	case *cli.StringFlag:
		return tf.Name, FlagSchema{
			Type:        "string",
			Required:    tf.Required,
			Default:     tf.Value,
			Description: tf.Usage,
		}
	case *cli.IntFlag:
		return tf.Name, FlagSchema{
			Type:        "int",
			Required:    tf.Required,
			Default:     tf.Value,
			Description: tf.Usage,
		}
	case *cli.Float64Flag:
		return tf.Name, FlagSchema{
			Type:        "float",
			Required:    tf.Required,
			Default:     tf.Value,
			Description: tf.Usage,
		}
	case *cli.BoolFlag:
		return tf.Name, FlagSchema{
			Type:        "bool",
			Required:    tf.Required,
			Default:     tf.Value,
			Description: tf.Usage,
		}
	default:
		// Fall back to string for unknown flag types.
		names := f.Names()
		if len(names) == 0 {
			return "", FlagSchema{}
		}
		return names[0], FlagSchema{
			Type: "string",
		}
	}
}
