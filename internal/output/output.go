// Package output provides JSON envelope types and writers for schwab-agent responses.
package output

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	schwabErrors "github.com/major/schwab-agent/internal/errors"
)

// Envelope is the standard JSON response wrapper for successful operations.
type Envelope struct {
	Data     any            `json:"data"`
	Errors   []string       `json:"errors,omitempty"`
	Metadata map[string]any `json:"metadata"`
}

// ErrorEnvelope is the standard JSON response wrapper for error responses.
type ErrorEnvelope struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail contains error code, message, and optional details.
type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

// WriteSuccess writes a successful response with data and metadata to the writer.
// The response is formatted as a JSON envelope with data and metadata fields.
func WriteSuccess(w io.Writer, data any, metadata map[string]any) error {
	envelope := Envelope{
		Data:     data,
		Metadata: metadata,
	}
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(envelope)
}

// WriteError writes an error response to the writer.
// It maps the error type to an appropriate error code string using schwabErrors.ErrorCode().
// The response is formatted as a JSON envelope with error details.
func WriteError(w io.Writer, err error) error {
	code := schwabErrors.ErrorCode(err)
	message := err.Error()

	// Extract details from specific error types
	var details string
	var schwabErr *schwabErrors.SchwabError
	if errors.As(err, &schwabErr) {
		details = schwabErr.Details
	}

	// Include the upstream response body when available so API errors from
	// Schwab are visible in the CLI output instead of just "status: NNN".
	var httpErr *schwabErrors.HTTPError
	if errors.As(err, &httpErr) {
		if httpErr.Body != "" {
			details = fmt.Sprintf("status: %d, body: %s", httpErr.StatusCode, httpErr.Body)
		} else {
			details = fmt.Sprintf("status: %d", httpErr.StatusCode)
		}
	}

	errorEnvelope := ErrorEnvelope{
		Error: ErrorDetail{
			Code:    code,
			Message: message,
			Details: details,
		},
	}
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(errorEnvelope)
}

// WritePartial writes a response with both data and errors (partial success).
// The response is formatted as a JSON envelope with data, errors, and metadata fields.
func WritePartial(w io.Writer, data any, errs []string, metadata map[string]any) error {
	envelope := Envelope{
		Data:     data,
		Errors:   errs,
		Metadata: metadata,
	}
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(envelope)
}

// TimestampMeta returns metadata map with current UTC timestamp in RFC3339 format.
func TimestampMeta() map[string]any {
	return map[string]any{
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}
}
