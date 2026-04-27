package commands

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/urfave/cli/v3"

	"github.com/major/schwab-agent/internal/apperr"
)

// requireArg returns a ValidationError if value is empty.
// Use this for required positional arguments and flag values.
func requireArg(value, name string) error {
	if value == "" {
		return apperr.NewValidationError(name+" is required", nil)
	}

	return nil
}

// parseEnum validates raw against valid, returning the matching enum member or
// fallback when raw is empty. Trims whitespace and uppercases before comparison.
func parseEnum[T ~string](raw string, valid []T, fallback T, label string) (T, error) {
	v := strings.TrimSpace(raw)
	if v == "" {
		return fallback, nil
	}

	candidate := T(strings.ToUpper(v))
	if slices.Contains(valid, candidate) {
		return candidate, nil
	}

	var zero T

	return zero, newValidationError(
		fmt.Sprintf("invalid %s: %q (valid: %s)", label, raw, validEnumString(valid)),
	)
}

// requireEnum validates raw against valid, returning a ValidationError when raw
// is empty. Use this for required enum flags that have no fallback.
func requireEnum[T ~string](raw string, valid []T, label string) (T, error) {
	v := strings.TrimSpace(raw)
	if v == "" {
		var zero T

		return zero, newValidationError(label + " is required")
	}

	candidate := T(strings.ToUpper(v))
	if slices.Contains(valid, candidate) {
		return candidate, nil
	}

	var zero T

	return zero, newValidationError(
		fmt.Sprintf("invalid %s: %q (valid: %s)", label, raw, validEnumString(valid)),
	)
}

// validEnumString joins valid enum values into a comma-separated string for
// error messages. Example: "BUY, SELL, BUY_TO_COVER".
func validEnumString[T ~string](valid []T) string {
	s := make([]string, len(valid))
	for i, v := range valid {
		s[i] = string(v)
	}

	return strings.Join(s, ", ")
}

// requireSubcommand returns a cli.ActionFunc that rejects direct invocation of
// a parent command with a clear error. Without this, urfave/cli produces a
// confusing "No help topic for '<arg>'" message when the user omits the
// subcommand (e.g. "quote AAPL" instead of "quote get AAPL").
func requireSubcommand() cli.ActionFunc {
	return func(_ context.Context, cmd *cli.Command) error {
		list := strings.Join(visibleSubcommandNames(cmd), ", ")

		if arg := cmd.Args().First(); arg != "" {
			return apperr.NewValidationError(
				fmt.Sprintf("unknown command %q for %q; available subcommands: %s", arg, cmd.Name, list),
				nil,
			)
		}

		return apperr.NewValidationError(
			fmt.Sprintf("%q requires a subcommand: %s", cmd.Name, list),
			nil,
		)
	}
}

// suggestSubcommands handles usage errors that happen before Action runs. This
// is most useful for parent commands whose subcommands own distinct flags: an
// unknown flag on the parent usually means the user skipped the subcommand.
func suggestSubcommands(_ context.Context, cmd *cli.Command, err error, _ bool) error {
	if !strings.Contains(err.Error(), "flag provided but not defined") {
		return err
	}

	names := visibleSubcommandNames(cmd)
	if len(names) == 0 {
		return err
	}

	return apperr.NewValidationError(
		fmt.Sprintf(
			"%q requires a subcommand: %s (%s)",
			cmd.FullName(),
			strings.Join(names, ", "),
			err.Error(),
		),
		err,
	)
}

// visibleSubcommandNames returns only invokable subcommands for user-facing
// errors, omitting hidden commands and urfave/cli's built-in help command.
func visibleSubcommandNames(cmd *cli.Command) []string {
	names := make([]string, 0, len(cmd.Commands))
	for _, sub := range cmd.Commands {
		if sub.Hidden || sub.Name == "help" {
			continue
		}
		names = append(names, sub.Name)
	}

	return names
}

// newValidationError creates a consistent validation error for command parsing.
func newValidationError(message string) error {
	return apperr.NewValidationError(message, nil)
}
