// Package output provides JSON envelope types and writers for schwab-agent responses.
package output

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/leodido/structcli"
	"github.com/spf13/cobra"

	"github.com/major/schwab-agent/internal/apperr"
)

// Metadata holds the standard metadata fields for response envelopes.
type Metadata struct {
	Timestamp         string `json:"timestamp"`
	Account           string `json:"account,omitempty"`
	Requested         int    `json:"requested,omitempty"`
	Returned          int    `json:"returned,omitempty"`
	PositionsIncluded bool   `json:"positionsIncluded,omitempty"`
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
// decode errors through this package type. Error responses now use structcli's
// top-level StructuredError contract instead of the legacy nested envelope.
type ErrorEnvelope = structcli.StructuredError

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
// The response uses structcli's top-level StructuredError contract so agents
// can parse one stable schema for both Schwab-domain and flag/config errors.
func WriteError(w io.Writer, err error) error {
	_, writeErr := WriteCommandError(w, nil, err)
	return writeErr
}

// WriteCommandError writes an error response and returns the process exit code
// that should accompany it. Schwab-domain errors are mapped locally so their
// existing 1-5 exit-code contract stays intact; Cobra and structcli errors are
// delegated to structcli.HandleError so flag metadata, command paths, enum
// values, env var hints, and available commands stay as rich as structcli can
// make them.
func WriteCommandError(w io.Writer, cmd *cobra.Command, err error) (int, error) {
	if err == nil {
		return 0, nil
	}

	if isSchwabDomainError(err) {
		se := schwabStructuredError(cmd, err)
		return se.ExitCode, writeStructuredError(w, &se)
	}

	if cmd != nil {
		return writeStructCLIError(w, cmd, err)
	}

	se := structcli.StructuredError{
		Error:    "error",
		ExitCode: 1,
		Message:  err.Error(),
	}
	return se.ExitCode, writeStructuredError(w, &se)
}

// isSchwabDomainError reports whether err belongs to schwab-agent's typed
// domain hierarchy. structcli has its own ValidationError type, so checking the
// shared SchwabError base avoids accidentally stealing structcli validation
// errors away from structcli.HandleError.
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

// schwabStructuredError maps project domain errors onto structcli's stable JSON
// shape. The old Details field becomes Hint because it is remediation text for
// agents and humans, not another machine-readable error code.
func schwabStructuredError(cmd *cobra.Command, err error) structcli.StructuredError {
	se := structcli.StructuredError{
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

// writeStructCLIError asks structcli to classify the error, then re-encodes the
// result with this package's JSON encoder settings. This avoids copying
// structcli's private classifier while still keeping SetEscapeHTML(false)
// consistent across every JSON writer in schwab-agent.
func writeStructCLIError(w io.Writer, cmd *cobra.Command, err error) (int, error) {
	var buf bytes.Buffer
	exitCode := structcli.HandleError(cmd, err, &buf)

	var se structcli.StructuredError
	if unmarshalErr := json.Unmarshal(buf.Bytes(), &se); unmarshalErr != nil {
		se = structcli.StructuredError{
			Error:    "error",
			ExitCode: exitCode,
			Message:  err.Error(),
			Command:  commandPath(cmd),
		}
	}

	return exitCode, writeStructuredError(w, &se)
}

// writeStructuredError writes a top-level structcli StructuredError.
func writeStructuredError(w io.Writer, se *structcli.StructuredError) error {
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
