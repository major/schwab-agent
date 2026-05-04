package commands

import (
	"fmt"
	"slices"
	"strings"

	"github.com/leodido/structcli"
	"github.com/spf13/cobra"

	"github.com/major/schwab-agent/internal/apperr"
)

const structcliFlagEnumAnnotation = "leodido/structcli/flag-enum"

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

// defineAndConstrain centralizes structcli flag registration plus Cobra flag
// relationships for order commands. Each pair represents a set of flags that
// must be mutually exclusive and where one member must be provided.
func defineAndConstrain[O structcli.Options](cmd *cobra.Command, opts O, exclusivePairs ...[]string) {
	if err := structcli.Define(cmd, opts); err != nil {
		panic(err)
	}
	for _, pair := range exclusivePairs {
		cmd.MarkFlagsMutuallyExclusive(pair...)
		cmd.MarkFlagsOneRequired(pair...)
	}
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

// defaultSubcommand returns a RunE for parent commands that have a preferred
// positional-only shorthand. Cobra has already failed to resolve args[0] as a
// subcommand when this parent RunE is called, so a non-subcommand first arg can
// be handed directly to the default child command's RunE.
func defaultSubcommand(sub *cobra.Command) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 || slices.Contains(visibleSubcommandNames(cmd), args[0]) {
			return requireSubcommand(cmd, args)
		}

		// Do not call Execute here: that would re-run the root PersistentPreRunE,
		// causing duplicate auth/client setup. The shorthand intentionally forwards
		// positional args only; flags still require the explicit subcommand form.
		return sub.RunE(sub, args)
	}
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
	return normalizeFlagValidationErrorWithCommand(nil, err)
}

func normalizeFlagValidationErrorWithCommand(cmd *cobra.Command, err error) error {
	message := err.Error()
	if !strings.Contains(message, "invalid value") || !strings.Contains(message, " for \"--") {
		return err
	}

	invalidValue := ""
	if before, after, ok := strings.Cut(message, "invalid argument \""); ok {
		_ = before
		invalidValue, _, _ = strings.Cut(after, "\"")
	}

	_, after, ok := strings.Cut(message, " for \"--")
	if !ok {
		return err
	}
	flagName, _, ok := strings.Cut(after, "\" flag")
	if !ok || flagName == "" {
		return err
	}

	validValues := enumValuesForFlag(cmd, flagName)
	if len(validValues) == 0 {
		validValues = enumValuesFromMessage(message)
	}
	if invalidValue == "" || len(validValues) == 0 {
		return err
	}

	return newValidationError(fmt.Sprintf("invalid %s: %q (valid: %s)", flagName, invalidValue, validEnumString(validValues)))
}

func normalizeFlagValidationErrorFunc(cmd *cobra.Command, err error) error {
	if cmd == nil {
		return normalizeFlagValidationError(err)
	}

	return normalizeFlagValidationErrorWithCommand(cmd, err)
}

func enumValuesForFlag(cmd *cobra.Command, flagName string) []string {
	if cmd == nil {
		return nil
	}

	flag := cmd.Flags().Lookup(flagName)
	if flag == nil {
		flag = cmd.InheritedFlags().Lookup(flagName)
	}
	if flag == nil || flag.Annotations == nil {
		return nil
	}

	return cleanEnumValues(flag.Annotations[structcliFlagEnumAnnotation])
}

func enumValuesFromMessage(message string) []string {
	_, after, ok := strings.Cut(message, "(allowed: ")
	if !ok {
		return nil
	}
	values, _, ok := strings.Cut(after, ")")
	if !ok {
		return nil
	}

	return cleanEnumValues(strings.Split(values, ","))
}

func cleanEnumValues(values []string) []string {
	clean := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		// structcli registrations for optional enum flags include an empty value so
		// the flag can be omitted. That zero value is implementation detail, not
		// useful remediation text for someone who typed a bad explicit value.
		if value == "" {
			continue
		}
		clean = append(clean, value)
	}

	return clean
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
