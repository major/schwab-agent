// Package output provides JSON envelope types and writers for schwab-agent responses.
package output

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/major/schwab-agent/internal/apperr"
)

// StructuredError is the top-level machine-readable error shape used by the
// CLI. All error paths produce this shape so agents can parse one stable
// JSON contract for domain, flag, and generic errors.
type StructuredError struct {
	Error      string      `json:"error"`
	ExitCode   int         `json:"exit_code"`
	Message    string      `json:"message"`
	Flag       string      `json:"flag,omitempty"`
	Got        string      `json:"got,omitempty"`
	Expected   string      `json:"expected,omitempty"`
	Command    string      `json:"command,omitempty"`
	Hint       string      `json:"hint,omitempty"`
	Available  []string    `json:"available,omitempty"`
	Violations []Violation `json:"violations,omitempty"`
	ConfigFile string      `json:"config_file,omitempty"`
	Key        string      `json:"key,omitempty"`
	EnvVar     string      `json:"env_var,omitempty"`
}

// Violation describes a single input validation failure.
type Violation struct {
	Subject string `json:"subject,omitempty"`
	Message string `json:"message,omitempty"`
}

// Metadata holds the standard metadata fields for response envelopes.
type Metadata struct {
	Timestamp           string `json:"timestamp"`
	Account             string `json:"account,omitempty"`
	Requested           int    `json:"requested,omitempty"`
	Returned            int    `json:"returned,omitempty"`
	PositionsIncluded   bool   `json:"positionsIncluded,omitempty"`
	AccountNickName     string `json:"accountNickName,omitempty"`
	AccountType         string `json:"accountType,omitempty"`
	AccountSource       string `json:"accountSource,omitempty"`
	AccountDisplayLabel string `json:"accountDisplayLabel,omitempty"`
}

