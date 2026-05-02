package commands

import (
	"fmt"
	"slices"
	"strings"

	"github.com/spf13/cobra"

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

// requireTypedEnum validates that a typed enum flag with no useful zero value
// was provided. Structcli validates explicit values, but omitted flags still
// arrive as the enum type's zero value.
func requireTypedEnum[T ~string](value T, label string) error {
	if strings.TrimSpace(string(value)) == "" {
		return newValidationError(label + " is required")
	}

	return nil
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

// newValidationError creates a consistent validation error for command parsing.
func newValidationError(message string) error {
	return apperr.NewValidationError(message, nil)
}

// requireSubcommand returns a cobra.RunE that rejects direct invocation of
// a parent command with a clear error. Without this, cobra produces a confusing
// "no subcommands available" message when the user omits the subcommand
// (e.g. "quote AAPL" instead of "quote get AAPL").
func requireSubcommand(cmd *cobra.Command, args []string) error {
	list := strings.Join(visibleSubcommandNames(cmd), ", ")

	if len(args) > 0 {
		return apperr.NewValidationError(
			fmt.Sprintf("unknown command %q for %q; available subcommands: %s", args[0], cmd.Name(), list),
			nil,
		)
	}

	return apperr.NewValidationError(
		fmt.Sprintf("%q requires a subcommand: %s", cmd.Name(), list),
		nil,
	)
}

// suggestSubcommands handles usage errors that happen before RunE runs. This
// is most useful for parent commands whose subcommands own distinct flags: an
// unknown flag on the parent usually means the user skipped the subcommand.
func suggestSubcommands(cmd *cobra.Command, err error) error {
	if !strings.Contains(err.Error(), "unknown flag") && !strings.Contains(err.Error(), "flag provided but not defined") {
		return err
	}

	names := visibleSubcommandNames(cmd)
	if len(names) == 0 {
		return err
	}

	return apperr.NewValidationError(
		fmt.Sprintf(
			"%q requires a subcommand: %s (%s)",
			cmd.CommandPath(),
			strings.Join(names, ", "),
			err.Error(),
		),
		err,
	)
}

// normalizeFlagValidationError keeps enum flag errors consistent with the older
// command-level validation messages while still letting structcli/pflag reject
// invalid enum values before RunE starts.
func normalizeFlagValidationError(err error) error {
	message := err.Error()
	if !strings.Contains(message, "invalid value") || !strings.Contains(message, " for \"--") {
		return err
	}

	_, after, ok := strings.Cut(message, " for \"--")
	if !ok {
		return err
	}
	flagName, _, ok := strings.Cut(after, "\" flag")
	if !ok || flagName == "" {
		return err
	}

	return newValidationError("invalid " + flagName)
}

func normalizeFlagValidationErrorFunc(_ *cobra.Command, err error) error {
	return normalizeFlagValidationError(err)
}

// visibleSubcommandNames returns only invokable subcommands for user-facing
// errors, omitting hidden commands.
func visibleSubcommandNames(cmd *cobra.Command) []string {
	names := make([]string, 0, len(cmd.Commands()))
	for _, sub := range cmd.Commands() {
		if sub.Hidden {
			continue
		}
		names = append(names, sub.Name())
	}

	return names
}
