package commands

import (
	"context"
	"fmt"
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
	for _, e := range valid {
		if candidate == e {
			return candidate, nil
		}
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
	for _, e := range valid {
		if candidate == e {
			return candidate, nil
		}
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
		names := make([]string, 0, len(cmd.Commands))
		for _, sub := range cmd.Commands {
			// Skip auto-registered help and any explicitly hidden subcommands
			// so users see only the real, invokable commands.
			if sub.Hidden || sub.Name == "help" {
				continue
			}
			names = append(names, sub.Name)
		}
		list := strings.Join(names, ", ")

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

// newValidationError creates a consistent validation error for command parsing.
func newValidationError(message string) error {
	return apperr.NewValidationError(message, nil)
}