// NewMetadata returns metadata pre-populated with the current UTC timestamp.
func NewMetadata() Metadata {
	return Metadata{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
}

// Envelope is the standard JSON response wrapper for successful operations.
type Envelope struct {
	Data     any      `json:"data"`
	Errors   []string `json:"errors,omitempty"`
	Metadata Metadata `json:"metadata"`
}

// ErrorEnvelope is kept as a compatibility alias for tests and callers that
// decode errors through this package type.
type ErrorEnvelope = StructuredError

// WriteSuccess writes a successful response with data and metadata to the writer.
// The response is formatted as a JSON envelope with data and metadata fields.
func WriteSuccess(w io.Writer, data any, metadata Metadata) error {
	envelope := Envelope{
		Data:     data,
		Metadata: metadata,
	}
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(envelope)
}

// WriteError writes an error response to the writer.
// The response uses the top-level StructuredError contract so agents can parse
// one stable schema for both Schwab-domain and flag/config errors.
func WriteError(w io.Writer, err error) error {
	_, writeErr := WriteCommandError(w, nil, err)
	return writeErr
}

// WriteCommandError writes an error response and returns the process exit code
// that should accompany it. Schwab-domain errors are mapped locally so their
// existing 1-5 exit-code contract stays intact. Cobra flag errors are mapped
// locally. Non-domain, non-flag errors produce a generic exit-code-1 envelope.
func WriteCommandError(w io.Writer, cmd *cobra.Command, err error) (int, error) {
	if err == nil {
		return 0, nil
	}

	if isSchwabDomainError(err) {
		se := schwabStructuredError(cmd, err)
		return se.ExitCode, writeStructuredError(w, &se)
	}

	if flagErr := flagErrorFrom(err); flagErr != nil {
		se := flagStructuredError(cmd, flagErr)
		return se.ExitCode, writeStructuredError(w, &se)
	}

	se := StructuredError{
		Error:    "error",
		ExitCode: 1,
		Message:  err.Error(),
		Command:  commandPath(cmd),
	}
	return se.ExitCode, writeStructuredError(w, &se)
}

// isSchwabDomainError reports whether err belongs to schwab-agent's typed
// domain hierarchy.
func isSchwabDomainError(err error) bool {
	if _, ok := errors.AsType[*apperr.AuthRequiredError](err); ok {
		return true
	}
	if _, ok := errors.AsType[*apperr.AuthExpiredError](err); ok {
		return true
	}
	if _, ok := errors.AsType[*apperr.AuthCallbackError](err); ok {
		return true
	}
	if _, ok := errors.AsType[*apperr.OrderRejectedError](err); ok {
		return true
	}
	if _, ok := errors.AsType[*apperr.SymbolNotFoundError](err); ok {
		return true
	}
	if _, ok := errors.AsType[*apperr.AccountNotFoundError](err); ok {
		return true
	}
	if _, ok := errors.AsType[*apperr.HTTPError](err); ok {
		return true
	}
	if _, ok := errors.AsType[*apperr.ValidationError](err); ok {
		return true
	}
	if _, ok := errors.AsType[*apperr.OrderBuildError](err); ok {
		return true
	}
	if _, ok := errors.AsType[*apperr.SchwabError](err); ok {
		return true
	}

	return false
}

// schwabStructuredError maps project domain errors onto the stable JSON shape.
// The old Details field becomes Hint because it is remediation text for agents
// and humans, not another machine-readable error code.
func schwabStructuredError(cmd *cobra.Command, err error) StructuredError {
	se := StructuredError{
		Error:    strings.ToLower(apperr.ErrorCode(err)),
		ExitCode: apperr.ExitCodeFor(err),
		Message:  err.Error(),
		Command:  commandPath(cmd),
	}

	se.Hint = schwabErrorDetails(err)

	// Preserve upstream HTTP context that used to live in the legacy details
	// string. `got` carries the observed response, while `expected` tells an
	// agent what outcome would have avoided this domain error.
	if httpErr, ok := errors.AsType[*apperr.HTTPError](err); ok {
		if httpErr.Body != "" {
			se.Got = fmt.Sprintf("status: %d, body: %s", httpErr.StatusCode, httpErr.Body)
		} else {
			se.Got = fmt.Sprintf("status: %d", httpErr.StatusCode)
		}
		se.Expected = "2xx response"
	}

	return se
}

func flagErrorFrom(err error) *apperr.FlagError {
	if flagErr, ok := errors.AsType[*apperr.FlagError](err); ok {
		return flagErr
	}

	// Some tests and command paths can still hand us Cobra's raw pflag error if a
	// command-specific SetFlagErrorFunc was not installed. Parse only recognized
	// pflag messages here; unlike NormalizeFlagError, this must not wrap arbitrary
	// command or config errors.
	if classified, ok := apperr.ClassifyFlagError(err); ok {
		return classified
	}

	return nil
}

func flagStructuredError(cmd *cobra.Command, err *apperr.FlagError) StructuredError {
	se := StructuredError{
		Error:    "invalid_flag_value",
		ExitCode: err.ExitCode(),
		Message:  err.Error(),
		Flag:     err.FlagName,
		Command:  commandPath(cmd),
	}
	if err.Kind == apperr.FlagErrorUnknown {
		se.Error = "unknown_flag"
	}
	if err.Value != "" {
		se.Got = err.Value
	}

	return se
}

// schwabErrorDetails extracts remediation details from every concrete domain
// error. The concrete checks are intentional because embedding SchwabError does
// not make errors.As match the embedded base type for wrapper errors.
func schwabErrorDetails(err error) string {
	if typedErr, ok := errors.AsType[*apperr.AuthRequiredError](err); ok {
		return typedErr.Details()
	}
	if typedErr, ok := errors.AsType[*apperr.AuthExpiredError](err); ok {
		return typedErr.Details()
	}
	if typedErr, ok := errors.AsType[*apperr.AuthCallbackError](err); ok {
		return typedErr.Details()
	}
	if typedErr, ok := errors.AsType[*apperr.OrderRejectedError](err); ok {
		return typedErr.Details()
	}
	if typedErr, ok := errors.AsType[*apperr.SymbolNotFoundError](err); ok {
		return typedErr.Details()
	}
	if typedErr, ok := errors.AsType[*apperr.AccountNotFoundError](err); ok {
		return typedErr.Details()
	}
	if typedErr, ok := errors.AsType[*apperr.HTTPError](err); ok {
		return typedErr.Details()
	}
	if typedErr, ok := errors.AsType[*apperr.ValidationError](err); ok {
		return typedErr.Details()
	}
	if typedErr, ok := errors.AsType[*apperr.OrderBuildError](err); ok {
		return typedErr.Details()
	}
	if typedErr, ok := errors.AsType[*apperr.SchwabError](err); ok {
		return typedErr.Details()
	}

	return ""
}

// writeStructuredError writes a top-level StructuredError.
func writeStructuredError(w io.Writer, se *StructuredError) error {
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(se)
}

// commandPath returns the user-facing command path when a command is available.
func commandPath(cmd *cobra.Command) string {
	if cmd == nil {
		return ""
	}

	return cmd.CommandPath()
}

// WritePartial writes a response with both data and errors (partial success).
// The response is formatted as a JSON envelope with data, errors, and metadata fields.
func WritePartial(w io.Writer, data any, errs []string, metadata Metadata) error {
	envelope := Envelope{
		Data:     data,
		Errors:   errs,
		Metadata: metadata,
	}
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(envelope)
}
