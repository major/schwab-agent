package apperr

import (
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

const (
	// FlagErrorInvalidValue marks pflag conversion failures, such as passing text
	// to an integer flag. It maps to the existing structured-error exit code 11.
	FlagErrorInvalidValue = "invalid_value"
	// FlagErrorUnknown marks flags that are not defined for the command Cobra is
	// parsing. It maps to the existing structured-error exit code 12.
	FlagErrorUnknown = "unknown"
)

var (
	pflagInvalidArgPattern  = regexp.MustCompile(`^invalid argument "([^"]*)" for "([^"]*)" flag`)
	pflagUnknownFlagPattern = regexp.MustCompile(`^unknown flag: --(.+)$`)
)

// FlagError is a typed wrapper around Cobra/pflag parse errors.
//
// Cobra intentionally returns plain string errors for parse failures. Wrapping
// those strings at the SetFlagErrorFunc boundary lets the output package use
// [errors.As] instead of re-parsing text after command execution has unwound.
type FlagError struct {
	Kind     string
	FlagName string
	Value    string
	Cause    error
}

// NewFlagError creates a typed flag parse error while preserving the original
// pflag error as the unwrap cause.
func NewFlagError(kind, flagName, value string, cause error) *FlagError {
	return &FlagError{
		Kind:     kind,
		FlagName: flagName,
		Value:    value,
		Cause:    cause,
	}
}

// Error returns the original pflag message so existing human-facing text and
// tests stay stable while callers can still inspect this typed error.
func (e *FlagError) Error() string {
	if e == nil || e.Cause == nil {
		return "flag parse error"
	}

	return e.Cause.Error()
}

// Unwrap returns the original pflag error.
func (e *FlagError) Unwrap() error {
	if e == nil {
		return nil
	}

	return e.Cause
}

// ExitCode returns the existing semantic process exit code for typed flag
// errors. Unknown flags are self-correctable input errors distinct from invalid
// values, so they retain the prior 12 code.
func (e *FlagError) ExitCode() int {
	if e != nil && e.Kind == FlagErrorUnknown {
		return 12
	}

	return 11
}

// NormalizeFlagError converts Cobra's pflag parse errors into typed errors.
// Use it with cobra.Command.SetFlagErrorFunc on commands that should emit the
// machine-readable flag error contract instead of a command-specific hint.
func NormalizeFlagError(_ *cobra.Command, err error) error {
	if err == nil {
		return nil
	}

	if flagErr, ok := ClassifyFlagError(err); ok {
		return flagErr
	}

	// SetFlagErrorFunc is only called for pflag parse failures. If pflag changes
	// its wording in a future release, keep the error typed so the output layer
	// still returns the input-error exit-code range instead of a generic error.
	return NewFlagError(FlagErrorInvalidValue, "", "", err)
}

// ClassifyFlagError parses Cobra/pflag's string-only parse errors into a typed
// flag error. It returns false for unrelated errors so callers outside
// SetFlagErrorFunc can safely try flag classification without stealing command,
// auth, or domain errors.
func ClassifyFlagError(err error) (*FlagError, bool) {
	if err == nil {
		return nil, false
	}

	message := err.Error()
	if matches := pflagInvalidArgPattern.FindStringSubmatch(message); matches != nil {
		return NewFlagError(FlagErrorInvalidValue, extractLongFlagName(matches[2]), matches[1], err), true
	}

	if matches := pflagUnknownFlagPattern.FindStringSubmatch(message); matches != nil {
		return NewFlagError(FlagErrorUnknown, matches[1], "", err), true
	}

	return nil, false
}

func extractLongFlagName(flagSpec string) string {
	for part := range strings.SplitSeq(flagSpec, ",") {
		candidate := strings.TrimSpace(part)
		if longName, ok := strings.CutPrefix(candidate, "--"); ok {
			return longName
		}
	}

	return strings.TrimLeft(strings.TrimSpace(flagSpec), "-")
}
